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

package apmoc

import (
	"go.opencensus.io/trace"

	"go.elastic.co/apm"
)

// Exporter is an implementation of opencensus/trace.Exporter that exports spans to Elastic APM.
type Exporter struct {
	Tracer *apm.Tracer
}

// ExportSpan reports s as a transaction or span to e.Tracer.
func (e *Exporter) ExportSpan(s *trace.SpanData) {
	// TODO(axw) need to do better for the type -- should be based on attributes.
	// TODO(axw) pull out well-known attributes, like "http.*", and construct the appropriate context.
	// TODO(axw) add support for marks, translate annotations to marks?
	var traceOptions apm.TraceOptions
	if s.TraceOptions.IsSampled() {
		traceOptions = traceOptions.WithRecorded(true)
	}

	if s.HasRemoteParent || s.ParentSpanID == (trace.SpanID{}) {
		tx := e.Tracer.StartTransactionOptions(
			s.Name, "request",
			apm.TransactionOptions{
				Start:         s.StartTime,
				TransactionID: apm.SpanID(s.SpanID),
				TraceContext: apm.TraceContext{
					Trace:   apm.TraceID(s.TraceID),
					Span:    apm.SpanID(s.ParentSpanID),
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
		transactionID := apm.SpanID(s.ParentSpanID)
		span := e.Tracer.StartSpan(
			s.Name, "request",
			transactionID,
			apm.SpanOptions{
				Start:  s.StartTime,
				SpanID: apm.SpanID(s.SpanID),
				Parent: apm.TraceContext{
					Trace:   apm.TraceID(s.TraceID),
					Span:    apm.SpanID(s.ParentSpanID),
					Options: traceOptions,
				},
			},
		)
		span.Duration = s.EndTime.Sub(s.StartTime)
		span.End()
	}
}
