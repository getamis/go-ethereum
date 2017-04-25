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
	"fmt"
	"reflect"
	"testing"
	"time"
)

func connectAll(bs []*testSystemBackend) {
	for i := uint64(0); i < uint64(len(bs)); i++ {
		x := bs[i]

		for j := uint64(0); j < uint64(len(bs)); j++ {
			y := bs[j]

			if x == y {
				continue
			}
			x.sys.backends[i].AddPeer(fmt.Sprintf("%d", y.sys.backends[j].ID()), &y.sys.backends[j].privateKey.PublicKey)
		}
	}
}

func TestCore_NewRequest(t *testing.T) {
	N := uint64(4)

	var backends []*testSystemBackend
	sys := newTestSystem()

	for i := uint64(0); i < N; i++ {
		backend := sys.NewBackend(uint64(i))
		backend.engine = New(backend)

		backends = append(backends, backend)
	}

	for _, backend := range sys.backends {
		backend.Start(nil)
		backend.engine.Start() // start PBFT core
	}
	connectAll(backends)

	sys.run()
	defer sys.stop()

	request1 := []byte("request 1")
	backends[0].NewRequest(request1)

	select {
	case <-time.After(1 * time.Second):
	}

	request2 := []byte("request 2")
	backends[0].NewRequest(request2)

	select {
	case <-time.After(1 * time.Second):
	}

	for _, backend := range backends {
		if len(backend.commitMsgs) != 2 {
			t.Error("expected execution of requests should be 2, id:", backend.ID())
		}
		if !reflect.DeepEqual(request1, backend.commitMsgs[0].Payload) {
			t.Error("payload is not the same (1)")
		}
		if !reflect.DeepEqual(request2, backend.commitMsgs[1].Payload) {
			t.Error("payload is not the same (2)")
		}
	}
}
