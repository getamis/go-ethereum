// Copyright 2017 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package istanbul

import (
	"errors"
	"sync"
)

const (
	eventQueueSize = 1000
)

var (
	ErrSubscribed = errors.New("event: queue has subscribed")
)

// EventQueue is message buffer queue between backend and core engine.
type EventQueue struct {
	mutex sync.Mutex

	sub *EventQueueSubscription
}

// Subscribe is called when core engine is starting.
// Return error if the event queue has subscribed already.
func (q *EventQueue) Subscribe() (*EventQueueSubscription, error) {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	if q.sub != nil {
		return nil, ErrSubscribed
	}
	sub := &EventQueueSubscription{
		eventQueue: q,
		queue:      make(chan interface{}, eventQueueSize),
	}
	q.sub = sub
	return sub, nil
}

// Post sends an event to core engine. It doesn't send any events
// if no one subscribes to the event queue.
func (q *EventQueue) Post(data interface{}) {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	if q.sub != nil {
		q.sub.deliver(data)
	}
}

func (q *EventQueue) delSubscription() {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	q.sub = nil
}

type EventQueueSubscription struct {
	mutex sync.Mutex

	eventQueue *EventQueue
	queue      chan interface{}
}

// Unsubscribe is called when core engine is closing.
func (s *EventQueueSubscription) Unsubscribe() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if s.queue != nil {
		s.eventQueue.delSubscription()
		close(s.queue)
		s.queue = nil
	}
}

func (s *EventQueueSubscription) Chan() <-chan interface{} {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.queue
}

func (s *EventQueueSubscription) deliver(data interface{}) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if s.queue != nil {
		s.queue <- data
	}
}
