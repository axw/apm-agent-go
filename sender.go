package elasticapm

import (
	"context"
	"sync"
	"time"

	"github.com/elastic/apm-agent-go/model"
	"github.com/elastic/apm-agent-go/stacktrace"
	"github.com/elastic/apm-agent-go/transport"
)

// notSampled is used as the pointee for the model.Transaction.Sampled field
// of non-sampled transactions.
var notSampled = false

type sender struct {
	tracer        *Tracer
	cfg           *tracerConfig
	stats         *TracerStats
	stream        *transport.Stream
	flushTimer    *time.Timer
	sendStream    chan struct{}
	sentStream    chan error
	streamOpen    bool
	sendingStream bool
	metrics       Metrics

	modelSpans      []model.Span
	modelStacktrace []model.StacktraceFrame
}

func newSender(t *Tracer, cfg *tracerConfig, stats *TracerStats) *sender {
	s := &sender{
		tracer:     t,
		cfg:        cfg,
		stats:      stats,
		flushTimer: time.NewTimer(0),
		sendStream: make(chan struct{}, 1),
		sentStream: make(chan error, 1),
		stream:     transport.NewStream(),
	}
	if !s.flushTimer.Stop() {
		<-s.flushTimer.C
	}
	go func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		defer s.closeStream()
		for {
			select {
			case <-t.closed:
				return
			case <-s.sendStream:
			}
			s.sentStream <- s.tracer.StreamSender.SendStream(ctx, s.stream)
		}
	}()
	return s
}

func (s *sender) maybeStartStream() {
	if s.streamOpen {
		return
	}
	s.stream.Reset()
	s.flushTimer.Reset(s.cfg.flushInterval)
	s.streamOpen = true
	s.sendingStream = true

	// TODO(axw) write metadata to stream
	/*
		service := makeService(s.tracer.Service.Name, s.tracer.Service.Version, s.tracer.Service.Environment)
		s.tracer.process,
		s.tracer.system,
	*/

	// TODO(axw) introduce "grace period" which
	// delays sending stream after an error.
	s.sendStream <- struct{}{}
}

func (s *sender) maybeCloseStream() {
	if !s.streamOpen {
		return
	}
	// TODO(axw) make the limit configurable
	const requestSizeLimit = 1024 * 1024
	if s.stream.Flushed() < requestSizeLimit {
		return
	}
	s.closeStream()
}

func (s *sender) closeStream() {
	if err := s.stream.Close(); err != nil {
		if s.cfg.logger != nil {
			s.cfg.logger.Debugf("failed to close stream: %s", err)
		}
	}
	s.streamOpen = false
	if !s.flushTimer.Stop() {
		select {
		case <-s.flushTimer.C:
		default:
		}
	}
}

// sendTransaction attempts to send a transaction to the APM server.
func (s *sender) sendTransaction(tx *Transaction) {
	// TODO(axw) need to start sending spans independently.
	s.modelSpans = s.modelSpans[:0]
	s.modelStacktrace = s.modelStacktrace[:0]

	var modelTx model.Transaction
	s.buildModelTransaction(&modelTx, tx)
	s.maybeStartStream()
	if err := s.stream.WriteTransaction(modelTx); err != nil {
		if s.cfg.logger != nil {
			s.cfg.logger.Debugf("failed to write transaction: %s", err)
		}
		return
	}
	s.stats.TransactionsSent++
	s.maybeCloseStream()
}

// sendError attempts to send an error to the APM server.
func (s *sender) sendError(e *Error) {
	s.buildModelError(e)
	s.maybeStartStream()
	if err := s.stream.WriteError(e.model); err != nil {
		if s.cfg.logger != nil {
			s.cfg.logger.Debugf("failed to write error: %s", err)
		}
		return
	}
	s.stats.ErrorsSent++
	s.maybeCloseStream()
}

// sendMetrics attempts to send metrics to the APM server. This must be
// called after gatherMetrics has signalled that metrics have all been
// gathered.
func (s *sender) sendMetrics() {
	if len(s.metrics.metrics) == 0 {
		return
	}
	s.maybeStartStream()
	for _, metrics := range s.metrics.metrics {
		if err := s.stream.WriteMetrics(*metrics); err != nil {
			if s.cfg.logger != nil {
				s.cfg.logger.Debugf("failed to send metrics: %s", err)
			}
		}
	}
	s.metrics.reset()
	s.maybeCloseStream()
}

// gatherMetrics gathers metrics from each of the registered
// metrics gatherers. Once all gatherers have returned, a value
// will be sent on the "gathered" channel.
func (s *sender) gatherMetrics(ctx context.Context, gathered chan<- struct{}) {
	// s.cfg must not be used within the goroutines, as it may be
	// concurrently mutated by the main tracer goroutine. Take a
	// copy of the current config.
	logger := s.cfg.logger

	timestamp := model.Time(time.Now().UTC())
	var group sync.WaitGroup
	for _, g := range s.cfg.metricsGatherers {
		group.Add(1)
		go func(g MetricsGatherer) {
			defer group.Done()
			gatherMetrics(ctx, g, &s.metrics, logger)
		}(g)
	}

	go func() {
		group.Wait()
		for _, m := range s.metrics.metrics {
			m.Timestamp = timestamp
		}
		gathered <- struct{}{}
	}()
}

func (s *sender) buildModelTransaction(out *model.Transaction, tx *Transaction) {
	if !tx.traceContext.Span.isZero() {
		out.TraceID = model.TraceID(tx.traceContext.Trace)
		out.ParentID = model.SpanID(tx.parentSpan)
		out.ID.SpanID = model.SpanID(tx.traceContext.Span)
	} else {
		out.ID.UUID = model.UUID(tx.traceContext.Trace)
	}

	out.Name = truncateString(tx.Name)
	out.Type = truncateString(tx.Type)
	out.Result = truncateString(tx.Result)
	out.Timestamp = model.Time(tx.Timestamp.UTC())
	out.Duration = tx.Duration.Seconds() * 1000
	out.SpanCount.Dropped.Total = tx.spansDropped

	if !tx.Sampled() {
		out.Sampled = &notSampled
	}

	out.Context = tx.Context.build()
	if s.cfg.sanitizedFieldNames != nil && out.Context != nil && out.Context.Request != nil {
		sanitizeRequest(out.Context.Request, s.cfg.sanitizedFieldNames)
	}

	spanOffset := len(s.modelSpans)
	for _, span := range tx.spans {
		s.modelSpans = append(s.modelSpans, model.Span{})
		modelSpan := &s.modelSpans[len(s.modelSpans)-1]
		s.buildModelSpan(modelSpan, span)
	}
	out.Spans = s.modelSpans[spanOffset:]
}

func (s *sender) buildModelSpan(out *model.Span, span *Span) {
	if !span.tx.traceContext.Span.isZero() {
		out.ID = model.SpanID(span.id)
		out.ParentID = model.SpanID(span.parent)
		out.TraceID = model.TraceID(span.tx.traceContext.Trace)
	}

	out.Name = truncateString(span.Name)
	out.Type = truncateString(span.Type)
	out.Start = span.Timestamp.Sub(span.tx.Timestamp).Seconds() * 1000
	out.Duration = span.Duration.Seconds() * 1000
	out.Context = span.Context.build()

	stacktraceOffset := len(s.modelStacktrace)
	s.modelStacktrace = appendModelStacktraceFrames(s.modelStacktrace, span.stacktrace)
	out.Stacktrace = s.modelStacktrace[stacktraceOffset:]
	s.setStacktraceContext(out.Stacktrace)
}

func (s *sender) buildModelError(e *Error) {
	// TODO(axw) move the model type outside of Error
	if e.Transaction != nil {
		if !e.Transaction.traceContext.Span.isZero() {
			e.model.TraceID = model.TraceID(e.Transaction.traceContext.Trace)
			e.model.ParentID = model.SpanID(e.Transaction.traceContext.Span)
		} else {
			e.model.Transaction.ID = model.UUID(e.Transaction.traceContext.Trace)
		}
	}
	s.setStacktraceContext(e.modelStacktrace)
	e.setStacktrace()
	e.setCulprit()
	e.model.ID = model.UUID(e.ID)
	e.model.Timestamp = model.Time(e.Timestamp.UTC())
	e.model.Context = e.Context.build()
	e.model.Exception.Handled = e.Handled
}

func (s *sender) setStacktraceContext(stack []model.StacktraceFrame) {
	if s.cfg.contextSetter == nil || len(stack) == 0 {
		return
	}
	err := stacktrace.SetContext(s.cfg.contextSetter, stack, s.cfg.preContext, s.cfg.postContext)
	if s.cfg.logger != nil {
		s.cfg.logger.Debugf("setting context failed: %s", err)
	}
	s.stats.Errors.SetContext++
}
