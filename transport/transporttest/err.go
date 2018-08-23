package transporttest

import (
	"context"
	"io"

	"github.com/elastic/apm-agent-go/transport"
)

// Discard is a transport.Transport which discards
// all streams, and returns no errors.
var Discard transport.Transport = ErrorTransport{}

// ErrorTransport is a transport that returns the stored error
// for each method call.
type ErrorTransport struct {
	Error error
}

// SendTransactions discards the stream and returns t.Error.
func (t ErrorTransport) SendStream(_ context.Context, r io.Reader) error {
	if t.Error != nil {
		return t.Error
	}
	//io.Copy(ioutil.Discard, s)
	return nil
}
