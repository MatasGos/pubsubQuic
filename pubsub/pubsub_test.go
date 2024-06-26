package pubsub

import (
	"context"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestSubscribe(t *testing.T) {
	agent := NewAgent[string]()
	sub := agent.AddSubscriber(0, nil)
	require.Equal(t, 1, len(agent.subs))
	require.NotNil(t, sub)
}

func TestBroadcast(t *testing.T) {
	agent := NewAgent[string]()
	sub := agent.AddSubscriber(0, nil)
	done := make(chan bool)
	go func() {
		select {
		case msg := <-sub:
			require.Equal(t, "foobar", msg)
			done <- true
		}

	}()
	ctx := context.Background()
	agent.BroadcastEvent(ctx, "foobar")
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Test timed out waiting for event")
	}
}
