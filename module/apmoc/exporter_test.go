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

package apmoc_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opencensus.io/trace"

	"go.elastic.co/apm/module/apmoc"
	"go.elastic.co/apm/transport/transporttest"
)

func TestStartSpan(t *testing.T) {
	exporter, recorder, closeExporter := registerTestExporter()
	defer closeExporter()

	ctx, parentSpan := trace.StartSpan(context.Background(), "parent")
	_, childSpan := trace.StartSpan(ctx, "child")
	childSpan.End()
	parentSpan.End()
	exporter.Tracer.Flush(nil)

	payloads := recorder.Payloads()
	require.Len(t, payloads.Transactions, 1)
	require.Len(t, payloads.Spans, 1)
}

func TestStartSpanParentFinished(t *testing.T) {
	exporter, recorder, closeExporter := registerTestExporter()
	defer closeExporter()

	ctx, parentSpan := trace.StartSpan(context.Background(), "parent")
	parentSpan.End()

	// Move forward in time to bump the span's start offset.
	time.Sleep(time.Millisecond)

	ctx, childSpan := trace.StartSpan(ctx, "child")
	childSpan.End()

	ctx, grandChildSpan := trace.StartSpan(ctx, "grandchild")
	grandChildSpan.End()

	exporter.Tracer.Flush(nil)
	payloads := recorder.Payloads()
	require.Len(t, payloads.Transactions, 1)
	require.Len(t, payloads.Spans, 2)

	tx := payloads.Transactions[0]
	assert.Equal(t, tx.ID, payloads.Spans[0].ParentID)
	assert.Equal(t, payloads.Spans[0].ID, payloads.Spans[1].ParentID)
	assert.Equal(t, tx.ID, payloads.Spans[0].ParentID)
	for _, span := range payloads.Spans {
		assert.NotEqual(t, tx.Timestamp, span.Timestamp)
		//assert.Equal(t, tx.ID, span.TransactionID) //BUG
		assert.Equal(t, tx.TraceID, span.TraceID)
	}
}

func registerTestExporter() (*apmoc.Exporter, *transporttest.RecorderTransport, func()) {
	apmtracer, recorder := transporttest.NewRecorderTracer()
	exporter := &apmoc.Exporter{Tracer: apmtracer}
	trace.RegisterExporter(exporter)
	trace.ApplyConfig(trace.Config{
		DefaultSampler: trace.AlwaysSample(),
	})
	return exporter, recorder, func() {
		trace.UnregisterExporter(exporter)
		exporter.Tracer.Close()
	}
}
