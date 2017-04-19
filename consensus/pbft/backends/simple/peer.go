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
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/pbft"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/p2p"
)

func newPeer(publicKey string, id uint64) pbft.Peer {
	return &peer{
		id:        id,
		publicKey: publicKey,
		address:   publicKey2Addr(publicKey),
	}
}

type peer struct {
	id        uint64
	publicKey string
	address   common.Address
}

func (p *peer) ID() uint64 {
	return p.id
}

func (p *peer) Address() common.Address {
	return p.address
}

func (p *peer) PublicKey() string {
	return p.publicKey
}

func (p *peer) SetPublicKey(pubKey string) {
	p.publicKey = pubKey
}

func (p *peer) IsConnected() bool {
	return p.publicKey != ""
}

func (p *peer) ReadMsg() (p2p.Msg, error) {
	return p2p.Msg{}, nil
}

func (p *peer) WriteMsg(msg p2p.Msg) error {
	return nil
}

func publicKey2Addr(pubKey string) common.Address {
	var address common.Address
	copy(address[:], crypto.Keccak256([]byte(pubKey)[1:])[12:])
	return address
}
