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
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	elog "github.com/ethereum/go-ethereum/log"
)

func makeBlock(number int64) *types.Block {
	header := &types.Header{
		Difficulty: big.NewInt(0),
		Number:     big.NewInt(number),
		GasLimit:   big.NewInt(0),
		GasUsed:    big.NewInt(0),
		Time:       big.NewInt(0),
	}
	block := &types.Block{}
	return block.WithSeal(header)
}

func TestNewRequest(t *testing.T) {
	testLogger.SetHandler(elog.StdoutHandler)

	N := uint64(4)
	F := uint64(1)

	sys := NewTestSystemWithBackend(N, F)

	close := sys.Run(true)
	defer close()

	request1 := makeBlock(1)
	sys.backends[0].NewRequest(request1)

	select {
	case <-time.After(1 * time.Second):
	}

	request2 := makeBlock(2)
	sys.backends[0].NewRequest(request2)

	select {
	case <-time.After(1 * time.Second):
	}

	for _, backend := range sys.backends {
		if len(backend.commitMsgs) != 2 {
			t.Error("expected execution of requests should be 2, but got:", len(backend.commitMsgs))
		}
		if !reflect.DeepEqual(request1.Number(), backend.commitMsgs[0].Number()) {
			t.Error("payload is not the same (1)")
		}
		if !reflect.DeepEqual(request2.Number(), backend.commitMsgs[1].Number()) {
			t.Error("payload is not the same (2)")
		}
	}
}
