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

package types

import (
	"bytes"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/params"
)

var (
	// The IstanbulDigest represents a constant of "Istanbul practical byzantine fault tolerance".
	IstanbulDigest = common.HexToHash("0x63746963616c2062797a616e74696e65206661756c7420746f6c6572616e6365")

	IstanbulExtraVanity        = int(params.MaximumExtraDataSize) // Fixed number of extra-data prefix bytes reserved for signer vanity
	IstanbulExtraValidatorSize = 1                                // Fixed number of extra-data infix bytes reserved for validator size
	IstanbulExtraSeal          = 65                               // Fixed number of extra-data suffix bytes reserved for signer seal
	IstanbulExtraCommittedSize = 1                                // Fixed number of extra-data infix bytes reserved for committed size
	IstanbulExtraCommittedSeal = 65                               // Fixed number of extra-data suffix bytes reserved for committed seal

	ErrInvalidIstanbulHeaderExtra   = fmt.Errorf("Invalid istanbul header extra-data")
	ErrInvalidIstanbulCommittedSeal = fmt.Errorf("Invalid istanbul committed seal")
)

// FIXME: I don't like this name :(
type Istanbul struct {
	Vanity        []byte
	Validators    []common.Address
	Seal          []byte
	CommittedSeal [][]byte
}

type IstanbulIndex struct {
	Vanity              int
	ValidatorSize       int
	ValidatorLength     int
	Seal                int
	CommittedSize       int
	CommittedSealLength int
}

func ExtractToIstanbul(h *Header) *Istanbul {
	index := ExtractToIstanbulIndex(h)

	validators := make([]common.Address, (index.Seal-index.ValidatorLength)/common.AddressLength)
	for i := 0; i < len(validators); i++ {
		copy(validators[i][:], h.Extra[index.ValidatorLength+i*common.AddressLength:])
	}

	cmttedSeals := make([][]byte, (len(h.Extra)-index.CommittedSealLength)/IstanbulExtraSeal)
	for i := 0; i < len(cmttedSeals); i++ {
		cmttedSeals[i] = make([]byte, IstanbulExtraCommittedSeal)
		copy(cmttedSeals[i][:], h.Extra[index.CommittedSealLength+i*IstanbulExtraSeal:])
	}

	return &Istanbul{
		Vanity:        h.Extra[index.Vanity:index.ValidatorSize],
		Validators:    validators,
		Seal:          h.Extra[index.Seal : index.Seal+IstanbulExtraSeal],
		CommittedSeal: cmttedSeals,
	}
}

// ExtractToIstanbulIndex returns the starting index for each fields from header.
// if h.Extra is insufficient, compensate the lack of bytes.
func ExtractToIstanbulIndex(h *Header) *IstanbulIndex {
	// Ensure the extra data has all it's components
	newHeader := ensureValidIstanbulExtra(h)
	// a sanity check has done before.
	validatorLength := getValidatorLength(newHeader)

	return &IstanbulIndex{
		Vanity:              0,
		ValidatorSize:       IstanbulExtraVanity,
		ValidatorLength:     IstanbulExtraVanity + IstanbulExtraValidatorSize,
		Seal:                IstanbulExtraVanity + IstanbulExtraValidatorSize + validatorLength,
		CommittedSize:       IstanbulExtraVanity + IstanbulExtraValidatorSize + validatorLength + IstanbulExtraSeal,
		CommittedSealLength: IstanbulExtraVanity + IstanbulExtraValidatorSize + validatorLength + IstanbulExtraSeal + IstanbulExtraValidatorSize}
}

// ValidateIstanbulExtra validates the extra-data field of a block header to
// ensure it conforms to Istanbul rules.
func ValidateIstanbulSeal(header *Header) error {
	defaultLength := IstanbulExtraVanity + IstanbulExtraValidatorSize + IstanbulExtraSeal
	// ensure we can get validator size at specific index
	if len(header.Extra) < IstanbulExtraVanity+IstanbulExtraValidatorSize+IstanbulExtraSeal {
		return ErrInvalidIstanbulHeaderExtra
	}

	// a sanity check has done before.
	valLength := getValidatorLength(header)
	if len(header.Extra) < defaultLength+valLength {
		return ErrInvalidIstanbulHeaderExtra
	}

	return nil
}

// PrepareIstanbulExtra returns a istanbul extra-data that use the given header, parent
// we copy vanity from header and validator, seal from parent
func PrepareIstanbulExtra(header, parent *Header) []byte {
	var buf bytes.Buffer

	if len(header.Extra) < IstanbulExtraVanity {
		header.Extra = append(header.Extra, bytes.Repeat([]byte{0x00}, IstanbulExtraVanity-len(header.Extra))...)
	}
	buf.Write(header.Extra[:IstanbulExtraVanity])

	index := ExtractToIstanbulIndex(parent)

	buf.Write(parent.Extra[index.ValidatorSize:index.Seal])
	buf.Write(make([]byte, IstanbulExtraSeal))

	return buf.Bytes()
}

// IstanbulExtraFilter is used to filter out the useless information.
//
// We just keep the following data in header.Extra.
// ┌─────────────────┬──────────────────────┬────────────────────────────────┬───────────────┐
// │ vanity(32 bytes)│ validator#N (1 bytes)│ validator address(N * 20 bytes)│ seal(65 bytes)│
// └─────────────────┴──────────────────────┴────────────────────────────────┴───────────────┘
func IstanbulExtraFilter(h *Header) *Header {
	newHeader := ensureValidIstanbulExtra(h)
	valLength := getValidatorLength(newHeader)
	newHeader.Extra = newHeader.Extra[0 : IstanbulExtraVanity+IstanbulExtraValidatorSize+valLength+IstanbulExtraSeal]

	return newHeader
}

// ensureValidIstanbulExtra returns a new copy header. The extra-data field of a block header
// may be insufficient, so we will append 0x00 to avoid panic in getting array.
//
// Extra-data format like below.
// ┌─────────────────┬──────────────────────┬────────────────────────────────┬───────────────┬─────────────────────┬──────────────────────────────┐
// │ vanity(32 bytes)│ validator#N (1 bytes)│ validator address(N * 20 bytes)│ seal(65 bytes)│ committed#N (1 byte)│ committed seals(N * 65 bytes)│
// └─────────────────┴──────────────────────┴────────────────────────────────┴───────────────┴─────────────────────┴──────────────────────────────┘
func ensureValidIstanbulExtra(h *Header) *Header {
	newHeader := CopyHeader(h)
	defaultLength := IstanbulExtraVanity + IstanbulExtraValidatorSize + IstanbulExtraSeal

	// The number of validator addresses is not known since the extra may be insufficient,
	// but we can checks whether extra is enough(vanity + validator#N + seal).
	// If it is not enough, compensate the lack of bytes to avoid panic in getting slice.
	if len(newHeader.Extra) < defaultLength {
		newHeader.Extra = append(newHeader.Extra, bytes.Repeat([]byte{0x00}, defaultLength-len(newHeader.Extra))...)
	}

	// Calculate the validator length. a sanity check has done before.
	valLength := getValidatorLength(newHeader)

	// Same as before, but validator length is also considered.
	if len(newHeader.Extra) < defaultLength+valLength {
		newHeader.Extra = append(newHeader.Extra, bytes.Repeat([]byte{0x00}, defaultLength+valLength-len(newHeader.Extra))...)
	}

	defaultLength += valLength
	if len(newHeader.Extra) < defaultLength+IstanbulExtraCommittedSize {
		newHeader.Extra = append(newHeader.Extra, bytes.Repeat([]byte{0x00}, defaultLength+IstanbulExtraCommittedSize-len(newHeader.Extra))...)
	}

	cmttedLength := int(newHeader.Extra[defaultLength : defaultLength+IstanbulExtraCommittedSize][0]) * IstanbulExtraCommittedSeal
	defaultLength += IstanbulExtraCommittedSize + cmttedLength
	if len(newHeader.Extra) < defaultLength {
		newHeader.Extra = append(newHeader.Extra, bytes.Repeat([]byte{0x00}, defaultLength-len(newHeader.Extra))...)
	}

	newHeader.Extra = newHeader.Extra[0:defaultLength]

	return newHeader
}

// updateIstanbulCommittedSeal updates header signatures that store consensus proof
func updateIstanbulCommittedSeal(h *Header, committedSeal []byte) error {
	// sanity check
	if len(committedSeal)%IstanbulExtraCommittedSeal != 0 {
		return ErrInvalidIstanbulCommittedSeal
	}
	size := len(committedSeal) / IstanbulExtraCommittedSeal
	h.Extra = append(h.Extra, byte(size))
	h.Extra = append(h.Extra, committedSeal...)
	return nil
}

// getValidatorLength returns a validator total length for a given header
func getValidatorLength(h *Header) int {
	if len(h.Extra) < IstanbulExtraVanity+IstanbulExtraValidatorSize {
		return -1
	}
	return int(h.Extra[IstanbulExtraVanity : IstanbulExtraVanity+IstanbulExtraValidatorSize][0]) * common.AddressLength
}
