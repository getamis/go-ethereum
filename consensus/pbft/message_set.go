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

package pbft

import (
	"errors"
	"fmt"
	"math/big"
	"reflect"

	set "gopkg.in/fatih/set.v0"
)

type MessageSet interface {
	Sequence() *big.Int
	ViewNumber() *big.Int
	Type() reflect.Type
	Add(interface{}, *Validator) (bool, error)
	Size() int
}

// TODO: pass validator set here
// Constructs a new MessageSet used to accumulate messages for given sequence/view number.
func NewMessageSet(view *View, msgType reflect.Type) MessageSet {
	return &messageSet{
		view:     view,
		msgType:  msgType,
		messages: set.New(),
	}
}

// ----------------------------------------------------------------------------

type messageSet struct {
	view     *View
	msgType  reflect.Type
	messages *set.Set
}

type validatorMessage struct {
	val     *Validator
	message interface{}
}

func (ms *messageSet) Sequence() *big.Int {
	if ms == nil || ms.view == nil {
		return new(big.Int)
	}
	return ms.view.Sequence
}

func (ms *messageSet) ViewNumber() *big.Int {
	if ms == nil || ms.view == nil {
		return big.NewInt(-1)
	}
	return ms.view.ViewNumber
}

func (ms *messageSet) Type() reflect.Type {
	if ms == nil || ms.msgType == nil {
		return reflect.TypeOf(nil)
	}
	return ms.msgType
}

func (ms *messageSet) Add(msg interface{}, validator *Validator) (bool, error) {
	m := validatorMessage{
		val:     validator,
		message: msg,
	}

	// we already have this message
	if ms.messages.Has(m) {
		return false, nil
	}

	if reflect.TypeOf(msg) != ms.msgType {
		return false, fmt.Errorf("%v is not supported by this message set", reflect.TypeOf(msg))
	}

	// TODO: verify signatures, validators, etc.

	return ms.addVerifiedMessage(m)
}

func (ms *messageSet) addVerifiedMessage(m validatorMessage) (bool, error) {
	ms.messages.Add(m)

	if !ms.messages.Has(m) {
		return false, errors.New("unknown error")
	}
	return true, nil
}

func (ms *messageSet) Size() int {
	return len(ms.messages.List())
}
