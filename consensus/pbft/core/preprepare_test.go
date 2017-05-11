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

	"github.com/ethereum/go-ethereum/consensus/pbft"
)

func TestHandlePreprepare(t *testing.T) {
	N := uint64(4) // replica 0 is primary, it will send messages to others
	F := uint64(1) // F does not affect tests

	testCases := []struct {
		system          *testSystem
		expectedRequest pbft.BlockContexter

		expectedErr error
	}{
		{
			// normal case
			func() *testSystem {
				sys := NewTestSystemWithBackend(N, F)

				for i, backend := range sys.backends {
					c := backend.engine.(*core)

					if i != 0 {
						c.state = StateAcceptRequest
					}
				}
				return sys
			}(),
			pbft.NewBlockContext([]byte("normal case"), big.NewInt(1)),
			nil,
		},
		{
			// future message
			func() *testSystem {
				sys := NewTestSystemWithBackend(N, F)

				for i, backend := range sys.backends {
					c := backend.engine.(*core)

					if i != 0 {
						c.state = StateAcceptRequest
						// hack: force set subject that future message can be simulated
						c.subject = &pbft.Subject{
							View: &pbft.View{
								Sequence:   big.NewInt(0),
								ViewNumber: big.NewInt(0),
							},
							Digest: []byte{1},
						}
					} else {
						c.sequence = big.NewInt(10)
					}
				}
				return sys
			}(),
			pbft.NewBlockContext([]byte("future message"), big.NewInt(1)),
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

					if i != 0 {
						// replica 0 is primary
						c.state = StatePreprepared
					}
				}
				return sys
			}(),
			pbft.NewBlockContext([]byte("not from proposer"), big.NewInt(1)),
			pbft.ErrNotFromProposer,
		},
		{
			// ErrInvalidMessage
			func() *testSystem {
				sys := NewTestSystemWithBackend(N, F)

				for i, backend := range sys.backends {
					c := backend.engine.(*core)

					if i != 0 {
						c.state = StatePreprepared
						c.sequence = big.NewInt(10)
						c.viewNumber = big.NewInt(10)
					}
				}
				return sys
			}(),
			pbft.NewBlockContext([]byte("invalid message"), big.NewInt(1)),
			pbft.ErrInvalidMessage,
		},
		{
			// proposal is not included
			// notice: force set the Preprepare.Proposal value to nil when test is started
			func() *testSystem {
				sys := NewTestSystemWithBackend(N, F)

				for i, backend := range sys.backends {
					c := backend.engine.(*core)

					if i == 0 {
						// replica 0 is primary
						c.state = StatePreprepared
					}
				}
				return sys
			}(),
			pbft.NewBlockContext([]byte("nil proposal"), big.NewInt(1)),
			pbft.ErrNilProposal,
		},
	}

OUTER:
	for _, test := range testCases {
		test.system.Run(false)

		v0 := test.system.backends[0]
		r0 := v0.engine.(*core)

		nextSeqView := r0.nextSequence()

		preprepare := &pbft.Preprepare{
			View:     nextSeqView,
			Proposal: r0.makeProposal(nextSeqView.Sequence, &pbft.Request{BlockContext: test.expectedRequest}),
		}

		for i, v := range test.system.backends {
			// i == 0 is primary backend, it is responsible for send preprepare messages to others.
			if i == 0 {
				continue
			}

			c := v.engine.(*core)

			// for case: proposal is not included, hack the variable to nil
			// FIXME: nil variable is not supported by rlp Encode/Decode
			//if test.expectedErr == pbft.ErrNilProposal {
			//	preprepare.Proposal = nil
			//}

			m, _ := Encode(preprepare)
			// run each backends and verify handlePreprepare function.
			if err := c.handlePreprepare(&message{
				Code:    msgPreprepare,
				Msg:     m,
				Address: v0.Address(),
			}, v0.Validators().GetByAddress(v0.Address())); err != nil {
				if err != test.expectedErr {
					t.Error("unexpected error: ", err)
				}
				continue OUTER
			}

			if c.state != StatePreprepared {
				t.Error("state should be preprepared")
			}

			if !reflect.DeepEqual(c.subject.View, nextSeqView) {
				t.Error("view should be the same")
			}

			if c.completed {
				t.Error("should not complete")
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
			var subject *pbft.Subject
			err = decodedMsg.Decode(&subject)
			if err != nil {
				t.Error("failed to decode Prepare")
			}
			if !reflect.DeepEqual(subject, c.subject) {
				t.Error("subject should be the same")
			}
		}
	}
}
