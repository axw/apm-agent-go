package shenanigans

import (
	"context"
	"fmt"
	"runtime/pprof"
	"strconv"
	"unsafe"
)

type labelMap map[string]string

const glsKey = "axwisterriblepeople"

//go:linkname runtime_getProfLabel runtime/pprof.runtime_getProfLabel
func runtime_getProfLabel() unsafe.Pointer

// Do runs f with ctx associated to the goroutine
// such that a call to Context returns ctx.
func Do(ctx context.Context, f func()) {
	ptrString := fmt.Sprintf("%x", &ctx)
	pprof.Do(context.Background(), pprof.Labels(glsKey, ptrString), func(ctx context.Context) {
		f()
	})
}

// Context returns the context associated with this goroutine, if any.
func Context() context.Context {
	m := (*labelMap)(runtime_getProfLabel())
	if m == nil {
		return nil
	}
	ptrString, ok := (*m)[glsKey]
	if !ok {
		return nil
	}
	ptrUint, err := strconv.ParseUint(ptrString, 16, 64)
	if err != nil {
		return nil
	}
	return *(*context.Context)(unsafe.Pointer(uintptr(ptrUint)))
}
