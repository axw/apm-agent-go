package apmot

import (
	"sync"
	"time"

	"github.com/opentracing/opentracing-go"

	"github.com/elastic/apm-agent-go"
)

type spanContext struct {
	tracer *otTracer // for origin of tx/span

	txSpanContext *spanContext // spanContext for OT span which created tx
	traceContext  elasticapm.TraceContext
	transactionID elasticapm.SpanID
	startTime     time.Time

	mu sync.RWMutex
	tx *elasticapm.Transaction
}

func (s *spanContext) TraceContext() elasticapm.TraceContext {
	return s.traceContext
}

func (s *spanContext) Transaction() *elasticapm.Transaction {
	if s.txSpanContext != nil {
		return s.txSpanContext.tx
	}
	return s.tx
}

// ForeachBaggageItem is a no-op; we do not support baggage propagation.
func (*spanContext) ForeachBaggageItem(handler func(k, v string) bool) {}

func parentSpanContext(refs []opentracing.SpanReference) (*spanContext, bool) {
	for _, ref := range refs {
		switch ref.Type {
		case opentracing.ChildOfRef, opentracing.FollowsFromRef:
			if ctx, ok := ref.ReferencedContext.(*spanContext); ok {
				return ctx, ok
			}
			if apmSpanContext, ok := ref.ReferencedContext.(interface {
				Transaction() *elasticapm.Transaction
				TraceContext() elasticapm.TraceContext
			}); ok {
				spanContext := &spanContext{
					tx:           apmSpanContext.Transaction(),
					traceContext: apmSpanContext.TraceContext(),
				}
				spanContext.transactionID = spanContext.tx.TraceContext().Span
				return spanContext, true
			}
		}
	}
	return nil, false
}
