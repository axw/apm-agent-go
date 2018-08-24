package ringbuffer

import (
	"bytes"
	"io"
	"io/ioutil"
)

// buffer is a buffer of blocks. Each block is written and read discretely.
type buffer struct {
	buf   []byte
	len   int
	write int
	read  int
}

// newBuffer returns a new buffer with the given size in bytes.
func newBuffer(size int) *buffer {
	return &buffer{
		// TODO(axw) grow to size as required? profile
		buf: make([]byte, size),
	}
}

func (b *buffer) Len() int {
	return b.len
}

func (b *buffer) Cap() int {
	return len(b.buf)
}

func (b *buffer) WriteTo(w io.Writer) (written int64, err error) {
	if b.len == 0 {
		return 0, io.EOF
	}
more:
	tailcap := b.Cap() - b.read
	taillen := tailcap
	if taillen > b.len {
		taillen = b.len
	}
	tail := b.buf[b.read : b.read+taillen]
	end := bytes.IndexByte(tail, 0)
	if end == -1 {
		if tailcap > taillen {
			panic("missing delimeter")
		}
		if n, err := w.Write(tail); err != nil {
			return int64(n), err
		}
		b.read = 0
		b.len -= taillen
		written += int64(taillen)
		goto more
	}
	b.read = (b.read + end + 1) % b.Cap()
	b.len -= end + 1
	n, err := w.Write(tail[:end])
	return written + int64(n), err
}

func (b *buffer) Write(p []byte) (int, error) {
	// TODO(axw) use size header? profile
	lenp := len(p) + 1 // +1 for delimeter
	if lenp > b.Cap() {
		// Buffer is too small to hold the object, silently drop.
		return 0, bytes.ErrTooLarge
	}
	for lenp > b.Cap()-b.Len() {
		b.WriteTo(ioutil.Discard)
	}
	n := copy(b.buf[b.write:], p)
	if n < lenp-1 {
		// Copy rest to beginning of buffer
		b.write = copy(b.buf, p[n:])
	} else {
		b.write = (b.write + n) % b.Cap()
	}
	b.buf[b.write] = 0
	b.write++
	b.len += lenp
	return lenp, nil
}
