package apm

import (
	"math/rand"
	"testing"
	"time"
)

func BenchmarkTransactionHistograms(b *testing.B) {
	rand.Seed(time.Now().UnixNano())
	newRNG := func() func() int64 {
		rng := rand.New(rand.NewSource(rand.Int63()))
		zipf := rand.NewZipf(rng, 1.1, 5, uint64(histogramMaxDuration))
		return func() int64 {
			return int64(zipf.Uint64())
		}
	}

	rng := newRNG()
	makeTransaction := func(name string) *TransactionData {
		return &TransactionData{
			Name:     name,
			Type:     "request",
			Result:   "2xx",
			Duration: time.Duration(rng()),
		}
	}
	txs := []*TransactionData{
		makeTransaction("GET /api/stats"),
		makeTransaction("GET /api/products"),
		makeTransaction("GET /api/products:id"),
		makeTransaction("GET /api/products/:id/customers"),
		makeTransaction("GET /api/types"),
		makeTransaction("GET /api/types/:id"),
		makeTransaction("GET /api/customers"),
		makeTransaction("GET /api/customers/:id"),
		makeTransaction("GET /api/orders"),
		makeTransaction("GET /api/orders/:id"),
		makeTransaction("POST /api/orders"),
		makeTransaction("POST /api/orders/csv"),
	}

	ts := newTransactionHistograms()
	b.ResetTimer()
	is := rand.Perm(len(txs))
	for i := 0; i < b.N; i++ {
		tx := txs[is[i%len(is)]]
		ts.record(tx)
	}
}
