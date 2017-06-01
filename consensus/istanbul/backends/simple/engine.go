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
	"crypto/ecdsa"
	"errors"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/istanbul"
	istanbulCore "github.com/ethereum/go-ethereum/consensus/istanbul/core"
	"github.com/ethereum/go-ethereum/consensus/istanbul/validator"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/crypto/sha3"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/rpc"
)

var (
	// errInvalidProposal is returned when a prposal is malformed.
	errInvalidProposal = errors.New("invalid proposal")
	// errInvalidSignature is returned when given signature is not signed by given
	// address.
	errInvalidSignature = errors.New("invalid signature")
	// errUnknownBlock is returned when the list of signers is requested for a block
	// that is not part of the local blockchain.
	errUnknownBlock = errors.New("unknown block")
	// errUnauthorized is returned if a header is signed by a non authorized entity.
	errUnauthorized = errors.New("unauthorized")
	// errInvalidDifficulty is returned if the difficulty of a block is not 1
	errInvalidDifficulty = errors.New("invalid difficulty")
	// errInvalidPeer is returned when a message from invalid peer comes
	errInvalidPeer = errors.New("invalid peer")
	// errInvalidExtraDataFormat is returned when the extra data format is incorrect
	errInvalidExtraDataFormat = errors.New("invalid extra data format")
	// errInvalidMixDigest is returned if a block's mix digest is non zero.
	errInvalidMixDigest = errors.New("non-zero mix digest")
	// errInvalidCoinbase is returned if a block's coinbase is non zero.
	errInvalidCoinbase = errors.New("non-zero coinbase")
	// errInvalidUncleHash is returned if a block contains an non-empty uncle list.
	errInvalidUncleHash = errors.New("non empty uncle hash")
	// errInconsistentValidatorSet is returned if the validator set is inconsistent
	errInconsistentValidatorSet = errors.New("non empty uncle hash")
	// errInvalidTimestamp is returned if the timestamp of a block is lower than the previous block's timestamp + the minimum block period.
	errInvalidTimestamp = errors.New("invalid timestamp")
)
var (
	defaultDifficulty = big.NewInt(1)
	nilUncleHash      = types.CalcUncleHash(nil) // Always Keccak256(RLP([])) as uncles are meaningless outside of PoW.
	now               = time.Now
)

// Author retrieves the Ethereum address of the account that minted the given
// block, which may be different from the header's coinbase if a consensus
// engine is based on signatures.
func (sb *simpleBackend) Author(header *types.Header) (common.Address, error) {
	return sb.ecrecover(header)
}

// VerifyHeader checks whether a header conforms to the consensus rules of a
// given engine. Verifying the seal may be done optionally here, or explicitly
// via the VerifySeal method.
func (sb *simpleBackend) VerifyHeader(chain consensus.ChainReader, header *types.Header, seal bool) error {
	number := header.Number.Uint64()
	return sb.verifyHeader(chain, header, chain.GetHeader(header.ParentHash, number-1))
}

// verifyHeader checks whether a header conforms to the consensus rules.The
// caller may optionally pass in a batch of parents (ascending order) to avoid
// looking those up from the database. This is useful for concurrently verifying
// a batch of new headers.
func (sb *simpleBackend) verifyHeader(chain consensus.ChainReader, header *types.Header, parent *types.Header) error {
	if header.Number == nil {
		return errUnknownBlock
	}

	// Don't waste time checking blocks from the future
	if header.Time.Cmp(big.NewInt(now().Unix())) > 0 {
		return consensus.ErrFutureBlock
	}

	// Ensure that the extra data format is satisfied
	if !sb.validExtraFormat(header) {
		return errInvalidExtraDataFormat
	}

	// Ensure that the coinbase is zero
	if header.Coinbase != (common.Address{}) {
		return errInvalidCoinbase
	}
	// Ensure that the mix digest is zero as we don't have fork protection currently
	if header.MixDigest != (common.Hash{}) {
		return errInvalidMixDigest
	}
	// Ensure that the block doesn't contain any uncles which are meaningless in Istanbul
	if header.UncleHash != nilUncleHash {
		return errInvalidUncleHash
	}
	// Ensure that the block's difficulty is meaningful (may not be correct at this point)
	if header.Difficulty == nil || header.Difficulty.Cmp(defaultDifficulty) != 0 {
		return errInvalidDifficulty
	}

	return sb.verifyCascadingFields(chain, header, parent)
}

// verifyCascadingFields verifies all the header fields that are not standalone,
// rather depend on a batch of previous headers. The caller may optionally pass
// in a batch of parents (ascending order) to avoid looking those up from the
// database. This is useful for concurrently verifying a batch of new headers.
func (sb *simpleBackend) verifyCascadingFields(chain consensus.ChainReader, header *types.Header, parent *types.Header) error {
	// The genesis block is the always valid dead-end
	number := header.Number.Uint64()
	if number == 0 {
		return nil
	}
	// Ensure that the block's timestamp isn't too close to it's parent
	if parent == nil || parent.Number.Uint64() != number-1 || parent.Hash() != header.ParentHash {
		return consensus.ErrUnknownAncestor
	}
	if parent.Time.Uint64()+sb.config.BlockPeriod > header.Time.Uint64() {
		return errInvalidTimestamp
	}
	// Fixed validator set
	if bytes.Compare(sb.getValidatorBytes(header), sb.getValidatorBytes(parent)) != 0 {
		return errInconsistentValidatorSet
	}
	return sb.verifySigner(chain, header, parent)
}

// VerifyHeaders is similar to VerifyHeader, but verifies a batch of headers
// concurrently. The method returns a quit channel to abort the operations and
// a results channel to retrieve the async verifications (the order is that of
// the input slice).
func (sb *simpleBackend) VerifyHeaders(chain consensus.ChainReader, headers []*types.Header, seals []bool) (chan<- struct{}, <-chan error) {
	abort := make(chan struct{})
	results := make(chan error, len(headers))
	go func() {
		for i, header := range headers {
			var err error
			if i == 0 {
				err = sb.VerifyHeader(chain, header, false)
			} else {
				// The headers are ordered, and the parent header is the previous one.
				err = sb.verifyHeader(chain, header, headers[i-1])
			}

			select {
			case <-abort:
				return
			case results <- err:
			}
		}
	}()
	return abort, results
}

// VerifyUncles verifies that the given block's uncles conform to the consensus
// rules of a given engine.
func (sb *simpleBackend) VerifyUncles(chain consensus.ChainReader, block *types.Block) error {
	if len(block.Uncles()) > 0 {
		return errInvalidUncleHash
	}
	return nil
}

// verifySigner checks whether the signer is in parent's validator set
func (sb *simpleBackend) verifySigner(chain consensus.ChainReader, header *types.Header, parent *types.Header) error {
	// resolve the authorization key and check against signers
	signer, err := sb.ecrecover(header)
	if err != nil {
		return err
	}

	validatorAddresses := validator.ExtractValidators(sb.getValidatorBytes(parent))
	// ensure the signer is in parent's validator set
	parentValSet := validator.NewSet(validatorAddresses, sb.config.ProposerPolicy)
	if parentValSet == nil {
		return errInvalidExtraDataFormat
	}
	if _, v := parentValSet.GetByAddress(signer); v == nil {
		return errUnauthorized
	}
	return nil
}

// VerifySeal checks whether the crypto seal on a header is valid according to
// the consensus rules of the given engine.
func (sb *simpleBackend) VerifySeal(chain consensus.ChainReader, header *types.Header) error {
	// get parent header and ensure the signer is in parent's validator set
	number := header.Number.Uint64()
	if number == 0 {
		return errUnknownBlock
	}

	// ensure that the difficulty equals to defaultDifficulty
	if header.Difficulty.Cmp(defaultDifficulty) != 0 {
		return errInvalidDifficulty
	}

	parent := chain.GetHeader(header.ParentHash, number-1)
	if parent == nil {
		return consensus.ErrUnknownAncestor
	}
	return sb.verifySigner(chain, header, parent)
}

// Prepare initializes the consensus fields of a block header according to the
// rules of a particular engine. The changes are executed inline.
func (sb *simpleBackend) Prepare(chain consensus.ChainReader, header *types.Header) error {
	// unused fields, force to set to empty
	header.Coinbase = common.Address{}
	header.Nonce = types.BlockNonce{}
	header.MixDigest = common.Hash{}

	// copy the parent extra data as the header extra data
	number := header.Number.Uint64()
	parent := chain.GetHeader(header.ParentHash, number-1)
	if parent == nil {
		return consensus.ErrUnknownAncestor
	}
	if !sb.validExtraFormat(parent) {
		return errInvalidExtraDataFormat
	}
	// Ensure the extra data has all it's components
	header.Extra = sb.prepareExtra(header, parent)
	// use the same difficulty for all blocks
	header.Difficulty = defaultDifficulty
	return nil
}

// Finalize runs any post-transaction state modifications (e.g. block rewards)
// and assembles the final block.
//
// Note, the block header and state database might be updated to reflect any
// consensus rules that happen at finalization (e.g. block rewards).
func (sb *simpleBackend) Finalize(chain consensus.ChainReader, header *types.Header, state *state.StateDB, txs []*types.Transaction,
	uncles []*types.Header, receipts []*types.Receipt) (*types.Block, error) {
	// No block rewards in Istanbul, so the state remains as is and uncles are dropped
	header.Root = state.IntermediateRoot(chain.Config().IsEIP158(header.Number))
	header.UncleHash = nilUncleHash

	// Assemble and return the final block for sealing
	return types.NewBlock(header, txs, nil, receipts), nil
}

// Seal generates a new block for the given input block with the local miner's
// seal place on top.
func (sb *simpleBackend) Seal(chain consensus.ChainReader, block *types.Block, stop <-chan struct{}) (*types.Block, error) {
	// update the block header timestamp and signature and propose the block to core engine
	header := block.Header()
	number := header.Number.Uint64()
	parent := chain.GetHeader(header.ParentHash, number-1)
	if parent == nil {
		return nil, consensus.ErrUnknownAncestor
	}
	block, err := sb.updateBlock(parent, block)
	if err != nil {
		return nil, err
	}

	// wait for the timestamp of header, use this to adjust the block period
	delay := time.Unix(block.Header().Time.Int64(), 0).Sub(now())
	select {
	case <-time.After(delay):
	case <-stop:
		return nil, nil
	}

	// get the proposed block hash and clear it if the seal() is completed.
	sb.sealMu.Lock()
	sb.proposedBlockHash = block.Hash()
	clear := func() {
		sb.proposedBlockHash = common.Hash{}
		sb.sealMu.Unlock()
	}
	defer clear()

	// post block into Istanbul engine
	go sb.EventMux().Post(istanbul.RequestEvent{
		Proposal: block,
	})

	for {
		select {
		case hash := <-sb.commitCh:
			// if the block hash and the hash from channel are the same,
			// return the block. Otherwise, keep waiting the next hash.
			if block.Hash() == hash {
				return block, nil
			}
		case <-stop:
			return nil, nil
		}
	}
}

// update timestamp and signature of the block based on its number of transactions
func (sb *simpleBackend) updateBlock(parent *types.Header, block *types.Block) (*types.Block, error) {
	// set block period based the number of tx
	var period uint64
	if len(block.Transactions()) == 0 {
		period = sb.config.BlockPauseTime
	} else {
		period = sb.config.BlockPeriod
	}

	// set header timestamp
	header := block.Header()
	header.Time = new(big.Int).Add(parent.Time, new(big.Int).SetUint64(period))
	time := now().Unix()
	if header.Time.Int64() < time {
		header.Time = big.NewInt(time)
	}
	// sign the hash
	sighash, err := sb.Sign(sigHash(header).Bytes())
	if err != nil {
		return nil, err
	}

	start, end := sb.signaturePosition(header)
	copy(header.Extra[start:end], sighash)

	return block.WithSeal(header), nil
}

// APIs returns the RPC APIs this consensus engine provides.
func (sb *simpleBackend) APIs(chain consensus.ChainReader) []rpc.API {
	return []rpc.API{{
		Namespace: "istanbul",
		Version:   "1.0",
		Service:   &API{chain: chain, pbft: sb},
		Public:    true,
	}}
}

// HandleMsg implements consensus.Istanbul.HandleMsg
func (sb *simpleBackend) HandleMsg(pubKey *ecdsa.PublicKey, data []byte) error {
	addr := crypto.PubkeyToAddress(*pubKey)
	if _, val := sb.valSet.GetByAddress(addr); val == nil {
		sb.logger.Error("Not in validator set", "addr", addr)
		return istanbul.ErrUnauthorizedAddress
	}

	go sb.istanbulEventMux.Post(istanbul.MessageEvent{
		Payload: data,
	})
	return nil
}

// NewChainHead implements consensus.Istanbul.NewChainHead
func (sb *simpleBackend) NewChainHead(block *types.Block) {
	p, err := sb.Author(block.Header())
	if err != nil {
		sb.logger.Error("Failed to get block proposer", "err", err)
		return
	}
	go sb.istanbulEventMux.Post(istanbul.FinalCommittedEvent{
		Proposal: block,
		Proposer: p,
	})
}

// Start implements consensus.Istanbul.Start
func (sb *simpleBackend) Start(chain consensus.ChainReader, inserter func(block *types.Block) error) error {
	if err := sb.initValidatorSet(chain); err != nil {
		return err
	}
	if _, v := sb.valSet.GetByAddress(sb.address); v == nil {
		return istanbul.ErrUnauthorizedAddress
	}
	sb.chain = chain
	sb.inserter = inserter
	sb.core = istanbulCore.New(sb, sb.config)

	curHeader := chain.CurrentHeader()
	lastSequence := new(big.Int).Set(curHeader.Number)
	lastProposer := common.Address{}
	// should get proposer if the block is not genesis
	if lastSequence.Cmp(common.Big0) > 0 {
		p, err := sb.Author(curHeader)
		if err != nil {
			return err
		}
		lastProposer = p
	}
	return sb.core.Start(lastSequence, lastProposer)
}

// Stop implements consensus.Istanbul.Stop
func (sb *simpleBackend) Stop() error {
	if sb.core != nil {
		return sb.core.Stop()
	}
	return nil
}

func (sb *simpleBackend) initValidatorSet(chain consensus.ChainReader) error {
	header := chain.CurrentHeader()
	// get the validator byte array and feed into validator set
	validatorAddresses := validator.ExtractValidators(sb.getValidatorBytes(header))
	valSet := validator.NewSet(validatorAddresses, sb.config.ProposerPolicy)
	if valSet == nil {
		return errInvalidExtraDataFormat
	}
	sb.valSet = valSet
	return nil
}

func (sb *simpleBackend) validExtraFormat(header *types.Header) bool {
	length := len(header.Extra)
	// ensure the bytes is enough
	if length < types.IstanbulExtraVanity+types.IstanbulExtraValidatorSize+types.IstanbulExtraSeal {
		return false
	}

	vl := sb.validatorLength(header)
	// validator length cannot be 0
	if vl == 0 {
		return false
	}
	if length != types.IstanbulExtraVanity+types.IstanbulExtraValidatorSize+vl+types.IstanbulExtraSeal {
		return false
	}

	return true
}

func (sb *simpleBackend) getValidatorBytes(header *types.Header) []byte {
	return header.Extra[types.IstanbulExtraVanity+types.IstanbulExtraValidatorSize : types.IstanbulExtraVanity+types.IstanbulExtraValidatorSize+sb.validatorLength(header)]
}

// prepareExtra creates a copy that includes vanity, validators, and a clean seal for the given header
//
// note that the header.Extra consisted of vanity, validator size, validators, seal, and committed signatures
func (sb *simpleBackend) prepareExtra(header, parent *types.Header) []byte {
	buf := make([]byte, 0)
	if len(header.Extra) < types.IstanbulExtraVanity {
		header.Extra = append(header.Extra, bytes.Repeat([]byte{0x00}, types.IstanbulExtraVanity-len(header.Extra))...)
	}
	buf = header.Extra[:types.IstanbulExtraVanity]

	buf = append(buf, parent.Extra[types.IstanbulExtraVanity:types.IstanbulExtraVanity+types.IstanbulExtraValidatorSize+sb.validatorLength(parent)]...)
	buf = append(buf, make([]byte, types.IstanbulExtraSeal)...)
	return buf
}

// signaturePosition returns start and end position for the given header
func (sb *simpleBackend) signaturePosition(header *types.Header) (int, int) {
	start := types.IstanbulExtraVanity + types.IstanbulExtraValidatorSize + sb.validatorLength(header)
	end := start + types.IstanbulExtraSeal
	return int(start), int(end)
}

// validatorLength returns the validator length for the given header
func (sb *simpleBackend) validatorLength(header *types.Header) int {
	validatorSize := int(header.Extra[types.IstanbulExtraVanity : types.IstanbulExtraVanity+types.IstanbulExtraValidatorSize][0])
	validatorLength := validatorSize * common.AddressLength
	return int(validatorLength)
}

// FIXME: Need to update this for Istanbul
// sigHash returns the hash which is used as input for the Istanbul
// signing. It is the hash of the entire header apart from the 65 byte signature
// contained at the end of the extra data.
//
// Note, the method requires the extra data to be at least 65 bytes, otherwise it
// panics. This is done to avoid accidentally using both forms (signature present
// or not), which could be abused to produce different hashes for the same header.
func sigHash(header *types.Header) (hash common.Hash) {
	hasher := sha3.NewKeccak256()

	rlp.Encode(hasher, []interface{}{
		header.ParentHash,
		header.UncleHash,
		header.Coinbase,
		header.Root,
		header.TxHash,
		header.ReceiptHash,
		header.Bloom,
		header.Difficulty,
		header.Number,
		header.GasLimit,
		header.GasUsed,
		header.Time,
		header.Extra[:len(header.Extra)-types.IstanbulExtraSeal], // Yes, this will panic if extra is too short
		header.MixDigest,
		header.Nonce,
	})
	hasher.Sum(hash[:0])
	return hash
}

// ecrecover extracts the Ethereum account address from a signed header.
func (sb *simpleBackend) ecrecover(header *types.Header) (common.Address, error) {
	// Retrieve the signature from the header extra-data
	if len(header.Extra) < types.IstanbulExtraSeal {
		return common.Address{}, consensus.ErrMissingSignature
	}
	start, end := sb.signaturePosition(header)
	signature := header.Extra[start:end]
	return sb.getSignatureAddress(sigHash(header).Bytes(), signature)
}
