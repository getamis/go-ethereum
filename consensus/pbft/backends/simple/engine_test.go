package simple

import (
	"bytes"
	"crypto/ecdsa"
	"math/big"
	"reflect"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/pbft"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/params"
)

// in this test, we can set n to 1, and it means we can process PBFT and commit a
// block by one node. Otherwise, if n is larger than 1, we have to generate
// other fake events to process PBFT.
func newBlockChain(n int) (*core.BlockChain, *simpleBackend) {
	genesis, nodeKeys := getGenesisAndKeys(n)
	eventMux := new(event.TypeMux)
	memDB, _ := ethdb.NewMemDatabase()
	// Use the first key as private key
	backend := New(3000, eventMux, nodeKeys[0], memDB)
	genesis.MustCommit(memDB)
	blockchain, err := core.NewBlockChain(memDB, genesis.Config, backend, eventMux, vm.Config{})
	if err != nil {
		panic(err)
	}
	backend.Start(blockchain)
	b, _ := backend.(*simpleBackend)

	proposerAddr := b.valSet.GetProposer().Address()
	// find proposer key
	for _, key := range nodeKeys {
		addr := crypto.PubkeyToAddress(key.PublicKey)
		if addr.String() == proposerAddr.String() {
			b.privateKey = key
			b.address = addr
		}
	}

	return blockchain, b
}

func getGenesisAndKeys(n int) (*core.Genesis, []*ecdsa.PrivateKey) {
	// Setup validators
	var nodeKeys = make([]*ecdsa.PrivateKey, n)
	var addrs = make([]common.Address, n)
	for i := 0; i < n; i++ {
		nodeKeys[i], _ = crypto.GenerateKey()
		addrs[i] = crypto.PubkeyToAddress(nodeKeys[i].PublicKey)
	}

	// generate genesis block
	genesis := core.DefaultGenesisBlock()
	genesis.Config = params.TestChainConfig
	// force enable PBFT engine
	genesis.Config.PBFT = &params.PBFTConfig{}
	genesis.Config.Ethash = nil

	appendValidators(genesis, addrs)
	return genesis, nodeKeys
}

func appendValidators(genesis *core.Genesis, addrs []common.Address) {
	if len(genesis.ExtraData) < extraVanity {
		genesis.ExtraData = append(genesis.ExtraData, bytes.Repeat([]byte{0x00}, extraVanity)...)
	}
	genesis.ExtraData = genesis.ExtraData[:extraVanity]

	for _, addr := range addrs {
		genesis.ExtraData = append(genesis.ExtraData, addr[:]...)
	}
	genesis.ExtraData = append(genesis.ExtraData, make([]byte, extraSeal)...)
}

func makeHeader(parent *types.Block) *types.Header {
	header := &types.Header{
		ParentHash: parent.Hash(),
		Number:     parent.Number().Add(parent.Number(), common.Big1),
		GasLimit:   core.CalcGasLimit(parent),
		GasUsed:    new(big.Int),
		Extra:      parent.Extra(),
		Time:       big.NewInt(time.Now().Unix()),
		Difficulty: defaultDifficulty,
	}
	return header
}

func makeBlock(chain *core.BlockChain, engine *simpleBackend, parent *types.Block) *types.Block {
	header := makeHeader(parent)
	engine.Prepare(chain, header)
	state, _ := chain.StateAt(parent.Root())
	block, _ := engine.Finalize(chain, header, state, nil, nil, nil)
	block, _ = engine.Seal(chain, block, nil)
	return block
}

// get expected final block
func getExpectedBlock(engine *simpleBackend, block *types.Block) *types.Block {
	header := block.Header()
	sighash, _ := engine.Sign(sigHash(header).Bytes())
	copy(header.Extra[len(header.Extra)-extraSeal:], sighash)
	return block.WithSeal(header)
}

func TestPrepare(t *testing.T) {
	chain, engine := newBlockChain(1)
	header := makeHeader(chain.Genesis())
	err := engine.Prepare(chain, header)
	if err != nil {
		t.Errorf("error should be nil, but got: %v", err)
	}
	header.ParentHash = common.StringToHash("1234567890")
	err = engine.Prepare(chain, header)
	if err != consensus.ErrUnknownAncestor {
		t.Errorf("error should be consensus.ErrUnknownAncestor, but got: %v", err)
	}
}

func TestSealStopChannel(t *testing.T) {
	chain, engine := newBlockChain(4)
	header := makeHeader(chain.Genesis())
	state, _ := chain.StateAt(chain.Genesis().Root())
	block, _ := engine.Finalize(chain, header, state, nil, nil, nil)
	stop := make(chan struct{}, 1)
	eventSub := engine.EventMux().Subscribe(pbft.RequestEvent{})
	eventLoop := func() {
		select {
		case ev := <-eventSub.Chan():
			_, ok := ev.Data.(pbft.RequestEvent)
			if !ok {
				t.Errorf("unexpected event comes, got: %v, expected: pbft.RequestEvent", reflect.TypeOf(ev.Data))
			}
			stop <- struct{}{}
		}
		eventSub.Unsubscribe()
	}
	go eventLoop()
	finalBlock, err := engine.Seal(chain, block, stop)
	if err != nil {
		t.Errorf("error should be nil, but got: %v", err)
	}
	if finalBlock != nil {
		t.Errorf("block should be nil, but got: %v", finalBlock)
	}
}

func TestSealViewChange(t *testing.T) {
	chain, engine := newBlockChain(4)
	header := makeHeader(chain.Genesis())
	state, _ := chain.StateAt(chain.Genesis().Root())
	block, _ := engine.Finalize(chain, header, state, nil, nil, nil)
	eventSub := engine.EventMux().Subscribe(pbft.RequestEvent{})
	eventLoop := func() {
		select {
		case ev := <-eventSub.Chan():
			_, ok := ev.Data.(pbft.RequestEvent)
			if !ok {
				t.Errorf("unexpected event comes, got: %v, expected: pbft.RequestEvent", reflect.TypeOf(ev.Data))
			}
			engine.viewChange <- false
		}
		eventSub.Unsubscribe()
	}
	go eventLoop()

	seal := func() {
		engine.Seal(chain, block, nil)
		t.Errorf("should not be called")
	}
	go seal()

	const timeoutDura = 2 * time.Second
	timeout := time.NewTimer(timeoutDura)
	select {
	case <-timeout.C:
		// wait 2 seconds to ensure we cannot get any blocks from PBFT
	}
}

func TestSealViewChangeNeedNewProposal(t *testing.T) {
	chain, engine := newBlockChain(4)
	header := makeHeader(chain.Genesis())
	state, _ := chain.StateAt(chain.Genesis().Root())
	block, _ := engine.Finalize(chain, header, state, nil, nil, nil)
	eventSub := engine.EventMux().Subscribe(pbft.RequestEvent{})
	eventLoop := func() {
		select {
		case ev := <-eventSub.Chan():
			_, ok := ev.Data.(pbft.RequestEvent)
			if !ok {
				t.Errorf("unexpected event comes, got: %v, expected: pbft.RequestEvent", reflect.TypeOf(ev.Data))
			}
			engine.viewChange <- true
		}
		eventSub.Unsubscribe()
	}
	go eventLoop()

	finalBlock, err := engine.Seal(chain, block, nil)
	if err != errViewChanged {
		t.Errorf("unexpected error comes, got: %v, expected: errViewChanged", err)
	}
	if finalBlock != nil {
		t.Errorf("block should be nil, but got: %v", finalBlock.Hash().Hex())
	}
}

func TestSealCommittedOtherHash(t *testing.T) {
	chain, engine := newBlockChain(4)
	header := makeHeader(chain.Genesis())
	state, _ := chain.StateAt(chain.Genesis().Root())
	block, _ := engine.Finalize(chain, header, state, nil, nil, nil)
	eventSub := engine.EventMux().Subscribe(pbft.RequestEvent{})
	eventLoop := func() {
		select {
		case ev := <-eventSub.Chan():
			_, ok := ev.Data.(pbft.RequestEvent)
			if !ok {
				t.Errorf("unexpected event comes, got: %v, expected: pbft.RequestEvent", reflect.TypeOf(ev.Data))
			}
			engine.commit <- common.StringToHash("1234567890")
		}
		eventSub.Unsubscribe()
	}
	go eventLoop()
	finalBlock, err := engine.Seal(chain, block, nil)
	if err != errOtherBlockCommitted {
		t.Errorf("unexpected error comes, got: %v, expected: errOtherBlockCommitted", err)
	}
	if finalBlock != nil {
		t.Errorf("block should be nil, but got: %v", finalBlock)
	}
}

func TestSealCommitted(t *testing.T) {
	chain, engine := newBlockChain(1)
	header := makeHeader(chain.Genesis())
	state, _ := chain.StateAt(chain.Genesis().Root())
	block, _ := engine.Finalize(chain, header, state, nil, nil, nil)
	expectedBlock := getExpectedBlock(engine, block)

	finalBlock, err := engine.Seal(chain, block, nil)
	if err != nil {
		t.Errorf("error should be nil, but got: %v", err)
	}
	if finalBlock.Hash().Hex() != expectedBlock.Hash().Hex() {
		t.Errorf("block should be equal, got: %v, expected: %v", finalBlock.Hash().Hex(), expectedBlock.Hash().Hex())
	}
}

func TestVerifyHeader(t *testing.T) {
	chain, engine := newBlockChain(1)

	// correct case
	block := makeBlock(chain, engine, chain.Genesis())
	err := engine.VerifyHeader(chain, block.Header(), false)
	if err != nil {
		t.Errorf("error should be nil, got: %v", err)
	}

	// short extra data
	header := block.Header()
	header.Extra = []byte{}
	block = block.WithSeal(header)
	err = engine.VerifyHeader(chain, block.Header(), false)
	if err != errInvalidExtraDataFormat {
		t.Errorf("unexpected error comes, got: %v, expected: errInvalidExtraDataFormat", err)
	}
	// incorrect extra format
	header.Extra = []byte("0000000000000000000000000000000012300000000000000000000000000000000000000000000000000000000000000000")
	block = block.WithSeal(header)
	err = engine.VerifyHeader(chain, block.Header(), false)
	if err != errInvalidExtraDataFormat {
		t.Errorf("unexpected error comes, got: %v, expected: errInvalidExtraDataFormat", err)
	}

	// non zero coinbase
	block = makeBlock(chain, engine, chain.Genesis())
	header = block.Header()
	header.Coinbase = common.StringToAddress("123456789")
	block = block.WithSeal(header)
	err = engine.VerifyHeader(chain, block.Header(), false)
	if err != errInvalidCoinbase {
		t.Errorf("unexpected error comes, got: %v, expected: errInvalidCoinbase", err)
	}

	// non zero MixDigest
	block = makeBlock(chain, engine, chain.Genesis())
	header = block.Header()
	header.MixDigest = common.StringToHash("123456789")
	block = block.WithSeal(header)
	err = engine.VerifyHeader(chain, block.Header(), false)
	if err != errInvalidMixDigest {
		t.Errorf("unexpected error comes, got: %v, expected: errInvalidMixDigest", err)
	}

	// invalid uncles hash
	block = makeBlock(chain, engine, chain.Genesis())
	header = block.Header()
	header.UncleHash = common.StringToHash("123456789")
	block = block.WithSeal(header)
	err = engine.VerifyHeader(chain, block.Header(), false)
	if err != errInvalidUncleHash {
		t.Errorf("unexpected error comes, got: %v, expected: errInvalidUncleHash", err)
	}

	// invalid difficulty
	block = makeBlock(chain, engine, chain.Genesis())
	header = block.Header()
	header.Difficulty = big.NewInt(2)
	block = block.WithSeal(header)
	err = engine.VerifyHeader(chain, block.Header(), false)
	if err != errInvalidDifficulty {
		t.Errorf("unexpected error comes, got: %v, expected: errInvalidDifficulty", err)
	}
}

func TestVerifySeal(t *testing.T) {
	chain, engine := newBlockChain(1)
	genesis := chain.Genesis()
	// cannot verify genesis
	err := engine.VerifySeal(chain, genesis.Header())
	if err != errUnknownBlock {
		t.Errorf("unexpected error comes, got: %v, expected: errUnknownBlock", err)
	}

	// change block content
	block := makeBlock(chain, engine, genesis)
	header := block.Header()
	header.Number = big.NewInt(4)
	block = block.WithSeal(header)
	err = engine.VerifySeal(chain, block.Header())
	if err != errUnauthorized {
		t.Errorf("unexpected error comes, got: %v, expected: errUnauthorized", err)
	}

	// unauthorized users but still can get correct signer address
	block = makeBlock(chain, engine, genesis)
	engine.privateKey, _ = crypto.GenerateKey()
	err = engine.VerifySeal(chain, block.Header())
	if err != nil {
		t.Errorf("error should be nil, but got: %v", err)
	}
}

func TestVerifyHeaders(t *testing.T) {
	chain, engine := newBlockChain(1)
	genesis := chain.Genesis()

	// success case
	headers := []*types.Header{}
	blocks := []*types.Block{}
	size := 100
	for i := 0; i < size; i++ {
		if i == 0 {
			blocks = append(blocks, makeBlock(chain, engine, genesis))
		} else {
			blocks = append(blocks, makeBlock(chain, engine, blocks[i-1]))
		}
		headers = append(headers, blocks[i].Header())
	}
	_, results := engine.VerifyHeaders(chain, headers, nil)
	const timeoutDura = 2 * time.Second
	timeout := time.NewTimer(timeoutDura)
	index := 0
OUT1:
	for {
		select {
		case err := <-results:
			if err != nil {
				t.Errorf("error should be nil, but got: %v", err)
				break OUT1
			}
			index++
			if index == size {
				break OUT1
			}
		case <-timeout.C:
			break OUT1
		}
	}

	// abort cases
	abort, results := engine.VerifyHeaders(chain, headers, nil)
	timeout = time.NewTimer(timeoutDura)
	index = 0
OUT2:
	for {
		select {
		case err := <-results:
			if err != nil {
				t.Errorf("error should be nil, but got: %v", err)
				break OUT2
			}
			index++
			if index == 5 {
				abort <- struct{}{}
			}
			// add some buffer here because we may not abort this channel immediately
			if index > 5+5 {
				t.Errorf("verifyheaders should be aborted")
				break OUT2
			}
		case <-timeout.C:
			break OUT2
		}
	}

	// error header cases
	headers[2].Number = big.NewInt(100)
	abort, results = engine.VerifyHeaders(chain, headers, nil)
	timeout = time.NewTimer(timeoutDura)
	index = 0
	errors := 0
	expectedErrors := 2
OUT3:
	for {
		select {
		case err := <-results:
			if err != nil {
				errors++
			}
			index++
			if index == size {
				if errors != expectedErrors {
					t.Errorf("Unexpected error number, got: %v, expected: %v", errors, expectedErrors)
				}
				break OUT3
			}
		case <-timeout.C:
			break OUT3
		}
	}
}
