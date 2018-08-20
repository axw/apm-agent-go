package transport

import (
	"compress/gzip"
	"io"

	"github.com/elastic/apm-agent-go/internal/fastjson"
	"github.com/elastic/apm-agent-go/model"
)

// Stream is an io.Reader that returns JSON-encoded and gzip-compressed
// model entities (transactions, spans, etc.). Reads on a stream will
// block until entities are written to it, or the stream is closed.
// Writes to the stream will block until read.
//
// The Write methods are not safe for concurrent use.
type Stream struct {
	r *io.PipeReader
	w *io.PipeWriter

	jsonWriter     fastjson.Writer
	pipeReader     *io.PipeReader
	pipeWriter     *io.PipeWriter
	countingWriter countingWriter
	gzipWriter     *gzip.Writer
}

func NewStream() *Stream {
	pipeReader, pipeWriter := io.Pipe()
	s := &Stream{
		pipeReader: pipeReader,
		pipeWriter: pipeWriter,
	}
	s.gzipWriter = gzip.NewWriter(io.MultiWriter(s.pipeWriter, &s.countingWriter))
	return s
}

// Close closes the stream such that subsequent reads will return io.EOF.
func (s *Stream) Close() error {
	return s.pipeWriter.Close()
}

// TODO(axw) should the methods take context? Or should we provide a
// method for closing the read side?

// WriteTransaction writes tx to the stream, returning the number of bytes written.
func (s *Stream) WriteTransaction(tx model.Transaction) (int, error) {
	s.jsonWriter.RawString(`{"transaction":`)
	tx.MarshalFastJSON(&s.jsonWriter)
	s.jsonWriter.RawByte('}')
	return s.write()
}

// WriteError writes e to the stream, returning the number of bytes written.
func (s *Stream) WriteError(e model.Error) (int, error) {
	s.jsonWriter.RawString(`{"error":`)
	e.MarshalFastJSON(&s.jsonWriter)
	s.jsonWriter.RawByte('}')
	return s.write()
}

// WriteMetrics writes m to the stream, returning the number of bytes written.
func (s *Stream) WriteMetrics(m model.Metrics) (int, error) {
	s.jsonWriter.RawString(`{"metrics":`)
	m.MarshalFastJSON(&s.jsonWriter)
	s.jsonWriter.RawByte('}')
	return s.write()
}

func (s *Stream) write() (int, error) {
	if _, err := s.gzipWriter.Write(s.jsonWriter.Bytes()); err != nil {
		return -1, err
	}
	if err := s.gzipWriter.Flush(); err != nil {
		return -1, err
	}
	s.jsonWriter.Reset()
	n := s.countingWriter.n
	s.countingWriter.n = 0
	return n, nil
}

type countingWriter struct {
	n int
}

func (cw *countingWriter) Write(data []byte) (n int, err error) {
	cw.n += len(data)
	return len(data), nil
}
