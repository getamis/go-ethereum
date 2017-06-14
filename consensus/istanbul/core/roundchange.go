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

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/istanbul"
)

func (c *core) sendRoundChange() {
	logger := c.logger.New("state", c.state)
	logger.Trace("sendRoundChange")

	cv := c.currentView()
	c.catchUpRound(&istanbul.View{
		// The round number we'd like to transfer to.
		Round:    new(big.Int).Add(cv.Round, common.Big1),
		Sequence: new(big.Int).Set(cv.Sequence),
	})

	// Now we have the new round number and sequence number
	cv = c.currentView()
	rc := &roundChange{
		Round:    new(big.Int).Set(cv.Round),
		Sequence: new(big.Int).Set(cv.Sequence),
		Digest:   common.Hash{},
	}

	payload, err := Encode(rc)
	if err != nil {
		logger.Error("Failed to encode round change", "rc", rc, "err", err)
		return
	}

	c.broadcast(&message{
		Code: msgRoundChange,
		Msg:  payload,
	})
}

func (c *core) handleRoundChange(msg *message, src istanbul.Validator) error {
	logger := c.logger.New("state", c.state)
	logger.Trace("handleRoundChange")

	// Decode round change message
	var rc *roundChange
	if err := msg.Decode(&rc); err != nil {
		logger.Error("Failed to decode round change", "err", err)
		return errInvalidMessage
	}

	cv := c.currentView()

	// We never accept round change message with different sequence number
	if rc.Sequence.Cmp(cv.Sequence) != 0 {
		logger.Warn("Inconsistent sequence number", "expected", cv.Sequence, "got", rc.Sequence)
		return errInvalidMessage
	}

	// We never accept round change message with smaller round number
	if rc.Round.Cmp(cv.Round) < 0 {
		logger.Warn("Old round change", "from", src, "expected", cv.Round, "got", rc.Round)
		return errOldMessage
	}

	// Add the round change message to its message set and return how many
	// messages we've got with the same round number and sequence number.
	num, err := c.roundChangeSet.Add(&istanbul.View{
		Round:    new(big.Int).Set(rc.Round),
		Sequence: new(big.Int).Set(rc.Sequence),
	}, msg)
	if err != nil {
		logger.Warn("Failed to add round change message", "from", src, "msg", msg, "err", err)
		return err
	}

	// Once we received f+1 round change messages, those messages form a weak certificate.
	// If our round number is smaller than the certificate's round number, we would
	// try to catch up the round number.
	if num == int(c.valSet.F()+1) {
		if cv.Round.Cmp(rc.Round) < 0 {
			c.catchUpRound(&istanbul.View{
				Round:    new(big.Int).Sub(rc.Round, common.Big1),
				Sequence: new(big.Int).Set(rc.Sequence),
			})
			c.sendRoundChange()
		}
	}

	// We've received 2f+1 round change messages, start a new round immediately.
	if num == int(2*c.valSet.F()+1) {
		c.startNewRound(&istanbul.View{
			Round:    new(big.Int).Set(rc.Round),
			Sequence: new(big.Int).Set(rc.Sequence),
		}, true)
	}

	return nil
}

// ----------------------------------------------------------------------------

func newRoundChangeSet(valSet istanbul.ValidatorSet) *roundChangeSet {
	return &roundChangeSet{
		validatorSet: valSet,
		roundChanges: make(map[common.Hash]*messageSet),
		mu:           new(sync.Mutex),
	}
}

type roundChangeSet struct {
	validatorSet istanbul.ValidatorSet
	roundChanges map[common.Hash]*messageSet
	mu           *sync.Mutex
}

func (rcs *roundChangeSet) Add(v *istanbul.View, msg *message) (int, error) {
	rcs.mu.Lock()
	defer rcs.mu.Unlock()

	h := istanbul.RLPHash(v)
	if rcs.roundChanges[h] == nil {
		rcs.roundChanges[h] = newMessageSet(rcs.validatorSet)
	}
	err := rcs.roundChanges[h].Add(msg)
	if err != nil {
		return 0, err
	}
	return rcs.roundChanges[h].Size(), nil
}

func (rcs *roundChangeSet) Clear(v *istanbul.View) {
	rcs.mu.Lock()
	defer rcs.mu.Unlock()

	for k, rms := range rcs.roundChanges {
		if len(rms.Values()) == 0 {
			delete(rcs.roundChanges, k)
		}

		var rc *roundChange
		if err := rms.Values()[0].Decode(&rc); err != nil {
			continue
		}

		if rc.Sequence.Cmp(v.Sequence) < 0 ||
			(rc.Sequence.Cmp(v.Sequence) == 0 && rc.Round.Cmp(v.Round) < 0) {
			delete(rcs.roundChanges, k)
		}
	}
}
