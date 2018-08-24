package elasticapm

import (
	"bytes"
	"io"
	"io/ioutil"

	"github.com/elastic/apm-agent-go/internal/fastjson"
	"github.com/elastic/apm-agent-go/model"
)

type buffer struct {
	buf   []byte
	len   int
	write int
	read  int
	json  fastjson.Writer
}

// newBuffer returns a new buffer with the given size in bytes.
func newBuffer(size int) *buffer {
	return &buffer{
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

// WriteTransaction writes tx to the buffer.
func (b *buffer) WriteTransaction(tx model.Transaction) {
	b.json.RawString(`{"transaction":`)
	tx.MarshalFastJSON(&b.json)
	b.json.RawByte('}')
	b.commit()
}

// WriteError writes e to the stream.
func (b *buffer) WriteError(e model.Error) {
	b.json.RawString(`{"error":`)
	e.MarshalFastJSON(&b.json)
	b.json.RawByte('}')
	b.commit()
}

// WriteMetrics writes m to the buffer.
func (b *buffer) WriteMetrics(m model.Metrics) {
	b.json.RawString(`{"metrics":`)
	m.MarshalFastJSON(&b.json)
	b.json.RawByte('}')
	b.commit()
}

func (b *buffer) commit() {
	// TODO(axw) use size header? profile
	b.json.RawByte(0) // delimiter
	bytes := b.json.Bytes()
	if len(bytes) > b.Cap() {
		// Buffer is too small to hold the object, silently drop.
		return
	}
	for len(bytes) > b.Cap()-b.Len() {
		b.WriteTo(ioutil.Discard)
	}
	n := copy(b.buf[b.write:], bytes)
	if n < len(bytes) {
		// Copy rest to beginning of buffer
		b.write = copy(b.buf, bytes[n:])
	} else {
		b.write = (b.write + n) % b.Cap()
	}
	b.len += len(bytes)
	b.json.Reset()
}
