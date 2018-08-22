package elasticapm

import (
	"testing"

	"github.com/elastic/apm-agent-go/model"
	"github.com/stretchr/testify/assert"
)

func TestBuffer(t *testing.T) {
	b := newBuffer(150)
	assert.Equal(t, 0, b.Len())
	assert.Equal(t, 150, b.Cap())

	for i := 0; i < 10; i++ {
		b.WriteTransaction(model.Transaction{})
		assert.NotEqual(t, 0, b.Len())
		assert.Equal(t, 150, b.Cap())

		const expect = `{"transaction":{"duration":0,"id":"00000000-0000-0000-0000-000000000000","name":"","timestamp":"0001-01-01T00:00:00Z","type":""}}`
		assert.Equal(t, expect, string(b.Pop()))
		assert.Equal(t, 0, b.Len())
		assert.Empty(t, b.Pop())
	}
}
