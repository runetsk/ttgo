package ai

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInflightRegistry(t *testing.T) {
	reg := newInflightRegistry()
	ctx, cancel := context.WithCancel(context.Background())
	reg.register("run-1", cancel)

	assert.True(t, reg.cancel("run-1"))
	assert.Error(t, ctx.Err(), "cancel fired the context")
	assert.False(t, reg.cancel("run-1"), "second cancel is a miss")
	assert.False(t, reg.cancel("unknown"))

	// unregister removes without firing.
	ctx2, cancel2 := context.WithCancel(context.Background())
	reg.register("run-2", cancel2)
	reg.unregister("run-2")
	assert.False(t, reg.cancel("run-2"))
	assert.NoError(t, ctx2.Err())
}
