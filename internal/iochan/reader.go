package iochan

import "sync"

// Reader is a channel-based io.Reader.
//
// Reader is safe for use in a single producer, single consumer pattern.
type Reader struct {
	// C can be used for receiving read requests.
	//
	// Once a read request is received, it must be responded
	// to, in order to avoid blocking the reader.
	C    <-chan ReadRequest
	c    chan ReadRequest
	resp chan readResponse

	mu     sync.Mutex
	closed bool
}

// NewReader returns a new Reader.
func NewReader() *Reader {
	c := make(chan ReadRequest)
	return &Reader{
		C:    c,
		c:    c,
		resp: make(chan readResponse),
	}
}

// Close closes the reader such that r.C is closed. Close is idempotent.
func (r *Reader) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.closed {
		close(r.c)
		r.closed = true
	}
	return nil
}

// Read sends a ReadRequest to r.C containing buf, and returns the
// response sent by the channel consumer via the read request's
// Response method.
func (r *Reader) Read(buf []byte) (int, error) {
	r.c <- ReadRequest{Buf: buf, response: r.resp}
	resp := <-r.resp
	return resp.N, resp.Err
}

// ReadRequest holds the buffer and response channel for a read request.
type ReadRequest struct {
	// Buf is the read buffer into which data should be read.
	Buf      []byte
	response chan<- readResponse
}

// Respond responds to the Read request.
func (rr *ReadRequest) Respond(n int, err error) {
	rr.response <- readResponse{N: n, Err: err}
}

type readResponse struct {
	N   int
	Err error
}
