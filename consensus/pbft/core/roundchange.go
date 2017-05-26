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
	"github.com/ethereum/go-ethereum/consensus/pbft"
	"github.com/ethereum/go-ethereum/rlp"
)

func (c *core) sendRoundChange() {
	logger := c.logger.New("state", c.state)

	cv := c.currentView()
	c.catchUpRound(&pbft.View{
		// The round number we'd like to transfer to
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

	payload, err := rlp.EncodeToBytes(rc)
	if err != nil {
		logger.Error("Failed to encode RoundChange", "rc", rc)
		return
	}

	c.broadcast(&message{
		Code: msgRoundChange,
		Msg:  payload,
	})

	logger.Debug("sendRoundChange")
}

func (c *core) handleRoundChange(msg *message, src pbft.Validator) error {
	logger := c.logger.New("state", c.state)
	logger.Debug("handleRoundChange")

	var rc *roundChange
	if err := msg.Decode(&rc); err != nil {
		logger.Error("Failed to decode RoundChange")
		return pbft.ErrInvalidMessage
	}

	cv := c.current

	if rc.Sequence.Cmp(cv.Sequence()) != 0 {
		logger.Warn("Wrong sequence number", "expected", cv.Sequence(), "got", rc.Sequence)
		return pbft.ErrInvalidMessage
	}

	if rc.Round.Cmp(cv.Round()) < 0 {
		logger.Warn("Old RoundChange", "from", src.Address().Hex(), "expected", cv.Round().Uint64(), "got", rc.Round.Uint64())
		return pbft.ErrOldMessage
	}

	num, err := c.roundChangeSet.Add(&pbft.View{
		Round:    new(big.Int).Set(rc.Round),
		Sequence: new(big.Int).Set(rc.Sequence),
	}, msg)
	if err != nil {
		logger.Warn("Failed to add RoundChange", "from", src.Address().Hex(), "msg", msg)
		return err
	}

	// If we've received f+1 RoundChange messages
	// catch up the round number
	if num == int(c.F+1) {
		if cv.Round().Cmp(rc.Round) < 0 {
			c.catchUpRound(&pbft.View{
				Round:    new(big.Int).Sub(rc.Round, common.Big1),
				Sequence: rc.Sequence,
			})
			c.sendRoundChange()
		}
	}

	// We've received 2f+1 RoundChange messages
	// enter new view
	if num == int(2*c.F+1) {
		c.startNewRound(&pbft.View{
			Round:    new(big.Int).Set(rc.Round),
			Sequence: new(big.Int).Set(rc.Sequence),
		}, true)
	}

	return nil
}

// ----------------------------------------------------------------------------

func newRoundChangeSet(valSet pbft.ValidatorSet) *roundChangeSet {
	return &roundChangeSet{
		validatorSet: valSet,
		roundChanges: make(map[common.Hash]*messageSet),
		mu:           new(sync.Mutex),
	}
}

type roundChangeSet struct {
	validatorSet pbft.ValidatorSet
	roundChanges map[common.Hash]*messageSet
	mu           *sync.Mutex
}

func (rcs *roundChangeSet) Add(v *pbft.View, msg *message) (int, error) {
	rcs.mu.Lock()
	defer rcs.mu.Unlock()

	h := hash(v)
	if rcs.roundChanges[h] == nil {
		rcs.roundChanges[h] = newMessageSet(rcs.validatorSet)
	}
	err := rcs.roundChanges[h].Add(msg)
	if err != nil {
		return 0, err
	}
	return rcs.roundChanges[h].Size(), nil
}

func (rcs *roundChangeSet) Clear(v *pbft.View) {
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
