package transport

import (
	"context"
)

// Transport provides an interface for sending streams of encoded model
// entities to the Elastic APM server. Methods are not required to be safe
// for concurrent use.
type Transport interface {
	// SendStream sends the stream to the server, returning when the
	// stream has been closed or the HTTP request terminates.
	SendStream(context.Context, *Stream) error
}
