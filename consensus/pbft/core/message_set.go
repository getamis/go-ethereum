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
	"io"
	"math/big"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/pbft"
	"github.com/ethereum/go-ethereum/consensus/pbft/validator"
	"github.com/ethereum/go-ethereum/rlp"
)

// Constructs a new messageSet to accumulate messages for given sequence/view number.
func newMessageSet(valSet pbft.ValidatorSet) *messageSet {
	return &messageSet{
		view: &pbft.View{
			ViewNumber: new(big.Int),
			Sequence:   new(big.Int),
		},
		messagesMu: new(sync.Mutex),
		messages:   make(map[common.Hash]*message),
		valSet:     valSet,
	}
}

// ----------------------------------------------------------------------------

type messageSet struct {
	view       *pbft.View
	valSet     pbft.ValidatorSet
	messagesMu *sync.Mutex
	messages   map[common.Hash]*message
}

type storageMessageSet struct {
	View       pbft.View
	Validators []common.Address
	Keys       []common.Hash
	Messages   [][]byte
}

func (ms *messageSet) View() *pbft.View {
	return ms.view
}

func (ms *messageSet) Add(msg *message) error {
	ms.messagesMu.Lock()
	defer ms.messagesMu.Unlock()

	if err := ms.verify(msg); err != nil {
		return err
	}

	return ms.addVerifiedMessage(msg)
}

func (ms *messageSet) Values() (result []*message) {
	ms.messagesMu.Lock()
	defer ms.messagesMu.Unlock()

	for _, v := range ms.messages {
		result = append(result, v)
	}

	return result
}

func (ms *messageSet) Size() int {
	ms.messagesMu.Lock()
	defer ms.messagesMu.Unlock()
	return len(ms.messages)
}

// The DecodeRLP method should read one value from the given
// Stream. It is not forbidden to read less or more, but it might
// be confusing.
func (ms *messageSet) DecodeRLP(s *rlp.Stream) error {
	var sms storageMessageSet
	err := s.Decode(&sms)
	if err == nil {
		ms.valSet = validator.NewSet(sms.Validators)
		ms.view = &sms.View
		ms.messagesMu = new(sync.Mutex)
		ms.messages = make(map[common.Hash]*message)

		if len(sms.Keys) != len(sms.Messages) {
			return errFailedDecodeMessageSet
		}

		for i, k := range sms.Keys {
			m := new(message)
			if err := m.FromPayload(sms.Messages[i], nil); err != nil {
				return err
			}

			ms.messages[k] = m
		}
	}
	return err
}

// EncodeRLP should write the RLP encoding of its receiver to w.
// If the implementation is a pointer method, it may also be
// called for nil pointers.
//
// Implementations should generate valid RLP. The data written is
// not verified at the moment, but a future version might. It is
// recommended to write only a single value but writing multiple
// values or no value at all is also permitted.
func (ms *messageSet) EncodeRLP(w io.Writer) error {
	ms.messagesMu.Lock()
	defer ms.messagesMu.Unlock()

	var addrs []common.Address
	for _, val := range ms.valSet.List() {
		addrs = append(addrs, val.Address())
	}

	var keys []common.Hash
	var msgs [][]byte
	for k, v := range ms.messages {
		keys = append(keys, k)
		b, err := v.Payload()
		if err != nil {
			return err
		}
		msgs = append(msgs, b)
	}

	return rlp.Encode(w, storageMessageSet{
		View:       *ms.view,
		Validators: addrs,
		Keys:       keys,
		Messages:   msgs,
	})
}

// ----------------------------------------------------------------------------

func (ms *messageSet) verify(msg *message) error {
	// verify if the message comes from one of the validators
	if v := ms.valSet.GetByAddress(msg.Address); v == nil {
		return pbft.ErrNoMatchingValidator
	}

	// TODO: check view number and sequence number

	return nil
}

func (ms *messageSet) addVerifiedMessage(msg *message) error {
	ms.messages[hash(msg)] = msg
	return nil
}
