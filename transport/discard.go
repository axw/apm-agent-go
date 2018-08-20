package transport

import (
	"context"
)

type discardStreamSender struct {
	err error
}

func (s discardStreamSender) SendStream(context.Context, *Stream) error {
	return s.err
}
