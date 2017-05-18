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
	"io"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
)

const (
	msgPreprepare uint64 = iota
	msgPrepare
	msgCommit
	msgCheckpoint
	msgRoundChange
	msgAll
)

type message struct {
	Code      uint64
	Msg       []byte
	Address   common.Address
	Signature []byte
}

// ==============================================
//
// define the functions that needs to be provided for rlp Encoder/Decoder.

// EncodeRLP serializes m into the Ethereum RLP format.
func (m *message) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, []interface{}{m.Code, m.Msg, m.Address, m.Signature})
}

// DecodeRLP implements rlp.Decoder, and load the consensus fields from a RLP stream.
func (m *message) DecodeRLP(s *rlp.Stream) error {
	var msg struct {
		Code      uint64
		Msg       []byte
		Address   common.Address
		Signature []byte
	}

	if err := s.Decode(&msg); err != nil {
		return err
	}
	m.Code, m.Msg, m.Address, m.Signature = msg.Code, msg.Msg, msg.Address, msg.Signature
	return nil
}

// ==============================================
//
// define the functions that needs to be provided for core.

func (m *message) FromPayload(b []byte, validateFn func([]byte, []byte) (common.Address, error)) error {
	// Decode message
	err := rlp.DecodeBytes(b, &m)
	if err != nil {
		return err
	}

	// Validate message (on a message without Signature)
	if validateFn != nil {
		var payload []byte
		payload, err = m.PayloadNoSig()
		if err != nil {
			return err
		}

		_, err = validateFn(payload, m.Signature)
	}
	// Still return the message even the err is not nil
	return err
}

func (m *message) Payload() ([]byte, error) {
	return rlp.EncodeToBytes(m)
}

func (m *message) PayloadNoSig() ([]byte, error) {
	return rlp.EncodeToBytes(&message{
		Code:    m.Code,
		Msg:     m.Msg,
		Address: m.Address,
	})
}

func (m *message) Decode(val interface{}) error {
	return rlp.DecodeBytes(m.Msg, val)
}

// ==============================================
//
// helper functions

func Encode(val interface{}) ([]byte, error) {
	return rlp.EncodeToBytes(val)
}

// ----------------------------------------------------------------------------

type roundChange struct {
	Round    *big.Int
	Sequence *big.Int
	Digest   common.Hash
}

// EncodeRLP serializes b into the Ethereum RLP format.
func (rc *roundChange) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, []interface{}{
		rc.Round,
		rc.Sequence,
		rc.Digest,
	})
}

// DecodeRLP implements rlp.Decoder, and load the consensus fields from a RLP stream.
func (rc *roundChange) DecodeRLP(s *rlp.Stream) error {
	var rawRoundChange struct {
		Round    *big.Int
		Sequence *big.Int
		Digest   common.Hash
	}
	if err := s.Decode(&rawRoundChange); err != nil {
		return err
	}
	rc.Round, rc.Sequence, rc.Digest = rawRoundChange.Round, rawRoundChange.Sequence, rawRoundChange.Digest
	return nil
}
