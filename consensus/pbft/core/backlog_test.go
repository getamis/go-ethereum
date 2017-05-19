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
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/pbft"
	"github.com/ethereum/go-ethereum/consensus/pbft/validator"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
	"gopkg.in/karalabe/cookiejar.v2/collections/prque"
)

func TestIsFutureMessage(t *testing.T) {
	c := &core{
		state:    StateAcceptRequest,
		sequence: big.NewInt(1),
		round:    big.NewInt(0),
	}

	// invalid view format
	r := c.isFutureMessage(msgPreprepare, nil)
	if r {
		t.Error("Should return false if nil view")
	}

	testStates := []State{StateAcceptRequest, StatePreprepared, StatePrepared, StateCommitted}
	testCode := []uint64{msgPreprepare, msgPrepare, msgCommit}

	// future sequence
	v := &pbft.View{
		Sequence: big.NewInt(2),
		Round:    big.NewInt(0),
	}
	for i := 0; i < len(testStates); i++ {
		c.state = testStates[i]
		for j := 0; j < len(testCode); j++ {
			r = c.isFutureMessage(testCode[j], v)
			if !r {
				t.Error("Should return true because it's a future sequence")
			}
		}
	}

	// future round
	v = &pbft.View{
		Sequence: big.NewInt(1),
		Round:    big.NewInt(1),
	}
	for i := 0; i < len(testStates); i++ {
		c.state = testStates[i]
		for j := 0; j < len(testCode); j++ {
			r = c.isFutureMessage(testCode[j], v)
			if !r {
				t.Error("Should return true because it's a future round")
			}
		}
	}

	v = c.currentView()
	// current view, state = StateAcceptRequest
	c.state = StateAcceptRequest
	for i := 0; i < len(testCode); i++ {
		r = c.isFutureMessage(testCode[i], v)
		if testCode[i] == msgPreprepare {
			if r {
				t.Error("Should return false because we can execute it now")
			}
		} else {
			if !r {
				t.Error("Should return true because it's a future round")
			}
		}
	}

	// current view, state = StatePreprepared
	c.state = StatePreprepared
	for i := 0; i < len(testCode); i++ {
		r = c.isFutureMessage(testCode[i], v)
		if testCode[i] <= msgPrepare {
			if r {
				t.Error("Should return false because we can execute it now")
			}
		} else {
			if !r {
				t.Error("Should return true because it's a future round")
			}
		}
	}

	// current view, state = StatePrepared
	c.state = StatePrepared
	for i := 0; i < len(testCode); i++ {
		r = c.isFutureMessage(testCode[i], v)
		if testCode[i] <= msgCommit {
			if r {
				t.Error("Should return false because we can execute it now")
			}
		} else {
			if !r {
				t.Error("Should return true because it's a future round")
			}
		}
	}

	// current view, state = StateCommitted
	c.state = StateCommitted
	for i := 0; i < len(testCode); i++ {
		r = c.isFutureMessage(testCode[i], v)
		if r {
			t.Error("Should return false because we can execute it now")
		}
	}

}

func TestStoreBacklog(t *testing.T) {
	c := &core{
		logger:     log.New("backend", "test", "id", 0),
		backlogs:   make(map[pbft.Validator]*prque.Prque),
		backlogsMu: new(sync.Mutex),
	}
	v := &pbft.View{
		Round:    big.NewInt(10),
		Sequence: big.NewInt(10),
	}
	p := validator.New(common.StringToAddress("12345667890"))
	// push preprepare msg
	preprepare := &pbft.Preprepare{
		View: v,
		Proposal: makeBlock(1),
	}
	prepreparePayload, _ := Encode(preprepare)
	m := &message{
		Code: msgPreprepare,
		Msg:  prepreparePayload,
	}
	c.storeBacklog(m, p)
	if !reflect.DeepEqual(c.backlogs[p].PopItem(), m) {
		t.Error("Should be equal")
	}

	// push prepare msg
	subject := &pbft.Subject{
		View:   v,
		Digest: common.StringToHash("1234567890"),
	}
	subjectPayload, _ := Encode(subject)

	m = &message{
		Code: msgPrepare,
		Msg:  subjectPayload,
	}
	c.storeBacklog(m, p)
	if !reflect.DeepEqual(c.backlogs[p].PopItem(), m) {
		t.Error("Should be equal")
	}

	// push commit msg
	m = &message{
		Code: msgCommit,
		Msg:  subjectPayload,
	}
	c.storeBacklog(m, p)
	if !reflect.DeepEqual(c.backlogs[p].PopItem(), m) {
		t.Error("Should be equal")
	}
}

func TestProcessFutureBacklog(t *testing.T) {
	backend := &testSystemBackend{
		events: new(event.TypeMux),
	}
	c := &core{
		logger:      log.New("backend", "test", "id", 0),
		backlogs:    make(map[pbft.Validator]*prque.Prque),
		backlogsMu:  new(sync.Mutex),
		backend:     backend,
		sequence:    big.NewInt(1),
		round:       big.NewInt(0),
		state:       StateAcceptRequest,
		internalMux: new(event.TypeMux),
	}
	c.subscribeEvents()
	defer c.unsubscribeEvents()

	v := &pbft.View{
		Round:    big.NewInt(10),
		Sequence: big.NewInt(10),
	}
	p := validator.New(common.StringToAddress("12345667890"))
	// push a future msg
	subject := &pbft.Subject{
		View:   v,
		Digest: common.StringToHash("1234567890"),
	}
	subjectPayload, _ := Encode(subject)
	m := &message{
		Code: msgCommit,
		Msg:  subjectPayload,
	}
	c.storeBacklog(m, p)
	c.processBacklog()

	const timeoutDura = 2 * time.Second
	timeout := time.NewTimer(timeoutDura)
	select {
	case <-c.internalEvents.Chan():
		t.Errorf("Should not receive any events")

	case <-timeout.C:
		// success
	}
}

func TestProcessBacklog(t *testing.T) {
	v := &pbft.View{
		Round:    big.NewInt(0),
		Sequence: big.NewInt(1),
	}
	preprepare := &pbft.Preprepare{
		View: v,
		Proposal: makeBlock(1),
	}
	prepreparePayload, _ := Encode(preprepare)

	subject := &pbft.Subject{
		View:   v,
		Digest: common.StringToHash("1234567890"),
	}
	subjectPayload, _ := Encode(subject)

	msgs := []*message{
		&message{
			Code: msgPreprepare,
			Msg:  prepreparePayload,
		},
		&message{
			Code: msgPrepare,
			Msg:  subjectPayload,
		},
		&message{
			Code: msgCommit,
			Msg:  subjectPayload,
		},
	}
	for i := 0; i < len(msgs); i++ {
		testProcessBacklog(t, msgs[i])
	}
}

func testProcessBacklog(t *testing.T, msg *message) {
	vset := newTestValidatorSet(1)
	backend := &testSystemBackend{
		events: new(event.TypeMux),
		peers:  vset,
	}
	c := &core{
		logger:      log.New("backend", "test", "id", 0),
		backlogs:    make(map[pbft.Validator]*prque.Prque),
		backlogsMu:  new(sync.Mutex),
		backend:     backend,
		internalMux: new(event.TypeMux),
		state:       State(msg.Code),
		sequence:    big.NewInt(1),
		round:       big.NewInt(0),
		subject: &pbft.Subject{
			View: &pbft.View{
				Sequence: big.NewInt(1),
				Round:    big.NewInt(0),
			},
		},
	}
	c.subscribeEvents()
	defer c.unsubscribeEvents()

	c.storeBacklog(msg, vset.GetByIndex(0))
	c.processBacklog()

	const timeoutDura = 2 * time.Second
	timeout := time.NewTimer(timeoutDura)
	select {
	case ev := <-c.internalEvents.Chan():
		e, ok := ev.Data.(backlogEvent)
		if !ok {
			t.Fatalf("Unexpected event comes")
		}
		if e.msg.Code != msg.Code {
			t.Fatalf("Unexpected message code, actual = %v, expected = %v", e.msg.Code, msg.Code)
		}
		// success
	case <-timeout.C:
		t.Error("Timeout. Cannot receive events as expected")
	}
}
