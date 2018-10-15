package shenanigans

import (
	"context"
	"fmt"
	"reflect"
	"runtime/pprof"
	"strconv"
	"unsafe"
)

type labelMap map[string]string

const (
	label1 = "axwisterriblepeople"
	label2 = "terriblepeopleisay"
)

//go:linkname runtime_getProfLabel runtime/pprof.runtime_getProfLabel
func runtime_getProfLabel() unsafe.Pointer

// Do runs f with ctx associated to the goroutine
// such that a call to Context returns ctx.
func Do(ctx context.Context, f func()) {
	// TODO(axw) use proper labels/values for
	// the active transaction (and span?)
	val1 := "abc"
	val2 := "def"
	ptrString := fmt.Sprintf("%016x", &ctx)
	s := fmt.Sprintf("%s%s%s", val1, ptrString, val2)

	pprof.Do(
		context.Background(),
		pprof.Labels(
			label1, s[:len(val1)],
			label2, s[len(val1)+len(ptrString):],
		),
		func(ctx context.Context) {
			f()
		},
	)
}

// Context returns the context associated with this goroutine, if any.
func Context() context.Context {
	m := (*labelMap)(runtime_getProfLabel())
	if m == nil {
		return nil
	}
	val1, ok := (*m)[label1]
	if !ok {
		return nil
	}
	val2, ok := (*m)[label2]
	if !ok {
		return nil
	}
	s1 := (*reflect.StringHeader)(unsafe.Pointer(&val1))
	s2 := (*reflect.StringHeader)(unsafe.Pointer(&val2))

	var ptrString string
	s3 := (*reflect.StringHeader)(unsafe.Pointer(&ptrString))
	s3.Data = s1.Data + uintptr(s1.Len)
	s3.Len = int(s2.Data - s3.Data)
	if s3.Len != 16 {
		// Sanity-check: we zero-pad the address so it's
		// guaranteed to be 16 hex bytes.
		//
		// TODO(axw) should we be extra paranoid and check
		// that the two data addresses sit within the same
		// memory page?
		return nil
	}
	ptrUint, err := strconv.ParseUint(ptrString, 16, 64)
	if err != nil {
		return nil
	}
	return *(*context.Context)(unsafe.Pointer(uintptr(ptrUint)))
}

func GoroutineLabels() map[string]string {
	m := (*labelMap)(runtime_getProfLabel())
	return *m
}
