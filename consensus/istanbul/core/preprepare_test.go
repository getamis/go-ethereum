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
	"testing"

	"github.com/ethereum/go-ethereum/consensus/istanbul"
)

func newTestPreprepare(v *istanbul.View) *istanbul.Preprepare {
	return &istanbul.Preprepare{
		View:     v,
		Proposal: newTestProposal(),
	}
}

func TestHandlePreprepare(t *testing.T) {
	N := uint64(4) // replica 0 is primary, it will send messages to others
	F := uint64(1) // F does not affect tests

	testCases := []struct {
		system          *testSystem
		expectedRequest istanbul.Proposal
		expectedErr     error
	}{
		{
			// normal case
			func() *testSystem {
				sys := NewTestSystemWithBackend(N, F)

				for i, backend := range sys.backends {
					c := backend.engine.(*core)
					c.valSet = backend.peers
					if i != 0 {
						c.state = StateAcceptRequest
					}
				}
				return sys
			}(),
			newTestProposal(),
			nil,
		},
		{
			// future message
			func() *testSystem {
				sys := NewTestSystemWithBackend(N, F)

				for i, backend := range sys.backends {
					c := backend.engine.(*core)
					c.valSet = backend.peers
					if i != 0 {
						c.state = StateAcceptRequest
						// hack: force set subject that future message can be simulated
						c.current = newTestRoundState(
							&istanbul.View{
								Round:    big.NewInt(0),
								Sequence: big.NewInt(0),
							},
							c.valSet,
						)

					} else {
						c.current.SetSequence(big.NewInt(10))
					}
				}
				return sys
			}(),
			makeBlock(1),
			errFutureMessage,
		},
		{
			// non-proposer
			func() *testSystem {
				sys := NewTestSystemWithBackend(N, F)

				// force remove replica 0, let replica 1 become primary
				sys.backends = sys.backends[1:]

				for i, backend := range sys.backends {
					c := backend.engine.(*core)
					c.valSet = backend.peers
					if i != 0 {
						// replica 0 is primary
						c.state = StatePreprepared
					}
				}
				return sys
			}(),
			makeBlock(1),
			errNotFromProposer,
		},
		{
			// ErrInvalidMessage
			func() *testSystem {
				sys := NewTestSystemWithBackend(N, F)

				for i, backend := range sys.backends {
					c := backend.engine.(*core)
					c.valSet = backend.peers
					if i != 0 {
						c.state = StatePreprepared
						c.current.SetSequence(big.NewInt(10))
						c.current.SetRound(big.NewInt(10))
					}
				}
				return sys
			}(),
			makeBlock(1),
			errOldMessage,
		},
	}

OUTER:
	for _, test := range testCases {
		test.system.Run(false)

		v0 := test.system.backends[0]
		r0 := v0.engine.(*core)

		curView := r0.currentView()

		preprepare := &istanbul.Preprepare{
			View:     curView,
			Proposal: test.expectedRequest,
		}

		for i, v := range test.system.backends {
			// i == 0 is primary backend, it is responsible for send preprepare messages to others.
			if i == 0 {
				continue
			}

			c := v.engine.(*core)

			m, _ := Encode(preprepare)
			_, val := r0.valSet.GetByAddress(v0.Address())
			// run each backends and verify handlePreprepare function.
			if err := c.handlePreprepare(&message{
				Code:    msgPreprepare,
				Msg:     m,
				Address: v0.Address(),
			}, val); err != nil {
				if err != test.expectedErr {
					t.Errorf("unexpected error, expect:%v, but got:%v", test.expectedErr, err)
				}
				continue OUTER
			}

			if c.state != StatePreprepared {
				t.Error("state should be preprepared")
			}

			if !reflect.DeepEqual(c.current.Subject().View, curView) {
				t.Error("view should be the same")
			}

			// verify prepare messages
			decodedMsg := new(message)
			err := decodedMsg.FromPayload(v.sentMsgs[0], nil)
			if err != nil {
				t.Error("failed to parse")
			}

			if decodedMsg.Code != msgPrepare {
				t.Error("message code is not the same")
			}
			var subject *istanbul.Subject
			err = decodedMsg.Decode(&subject)
			if err != nil {
				t.Error("failed to decode Prepare")
			}
			if !reflect.DeepEqual(subject, c.current.Subject()) {
				t.Error("subject should be the same")
			}
		}
	}
}
