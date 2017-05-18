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

	"github.com/ethereum/go-ethereum/consensus/pbft"
)

func newSnapshot(view *pbft.View, validatorSet pbft.ValidatorSet) *snapshot {
	return &snapshot{
		round:       view.Round,
		sequence:    view.Sequence,
		Preprepare:  nil,
		Prepares:    newMessageSet(validatorSet),
		Commits:     newMessageSet(validatorSet),
		Checkpoints: newMessageSet(validatorSet),
		mu:          new(sync.Mutex),
	}
}

type snapshot struct {
	round       *big.Int
	sequence    *big.Int
	Preprepare  *pbft.Preprepare
	Prepares    *messageSet
	Commits     *messageSet
	Checkpoints *messageSet

	mu *sync.Mutex
}

func (s *snapshot) SetRound(r *big.Int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.round = new(big.Int).Set(r)
}

func (s *snapshot) Round() *big.Int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.round
}

func (s *snapshot) SetSequence(seq *big.Int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.sequence = seq
}

func (s *snapshot) Sequence() *big.Int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.sequence
}
