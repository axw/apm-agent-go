package elasticapm

import (
	"bytes"

	"github.com/elastic/apm-agent-go/internal/fastjson"
	"github.com/elastic/apm-agent-go/model"
)

type buffer struct {
	buf     []byte
	readbuf []byte
	len     int
	write   int // index into "start" at which
	read    int
	json    fastjson.Writer
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

func (b *buffer) Pop() []byte {
	if b.len == 0 {
		return nil
	}
	b.readbuf = b.readbuf[:0]
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
		b.readbuf = append(b.readbuf, tail...)
		b.read = 0
		b.len -= taillen
		goto more
	}
	b.read = (b.read + end + 1) % b.Cap()
	b.len -= end + 1
	b.readbuf = append(b.readbuf, tail[:end]...)
	return b.readbuf
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
	b.json.RawByte(0) // delimiter
	bytes := b.json.Bytes()
	if len(bytes) > b.Cap() {
		// Buffer is too small to hold the object, silently drop.
		return
	}
	space := b.Cap() - b.Len()
	if len(bytes) > space {
		// TODO(axw) evacuate oldest entries (i.e. move read pointer
		// along) such that we can fit the new entry.
		panic("not enough space, evacuation not implemented")
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
