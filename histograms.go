package apm

import (
	"sync"
	"time"

	"go.elastic.co/apm/internal/hdrhistogram"
)

const (
	// histogramMaxDuration is the maximum value we will
	// record for transaction histograms. Any transactions
	// that exceed this duration will be reported with this
	// value.
	histogramMaxDuration = time.Hour

	// histogramResolution is the resolution of duration
	// histogram recordings. We only record to microsecond
	// resolution.
	histogramResolution = time.Microsecond
)

type transactionHistograms struct {
	mu     sync.Mutex
	groups map[transactionGroupKey]*transactionHistogram
}

func (hs *transactionHistograms) record(tx *TransactionData) bool {
	if tx.Duration > histogramMaxDuration {
		tx.Duration = histogramMaxDuration
	}
	if h := hs.get(tx); h != nil {
		h.increment(tx.Duration)
		return true
	}
	return false
}

func (hs *transactionHistograms) get(tx *TransactionData) *transactionHistogram {
	// TODO(axw) record last N x 5 minute intervals,
	// choose period containing tx.Start. If tx is
	// too old, just discard it.
	k := makeTransactionGroupKey(tx)
	hs.mu.Lock()
	h, ok := hs.groups[k]
	if !ok {
		h = newTransactionHistogram()
		hs.groups[k] = h
	}
	hs.mu.Unlock()
	return h
}

type transactionGroupKey struct {
	transactionType   string
	transactionName   string
	transactionResult string
}

func makeTransactionGroupKey(tx *TransactionData) transactionGroupKey {
	return transactionGroupKey{
		transactionType:   tx.Type,
		transactionName:   tx.Name,
		transactionResult: tx.Result,
	}
}

type transactionHistogram struct {
	mu  sync.Mutex
	hdr *hdrhistogram.Histogram
}

func newTransactionHistogram() *transactionHistogram {
	const (
		minValue = 0
		maxValue = int64(histogramMaxDuration / histogramResolution)
		sigFigs  = 2
	)
	// TODO(axw) defer creation of histogram until
	// we hit a threshold of values?
	return &transactionHistogram{
		hdr: hdrhistogram.New(minValue, maxValue, sigFigs),
	}
}

func (h *transactionHistogram) increment(d time.Duration) {
	h.mu.Lock()
	h.hdr.RecordValue(int64(d / histogramResolution))
	h.mu.Unlock()
}
