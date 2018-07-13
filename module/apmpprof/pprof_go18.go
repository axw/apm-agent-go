// +build !go1.9

package apmpprof

import "context"

func do(ctx context.Context, f func(context.Context)) {
	f(ctx)
}
