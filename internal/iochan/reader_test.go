package iochan

import (
	"fmt"
	"testing"
)

func TestRead(t *testing.T) {
	r := NewReader()
	go func() {
		for req := range r.C {
			fmt.Println(req.Buf)
			req.Respond(len(req.Buf), nil)
		}
	}()
	fmt.Println(r.Read(nil))
	fmt.Println(r.Read(make([]byte, 2)))
	r.Close()
}
