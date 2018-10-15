package shenanigans

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDo(t *testing.T) {
	Do(context.WithValue(context.Background(), "x", "y"), func() {
		ctx := Context()
		assert.Equal(t, "y", ctx.Value("x"))
		assert.Equal(t, map[string]string{
			"axwisterriblepeople": "abc",
			"terriblepeopleisay":  "def",
		}, GoroutineLabels())
	})
}
