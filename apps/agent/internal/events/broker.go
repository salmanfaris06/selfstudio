package events

import "sync"

type Broker struct {
	subscribers map[chan Event]struct{}
	mu          sync.RWMutex
}

func NewBroker() *Broker {
	return &Broker{subscribers: make(map[chan Event]struct{})}
}

func (b *Broker) Subscribe() (<-chan Event, func()) {
	ch := make(chan Event, 8)

	b.mu.Lock()
	b.subscribers[ch] = struct{}{}
	b.mu.Unlock()

	unsubscribe := func() {
		b.mu.Lock()
		if _, ok := b.subscribers[ch]; ok {
			delete(b.subscribers, ch)
			close(ch)
		}
		b.mu.Unlock()
	}

	return ch, unsubscribe
}

func (b *Broker) Publish(event Event) {
	if !IsDotNotation(event.EventType) {
		return
	}

	b.mu.RLock()
	defer b.mu.RUnlock()

	for ch := range b.subscribers {
		select {
		case ch <- event:
		default:
		}
	}
}

func (b *Broker) SubscriberCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return len(b.subscribers)
}
