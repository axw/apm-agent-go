package transport

import (
	"context"
)

type discardTransport struct {
	err error
}

func (s discardTransport) SendStream(context.Context, *Stream) error {
	return s.err
}
