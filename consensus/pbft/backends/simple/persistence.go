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
	"github.com/ethereum/go-ethereum/rlp"
)

const (
	dbKeyPrefix = "pbft-backend-"
)

func (sb *simpleBackend) Save(key string, val interface{}) error {
	blob, err := rlp.EncodeToBytes(val)
	if err != nil {
		return err
	}
	return sb.db.Put(toDatabaseKey(sb.Hash, key), blob)
}

func (sb *simpleBackend) Restore(key string, val interface{}) error {
	blob, err := sb.db.Get(toDatabaseKey(sb.Hash, key))
	if err != nil {
		return err
	}
	return rlp.DecodeBytes(blob, val)
}

func toDatabaseKey(hashfn func(val interface{}) common.Hash, key string) []byte {
	return hashfn(dbKeyPrefix + key).Bytes()
}
