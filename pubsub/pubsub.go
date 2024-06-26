package pubsub

import (
	"context"
	"fmt"
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
	pubs         map[pubId]chan T
	subs         map[subId]chan T
	closeSubChan chan subId
	closePubChan chan pubId
	closeChan    chan bool
	timeout      time.Duration
}

func NewAgent[T any]() *Agent[T] {
	agent := &Agent[T]{
		pubs:         make(map[pubId]chan T),
		subs:         make(map[subId]chan T),
		closeSubChan: make(chan subId, 100),
		closePubChan: make(chan pubId, 100),
		timeout:      timeout,
		closeChan:    make(chan bool),
	}

	return agent
}

// Funcion to close publisher and subscriber connections
// Meant to be ran as a goroutine
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
		}
	}
}

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

func (a *Agent[T]) AddPublisher(bufferSize int, quit <-chan struct{}) <-chan T {
	a.Lock()
	defer a.Unlock()
	pub := make(chan T, bufferSize)
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

func (a *Agent[T]) BroadcastEvent(ctx context.Context, event T) {
	a.RLock()
	defer a.RUnlock()
	var wg sync.WaitGroup
	for _, subscriber := range a.subs {
		wg.Add(1)
		go func(listener chan T, w *sync.WaitGroup) {
			defer w.Done()
			select {
			case listener <- event:
			case <-time.After(a.timeout):
				fmt.Printf("Connection timed out\n")
			case <-ctx.Done():
			}
		}(subscriber, &wg)
	}
	wg.Wait()
}
