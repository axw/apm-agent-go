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

package apm

import (
	"bytes"
	"context"
	"io"
	"runtime/pprof"
	"time"

	"github.com/pkg/errors"

	"go.elastic.co/fastjson"
)

func (t *Tracer) profileCPU(ctx context.Context, duration time.Duration) error {
	var buf bytes.Buffer
	if err := pprof.StartCPUProfile(&buf); err != nil {
		return errors.Wrap(err, "failed to start CPU profile")
	}
	time.Sleep(duration)
	pprof.StopCPUProfile()
	return errors.Wrap(t.sendProfile(ctx, &buf), "failed to send CPU profile")
}

/*
func (t *Tracer) profileHeap(ctx context.Context) error {
	//if err := pprof.Lookup("heap").WriteTo(&heapBuf, 0); err != nil {
	//	// TODO(axw) log warning, continue to report what we have.
	//	panic(err)
	//}
}
*/

func (t *Tracer) sendProfile(ctx context.Context, profileBuf *bytes.Buffer) error {
	var jsonWriter fastjson.Writer
	t.encodeRequestMetadata(&jsonWriter)
	return t.profileSender.SendProfile(ctx,
		bytes.NewReader(jsonWriter.Bytes()),
		profileBuf,
	)
}

type profileSender interface {
	SendProfile(ctx context.Context, metadata io.Reader, profile ...io.Reader) error
}
