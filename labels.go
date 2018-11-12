package apm

import (
	"encoding/hex"
	"strconv"
	"strings"
	"unsafe"
)

//go:linkname runtime_getProfLabel runtime/pprof.runtime_getProfLabel
func runtime_getProfLabel() unsafe.Pointer

type labelMap map[string]string

func traceContext() TraceContext {
	m := (*labelMap)(runtime_getProfLabel())
	if m == nil {
		return TraceContext{}
	}
	traceContextString, ok := (*m)["trace_context"]
	if !ok {
		return TraceContext{}
	}
	fields := strings.Split(traceContextString, "-")

	var out TraceContext
	hex.Decode(out.Trace[:], []byte(fields[0]))
	hex.Decode(out.Span[:], []byte(fields[1]))
	options, _ := strconv.Atoi(fields[2])
	out.Options = TraceOptions(options)
	return out
}
