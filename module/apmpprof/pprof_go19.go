// +build go1.9

package apmpprof

import (
	"context"
	"runtime/pprof"

	"github.com/elastic/apm-agent-go"
)

const (
	// TransactionLabelKey is the label key identifying
	// the active transaction.
	TransactionLabelKey = "elastic.transaction"
)

func do(ctx context.Context, f func(context.Context)) {
	tx := elasticapm.TransactionFromContext(ctx)
	if tx == nil {
		f(ctx)
		return
	}
	pprof.Do(ctx, pprof.Labels(TransactionLabelKey, tx.Name), f)
}
