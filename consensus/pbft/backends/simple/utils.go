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
	"bytes"
	"encoding/gob"

	"github.com/ethereum/go-ethereum/consensus/pbft"
)

func Decode(b []byte) (*pbft.MessageEvent, error) {
	msgEvent := &pbft.MessageEvent{}
	if err := gob.NewDecoder(bytes.NewBuffer(b)).Decode(msgEvent); err != nil {
		return nil, err
	}
	return msgEvent, nil
}

func Encode(msgEvent *pbft.MessageEvent) ([]byte, error) {
	var buf bytes.Buffer
	err := gob.NewEncoder(&buf).Encode(msgEvent)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
