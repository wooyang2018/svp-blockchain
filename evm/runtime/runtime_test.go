// Copyright 2015 The go-ethereum Authors
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

package runtime

import (
	"fmt"
	"math/big"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"

	//"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/asm"
	"github.com/ethereum/go-ethereum/core/types"

	"github.com/wooyang2018/svp-blockchain/evm"
	"github.com/wooyang2018/svp-blockchain/evm/params"
	"github.com/wooyang2018/svp-blockchain/storage"
	"github.com/wooyang2018/svp-blockchain/storage/statedb"
)

func TestDefaults(t *testing.T) {
	cfg := new(Config)
	setDefaults(cfg)

	if cfg.Difficulty == nil {
		t.Error("expected difficulty to be non nil")
	}
	if cfg.Time == nil {
		t.Error("expected time to be non nil")
	}
	if cfg.GasLimit == 0 {
		t.Error("didn't expect gaslimit to be zero")
	}
	if cfg.GasPrice == nil {
		t.Error("expected time to be non nil")
	}
	if cfg.Value == nil {
		t.Error("expected time to be non nil")
	}
	if cfg.GetHashFn == nil {
		t.Error("expected time to be non nil")
	}
	if cfg.BlockNumber == nil {
		t.Error("expected block number to be non nil")
	}
}

func TestEVM(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("crashed with: %v", r)
		}
	}()

	_, _, _ = Execute([]byte{
		byte(evm.DIFFICULTY),
		byte(evm.TIMESTAMP),
		byte(evm.GASLIMIT),
		byte(evm.PUSH1),
		byte(evm.ORIGIN),
		byte(evm.BLOCKHASH),
		byte(evm.COINBASE),
	}, nil, nil)
}

func TestExecute(t *testing.T) {
	ret, _, err := Execute([]byte{
		byte(evm.PUSH1), 10,
		byte(evm.PUSH1), 0,
		byte(evm.MSTORE),
		byte(evm.PUSH1), 32,
		byte(evm.PUSH1), 0,
		byte(evm.RETURN),
	}, nil, nil)
	if err != nil {
		t.Fatal("didn't expect error", err)
	}

	num := new(big.Int).SetBytes(ret)
	if num.Cmp(big.NewInt(10)) != 0 {
		t.Error("Expected 10, got", num)
	}
}

func TestCall(t *testing.T) {
	db := statedb.NewCacheDB(storage.NewMemLevelDBStore())
	statedb := statedb.NewStateDB(db, common.Hash{}, common.Hash{}, statedb.NewDummy())
	address := common.HexToAddress("0x0a")
	statedb.SetCode(address, []byte{
		byte(evm.PUSH1), 10,
		byte(evm.PUSH1), 0,
		byte(evm.MSTORE),
		byte(evm.PUSH1), 32,
		byte(evm.PUSH1), 0,
		byte(evm.RETURN),
	})

	ret, _, err := Call(address, nil, &Config{State: statedb})
	if err != nil {
		t.Fatal("didn't expect error", err)
	}

	num := new(big.Int).SetBytes(ret)
	if num.Cmp(big.NewInt(10)) != 0 {
		t.Error("Expected 10, got", num)
	}
}

func BenchmarkCall(b *testing.B) {
	var definition = `[{"constant":true,"inputs":[],"name":"seller","outputs":[{"name":"","type":"address"}],"type":"function"},{"constant":false,"inputs":[],"name":"abort","outputs":[],"type":"function"},{"constant":true,"inputs":[],"name":"value","outputs":[{"name":"","type":"uint256"}],"type":"function"},{"constant":false,"inputs":[],"name":"refund","outputs":[],"type":"function"},{"constant":true,"inputs":[],"name":"buyer","outputs":[{"name":"","type":"address"}],"type":"function"},{"constant":false,"inputs":[],"name":"confirmReceived","outputs":[],"type":"function"},{"constant":true,"inputs":[],"name":"state","outputs":[{"name":"","type":"uint8"}],"type":"function"},{"constant":false,"inputs":[],"name":"confirmPurchase","outputs":[],"type":"function"},{"inputs":[],"type":"constructor"},{"anonymous":false,"inputs":[],"name":"Aborted","type":"event"},{"anonymous":false,"inputs":[],"name":"PurchaseConfirmed","type":"event"},{"anonymous":false,"inputs":[],"name":"ItemReceived","type":"event"},{"anonymous":false,"inputs":[],"name":"Refunded","type":"event"}]`
	var code = common.Hex2Bytes("6060604052361561006c5760e060020a600035046308551a53811461007457806335a063b4146100865780633fa4f245146100a6578063590e1ae3146100af5780637150d8ae146100cf57806373fac6f0146100e1578063c19d93fb146100fe578063d696069714610112575b610131610002565b610133600154600160a060020a031681565b610131600154600160a060020a0390811633919091161461015057610002565b61014660005481565b610131600154600160a060020a039081163391909116146102d557610002565b610133600254600160a060020a031681565b610131600254600160a060020a0333811691161461023757610002565b61014660025460ff60a060020a9091041681565b61013160025460009060ff60a060020a9091041681146101cc57610002565b005b600160a060020a03166060908152602090f35b6060908152602090f35b60025460009060a060020a900460ff16811461016b57610002565b600154600160a060020a03908116908290301631606082818181858883f150506002805460a060020a60ff02191660a160020a179055506040517f72c874aeff0b183a56e2b79c71b46e1aed4dee5e09862134b8821ba2fddbf8bf9250a150565b80546002023414806101dd57610002565b6002805460a060020a60ff021973ffffffffffffffffffffffffffffffffffffffff1990911633171660a060020a1790557fd5d55c8a68912e9a110618df8d5e2e83b8d83211c57a8ddd1203df92885dc881826060a15050565b60025460019060a060020a900460ff16811461025257610002565b60025460008054600160a060020a0390921691606082818181858883f150508354604051600160a060020a0391821694503090911631915082818181858883f150506002805460a060020a60ff02191660a160020a179055506040517fe89152acd703c9d8c7d28829d443260b411454d45394e7995815140c8cbcbcf79250a150565b60025460019060a060020a900460ff1681146102f057610002565b6002805460008054600160a060020a0390921692909102606082818181858883f150508354604051600160a060020a0391821694503090911631915082818181858883f150506002805460a060020a60ff02191660a160020a179055506040517f8616bbbbad963e4e65b1366f1d75dfb63f9e9704bbbf91fb01bec70849906cf79250a15056")

	abi, err := abi.JSON(strings.NewReader(definition))
	if err != nil {
		b.Fatal(err)
	}

	cpurchase, err := abi.Pack("confirmPurchase")
	if err != nil {
		b.Fatal(err)
	}
	creceived, err := abi.Pack("confirmReceived")
	if err != nil {
		b.Fatal(err)
	}
	refund, err := abi.Pack("refund")
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := 0; j < 400; j++ {
			Execute(code, cpurchase, nil)
			Execute(code, creceived, nil)
			Execute(code, refund, nil)
		}
	}
}

func benchmarkEVM_Create(bench *testing.B, code string) {
	var (
		db       = statedb.NewCacheDB(storage.NewMemLevelDBStore())
		statedb  = statedb.NewStateDB(db, common.Hash{}, common.Hash{}, statedb.NewDummy())
		sender   = common.BytesToAddress([]byte("sender"))
		receiver = common.BytesToAddress([]byte("receiver"))
	)

	statedb.CreateAccount(sender)
	statedb.SetCode(receiver, common.FromHex(code))
	runtimeConfig := Config{
		Origin:      sender,
		State:       statedb,
		GasLimit:    10000000,
		Difficulty:  big.NewInt(0x200000),
		Time:        new(big.Int).SetUint64(0),
		Coinbase:    common.Address{},
		BlockNumber: new(big.Int).SetUint64(1),
		ChainConfig: &params.ChainConfig{
			ChainID:             big.NewInt(1),
			HomesteadBlock:      new(big.Int),
			ByzantiumBlock:      new(big.Int),
			ConstantinopleBlock: new(big.Int),
			DAOForkBlock:        new(big.Int),
			DAOForkSupport:      false,
			EIP150Block:         new(big.Int),
			EIP155Block:         new(big.Int),
			EIP158Block:         new(big.Int),
		},
		EVMConfig: evm.Config{},
	}
	// Warm up the intpools and stuff
	bench.ResetTimer()
	for i := 0; i < bench.N; i++ {
		Call(receiver, []byte{}, &runtimeConfig)
	}
	bench.StopTimer()
}

func BenchmarkEVM_CREATE_500(bench *testing.B) {
	// initcode size 500K, repeatedly calls CREATE and then modifies the mem contents
	benchmarkEVM_Create(bench, "5b6207a120600080f0600152600056")
}
func BenchmarkEVM_CREATE2_500(bench *testing.B) {
	// initcode size 500K, repeatedly calls CREATE2 and then modifies the mem contents
	benchmarkEVM_Create(bench, "5b586207a120600080f5600152600056")
}
func BenchmarkEVM_CREATE_1200(bench *testing.B) {
	// initcode size 1200K, repeatedly calls CREATE and then modifies the mem contents
	benchmarkEVM_Create(bench, "5b62124f80600080f0600152600056")
}
func BenchmarkEVM_CREATE2_1200(bench *testing.B) {
	// initcode size 1200K, repeatedly calls CREATE2 and then modifies the mem contents
	benchmarkEVM_Create(bench, "5b5862124f80600080f5600152600056")
}

func fakeHeader(n uint64, parentHash common.Hash) *types.Header {
	header := types.Header{
		Coinbase:   common.HexToAddress("0x00000000000000000000000000000000deadbeef"),
		Number:     big.NewInt(int64(n)),
		ParentHash: parentHash,
		Time:       1000,
		Nonce:      types.BlockNonce{0x1},
		Extra:      []byte{},
		Difficulty: big.NewInt(0),
		GasLimit:   100000,
	}
	return &header
}

type dummyChain struct {
	counter int
}

// Engine retrieves the chain's consensus engine.
func (d *dummyChain) Engine() consensus.Engine {
	return nil
}

// GetHeader returns the hash corresponding to their hash.
func (d *dummyChain) GetHeader(h common.Hash, n uint64) *types.Header {
	d.counter++
	parentHash := common.Hash{}
	s := common.LeftPadBytes(big.NewInt(int64(n-1)).Bytes(), 32)
	copy(parentHash[:], s)

	//parentHash := common.Hash{byte(n - 1)}
	//fmt.Printf("GetHeader(%x, %d) => header with parent %x\n", h, n, parentHash)
	return fakeHeader(n, parentHash)
}

// TestBlockhash tests the blockhash operation. It's a bit special, since it internally
// requires access to a chain reader.
func TestBlockhash(t *testing.T) {
	// Current head
	n := uint64(1000)
	parentHash := common.Hash{}
	s := common.LeftPadBytes(big.NewInt(int64(n-1)).Bytes(), 32)
	copy(parentHash[:], s)
	header := fakeHeader(n, parentHash)

	// This is the contract we're using. It requests the blockhash for current num (should be all zeroes),
	// then iteratively fetches all blockhashes back to n-260.
	// It returns
	// 1. the first (should be zero)
	// 2. the second (should be the parent hash)
	// 3. the last non-zero hash
	// By making the chain reader return hashes which correlate to the number, we can
	// verify that it obtained the right hashes where it should

	/*
		pragma solidity ^0.5.3;
		contract Hasher{
			function test() public view returns (bytes32, bytes32, bytes32){
				uint256 x = block.number;
				bytes32 first;
				bytes32 last;
				bytes32 zero;
				zero = blockhash(x); // Should be zeroes
				first = blockhash(x-1);
				for(uint256 i = 2 ; i < 260; i++){
					bytes32 hash = blockhash(x - i);
					if (uint256(hash) != 0){
						last = hash;
					}
				}
				return (zero, first, last);
			}
		}
	*/
	// The contract above
	data := common.Hex2Bytes("6080604052348015600f57600080fd5b50600436106045576000357c010000000000000000000000000000000000000000000000000000000090048063f8a8fd6d14604a575b600080fd5b60506074565b60405180848152602001838152602001828152602001935050505060405180910390f35b600080600080439050600080600083409050600184034092506000600290505b61010481101560c35760008186034090506000816001900414151560b6578093505b5080806001019150506094565b508083839650965096505050505090919256fea165627a7a72305820462d71b510c1725ff35946c20b415b0d50b468ea157c8c77dff9466c9cb85f560029")
	// The method call to 'test()'
	input := common.Hex2Bytes("f8a8fd6d")
	chain := &dummyChain{}
	ret, _, err := Execute(data, input, &Config{
		GetHashFn:   core.GetHashFn(header, chain),
		BlockNumber: new(big.Int).Set(header.Number),
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(ret) != 96 {
		t.Fatalf("expected returndata to be 96 bytes, got %d", len(ret))
	}

	zero := new(big.Int).SetBytes(ret[0:32])
	first := new(big.Int).SetBytes(ret[32:64])
	last := new(big.Int).SetBytes(ret[64:96])
	if zero.BitLen() != 0 {
		t.Fatalf("expected zeroes, got %x", ret[0:32])
	}
	if first.Uint64() != 999 {
		t.Fatalf("second block should be 999, got %d (%x)", first, ret[32:64])
	}
	if last.Uint64() != 744 {
		t.Fatalf("last block should be 744, got %d (%x)", last, ret[64:96])
	}
	if exp, got := 255, chain.counter; exp != got {
		t.Errorf("suboptimal; too much chain iteration, expected %d, got %d", exp, got)
	}
}

type stepCounter struct {
	inner *evm.JSONLogger
	steps int
}

func (s *stepCounter) CaptureStart(from common.Address, to common.Address, create bool, input []byte, gas uint64, value *big.Int) {
}

func (s *stepCounter) CaptureState(env *evm.EVM, pc uint64, op evm.OpCode, gas, cost uint64,
	memory *evm.Memory, stack *evm.Stack, rStack *evm.ReturnStack, rData []byte,
	contract *evm.Contract, depth int, err error) {
	s.steps++
	// Enable this for more output
	// s.inner.CaptureState(env, pc, op, gas, cost, memory, stack, rStack, contract, depth, err)
}

func (s *stepCounter) CaptureFault(env *evm.EVM, pc uint64, op evm.OpCode, gas, cost uint64,
	memory *evm.Memory, stack *evm.Stack, rStack *evm.ReturnStack, contract *evm.Contract, depth int, err error) {
}

func (s *stepCounter) CaptureEnd(output []byte, gasUsed uint64, t time.Duration, err error) {
}

func TestJumpSub1024Limit(t *testing.T) {
	db := statedb.NewCacheDB(storage.NewMemLevelDBStore())
	statedb := statedb.NewStateDB(db, common.Hash{}, common.Hash{}, statedb.NewDummy())
	address := common.HexToAddress("0x0a")
	// Code is
	// 0 beginsub
	// 1 push 0
	// 3 jumpsub
	//
	// The code recursively calls itself. It should error when the returns-stack
	// grows above 1023
	statedb.SetCode(address, []byte{
		byte(evm.PUSH1), 3,
		byte(evm.JUMPSUB),
		byte(evm.BEGINSUB),
		byte(evm.PUSH1), 3,
		byte(evm.JUMPSUB),
	})
	tracer := stepCounter{inner: evm.NewJSONLogger(nil, os.Stdout)}
	// Enable 2315
	_, _, err := Call(address, nil, &Config{State: statedb,
		GasLimit:    20000,
		ChainConfig: params.AllEthashProtocolChanges,
		EVMConfig: evm.Config{
			ExtraEips: []int{2315},
			Debug:     true,
			Tracer:    &tracer,
		}})
	exp := "return stack limit reached"
	if err.Error() != exp {
		t.Fatalf("expected %v, got %v", exp, err)
	}
	if exp, got := 2048, tracer.steps; exp != got {
		t.Fatalf("expected %d steps, got %d", exp, got)
	}
}

func TestReturnSubShallow(t *testing.T) {
	db := statedb.NewCacheDB(storage.NewMemLevelDBStore())
	statedb := statedb.NewStateDB(db, common.Hash{}, common.Hash{}, statedb.NewDummy())
	address := common.HexToAddress("0x0a")
	// The code does returnsub without having anything on the returnstack.
	// It should not panic, but just fail after one step
	statedb.SetCode(address, []byte{
		byte(evm.PUSH1), 5,
		byte(evm.JUMPSUB),
		byte(evm.RETURNSUB),
		byte(evm.PC),
		byte(evm.BEGINSUB),
		byte(evm.RETURNSUB),
		byte(evm.PC),
	})
	tracer := stepCounter{}

	// Enable 2315
	_, _, err := Call(address, nil, &Config{State: statedb,
		GasLimit:    10000,
		ChainConfig: params.AllEthashProtocolChanges,
		EVMConfig: evm.Config{
			ExtraEips: []int{2315},
			Debug:     true,
			Tracer:    &tracer,
		}})

	exp := "invalid retsub"
	if err.Error() != exp {
		t.Fatalf("expected %v, got %v", exp, err)
	}
	if exp, got := 4, tracer.steps; exp != got {
		t.Fatalf("expected %d steps, got %d", exp, got)
	}
}

func TestReturnCases(t *testing.T) {
	cfg := &Config{
		EVMConfig: evm.Config{
			Debug:     true,
			Tracer:    evm.NewMarkdownLogger(nil, os.Stdout),
			ExtraEips: []int{2315},
		},
	}
	// This should fail at first opcode
	Execute([]byte{
		byte(evm.RETURNSUB),
		byte(evm.PC),
		byte(evm.PC),
	}, nil, cfg)

	// Should also fail
	Execute([]byte{
		byte(evm.PUSH1), 5,
		byte(evm.JUMPSUB),
		byte(evm.RETURNSUB),
		byte(evm.PC),
		byte(evm.BEGINSUB),
		byte(evm.RETURNSUB),
		byte(evm.PC),
	}, nil, cfg)

	// This should complete
	Execute([]byte{
		byte(evm.PUSH1), 0x4,
		byte(evm.JUMPSUB),
		byte(evm.STOP),
		byte(evm.BEGINSUB),
		byte(evm.PUSH1), 0x9,
		byte(evm.JUMPSUB),
		byte(evm.RETURNSUB),
		byte(evm.BEGINSUB),
		byte(evm.RETURNSUB),
	}, nil, cfg)
}

// DisabledTestEipExampleCases contains various testcases that are used for the
// EIP examples
func TestEipExampleCases(t *testing.T) {
	cfg := &Config{
		EVMConfig: evm.Config{
			Debug:     true,
			Tracer:    evm.NewMarkdownLogger(nil, os.Stdout),
			ExtraEips: []int{2315},
		},
	}
	prettyPrint := func(comment string, code []byte) {
		instrs := make([]string, 0)
		it := asm.NewInstructionIterator(code)
		for it.Next() {
			if it.Arg() != nil && 0 < len(it.Arg()) {
				instrs = append(instrs, fmt.Sprintf("%v 0x%x", it.Op(), it.Arg()))
			} else {
				instrs = append(instrs, fmt.Sprintf("%v", it.Op()))
			}
		}
		ops := strings.Join(instrs, ", ")

		fmt.Printf("%v\nBytecode: `0x%x` (`%v`)\n",
			comment,
			code, ops)
		Execute(code, nil, cfg)
	}

	{ // First eip testcase
		code := []byte{
			byte(evm.PUSH1), 4,
			byte(evm.JUMPSUB),
			byte(evm.STOP),
			byte(evm.BEGINSUB),
			byte(evm.RETURNSUB),
		}
		prettyPrint("This should jump into a subroutine, back out and stop.", code)
	}

	{
		code := []byte{
			byte(evm.PUSH9), 0x00, 0x00, 0x00, 0x00, 0x0, 0x00, 0x00, 0x00, (4 + 8),
			byte(evm.JUMPSUB),
			byte(evm.STOP),
			byte(evm.BEGINSUB),
			byte(evm.PUSH1), 8 + 9,
			byte(evm.JUMPSUB),
			byte(evm.RETURNSUB),
			byte(evm.BEGINSUB),
			byte(evm.RETURNSUB),
		}
		prettyPrint("This should execute fine, going into one two depths of subroutines", code)
	}
	// TODO move this test into an actual test, which not only prints
	// out the trace.
	{
		code := []byte{
			byte(evm.PUSH9), 0x01, 0x00, 0x00, 0x00, 0x0, 0x00, 0x00, 0x00, (4 + 8),
			byte(evm.JUMPSUB),
			byte(evm.STOP),
			byte(evm.BEGINSUB),
			byte(evm.PUSH1), 8 + 9,
			byte(evm.JUMPSUB),
			byte(evm.RETURNSUB),
			byte(evm.BEGINSUB),
			byte(evm.RETURNSUB),
		}
		prettyPrint("This should fail, since the given location is outside of the "+
			"code-range. The code is the same as previous example, except that the "+
			"pushed location is `0x01000000000000000c` instead of `0x0c`.", code)
	}
	{
		// This should fail at first opcode
		code := []byte{
			byte(evm.RETURNSUB),
			byte(evm.PC),
			byte(evm.PC),
		}
		prettyPrint("This should fail at first opcode, due to shallow `return_stack`", code)

	}
	{
		code := []byte{
			byte(evm.PUSH1), 5, // Jump past the subroutine
			byte(evm.JUMP),
			byte(evm.BEGINSUB),
			byte(evm.RETURNSUB),
			byte(evm.JUMPDEST),
			byte(evm.PUSH1), 3, // Now invoke the subroutine
			byte(evm.JUMPSUB),
		}
		prettyPrint("In this example. the JUMPSUB is on the last byte of code. When the "+
			"subroutine returns, it should hit the 'virtual stop' _after_ the bytecode, "+
			"and not exit with error", code)
	}

	{
		code := []byte{
			byte(evm.BEGINSUB),
			byte(evm.RETURNSUB),
			byte(evm.STOP),
		}
		prettyPrint("In this example, the code 'walks' into a subroutine, which is not "+
			"allowed, and causes an error", code)
	}
}

// benchmarkNonModifyingCode benchmarks code, but if the code modifies the
// state, this should not be used, since it does not reset the state between runs.
func benchmarkNonModifyingCode(gas uint64, code []byte, name string, b *testing.B) {
	cfg := new(Config)
	setDefaults(cfg)
	db := statedb.NewCacheDB(storage.NewMemLevelDBStore())
	cfg.State = statedb.NewStateDB(db, common.Hash{}, common.Hash{}, statedb.NewDummy())
	cfg.GasLimit = gas
	var (
		destination = common.BytesToAddress([]byte("contract"))
		vmenv       = NewEnv(cfg)
		sender      = evm.AccountRef(cfg.Origin)
	)
	cfg.State.CreateAccount(destination)
	eoa := common.HexToAddress("E0")
	{
		cfg.State.CreateAccount(eoa)
		cfg.State.SetNonce(eoa, 100)
	}
	reverting := common.HexToAddress("EE")
	{
		cfg.State.CreateAccount(reverting)
		cfg.State.SetCode(reverting, []byte{
			byte(evm.PUSH1), 0x00,
			byte(evm.PUSH1), 0x00,
			byte(evm.REVERT),
		})
	}

	// cfg.State.CreateAccount(cfg.Origin)
	// set the receiver's (the executing contract) code for execution.
	cfg.State.SetCode(destination, code)
	vmenv.Call(sender, destination, nil, gas, cfg.Value)

	b.Run(name, func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			vmenv.Call(sender, destination, nil, gas, cfg.Value)
		}
	})
}

// BenchmarkSimpleLoop test a pretty simple loop which loops until OOG
// 55 ms
func BenchmarkSimpleLoop(b *testing.B) {
	staticCallIdentity := []byte{
		byte(evm.JUMPDEST), //  [ count ]
		// push args for the call
		byte(evm.PUSH1), 0, // out size
		byte(evm.DUP1),       // out offset
		byte(evm.DUP1),       // out insize
		byte(evm.DUP1),       // in offset
		byte(evm.PUSH1), 0x4, // address of identity
		byte(evm.GAS), // gas
		byte(evm.STATICCALL),
		byte(evm.POP),      // pop return value
		byte(evm.PUSH1), 0, // jumpdestination
		byte(evm.JUMP),
	}

	callIdentity := []byte{
		byte(evm.JUMPDEST), //  [ count ]
		// push args for the call
		byte(evm.PUSH1), 0, // out size
		byte(evm.DUP1),       // out offset
		byte(evm.DUP1),       // out insize
		byte(evm.DUP1),       // in offset
		byte(evm.DUP1),       // value
		byte(evm.PUSH1), 0x4, // address of identity
		byte(evm.GAS), // gas
		byte(evm.CALL),
		byte(evm.POP),      // pop return value
		byte(evm.PUSH1), 0, // jumpdestination
		byte(evm.JUMP),
	}

	callInexistant := []byte{
		byte(evm.JUMPDEST), //  [ count ]
		// push args for the call
		byte(evm.PUSH1), 0, // out size
		byte(evm.DUP1),        // out offset
		byte(evm.DUP1),        // out insize
		byte(evm.DUP1),        // in offset
		byte(evm.DUP1),        // value
		byte(evm.PUSH1), 0xff, // address of existing contract
		byte(evm.GAS), // gas
		byte(evm.CALL),
		byte(evm.POP),      // pop return value
		byte(evm.PUSH1), 0, // jumpdestination
		byte(evm.JUMP),
	}

	callEOA := []byte{
		byte(evm.JUMPDEST), //  [ count ]
		// push args for the call
		byte(evm.PUSH1), 0, // out size
		byte(evm.DUP1),        // out offset
		byte(evm.DUP1),        // out insize
		byte(evm.DUP1),        // in offset
		byte(evm.DUP1),        // value
		byte(evm.PUSH1), 0xE0, // address of EOA
		byte(evm.GAS), // gas
		byte(evm.CALL),
		byte(evm.POP),      // pop return value
		byte(evm.PUSH1), 0, // jumpdestination
		byte(evm.JUMP),
	}

	loopingCode := []byte{
		byte(evm.JUMPDEST), //  [ count ]
		// push args for the call
		byte(evm.PUSH1), 0, // out size
		byte(evm.DUP1),       // out offset
		byte(evm.DUP1),       // out insize
		byte(evm.DUP1),       // in offset
		byte(evm.PUSH1), 0x4, // address of identity
		byte(evm.GAS), // gas

		byte(evm.POP), byte(evm.POP), byte(evm.POP), byte(evm.POP), byte(evm.POP), byte(evm.POP),
		byte(evm.PUSH1), 0, // jumpdestination
		byte(evm.JUMP),
	}

	calllRevertingContractWithInput := []byte{
		byte(evm.JUMPDEST), //
		// push args for the call
		byte(evm.PUSH1), 0, // out size
		byte(evm.DUP1),        // out offset
		byte(evm.PUSH1), 0x20, // in size
		byte(evm.PUSH1), 0x00, // in offset
		byte(evm.PUSH1), 0x00, // value
		byte(evm.PUSH1), 0xEE, // address of reverting contract
		byte(evm.GAS), // gas
		byte(evm.CALL),
		byte(evm.POP),      // pop return value
		byte(evm.PUSH1), 0, // jumpdestination
		byte(evm.JUMP),
	}

	//100M gas
	benchmarkNonModifyingCode(100000000, staticCallIdentity, "staticcall-identity-100M", b)
	benchmarkNonModifyingCode(100000000, callIdentity, "call-identity-100M", b)
	benchmarkNonModifyingCode(100000000, loopingCode, "loop-100M", b)
	benchmarkNonModifyingCode(100000000, callInexistant, "call-nonexist-100M", b)
	benchmarkNonModifyingCode(100000000, callEOA, "call-EOA-100M", b)
	benchmarkNonModifyingCode(100000000, calllRevertingContractWithInput, "call-reverting-100M", b)
}

// TestEip2929Cases contains various testcases that are used for
// EIP-2929 about gas repricings
func TestEip2929Cases(t *testing.T) {
	id := 1
	prettyPrint := func(comment string, code []byte) {
		instrs := make([]string, 0)
		it := asm.NewInstructionIterator(code)
		for it.Next() {
			if it.Arg() != nil && 0 < len(it.Arg()) {
				instrs = append(instrs, fmt.Sprintf("%v 0x%x", it.Op(), it.Arg()))
			} else {
				instrs = append(instrs, fmt.Sprintf("%v", it.Op()))
			}
		}
		ops := strings.Join(instrs, ", ")
		fmt.Printf("### Case %d\n\n", id)
		id++
		fmt.Printf("%v\n\nBytecode: \n```\n0x%x\n```\nOperations: \n```\n%v\n```\n\n",
			comment,
			code, ops)
		Execute(code, nil, &Config{
			EVMConfig: evm.Config{
				Debug:     true,
				Tracer:    evm.NewMarkdownLogger(nil, os.Stdout),
				ExtraEips: []int{2929},
			},
		})
	}

	{ // First eip testcase
		code := []byte{
			// Three checks against a precompile
			byte(evm.PUSH1), 1, byte(evm.EXTCODEHASH), byte(evm.POP),
			byte(evm.PUSH1), 2, byte(evm.EXTCODESIZE), byte(evm.POP),
			byte(evm.PUSH1), 3, byte(evm.BALANCE), byte(evm.POP),
			// Three checks against a non-precompile
			byte(evm.PUSH1), 0xf1, byte(evm.EXTCODEHASH), byte(evm.POP),
			byte(evm.PUSH1), 0xf2, byte(evm.EXTCODESIZE), byte(evm.POP),
			byte(evm.PUSH1), 0xf3, byte(evm.BALANCE), byte(evm.POP),
			// Same three checks (should be cheaper)
			byte(evm.PUSH1), 0xf2, byte(evm.EXTCODEHASH), byte(evm.POP),
			byte(evm.PUSH1), 0xf3, byte(evm.EXTCODESIZE), byte(evm.POP),
			byte(evm.PUSH1), 0xf1, byte(evm.BALANCE), byte(evm.POP),
			// Check the origin, and the 'this'
			byte(evm.ORIGIN), byte(evm.BALANCE), byte(evm.POP),
			byte(evm.ADDRESS), byte(evm.BALANCE), byte(evm.POP),

			byte(evm.STOP),
		}
		prettyPrint("This checks `EXT`(codehash,codesize,balance) of precompiles, which should be `100`, "+
			"and later checks the same operations twice against some non-precompiles. "+
			"Those are cheaper second time they are accessed. Lastly, it checks the `BALANCE` of `origin` and `this`.", code)
	}

	{ // EXTCODECOPY
		code := []byte{
			// extcodecopy( 0xff,0,0,0,0)
			byte(evm.PUSH1), 0x00, byte(evm.PUSH1), 0x00, byte(evm.PUSH1), 0x00, //length, codeoffset, memoffset
			byte(evm.PUSH1), 0xff, byte(evm.EXTCODECOPY),
			// extcodecopy( 0xff,0,0,0,0)
			byte(evm.PUSH1), 0x00, byte(evm.PUSH1), 0x00, byte(evm.PUSH1), 0x00, //length, codeoffset, memoffset
			byte(evm.PUSH1), 0xff, byte(evm.EXTCODECOPY),
			// extcodecopy( this,0,0,0,0)
			byte(evm.PUSH1), 0x00, byte(evm.PUSH1), 0x00, byte(evm.PUSH1), 0x00, //length, codeoffset, memoffset
			byte(evm.ADDRESS), byte(evm.EXTCODECOPY),

			byte(evm.STOP),
		}
		prettyPrint("This checks `extcodecopy( 0xff,0,0,0,0)` twice, (should be expensive first time), "+
			"and then does `extcodecopy( this,0,0,0,0)`.", code)
	}

	{ // SLOAD + SSTORE
		code := []byte{
			// Add slot `0x1` to access list
			byte(evm.PUSH1), 0x01, byte(evm.SLOAD), byte(evm.POP), // SLOAD( 0x1) (add to access list)
			// Write to `0x1` which is already in access list
			byte(evm.PUSH1), 0x11, byte(evm.PUSH1), 0x01, byte(evm.SSTORE), // SSTORE( loc: 0x01, val: 0x11)
			// Write to `0x2` which is not in access list
			byte(evm.PUSH1), 0x11, byte(evm.PUSH1), 0x02, byte(evm.SSTORE), // SSTORE( loc: 0x02, val: 0x11)
			// Write again to `0x2`
			byte(evm.PUSH1), 0x11, byte(evm.PUSH1), 0x02, byte(evm.SSTORE), // SSTORE( loc: 0x02, val: 0x11)
			// Read slot in access list (0x2)
			byte(evm.PUSH1), 0x02, byte(evm.SLOAD), // SLOAD( 0x2)
			// Read slot in access list (0x1)
			byte(evm.PUSH1), 0x01, byte(evm.SLOAD), // SLOAD( 0x1)
		}
		prettyPrint("This checks `sload( 0x1)` followed by `sstore(loc: 0x01, val:0x11)`, then 'naked' sstore:"+
			"`sstore(loc: 0x02, val:0x11)` twice, and `sload(0x2)`, `sload(0x1)`. ", code)
	}
	{ // Call variants
		code := []byte{
			// identity precompile
			byte(evm.PUSH1), 0x0, byte(evm.DUP1), byte(evm.DUP1), byte(evm.DUP1), byte(evm.DUP1),
			byte(evm.PUSH1), 0x04, byte(evm.PUSH1), 0x0, byte(evm.CALL), byte(evm.POP),

			// random account - call 1
			byte(evm.PUSH1), 0x0, byte(evm.DUP1), byte(evm.DUP1), byte(evm.DUP1), byte(evm.DUP1),
			byte(evm.PUSH1), 0xff, byte(evm.PUSH1), 0x0, byte(evm.CALL), byte(evm.POP),

			// random account - call 2
			byte(evm.PUSH1), 0x0, byte(evm.DUP1), byte(evm.DUP1), byte(evm.DUP1), byte(evm.DUP1),
			byte(evm.PUSH1), 0xff, byte(evm.PUSH1), 0x0, byte(evm.STATICCALL), byte(evm.POP),
		}
		prettyPrint("This calls the `identity`-precompile (cheap), then calls an account (expensive) and `staticcall`s the same"+
			"account (cheap)", code)
	}
}
