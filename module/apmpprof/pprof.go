package apmpprof

import "context"

// Do calls f, adding a label to ctx for the active
// transaction on supported versions of Go.
func Do(ctx context.Context, f func(context.Context)) {
	do(ctx, f)
}
