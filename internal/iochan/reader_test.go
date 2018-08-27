package iochan

import (
	"io"
	"io/ioutil"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRead(t *testing.T) {
	target := 9999
	r := NewReader()
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		var bytes int
		for req := range r.C {
			for i := range req.Buf {
				req.Buf[i] = '*'
			}
			n := len(req.Buf)
			var err error
			if bytes+n >= target {
				if bytes+n > target {
					n = target - bytes
				}
				err = io.EOF
			}
			bytes += n
			req.Respond(n, err)
		}
	}()

	data, err := ioutil.ReadAll(r)
	assert.NoError(t, err)
	assert.NoError(t, r.Close())
	assert.Equal(t, strings.Repeat("*", target), string(data))
	wg.Wait()
}
