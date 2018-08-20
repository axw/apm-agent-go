package transport

import (
	"compress/flate"
	"io"
	"io/ioutil"

	"github.com/elastic/apm-agent-go/internal/fastjson"
	"github.com/elastic/apm-agent-go/model"
)

// Stream is an io.Reader that returns JSON-encoded and DEFLATE-compressed
// model entities (transactions, spans, etc.). Reads on a stream will block
// until entities are written to it, or the stream is closed.
//
// Writes to the stream may be buffered due to DEFLATE compression. When
// the flate buffer is full, or the stream is explicitly flushed, then
// the compressed data will be flushed to the writer, which will block
// until the stream is read.
//
// The Write methods are not safe for concurrent use.
type Stream struct {
	r *io.PipeReader
	w *io.PipeWriter

	jsonWriter     fastjson.Writer
	pipeReader     *io.PipeReader
	pipeWriter     *io.PipeWriter
	countingWriter countingWriter
	flateWriter    *flate.Writer
}

// NewStream is equivalent to NewStreamLevel(flate.DefaultCompression).
func NewStream() *Stream {
	stream, err := NewStreamLevel(flate.DefaultCompression)
	if err != nil {
		panic(err)
	}
	return stream
}

// NewStreamLevel returns a new Stream with the given compression level.
func NewStreamLevel(compressionLevel int) (*Stream, error) {
	flateWriter, err := flate.NewWriter(ioutil.Discard, compressionLevel)
	if err != nil {
		return nil, err
	}
	s := Stream{flateWriter: flateWriter}
	s.Reset()
	return &s, nil
}

// Reset resets the stream such that it is equivalent to its initial state.
func (s *Stream) Reset() {
	s.countingWriter = countingWriter{}
	s.pipeReader, s.pipeWriter = io.Pipe()
	s.flateWriter.Reset(io.MultiWriter(s.pipeWriter, &s.countingWriter))
}

// Read reads the flate-compressed, JSON-encoded model entities written to the stream.
func (s *Stream) Read(buf []byte) (int, error) {
	return s.pipeReader.Read(buf)
}

// Flushed returns the number of bytes read from the stream.
func (s *Stream) Flushed() int64 {
	return s.countingWriter.n
}

// Close flushes any buffered data and closes the stream such that
// subsequent reads will return io.EOF.
func (s *Stream) Close() error {
	if err := s.flateWriter.Close(); err != nil {
		return err
	}
	return s.pipeWriter.Close()
}

// TODO(axw) should the methods take context? Or should we provide a
// method for closing the read side?

// WriteTransaction writes tx to the stream, returning the number of bytes written.
func (s *Stream) WriteTransaction(tx model.Transaction) error {
	s.jsonWriter.RawString(`{"transaction":`)
	tx.MarshalFastJSON(&s.jsonWriter)
	s.jsonWriter.RawByte('}')
	return s.write()
}

// WriteError writes e to the stream, returning the number of bytes written.
func (s *Stream) WriteError(e model.Error) error {
	s.jsonWriter.RawString(`{"error":`)
	e.MarshalFastJSON(&s.jsonWriter)
	s.jsonWriter.RawByte('}')
	return s.write()
}

// WriteMetrics writes m to the stream, returning the number of bytes written.
func (s *Stream) WriteMetrics(m model.Metrics) error {
	s.jsonWriter.RawString(`{"metrics":`)
	m.MarshalFastJSON(&s.jsonWriter)
	s.jsonWriter.RawByte('}')
	return s.write()
}

func (s *Stream) write() error {
	if _, err := s.flateWriter.Write(s.jsonWriter.Bytes()); err != nil {
		return err
	}
	s.jsonWriter.Reset()
	return nil
}

type countingWriter struct {
	n int64
}

func (cw *countingWriter) Write(data []byte) (n int, err error) {
	cw.n += int64(len(data))
	return len(data), nil
}
