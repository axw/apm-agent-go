package elasticapm

import (
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/log"
)

type otSpanContext struct {
	span        *Span
	transaction *Transaction
}

func (ctx otSpanContext) TraceContext() TraceContext {
	if ctx.span != nil {
		return ctx.span.TraceContext()
	}
	return ctx.transaction.TraceContext()
}

func (ctx otSpanContext) Transaction() *Transaction {
	return ctx.transaction
}

// ForeachBaggageItem is a no-op; we do not support baggage propagation.
func (otSpanContext) ForeachBaggageItem(handler func(k, v string) bool) {}

type otSpan struct {
	SpanContext otSpanContext
}

func (s otSpan) Span() *Span {
	return s.SpanContext.span
}

// SetOperationName sets or changes the operation name.
func (s otSpan) SetOperationName(operationName string) opentracing.Span {
	return s
}

// SetTag adds or changes a tag.
func (s otSpan) SetTag(key string, value interface{}) opentracing.Span {
	return s
}

// Finish ends the span; this (or FinishWithOptions) must be the last method
// call on the span, except for calls to Context which may be called at any
// time.
func (s otSpan) Finish() {
	s.FinishWithOptions(opentracing.FinishOptions{})
}

// FinishWithOptions is like Finish, but provides explicit control over the
// end timestamp and log data.
func (otSpan) FinishWithOptions(opts opentracing.FinishOptions) {
}

// Tracer returns the Tracer that created this span.
func (s otSpan) Tracer() opentracing.Tracer {
	panic("TODO")
}

// Context returns the span's current context.
//
// It is valid to call Context after calling Finish or FinishWithOptions.
// The resulting context is also valid after the span is finished.
func (s otSpan) Context() opentracing.SpanContext {
	return s.SpanContext
}

// SetBaggageItem is a no-op; we do not support baggage.
func (s otSpan) SetBaggageItem(key, val string) opentracing.Span {
	// We do not support baggage.
	return s
}

// BaggageItem returns the empty string; we do not support baggage.
func (otSpan) BaggageItem(key string) string {
	return ""
}

func (otSpan) LogKV(keyValues ...interface{}) {
	// No-op.
}

func (otSpan) LogFields(fields ...log.Field) {
	// No-op.
}

func (otSpan) LogEvent(event string) {
	// No-op.
}

func (otSpan) LogEventWithPayload(event string, payload interface{}) {
	// No-op.
}

func (otSpan) Log(ld opentracing.LogData) {
	// No-op.
}

type otTransactionSpanContext struct {
	transaction *Transaction
}

func (ctx otTransactionSpanContext) TraceContext() TraceContext {
	return ctx.transaction.TraceContext()
}

func (ctx otTransactionSpanContext) Transaction() *Transaction {
	return ctx.transaction
}

// ForeachBaggageItem is a no-op; we do not support baggage propagation.
func (otTransactionSpanContext) ForeachBaggageItem(handler func(k, v string) bool) {}

type otTransactionSpan struct {
	SpanContext otTransactionSpanContext
}

func (s otTransactionSpan) Span() *Span {
	return nil
}

// SetOperationName sets or changes the operation name.
func (s otTransactionSpan) SetOperationName(operationName string) opentracing.Span {
	return s
}

// SetTag adds or changes a tag.
func (s otTransactionSpan) SetTag(key string, value interface{}) opentracing.Span {
	return s
}

// Finish ends the span; this (or FinishWithOptions) must be the last method
// call on the span, except for calls to Context which may be called at any
// time.
func (s otTransactionSpan) Finish() {
	s.FinishWithOptions(opentracing.FinishOptions{})
}

// FinishWithOptions is like Finish, but provides explicit control over the
// end timestamp and log data.
func (otTransactionSpan) FinishWithOptions(opts opentracing.FinishOptions) {
}

// Tracer returns the Tracer that created this span.
func (s otTransactionSpan) Tracer() opentracing.Tracer {
	panic("TODO")
}

// Context returns the span's current context.
//
// It is valid to call Context after calling Finish or FinishWithOptions.
// The resulting context is also valid after the span is finished.
func (s otTransactionSpan) Context() opentracing.SpanContext {
	return s.SpanContext
}

// SetBaggageItem is a no-op; we do not support baggage.
func (s otTransactionSpan) SetBaggageItem(key, val string) opentracing.Span {
	// We do not support baggage.
	return s
}

// BaggageItem returns the empty string; we do not support baggage.
func (otTransactionSpan) BaggageItem(key string) string {
	return ""
}

func (otTransactionSpan) LogKV(keyValues ...interface{}) {
	// No-op.
}

func (otTransactionSpan) LogFields(fields ...log.Field) {
	// No-op.
}

func (otTransactionSpan) LogEvent(event string) {
	// No-op.
}

func (otTransactionSpan) LogEventWithPayload(event string, payload interface{}) {
	// No-op.
}

func (otTransactionSpan) Log(ld opentracing.LogData) {
	// No-op.
}
