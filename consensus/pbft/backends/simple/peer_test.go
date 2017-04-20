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

package simple

import (
	"crypto/ecdsa"
	"encoding/hex"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
)

var (
	testHexPub  = "044b9465c7fc12206dcbcb21167c25971d5ffc60232133c7656bccfa3d868a832b94cc276bbf3c44cb81a35ed15c7c34fe8d9473edba76e6a06a13f306af3622eb"
	testHexPub2 = "0414d4fea68335ff920b699e3789ddf01731dac16278790bc5f39bf295f320499edb9763565970d3c835318949a0faaf2ac5c68e3f5219c06cd7edd07898c1a2fd"
)

func TestPeerSet(t *testing.T) {
	testAddPeer(t)
	testGetPeer(t)
	testRemovePeer(t)
}

func testAddPeer(t *testing.T) {
	peerSet := newPeerSet()
	pub, _ := hexToECDSAPubKey(testHexPub)
	peerID := "test-peer-id"
	peer := newPeer(peerID, pub)

	// test add valid peer
	peerSet.Add(peer)
	if size := len(peerSet.List()); size != 1 {
		t.Errorf("wrong peer set size, got: %v, expected: 1", size)

	}
	if p := peerSet.Get(peerID); p != peer {
		t.Errorf("get wrong peer, got:%v, expected: %v", p, peer)
	}

	// test add invalid peer
	peerSet.Add(nil)
	if size := len(peerSet.List()); size != 1 {
		t.Errorf("wrong peer set size, got: %v, expected: 1", size)

	}
}

func testGetPeer(t *testing.T) {
	peerSet := newPeerSet()
	pub, _ := hexToECDSAPubKey(testHexPub)
	peerID := "test-peer-id"
	peer := newPeer(peerID, pub)

	peerSet.Add(peer)
	// test get peer by invalid peer id
	if p := peerSet.Get("invalid-peer-id"); p != nil {
		t.Errorf("should not get any peer, got: %v, expected: nil", p)
	}
}

func testRemovePeer(t *testing.T) {
	peerSet := newPeerSet()
	pub, _ := hexToECDSAPubKey(testHexPub)
	peerID := "test-peer-id"
	peer := newPeer(peerID, pub)
	peerSet.Add(peer)

	pub2, _ := hexToECDSAPubKey(testHexPub)
	peerID2 := "test-peer-id2"
	peer2 := newPeer(peerID2, pub2)
	peerSet.Add(peer2)

	// test remove peer by invalid peer id
	peerSet.Remove("invalid-peer-id")
	if size := len(peerSet.List()); size != 2 {
		t.Errorf("wrong peer set size, got: %v, expected: 2", size)

	}

	peerSet.Remove(peerID)
	if size := len(peerSet.List()); size != 1 {
		t.Errorf("wrong peer set size, got: %v, expected: 1", size)

	}
	if p := peerSet.Get(peerID); p != nil {
		t.Errorf("should not get any peer, got: %v, expected: nil", p)

	}
	if p := peerSet.Get(peerID2); p != peer2 {
		t.Errorf("should not get any peer, got: %v, expected: %v", p, peer2)

	}
}

func hexToECDSAPubKey(hexPub string) (*ecdsa.PublicKey, error) {
	pub, err := hex.DecodeString(hexPub)
	if err != nil {
		return nil, err
	}
	return crypto.ToECDSAPub(pub), nil

}
