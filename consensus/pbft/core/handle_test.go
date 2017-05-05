package core

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/consensus/pbft"
)

// notice: the normal case have been tested in integration tests.
func TestHandleMsg(t *testing.T) {
	N := uint64(4)
	F := uint64(1)
	sys := NewTestSystemWithBackend(N, F)

	closer := sys.Run(true, true)
	defer closer()

	v0 := sys.backends[0]
	r0 := v0.engine.(*core)

	// with a unmatched payload. pbft.MsgPreprepare should match with *pbft.Preprepare in normal case.
	msg, _ := pbft.Encode(pbft.MsgPreprepare, &pbft.Subject{
		View: &pbft.View{
			Sequence:   big.NewInt(0),
			ViewNumber: big.NewInt(0),
		},
		Digest: []byte{1},
	})

	if err := r0.handle(msg, v0.Validators().GetByAddress(v0.Address())); err != errFailedDecodePreprepare {
		t.Error("message should decode failed")
	}

	// with a unmatched payload. pbft.MsgPrepare should match with *pbft.Subject in normal case.
	msg, _ = pbft.Encode(pbft.MsgPrepare, &pbft.Preprepare{
		View: &pbft.View{
			Sequence:   big.NewInt(0),
			ViewNumber: big.NewInt(0),
		},
		Proposal: nil,
	})

	if err := r0.handle(msg, v0.Validators().GetByAddress(v0.Address())); err != errFailedDecodePrepare {
		t.Error("message should decode failed")
	}

	// with a unmatched payload. pbft.MsgCommit should match with *pbft.Subject in normal case.
	msg, _ = pbft.Encode(pbft.MsgCommit, &pbft.Preprepare{
		View: &pbft.View{
			Sequence:   big.NewInt(0),
			ViewNumber: big.NewInt(0),
		},
		Proposal: nil,
	})

	if err := r0.handle(msg, v0.Validators().GetByAddress(v0.Address())); err != errFailedDecodeCommit {
		t.Error("message should decode failed")
	}

	// invalid message code. message code is not exists in list
	msg, _ = pbft.Encode(uint64(99), &pbft.Preprepare{
		View: &pbft.View{
			Sequence:   big.NewInt(0),
			ViewNumber: big.NewInt(0),
		},
		Proposal: nil,
	})

	if err := r0.handle(msg, v0.Validators().GetByAddress(v0.Address())); err != nil {
		t.Error("should not return failed message, but:", err)
	}

	// with malicious payload
	if err := r0.handleMsg([]byte{1}, v0.Validators().GetByAddress(v0.Address())); err == nil {
		t.Error("message should decode failed..., but:", err)
	}
}
