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
	"encoding/binary"
	"math"
	"math/big"

	"github.com/pkg/errors"
)

// Sampler provides a means of sampling transactions.
type Sampler interface {
	// Sample indicates whether or not a transaction
	// should be sampled. This method will be invoked
	// by calls to Tracer.StartTransaction for the root
	// of a trace, so it must be goroutine-safe, and
	// should avoid synchronization as far as possible.
	//
	// NOTE Sampler will be changed in the future to
	// accept a *Transaction, and TransactionSampler
	// will be removed.
	Sample(TraceContext) bool
}

// TransactionSampler is an interface that Samplers may optionally
// implement, in which case SampleTransaction will be used in favour
// of Sample.
type TransactionSampler interface {
	// SampleTransaction indicates whether or not tx should
	// be sampled. This method will be invoked by calls to
	// Tracer.StartTransaction for the root of a trace, so it
	// must be goroutine-safe, and should avoid synchronization
	// as far as possible.
	//
	// If a Sampler implements TransactionSampler, then only
	// SampleTransaction will be called, and not Sample.
	//
	// SampleTransaction must not modify tx.
	SampleTransaction(tx *Transaction) bool
}

// NewRatioSampler returns a new Sampler with the given ratio
//
// A ratio of 1.0 samples 100% of transactions, a ratio of 0.5
// samples ~50%, and so on. If the ratio provided does not lie
// within the range [0,1.0], NewRatioSampler will panic.
//
// The returned Sampler bases its decision on the value of the
// transaction ID, so there is no synchronization involved.
func NewRatioSampler(r float64) Sampler {
	if r < 0 || r > 1.0 {
		panic(errors.Errorf("ratio %v out of range [0,1.0]", r))
	}
	var x big.Float
	x.SetUint64(math.MaxUint64)
	x.Mul(&x, big.NewFloat(r))
	ceil, _ := x.Uint64()
	return ratioSampler{ceil}
}

type ratioSampler struct {
	ceil uint64
}

// Sample samples the transaction according to the configured
// ratio and pseudo-random source.
func (s ratioSampler) Sample(c TraceContext) bool {
	v := binary.BigEndian.Uint64(c.Span[:])
	return v > 0 && v-1 < s.ceil
}

type basicTransactionSampler struct {
	Sampler
}

func (s basicTransactionSampler) SampleTransaction(tx *Transaction) bool {
	return s.Sample(tx.traceContext)
}

func makeTransactionSampler(s Sampler) TransactionSampler {
	switch s := s.(type) {
	case nil:
		return nil
	case TransactionSampler:
		return s
	}
	return basicTransactionSampler{s}
}
