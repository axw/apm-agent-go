package elasticapm

import (
	"bytes"
	"compress/zlib"
	"context"
	"io"
	"time"

	"github.com/elastic/apm-agent-go/internal/fastjson"
	"github.com/elastic/apm-agent-go/internal/iochan"
	"github.com/elastic/apm-agent-go/model"
	"github.com/elastic/apm-agent-go/stacktrace"
)

// notSampled is used as the pointee for the model.Transaction.Sampled field
// of non-sampled transactions.
var notSampled = false

type sender struct {
	tracer  *Tracer
	buffer  *buffer
	statsC  chan TracerStats
	stats   TracerStats
	cfgC    chan tracerConfig
	cfg     tracerConfig
	metrics Metrics

	modelSpans      []model.Span
	modelStacktrace []model.StacktraceFrame
}

func newSender(t *Tracer) *sender {
	s := &sender{
		tracer: t,
		statsC: make(chan TracerStats),
		cfgC:   make(chan tracerConfig),
	}
	go s.loop()
	return s
}

func (s *sender) loop() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s.cfg = <-s.cfgC

	// TODO(axw) make the buffer size configurable
	s.buffer = newBuffer(10 * 1024 * 1024)

	// TODO(axw) make this configurable
	requestSize := 768 * 1024

	var req iochan.ReadRequest
	var requestBuf bytes.Buffer
	var metadata []byte
	var gracePeriod time.Duration = -1
	zlibWriter := zlib.NewWriter(&requestBuf)
	zlibFlushed := true
	zlibClosed := false
	iochanReader := iochan.NewReader()
	requestBytesRead := 0
	requestActive := false
	closeRequest := false
	flushRequest := false
	requestResult := make(chan error, 1)
	requestTimer := time.NewTimer(0)
	if !requestTimer.Stop() {
		<-requestTimer.C
	}

	for {
		statsC := s.statsC
		if s.stats.isZero() {
			statsC = nil
		}

		select {
		case <-s.tracer.closed:
			return
		case s.cfg = <-s.cfgC:
			continue
		case statsC <- s.stats:
			s.stats = TracerStats{}
			continue
		case tx := <-s.tracer.transactions:
			s.writeTransaction(tx)
		case e := <-s.tracer.errors:
			s.writeError(e)
			flushRequest = true
		case <-requestTimer.C:
			closeRequest = true
		case req = <-iochanReader.C:
		case err := <-requestResult:
			if err != nil {
				gracePeriod = nextGracePeriod(gracePeriod)
				if s.cfg.logger != nil {
					s.cfg.logger.Debugf("request failed (next request in %s): %s", err, gracePeriod)
				}
			} else {
				// Reset grace period after success.
				gracePeriod = -1
			}
			flushRequest = false
			closeRequest = false
			requestActive = false
			requestBytesRead = 0
			requestBuf.Reset()
			if !requestTimer.Stop() {
				<-requestTimer.C
			}
		}

		// TODO(axw) make the goroutine below long-running, and send
		// requests to start new requests?
		if !requestActive && (s.buffer.Len() > 0 || requestBuf.Len() > 0) {
			go func() {
				if gracePeriod > 0 {
					select {
					case <-time.After(gracePeriod):
					case <-ctx.Done():
					}
				}
				requestResult <- s.tracer.Transport.SendStream(ctx, iochanReader)
			}()
			if metadata == nil {
				metadata = s.requestMetadata()
			}
			zlibWriter.Write(metadata)
			zlibFlushed = false
			requestActive = true
			requestTimer.Reset(s.cfg.requestDuration)
		}

		if requestActive && (!closeRequest || !zlibClosed) {
			for requestBytesRead+requestBuf.Len() < requestSize && s.buffer.Len() > 0 {
				s.buffer.WriteTo(zlibWriter)
				zlibWriter.Write([]byte("\n"))
				zlibFlushed = false
			}
			if !closeRequest {
				closeRequest = requestBytesRead+requestBuf.Len() >= requestSize
			}
		}
		if closeRequest {
			if !zlibClosed {
				zlibWriter.Close()
				zlibClosed = true
			}
		} else if flushRequest && !zlibFlushed {
			zlibWriter.Flush()
			flushRequest = false
			zlibFlushed = true
		}

		if req.Buf != nil && (requestBuf.Len() > 0 || closeRequest) {
			n, err := requestBuf.Read(req.Buf)
			if closeRequest && err == nil && n <= len(req.Buf) {
				err = io.EOF
			}
			req.Respond(n, err)
			req.Buf = nil
			if n > 0 {
				requestBytesRead += n
			}
		}
	}
}

// requestMetadata returns a JSON-encoded metadata object that features
// at the head of every request body.
func (s *sender) requestMetadata() []byte {
	var json fastjson.Writer
	service := makeService(s.tracer.Service.Name, s.tracer.Service.Version, s.tracer.Service.Environment)
	json.RawString(`{"metadata":{`)
	json.RawString(`"system":`)
	s.tracer.system.MarshalFastJSON(&json)
	json.RawString(`,"process":`)
	s.tracer.process.MarshalFastJSON(&json)
	json.RawString(`,"service":`)
	service.MarshalFastJSON(&json)
	json.RawString(`}}`)
	return json.Bytes()
}

func (s *sender) writeTransaction(tx *Transaction) {
	// TODO(axw) need to start sending spans independently.
	s.modelSpans = s.modelSpans[:0]
	s.modelStacktrace = s.modelStacktrace[:0]

	var modelTx model.Transaction
	s.buildModelTransaction(&modelTx, tx)
	s.buffer.WriteTransaction(modelTx)
	tx.reset()
	s.stats.TransactionsSent++
}

func (s *sender) writeError(e *Error) {
	s.buffer.WriteError(e.model)
	e.reset()
	s.stats.ErrorsSent++
}

// sendMetrics attempts to send metrics to the APM server. This must be
// called after gatherMetrics has signalled that metrics have all been
// gathered.
func (s *sender) sendMetrics(logger Logger) {
	/*
		if len(s.metrics.metrics) == 0 {
			return
		}
		for _, metrics := range s.metrics.metrics {
			if err := s.stream.WriteMetrics(*metrics); err != nil {
				if logger != nil {
					logger.Debugf("failed to send metrics: %s", err)
				}
				s.stats.Errors.WriteMetrics++
			}
		}
		s.metrics.reset()
	*/
}

// gatherMetrics gathers metrics from each of the registered
// metrics gatherers. Once all gatherers have returned, a value
// will be sent on the "gathered" channel.
func (s *sender) gatherMetrics(ctx context.Context, gathered chan<- struct{}) {
	/*
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
	*/
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
	if err != nil {
		if s.cfg.logger != nil {
			s.cfg.logger.Debugf("setting context failed: %s", err)
		}
		s.stats.Errors.SetContext++
	}
}
