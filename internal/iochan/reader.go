package iochan

type Reader struct {
	C    <-chan ReadRequest
	c    chan ReadRequest
	resp chan readResponse
}

func NewReader() *Reader {
	c := make(chan ReadRequest)
	return &Reader{
		C:    c,
		c:    c,
		resp: make(chan readResponse),
	}
}

// Close closes the reader such that r.C is closed.
func (r *Reader) Close() error {
	select {
	case <-r.c:
	default:
		close(r.c)
	}
	return nil
}

func (r *Reader) Read(buf []byte) (int, error) {
	r.c <- ReadRequest{Buf: buf, response: r.resp}
	resp := <-r.resp
	return resp.N, resp.Err
}

type ReadRequest struct {
	Buf      []byte
	response chan<- readResponse
}

func (rr *ReadRequest) Respond(n int, err error) {
	rr.response <- readResponse{N: n, Err: err}
}

type readResponse struct {
	N   int
	Err error
}
