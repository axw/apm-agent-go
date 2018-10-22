// +build go1.9

package hdrhistogram

import "math/bits"

func bitLen(x int64) (n int64) {
	return int64(bits.Len64(uint64(x)))
}
