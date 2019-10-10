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

package apmotel_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	sdktrace "go.opentelemetry.io/sdk/trace"

	"go.elastic.co/apm"
	"go.elastic.co/apm/apmtest"
	"go.elastic.co/apm/module/apmotel"
)

var globalTracer = sdktrace.Register()

func init() {
	sdktrace.ApplyConfig(sdktrace.Config{DefaultSampler: sdktrace.AlwaysSample()})
}

func newSpanProcessor(tracer *apm.Tracer) *sdktrace.SimpleSpanProcessor {
	return sdktrace.NewSimpleSpanProcessor(newExporter(tracer))
}

func newExporter(tracer *apm.Tracer) *apmotel.Exporter {
	return &apmotel.Exporter{Tracer: tracer}
}

func TestExporter(t *testing.T) {
	apmTracer := apmtest.NewRecordingTracer()
	spanProcessor := newSpanProcessor(apmTracer.Tracer)
	sdktrace.RegisterSpanProcessor(spanProcessor)
	defer sdktrace.UnregisterSpanProcessor(spanProcessor)

	globalTracer.WithSpan(context.Background(), "foo", func(ctx context.Context) error {
		globalTracer.WithSpan(ctx, "bar", func(ctx context.Context) error {
			globalTracer.WithSpan(ctx, "baz", func(ctx context.Context) error {
				return nil
			})
			return nil
		})
		return nil
	})

	apmTracer.Flush(nil)
	assert.Len(t, apmTracer.Payloads().Transactions, 1)
	assert.Len(t, apmTracer.Payloads().Spans, 2)
}
