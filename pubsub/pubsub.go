package pubsub

import (
	"context"
	"log"
	"sync"
	"time"
)

type subId uint64
type pubId uint64

const timeout = 1 * time.Minute

type Agent[T any] struct {
	sync.RWMutex
	subId        subId
	pubId        pubId
	pubs         map[pubId]chan string
	subs         map[subId]chan T
	closeSubChan chan subId
	closePubChan chan pubId
	closeChan    <-chan struct{}
	timeout      time.Duration
}

// NewAgent Creates new PubSub Agent
// Optional quit channel, pass nil if not needed
func NewAgent[T any](quit <-chan struct{}) *Agent[T] {
	agent := &Agent[T]{
		pubs:         make(map[pubId]chan string),
		subs:         make(map[subId]chan T),
		closeSubChan: make(chan subId, 100),
		closePubChan: make(chan pubId, 100),
		timeout:      timeout,
		closeChan:    quit,
	}

	return agent
}

func (a *Agent[T]) ConnectedSubscribers() int {
	return len(a.subs)
}

// CloseConnections is a function used to close publisher and subscriber connections
// Meant to be run as a goroutine
func (a *Agent[T]) CloseConnections() {
	for {
		select {
		case id := <-a.closeSubChan:
			a.Lock()
			if sub, exists := a.subs[id]; exists {
				close(sub)
				delete(a.subs, id)
			}
			a.Unlock()
		case id := <-a.closePubChan:
			a.Lock()
			if pub, exists := a.pubs[id]; exists {
				close(pub)
				delete(a.pubs, id)
			}
			a.Unlock()
		case <-a.closeChan:
			return
		}
	}
}

// AddSubscriber Add new subscriber to the agent
// Optional quit channel, pass nil if not needed
// Return channel of a new subscriber
func (a *Agent[T]) AddSubscriber(bufferSize int, quit <-chan struct{}) <-chan T {
	a.Lock()
	defer a.Unlock()
	sub := make(chan T, bufferSize)
	id := a.subId
	a.subId++
	a.subs[id] = sub
	go func() {
		select {
		case <-quit:
		case <-a.closeChan:
		}

		a.Lock()
		defer a.Unlock()
		delete(a.subs, id)
		close(sub)
	}()
	return sub
}

// AddPublisher Add new publisher to the agent
// Optional quit channel, pass nil if not needed
// Return channel of a new publisher
func (a *Agent[T]) AddPublisher(bufferSize int, quit <-chan struct{}) <-chan string {
	a.Lock()
	defer a.Unlock()
	pub := make(chan string, bufferSize)
	id := a.pubId
	a.pubId++
	a.pubs[id] = pub
	go func() {
		select {
		case <-quit:
		case <-a.closeChan:
		}

		a.Lock()
		defer a.Unlock()
		delete(a.pubs, id)
		close(pub)
	}()
	return pub
}

// BroadcastEvent is used to send a message to subscribers
func (a *Agent[T]) BroadcastEvent(ctx context.Context, event T) {
	a.RLock()
	defer a.RUnlock()
	var wg sync.WaitGroup
	for id, subscriber := range a.subs {
		wg.Add(1)
		go func(listener chan T, w *sync.WaitGroup) {
			defer w.Done()
			select {
			case listener <- event:
			case <-time.After(a.timeout):
				a.closeSubChan <- id
				log.Printf("Connection %d has timed out\n", id)
			case <-ctx.Done():
			}
		}(subscriber, &wg)
	}
	wg.Wait()
}

// NotifyPublishers is used to notify all Publishers
func (a *Agent[T]) NotifyPublishers(message string) {
	a.RLock()
	defer a.RUnlock()
	var wg sync.WaitGroup
	for id, publisher := range a.pubs {
		wg.Add(1)
		go func(listener chan string, w *sync.WaitGroup) {
			defer w.Done()
			select {
			case listener <- message:
			case <-time.After(a.timeout):
				a.closePubChan <- id
				log.Printf("Connection %d timed out\n", id)
			}
		}(publisher, &wg)
	}
	wg.Wait()
}
