package transporttest

import (
	"context"
	"io"
)

// ChannelTransport implements transport.Transport,
// sending payloads to the provided channels as
// request objects. Once a request object has been
// received, an error should be sent to its Result
// channel to unblock the tracer.
type ChannelTransport struct {
	Streams chan<- SendStreamRequest
}

// SendStreamRequest is the type of values sent over the
// ChannelTransport.Streams channel when its SendStream
// method is called.
type SendStreamRequest struct {
	Stream io.Reader
	Result chan<- error
}

// SendTransactions sends a SendStreamRequest value over the
// c.Streams channel with the given payload, and waits for a
// response on the error channel included in the request, or
// for the context to be canceled.
func (c *ChannelTransport) SendStream(ctx context.Context, r io.Reader) error {
	result := make(chan error, 1)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case c.Streams <- SendStreamRequest{r, result}:
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-result:
			return err
		}
	}
}
