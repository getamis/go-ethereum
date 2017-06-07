// Copyright 2017 AMIS Technologies
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

package core

import (
	"math/big"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/consensus/istanbul"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
	"gopkg.in/karalabe/cookiejar.v2/collections/prque"
)

func TestCheckRequestMsg(t *testing.T) {
	c := &core{
		state: StateAcceptRequest,
		current: newRoundState(&istanbul.View{
			Sequence: big.NewInt(1),
			Round:    big.NewInt(0),
		}, newTestValidatorSet(4)),
	}

	// invalid request
	err := c.checkRequestMsg(nil)
	if err != errInvalidMessage {
		t.Error("Should return errInvalidMessage if nil request")
	}
	r := &istanbul.Request{
		Proposal: nil,
	}
	err = c.checkRequestMsg(r)
	if err != errInvalidMessage {
		t.Error("Should return errInvalidMessage if nil proposal")
	}

	// old request
	r = &istanbul.Request{
		Proposal: makeBlock(0),
	}
	err = c.checkRequestMsg(r)
	if err != errOldMessage {
		t.Error("Should return errOldMessage if old request")
	}

	// future request
	r = &istanbul.Request{
		Proposal: makeBlock(2),
	}
	err = c.checkRequestMsg(r)
	if err != errFutureMessage {
		t.Error("Should return errFutureMessage if future request")
	}

	// current request
	r = &istanbul.Request{
		Proposal: makeBlock(1),
	}
	err = c.checkRequestMsg(r)
	if err != nil {
		t.Error("Should return nil if current request")
	}
}

func TestStoreRequestMsg(t *testing.T) {
	backend := &testSystemBackend{
		events: new(event.TypeMux),
	}
	c := &core{
		logger:  log.New("backend", "test", "id", 0),
		backend: backend,
		state:   StateAcceptRequest,
		current: newRoundState(&istanbul.View{
			Sequence: big.NewInt(0),
			Round:    big.NewInt(0),
		}, newTestValidatorSet(4)),
		pendingRequests:   prque.New(),
		pendingRequestsMu: new(sync.Mutex),
	}
	requests := []istanbul.Request{
		{
			Proposal: makeBlock(1),
		},
		{
			Proposal: makeBlock(2),
		},
		{
			Proposal: makeBlock(3),
		},
	}

	c.storeRequestMsg(&requests[1])
	c.storeRequestMsg(&requests[0])
	c.storeRequestMsg(&requests[2])
	if c.pendingRequests.Size() != len(requests) {
		t.Fatalf("Unexpected pending requests size, got: %v, expected: %v", c.pendingRequests.Size(), len(requests))
	}

	c.current.sequence = big.NewInt(3)

	c.subscribeEvents()
	defer c.unsubscribeEvents()

	c.processPendingRequests()

	const timeoutDura = 2 * time.Second
	timeout := time.NewTimer(timeoutDura)
	select {
	case ev := <-c.events.Chan():
		e, ok := ev.Data.(istanbul.RequestEvent)
		if !ok {
			t.Fatalf("Unexpected event comes")
		}
		if e.Proposal.Number().Cmp(requests[2].Proposal.Number()) != 0 {
			t.Fatalf("Unexpected request number, got: %v, expected: %v", e.Proposal.Number(), requests[2].Proposal.Number())
		}
	case <-timeout.C:
		t.Error("Timeout. Cannot receive events as expected")
	}
}
