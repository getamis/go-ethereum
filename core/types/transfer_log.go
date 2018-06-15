// Copyright 2018 The go-ethereum Authors
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

package types

import (
	"io"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rlp"
)

//go:generate gencodec -type TransferLog -field-override transferLogMarshaling -out gen_transfer_log_json.go

// TransferLog represents an ether transfer event. These events are generated by ether transfer
// including transfer in contract and stored/indexed by the node.
type TransferLog struct {
	// Consensus fields:
	// address of the sender
	From common.Address `json:"from" gencodec:"required"`
	// address of the recipient
	To common.Address `json:"to" gencodec:"required"`
	// the amount of ether
	Value *big.Int `json:"value" gencodec:"required"`

	// Derived fields. These fields are filled in by the node
	// but not secured by consensus.
	// hash of the transaction
	TxHash common.Hash `json:"transactionHash" gencodec:"required"`
}

type transferLogMarshaling struct {
	Value *hexutil.Big
}

type rlpTransferLog struct {
	From   common.Address
	To     common.Address
	Value  *big.Int
	TxHash common.Hash
}

// EncodeRLP implements rlp.Encoder.
func (l *TransferLog) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, []interface{}{
		l.From,
		l.To,
		l.Value,
		l.TxHash,
	})
}

// DecodeRLP implements rlp.Decoder.
func (l *TransferLog) DecodeRLP(s *rlp.Stream) error {
	var transferLog struct {
		From   common.Address
		To     common.Address
		Value  *big.Int
		TxHash common.Hash
	}
	if err := s.Decode(&transferLog); err != nil {
		return err
	}
	l.From, l.To, l.Value, l.TxHash = transferLog.From, transferLog.To, transferLog.Value, transferLog.TxHash
	return nil
}
