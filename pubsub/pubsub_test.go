package pubsub

import (
	"context"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestSubscribe(t *testing.T) {
	agent := NewAgent[string](nil)
	sub := agent.AddSubscriber(0, nil)
	require.Equal(t, 1, len(agent.subs))
	require.NotNil(t, sub)
}

func TestBroadcast(t *testing.T) {
	agent := NewAgent[string](nil)
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

func TestClosingConnections(t *testing.T) {
	agentQuitChan := make(chan struct{})
	agent := NewAgent[string](agentQuitChan)
	subQuitChan := make(chan struct{})
	pubQuitChan := make(chan struct{})
	sub := agent.AddSubscriber(0, subQuitChan)
	pub := agent.AddPublisher(0, pubQuitChan)
	pub2 := agent.AddPublisher(0, nil)
	sub2 := agent.AddSubscriber(0, nil)

	subQuitChan <- struct{}{}
	pubQuitChan <- struct{}{}

	go agent.CloseConnections()

	require.Empty(t, sub)
	require.Empty(t, pub)
	require.NotNil(t, pub2)
	require.NotNil(t, sub2)

	agentQuitChan <- struct{}{}
	require.Empty(t, pub2)
	require.Empty(t, sub2)
}

func TestSubscriberNotification(t *testing.T) {
	agent := NewAgent[string](nil)
	pub := agent.AddPublisher(0, nil)
	agent.AddSubscriber(0, nil)
	done := make(chan bool)
	go func() {
		select {
		case msg := <-pub:
			require.NotNil(t, msg)
			require.Equal(t, "message", msg)
			done <- true
		}

	}()
	agent.NotifyPublishers("message")
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Test timed out waiting for event")
	}

	require.Equal(t, 1, agent.ConnectedSubscribers())
}

//TODO timeout test
