package transport_test

import (
	"bytes"
	"compress/zlib"
	"encoding/json"
	"io"
	"io/ioutil"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-agent-go/internal/fastjson"
	"github.com/elastic/apm-agent-go/model"
	"github.com/elastic/apm-agent-go/transport"
)

func TestStreamReadBlocks(t *testing.T) {
	s := transport.NewStream()
	done := make(chan error)
	go func() {
		_, err := ioutil.ReadAll(s)
		done <- err
	}()

	select {
	case <-done:
		t.Fatal("unexpected result from Read")
	case <-time.After(100 * time.Millisecond):
	}

	assert.NoError(t, s.Close())
	select {
	case err := <-done:
		assert.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for io.EOF")
	}
}

func TestStreamWriteRead(t *testing.T) {
	var tx model.Transaction
	s := transport.NewStream()
	written := make(chan error)
	go func() {
		err := s.WriteTransaction(tx)
		s.Close()
		written <- err
	}()

	var buf bytes.Buffer
	zlibReader, err := zlib.NewReader(s)
	assert.NoError(t, err)
	n, err := io.Copy(&buf, zlibReader)
	assert.NoError(t, err)
	assert.NotZero(t, n)

	writeErr := <-written
	assert.NoError(t, writeErr)

	var transactionPayload struct {
		Transaction json.RawMessage
	}
	d := json.NewDecoder(&buf)
	d.DisallowUnknownFields()
	err = d.Decode(&transactionPayload)
	assert.NoError(t, err)

	var fjw fastjson.Writer
	tx.MarshalFastJSON(&fjw)
	assert.Equal(t, string(fjw.Bytes()), string(transactionPayload.Transaction))
}

func TestStreamWriteBuffers(t *testing.T) {
	var tx model.Transaction
	s := transport.NewStream()

	headerReader, headerWriter := io.Pipe()
	go func() {
		var header [2]byte
		s.Read(header[:])
		headerWriter.Write(header[:])
		headerWriter.Close()
	}()

	assert.NoError(t, s.WriteTransaction(tx))
	assert.Equal(t, int64(2), s.Flushed()) // header gets written synchronously
	closed := make(chan error)
	go func() {
		closed <- s.Close()
	}()

	var buf bytes.Buffer
	zlibReader, err := zlib.NewReader(io.MultiReader(headerReader, s))
	assert.NoError(t, err)
	n, err := io.Copy(&buf, zlibReader)
	assert.NoError(t, err)
	assert.NotZero(t, n)
	assert.NoError(t, <-closed)

	var transactionPayload struct {
		Transaction json.RawMessage
	}
	d := json.NewDecoder(&buf)
	d.DisallowUnknownFields()
	err = d.Decode(&transactionPayload)
	assert.NoError(t, err)

	var fjw fastjson.Writer
	tx.MarshalFastJSON(&fjw)
	assert.Equal(t, string(fjw.Bytes()), string(transactionPayload.Transaction))
}

func TestStreamWriteFlushesAutomatically(t *testing.T) {
	var tx model.Transaction
	s := transport.NewStream()

	transactionsWritten := make(chan int)
	go func() {
		var n int
		for s.Flushed() <= 2 {
			assert.NoError(t, s.WriteTransaction(tx))
			n++
		}
		assert.NoError(t, s.Close())
		transactionsWritten <- n
	}()

	zlibReader, err := zlib.NewReader(s)
	assert.NoError(t, err)
	d := json.NewDecoder(zlibReader)
	d.DisallowUnknownFields()

	var transactionPayload struct {
		Transaction json.RawMessage
	}
	var transactionsRead int
	for {
		err := d.Decode(&transactionPayload)
		if err == io.EOF {
			break
		}
		assert.NoError(t, err)
		transactionsRead++
	}
	assert.NotZero(t, transactionsRead)
	assert.Equal(t, <-transactionsWritten, transactionsRead)
	t.Logf("wrote/read %d transactions", transactionsRead)
}
