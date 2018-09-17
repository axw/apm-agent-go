package apmot

import (
	"context"

	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/log"

	"github.com/elastic/apm-agent-go"
	"github.com/elastic/apm-agent-go/internal/contextutil"
)

func init() {
	// We override the contextutil functions so that transactions
	// and spans started with the native API are wrapped and made
	// available as OpenTracing spans.
	contextutil.ContextWithSpan = contextWithSpan
	contextutil.ContextWithTransaction = contextWithTransaction
	contextutil.SpanFromContext = spanFromContext
	contextutil.TransactionFromContext = transactionFromContext
}

func contextWithSpan(ctx context.Context, apmSpan interface{}) context.Context {
	tx, _ := transactionFromContext(ctx).(*elasticapm.Transaction)
	return opentracing.ContextWithSpan(ctx, apmSpanWrapper{
		SpanContext: apmSpanWrapperContext{
			span:        apmSpan.(*elasticapm.Span),
			transaction: tx,
		},
	})
}

func contextWithTransaction(ctx context.Context, apmTransaction interface{}) context.Context {
	return opentracing.ContextWithSpan(ctx, apmTransactionWrapper{
		SpanContext: apmTransactionWrapperContext{
			transaction: apmTransaction.(*elasticapm.Transaction),
		},
	})
}

func spanFromContext(ctx context.Context) interface{} {
	otSpan, _ := opentracing.SpanFromContext(ctx).(interface {
		Span() *elasticapm.Span
	})
	if otSpan == nil {
		return nil
	}
	return otSpan.Span()
}

func transactionFromContext(ctx context.Context) interface{} {
	otSpan := opentracing.SpanFromContext(ctx)
	if otSpan == nil {
		return nil
	}
	if apmSpanContext, ok := otSpan.Context().(interface {
		Transaction() *elasticapm.Transaction
	}); ok {
		return apmSpanContext.Transaction()
	}
	return nil
}

type apmSpanWrapperContext struct {
	span        *elasticapm.Span
	transaction *elasticapm.Transaction
}

// TraceContext returns ctx.span.TraceContext(). This is used to set the
// parent trace context for spans created through the OpenTracing API.
func (ctx apmSpanWrapperContext) TraceContext() elasticapm.TraceContext {
	return ctx.span.TraceContext()
}

// Transaction returns ctx.transaction. This is used to obtain the transaction
// to use for creating spans through the OpenTracing API.
func (ctx apmSpanWrapperContext) Transaction() *elasticapm.Transaction {
	return ctx.transaction
}

// ForeachBaggageItem is a no-op; we do not support baggage propagation.
func (apmSpanWrapperContext) ForeachBaggageItem(handler func(k, v string) bool) {}

type apmSpanWrapper struct {
	SpanContext apmSpanWrapperContext
}

// Span returns s.SpanContext.span. This is used by spanFromContext.
func (s apmSpanWrapper) Span() *elasticapm.Span {
	return s.SpanContext.span
}

// SetOperationName sets or changes the operation name.
func (s apmSpanWrapper) SetOperationName(operationName string) opentracing.Span {
	return s
}

// SetTag adds or changes a tag.
func (s apmSpanWrapper) SetTag(key string, value interface{}) opentracing.Span {
	return s
}

// Finish ends the span; this (or FinishWithOptions) must be the last method
// call on the span, except for calls to Context which may be called at any
// time.
func (s apmSpanWrapper) Finish() {}

// FinishWithOptions is like Finish, but provides explicit control over the
// end timestamp and log data.
func (apmSpanWrapper) FinishWithOptions(opentracing.FinishOptions) {}

// Tracer returns the Tracer that created this span.
func (apmSpanWrapper) Tracer() opentracing.Tracer {
	return opentracing.NoopTracer{}
}

// Context returns the span's current context.
//
// It is valid to call Context after calling Finish or FinishWithOptions.
// The resulting context is also valid after the span is finished.
func (s apmSpanWrapper) Context() opentracing.SpanContext {
	return s.SpanContext
}

// SetBaggageItem is a no-op; we do not support baggage.
func (s apmSpanWrapper) SetBaggageItem(key, val string) opentracing.Span {
	// We do not support baggage.
	return s
}

// BaggageItem returns the empty string; we do not support baggage.
func (apmSpanWrapper) BaggageItem(key string) string {
	return ""
}

func (apmSpanWrapper) LogKV(keyValues ...interface{}) {}

func (apmSpanWrapper) LogFields(fields ...log.Field) {}

func (apmSpanWrapper) LogEvent(event string) {}

func (apmSpanWrapper) LogEventWithPayload(event string, payload interface{}) {}

func (apmSpanWrapper) Log(ld opentracing.LogData) {}

type apmTransactionWrapperContext struct {
	transaction *elasticapm.Transaction
}

// TraceContext returns ctx.transaction.TraceContext(). This is used to set the
// parent trace context for spans created through the OpenTracing API.
func (ctx apmTransactionWrapperContext) TraceContext() elasticapm.TraceContext {
	return ctx.transaction.TraceContext()
}

// Transaction returns ctx.transaction. This is used to obtain the transaction
// to use for creating spans through the OpenTracing API.
func (ctx apmTransactionWrapperContext) Transaction() *elasticapm.Transaction {
	return ctx.transaction
}

// ForeachBaggageItem is a no-op; we do not support baggage propagation.
func (apmTransactionWrapperContext) ForeachBaggageItem(handler func(k, v string) bool) {}

type apmTransactionWrapper struct {
	SpanContext apmTransactionWrapperContext
}

// SetOperationName sets or changes the operation name.
func (s apmTransactionWrapper) SetOperationName(operationName string) opentracing.Span {
	return s
}

// SetTag adds or changes a tag.
func (s apmTransactionWrapper) SetTag(key string, value interface{}) opentracing.Span {
	return s
}

// Finish ends the span; this (or FinishWithOptions) must be the last method
// call on the span, except for calls to Context which may be called at any
// time.
func (s apmTransactionWrapper) Finish() {}

// FinishWithOptions is like Finish, but provides explicit control over the
// end timestamp and log data.
func (apmTransactionWrapper) FinishWithOptions(opentracing.FinishOptions) {}

// Tracer returns the Tracer that created this span.
func (s apmTransactionWrapper) Tracer() opentracing.Tracer {
	return opentracing.NoopTracer{}
}

// Context returns the span's current context.
//
// It is valid to call Context after calling Finish or FinishWithOptions.
// The resulting context is also valid after the span is finished.
func (s apmTransactionWrapper) Context() opentracing.SpanContext {
	return s.SpanContext
}

// SetBaggageItem is a no-op; we do not support baggage.
func (s apmTransactionWrapper) SetBaggageItem(key, val string) opentracing.Span {
	// We do not support baggage.
	return s
}

// BaggageItem returns the empty string; we do not support baggage.
func (apmTransactionWrapper) BaggageItem(key string) string {
	return ""
}

func (apmTransactionWrapper) LogKV(keyValues ...interface{}) {}

func (apmTransactionWrapper) LogFields(fields ...log.Field) {}

func (apmTransactionWrapper) LogEvent(event string) {}

func (apmTransactionWrapper) LogEventWithPayload(event string, payload interface{}) {}

func (apmTransactionWrapper) Log(ld opentracing.LogData) {}
