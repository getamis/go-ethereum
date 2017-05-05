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
		sequence: big.NewInt(0)}
	r := c.isFutureMessage(pbft.MsgPreprepare, nil)
	if r {
		t.Error("Should return false if nil view")
	}
	v := &pbft.View{
		ViewNumber: big.NewInt(10),
		Sequence:   big.NewInt(10),
	}
	r = c.isFutureMessage(pbft.MsgPreprepare, v)
	if r {
		t.Error("Should return false if nil subject")
	}

	// for completed
	c.subject = &pbft.Subject{
		View: v,
	}
	c.sequence = big.NewInt(10)
	c.viewNumber = big.NewInt(10)
	c.state = StateAcceptRequest
	c.completed = true
	nextSeq := c.nextSequence()
	r = c.isFutureMessage(pbft.MsgPreprepare, nextSeq)
	if r {
		t.Error("Should false because we can execute it now")
	}
	r = c.isFutureMessage(pbft.MsgPrepare, nextSeq)
	if !r {
		t.Error("Should return true because it's a future sequence")
	}
	nextViewNumber := c.nextViewNumber()
	r = c.isFutureMessage(pbft.MsgPreprepare, nextViewNumber)
	if r {
		t.Error("Should false because we can execute it now")
	}
	r = c.isFutureMessage(pbft.MsgPrepare, nextViewNumber)
	if !r {
		t.Error("Should return true because it's a future number")
	}
	r = c.isFutureMessage(pbft.MsgCommit, v)
	if r {
		t.Error("Should return false because this round is completed")
	}

	// for non-completed
	c.completed = false
	r = c.isFutureMessage(pbft.MsgPreprepare, nextSeq)
	if !r {
		t.Error("Should true because of next sequence")
	}
	r = c.isFutureMessage(pbft.MsgPreprepare, nextViewNumber)
	if !r {
		t.Error("Should false because of next view")
	}
}

func TestStoreBacklog(t *testing.T) {
	c := &core{
		logger:     log.New("backend", "test", "id", 0),
		backlogs:   make(map[pbft.Validator]*prque.Prque),
		backlogsMu: new(sync.Mutex),
	}
	v := &pbft.View{
		ViewNumber: big.NewInt(10),
		Sequence:   big.NewInt(10),
	}
	p := validator.New(common.StringToAddress("12345667890"))
	// push preprepare msg
	preprepare := &pbft.Preprepare{
		View: v,
		Proposal: &pbft.Proposal{
			Header: &pbft.ProposalHeader{
				Sequence:   big.NewInt(10),
				ParentHash: common.HexToHash("0x1234567890"),
				DataHash:   common.HexToHash("0x9876543210"),
			},
			Payload:    []byte("payload"),
			Signatures: [][]byte{[]byte("sig1")},
		},
	}
	m := &pbft.Message{
		Code: pbft.MsgPreprepare,
		Msg:  preprepare,
	}
	c.storeBacklog(m, p)
	if !reflect.DeepEqual(c.backlogs[p].PopItem(), m) {
		t.Error("Should be equal")
	}

	// push prepare msg
	subject := &pbft.Subject{
		View:   v,
		Digest: []byte("digest"),
	}
	m = &pbft.Message{
		Code: pbft.MsgPrepare,
		Msg:  subject,
	}
	c.storeBacklog(m, p)
	if !reflect.DeepEqual(c.backlogs[p].PopItem(), m) {
		t.Error("Should be equal")
	}

	// push commit msg
	m = &pbft.Message{
		Code: pbft.MsgCommit,
		Msg:  subject,
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
		internalMux: new(event.TypeMux),
	}
	c.subscribeEvents()
	defer c.unsubscribeEvents()

	v := &pbft.View{
		ViewNumber: big.NewInt(10),
		Sequence:   big.NewInt(10),
	}
	p := validator.New(common.StringToAddress("12345667890"))
	// push a future msg
	subject := &pbft.Subject{
		View:   v,
		Digest: []byte("digest"),
	}
	m := &pbft.Message{
		Code: pbft.MsgCommit,
		Msg:  subject,
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
		ViewNumber: big.NewInt(0),
		Sequence:   big.NewInt(1),
	}
	preprepare := &pbft.Preprepare{
		View: v,
		Proposal: &pbft.Proposal{
			Header: &pbft.ProposalHeader{
				Sequence:   big.NewInt(10),
				ParentHash: common.HexToHash("0x1234567890"),
				DataHash:   common.HexToHash("0x9876543210"),
			},
			Payload:    []byte("payload"),
			Signatures: [][]byte{[]byte("sig1")},
		},
	}
	subject := &pbft.Subject{
		View:   v,
		Digest: []byte("digest"),
	}

	msgs := []*pbft.Message{
		&pbft.Message{
			Code: pbft.MsgPreprepare,
			Msg:  preprepare,
		},
		&pbft.Message{
			Code: pbft.MsgPrepare,
			Msg:  subject,
		},
		&pbft.Message{
			Code: pbft.MsgCommit,
			Msg:  subject,
		},
	}
	for i := 0; i < len(msgs); i++ {
		testProcessBacklog(t, msgs[i])
	}
}

func testProcessBacklog(t *testing.T, msg *pbft.Message) {
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
		subject: &pbft.Subject{
			View: &pbft.View{
				Sequence:   big.NewInt(1),
				ViewNumber: big.NewInt(0),
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
