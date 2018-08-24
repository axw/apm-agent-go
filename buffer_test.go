package elasticapm

import (
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-agent-go/model"
)

func TestBuffer(t *testing.T) {
	b := newBuffer(150)
	assert.Equal(t, 0, b.Len())
	assert.Equal(t, 150, b.Cap())

	for i := 0; i < 10; i++ {
		b.WriteTransaction(model.Transaction{})
		blen := b.Len()
		assert.NotEqual(t, 0, blen)
		assert.Equal(t, 150, b.Cap())

		const expect = `{"transaction":{"duration":0,"id":"00000000-0000-0000-0000-000000000000","name":"","timestamp":"0001-01-01T00:00:00Z","type":""}}`
		var bb bytes.Buffer
		n, err := b.WriteTo(&bb)
		assert.Equal(t, int64(blen-1), n) // delimeter excluded
		assert.Equal(t, expect, bb.String())
		assert.Equal(t, 0, b.Len())
		n, err = b.WriteTo(&bb)
		assert.Zero(t, n)
		assert.Equal(t, io.EOF, err)
	}
}

func TestBufferEviction(t *testing.T) {
	b := newBuffer(300)
	for i := 0; i < 100; i++ {
		b.WriteTransaction(model.Transaction{})
	}
}
