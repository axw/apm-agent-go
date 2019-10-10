// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package apmotel

import (
	"context"
	"encoding/binary"

	"go.opentelemetry.io/api/core"
	"go.opentelemetry.io/sdk/export"

	"go.elastic.co/apm"
)

// Exporter implements go.opentelemetry.io/sdk/export.SpanSyncer,
// sending spans to Elastic APM.
type Exporter struct {
	Tracer *apm.Tracer
}

// ExportSpan reports s as a transaction or span to e.Tracer.
func (e *Exporter) ExportSpan(ctx context.Context, s *export.SpanData) {
	// TODO(axw) need to do better for the type -- should be based on attributes.
	// TODO(axw) pull out well-known attributes, like "http.*", and construct the appropriate context.
	// TODO(axw) do something with MessageEvents? Translate to marks?
	// TODO(axw) number of spans started is captured in ChildSpanCount, we could translate to SpanCount.Started
	var traceOptions apm.TraceOptions
	if s.SpanContext.IsSampled() {
		traceOptions = traceOptions.WithRecorded(true)
	}

	if s.HasRemoteParent || s.ParentSpanID == 0 {
		tx := e.Tracer.StartTransactionOptions(
			s.Name, "request",
			apm.TransactionOptions{
				Start:         s.StartTime,
				TransactionID: toSpanID(s.SpanContext.SpanID),
				TraceContext: apm.TraceContext{
					Trace:   toTraceID(s.SpanContext.TraceID),
					Span:    toSpanID(s.ParentSpanID),
					Options: traceOptions,
				},
			},
		)
		tx.Duration = s.EndTime.Sub(s.StartTime)
		tx.End()
	} else {
		// BUG(axw) if the parent is another span, then the transaction
		// ID here will be invalid. We should instead stop specifying the
		// transaction ID (requires APM Server 7.1+)

		transactionID := toSpanID(s.ParentSpanID)
		span := e.Tracer.StartSpan(
			s.Name, "request",
			transactionID,
			apm.SpanOptions{
				Start:  s.StartTime,
				SpanID: toSpanID(s.SpanContext.SpanID),
				Parent: apm.TraceContext{
					Trace:   toTraceID(s.SpanContext.TraceID),
					Span:    toSpanID(s.ParentSpanID),
					Options: traceOptions,
				},
			},
		)
		span.Duration = s.EndTime.Sub(s.StartTime)
		span.End()
	}
}

func toSpanID(id uint64) apm.SpanID {
	var out apm.SpanID
	binary.BigEndian.PutUint64(out[:], id)
	return out
}

func toTraceID(id core.TraceID) apm.TraceID {
	var out apm.TraceID
	binary.BigEndian.PutUint64(out[:8], id.High)
	binary.BigEndian.PutUint64(out[8:], id.Low)
	return out
}
