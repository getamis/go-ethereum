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

package simulation

import (
	"crypto/ecdsa"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

func NewNodeKey() *NodeKey {
	key, _ := crypto.GenerateKey()
	return &NodeKey{
		key: key,
	}
}

type NodeKey struct {
	key *ecdsa.PrivateKey
}

func (p *NodeKey) Address() common.Address {
	return crypto.PubkeyToAddress(p.key.PublicKey)
}

func (p *NodeKey) PublicKey() *ecdsa.PublicKey {
	return &p.key.PublicKey
}

func (p *NodeKey) PrivateKey() *ecdsa.PrivateKey {
	return p.key
}
