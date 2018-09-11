package apmot

import (
	"context"
	"io"
	"net/http"
	"net/textproto"
	"time"

	"github.com/opentracing/opentracing-go"

	"github.com/elastic/apm-agent-go"
	"github.com/elastic/apm-agent-go/module/apmhttp"
)

// New returns a new opentracing.Tracer backed by the supplied
// Elastic APM tracer.
//
// By default, the returned tracer will use elasticapm.DefaultTracer.
// This can be overridden by using a WithTracer option.
func New(opts ...Option) opentracing.Tracer {
	t := &otTracer{tracer: elasticapm.DefaultTracer}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

// otTracer is an opentracing.Tracer backed by an elasticapm.Tracer.
type otTracer struct {
	tracer *elasticapm.Tracer
}

func (t *otTracer) StartSpanFromContext(
	ctx context.Context,
	operationName string,
	opts ...opentracing.StartSpanOption,
) (opentracing.Span, context.Context) {
	apmParentSpan := elasticapm.SpanFromContext(ctx)
	parentSpan := opentracing.SpanFromContext(ctx)
	if parentSpan != nil && parentSpan.(*otSpan).span == apmParentSpan { // XXX panic
		opts = append(opts, opentracing.ChildOf(parentSpan.Context()))
	} else if tx := elasticapm.TransactionFromContext(ctx); tx != nil {
		txTraceContext := tx.TraceContext()
		apmParentSpan := elasticapm.SpanFromContext(ctx)
		parentContext := spanContext{
			tracer:        t,
			transactionID: txTraceContext.Span,
			tx:            tx,
		}
		parentContext.txSpanContext = &parentContext
		if apmParentSpan != nil {
			parentContext.traceContext = apmParentSpan.TraceContext()
		} else {
			parentContext.traceContext = txTraceContext
		}
		opts = append(opts, opentracing.ChildOf(&parentContext))
	}
	span := t.StartSpan(operationName, opts...)
	ctx = opentracing.ContextWithSpan(ctx, span)
	otSpan := span.(*otSpan)
	if otSpan.span != nil {
		ctx = elasticapm.ContextWithSpan(ctx, otSpan.span)
	} else {
		ctx = elasticapm.ContextWithTransaction(ctx, otSpan.ctx.tx)
	}
	return span, ctx
}

// StartSpan starts a new OpenTracing span with the given name and zero or more options.
func (t *otTracer) StartSpan(name string, opts ...opentracing.StartSpanOption) opentracing.Span {
	sso := opentracing.StartSpanOptions{}
	for _, o := range opts {
		o.Apply(&sso)
	}
	return t.StartSpanWithOptions(name, sso)
}

// StartSpanWithOptions starts a new OpenTracing span with the given name and options.
func (t *otTracer) StartSpanWithOptions(name string, opts opentracing.StartSpanOptions) opentracing.Span {
	// Because the Context method can be called at any time after
	// the span is finished, we cannot pool the objects.
	otSpan := &otSpan{
		tracer: t,
		tags:   opts.Tags,
		ctx: spanContext{
			tracer:    t,
			startTime: opts.StartTime,
		},
	}
	if opts.StartTime.IsZero() {
		otSpan.ctx.startTime = time.Now()
	}

	var parentTraceContext elasticapm.TraceContext
	if parentCtx, ok := parentSpanContext(opts.References); ok {
		if parentCtx.tracer == t && parentCtx.txSpanContext != nil {
			parentCtx.txSpanContext.mu.RLock()
			defer parentCtx.txSpanContext.mu.RUnlock()
			opts := elasticapm.SpanOptions{
				Parent: parentCtx.traceContext,
				Start:  otSpan.ctx.startTime,
			}
			if parentCtx.tx != nil {
				otSpan.span = parentCtx.tx.StartSpanOptions(name, "", opts)
			} else {
				otSpan.span = t.tracer.StartSpan(name, "",
					parentCtx.transactionID,
					parentCtx.txSpanContext.startTime,
					opts,
				)
			}
			otSpan.ctx.traceContext = otSpan.span.TraceContext()
			otSpan.ctx.transactionID = parentCtx.transactionID
			otSpan.ctx.txSpanContext = parentCtx.txSpanContext
			return otSpan
		}
		parentTraceContext = parentCtx.traceContext
	}

	// There's no local parent context created by this tracer.
	otSpan.ctx.tx = t.tracer.StartTransactionOptions(name, "", elasticapm.TransactionOptions{
		TraceContext: parentTraceContext,
		Start:        otSpan.ctx.startTime,
	})
	otSpan.ctx.traceContext = otSpan.ctx.tx.TraceContext()
	otSpan.ctx.transactionID = otSpan.ctx.traceContext.Span
	otSpan.ctx.txSpanContext = &otSpan.ctx
	return otSpan
}

func (t *otTracer) Inject(sc opentracing.SpanContext, format interface{}, carrier interface{}) error {
	spanContext, ok := sc.(*spanContext)
	if !ok {
		return opentracing.ErrInvalidSpanContext
	}
	switch format {
	case opentracing.TextMap, opentracing.HTTPHeaders:
		writer, ok := carrier.(opentracing.TextMapWriter)
		if !ok {
			return opentracing.ErrInvalidCarrier
		}
		headerValue := apmhttp.FormatTraceparentHeader(spanContext.traceContext)
		writer.Set(apmhttp.TraceparentHeader, headerValue)
		return nil
	case opentracing.Binary:
		writer, ok := carrier.(io.Writer)
		if !ok {
			return opentracing.ErrInvalidCarrier
		}
		return binaryInject(writer, spanContext.traceContext)
	default:
		return opentracing.ErrUnsupportedFormat
	}
}

func (t *otTracer) Extract(format interface{}, carrier interface{}) (opentracing.SpanContext, error) {
	switch format {
	case opentracing.TextMap, opentracing.HTTPHeaders:
		var headerValue string
		switch carrier := carrier.(type) {
		case opentracing.HTTPHeadersCarrier:
			headerValue = http.Header(carrier).Get(apmhttp.TraceparentHeader)
		case opentracing.TextMapReader:
			carrier.ForeachKey(func(key, val string) error {
				if textproto.CanonicalMIMEHeaderKey(key) == apmhttp.TraceparentHeader {
					headerValue = val
					return io.EOF // arbitrary error to break loop
				}
				return nil
			})
		default:
			return nil, opentracing.ErrInvalidCarrier
		}
		if headerValue == "" {
			return nil, opentracing.ErrSpanContextNotFound
		}
		traceContext, err := apmhttp.ParseTraceparentHeader(headerValue)
		if err != nil {
			return nil, err
		}
		return &spanContext{tracer: t, traceContext: traceContext}, nil
	case opentracing.Binary:
		reader, ok := carrier.(io.Reader)
		if !ok {
			return nil, opentracing.ErrInvalidCarrier
		}
		traceContext, err := binaryExtract(reader)
		if err != nil {
			return nil, err
		}
		return &spanContext{tracer: t, traceContext: traceContext}, nil
	default:
		return nil, opentracing.ErrUnsupportedFormat
	}
}

// Option sets options for the OpenTracing Tracer implementation.
type Option func(*otTracer)

// WithTracer returns an Option which sets t as the underlying
// elasticapm.Tracer for constructing an OpenTracing Tracer.
func WithTracer(t *elasticapm.Tracer) Option {
	if t == nil {
		panic("t == nil")
	}
	return func(o *otTracer) {
		o.tracer = t
	}
}

// TODO(axw) handle binary format once Trace-Context defines one.
// OpenTracing mandates that all implementations "MUST" support all
// of the builtin formats.

var (
	binaryInject  = binaryInjectUnsupported
	binaryExtract = binaryExtractUnsupported
)

func binaryInjectUnsupported(w io.Writer, traceContext elasticapm.TraceContext) error {
	return opentracing.ErrUnsupportedFormat
}

func binaryExtractUnsupported(r io.Reader) (elasticapm.TraceContext, error) {
	return elasticapm.TraceContext{}, opentracing.ErrUnsupportedFormat
}
