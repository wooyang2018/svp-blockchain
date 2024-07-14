// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package execution

import (
	"encoding/json"
	"math/big"
	"runtime"
	"testing"
	"time"

	ethcomm "github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"

	"github.com/wooyang2018/svp-blockchain/core"
	"github.com/wooyang2018/svp-blockchain/evm/statedb"
	"github.com/wooyang2018/svp-blockchain/execution/common"
	"github.com/wooyang2018/svp-blockchain/execution/evm"
	"github.com/wooyang2018/svp-blockchain/native"
	"github.com/wooyang2018/svp-blockchain/native/pcoin"
	"github.com/wooyang2018/svp-blockchain/native/taddr"
	"github.com/wooyang2018/svp-blockchain/native/xcoin"
)

const storageSolPath = "../evm/testdata/contracts/Storage.sol"

func newTestExecution() (*common.MemStateStore, *Execution) {
	state := common.NewMemStateStore()
	reg := newCodeRegistry()
	reg.registerDriver(common.DriverTypeNative, native.NewCodeDriver())
	execution := &Execution{
		stateStore:   state,
		codeRegistry: reg,
		config:       DefaultConfig,
	}
	execution.config.TxExecTimeout = 1 * time.Second
	return state, execution
}

// newGenesisTxs initializes count of accounts, each with an initial balance of 100,
// returning a list of private keys and two genesis transactions for XCoin and TAddr.
func newGenesisTxs(count int) ([]*core.Transaction, []*core.PrivateKey) {
	values0 := make(map[string]uint64)
	values1 := make([]string, count)
	priKeys := make([]*core.PrivateKey, count)
	for i := 0; i < count; i++ {
		priKeys[i] = core.GenerateKey(nil)
		values0[priKeys[i].PublicKey().String()] = 100
		values1[i] = priKeys[i].PublicKey().String()
	}

	input0 := makeNativeDepInput(native.CodeXCoin)
	input0.InitInput, _ = json.Marshal(xcoin.InitInput{values0})
	b0, _ := json.Marshal(input0)
	tx0 := core.NewTransaction().
		SetNonce(time.Now().UnixNano()).
		SetInput(b0).
		Sign(priKeys[0])

	input1 := makeNativeDepInput(native.CodeTAddr)
	input1.InitInput, _ = json.Marshal(taddr.InitInput{values1})
	b1, _ := json.Marshal(input1)
	tx1 := core.NewTransaction().
		SetNonce(time.Now().UnixNano()).
		SetInput(b1).
		Sign(priKeys[0])

	common.RegisterCode(native.FileCodeXCoin, tx0.Hash())
	common.RegisterCode(native.FileCodeTAddr, tx1.Hash())
	return []*core.Transaction{tx0, tx1}, priKeys
}

func newEVMRunner(state *common.MemStateStore, execution *Execution, codePath string) (*evm.Runner, *common.StateTracker) {
	nativeDriver, _ := execution.codeRegistry.getDriver(common.DriverTypeNative)
	cache := statedb.NewCacheDB(state.Storage)
	proxy := evm.NewNativeProxy(nativeDriver)
	runner := &evm.Runner{
		CodePath: codePath,
		Proxy:    proxy,
		Storage:  state.Storage,
		StateDB:  statedb.NewStateDB(cache, ethcomm.Hash{}, ethcomm.Hash{}, proxy),
	}
	txTrk := common.NewStateTracker(state, nil)
	runner.SetTxTrk(txTrk)
	return runner, txTrk
}

func executeBlock(t *testing.T, state *common.MemStateStore, execution *Execution, blk *core.Block, txs []*core.Transaction) {
	asrt := assert.New(t)
	bcm, txcs := execution.Execute(blk, txs)
	asrt.Equal(blk.Hash(), bcm.Hash())
	asrt.EqualValues(len(txs), len(txcs))
	asrt.NotEmpty(bcm.StateChanges())
	for i := range txs {
		asrt.Equal(txs[i].Hash(), txcs[i].Hash())
	}
	for _, sc := range bcm.StateChanges() {
		state.SetState(sc.Key(), sc.Value())
	}
}

func TestExecution(t *testing.T) {
	asrt := assert.New(t)

	depInput1 := makeNativeDepInput(native.CodePCoin)
	input1, _ := json.Marshal(depInput1)
	depInput2 := makeNativeDepInput([]byte{2, 2, 2}) // invalid code id
	input2, _ := json.Marshal(depInput2)

	priv := core.GenerateKey(nil)
	tx1 := core.NewTransaction().SetNonce(time.Now().Unix()).SetInput(input1).Sign(priv)
	tx2 := core.NewTransaction().SetNonce(time.Now().Unix()).SetInput(input2).Sign(priv)
	tx3 := core.NewTransaction().SetNonce(time.Now().Unix()).SetInput(input1).Sign(priv)
	txs := []*core.Transaction{tx1, tx2, tx3}

	state, execution := newTestExecution()
	blk := core.NewBlock().SetHeight(10).Sign(priv)
	executeBlock(t, state, execution, blk, txs)

	reg := execution.codeRegistry
	regTrk := common.NewStateTracker(state, codeRegistryAddr)
	codeInfo, err := reg.getCodeInfo(tx1.Hash(), regTrk)
	asrt.NoError(err)
	asrt.Equal(native.CodePCoin, codeInfo.CodeID)

	codeInfo, err = reg.getCodeInfo(tx2.Hash(), regTrk)
	asrt.Error(err)
	asrt.Nil(codeInfo)

	codeInfo, err = reg.getCodeInfo(tx3.Hash(), regTrk)
	asrt.NoError(err)
	asrt.Equal(native.CodePCoin, codeInfo.CodeID)

	ccInput, _ := json.Marshal(pcoin.Input{Method: "minter"})
	minter, err := execution.Query(&common.QueryData{tx1.Hash(), ccInput, nil})
	asrt.NoError(err)
	asrt.Equal(priv.PublicKey().Bytes(), minter)

	minter, err = execution.Query(&common.QueryData{tx2.Hash(), ccInput, nil})
	asrt.Error(err)
	asrt.Nil(minter)

	minter, err = execution.Query(&common.QueryData{tx3.Hash(), ccInput, nil})
	asrt.NoError(err)
	asrt.Equal(priv.PublicKey().Bytes(), minter)
}

func TestEVMRunner(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("compiling contracts on windows is not supported")
	}
	asrt := assert.New(t)

	txs, priKeys := newGenesisTxs(4)
	state, execution := newTestExecution()
	runner, txTrk := newEVMRunner(state, execution, storageSolPath)
	sender := priKeys[0]
	blk := core.NewBlock().SetHeight(0).Sign(sender)
	executeBlock(t, state, execution, blk, txs)
	tx := core.NewTransaction().SetNonce(time.Now().Unix()).Sign(sender)
	invokeTrk := txTrk.Spawn(tx.Hash())

	ctx := &common.CallContextTx{
		StateTracker: invokeTrk,
		RawSender:    sender.PublicKey().Bytes(),
	}
	initInput := &evm.InitInput{Class: "Storage"}
	ctx.RawInput, _ = json.Marshal(initInput)
	err := runner.Init(ctx)
	asrt.NoError(err)
	txTrk.Merge(invokeTrk)

	input := &evm.Input{
		Method: "store",
		Params: []string{"1024"},
		Types:  []string{"uint256"},
	}
	ctx.RawInput, _ = json.Marshal(input)
	err = runner.Invoke(ctx)
	asrt.NoError(err)
	txTrk.Merge(invokeTrk)

	query := &common.CallContextQuery{
		StateGetter: invokeTrk,
		RawSender:   sender.PublicKey().Bytes(),
	}
	input = &evm.Input{Method: "retrieve"}
	query.RawInput, _ = json.Marshal(input)
	ret, err := runner.Query(query)
	asrt.NoError(err)
	asrt.EqualValues(big.NewInt(1024), big.NewInt(0).SetBytes(ret))
}

func TestEVMTransfer(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("compiling contracts on windows is not supported")
	}
	asrt := assert.New(t)

	txs, priKeys := newGenesisTxs(4)
	state, execution := newTestExecution()
	runner, txTrk := newEVMRunner(state, execution, storageSolPath)
	sender := priKeys[0]
	blk := core.NewBlock().SetHeight(0).Sign(sender)
	executeBlock(t, state, execution, blk, txs)
	tx := core.NewTransaction().SetNonce(time.Now().Unix()).Sign(sender)
	invokeTrk := txTrk.Spawn(tx.Hash())

	ctx := &common.CallContextTx{
		StateTracker: invokeTrk,
		RawSender:    sender.PublicKey().Bytes(),
	}
	initInput := &evm.InitInput{Class: "Storage"}
	ctx.RawInput, _ = json.Marshal(initInput)
	err := runner.Init(ctx)
	asrt.NoError(err)
	txTrk.Merge(invokeTrk)

	addr32, err := runner.Proxy.QueryContractAddr32(tx.Hash())
	asrt.NoError(err)
	err = runner.Proxy.SetBalanceByAddr32(addr32, 5000)
	asrt.NoError(err)

	input := &evm.Input{
		Method: "withdraw",
		Params: []string{"1000"},
		Types:  []string{"uint256"},
	}
	ctx.RawInput, _ = json.Marshal(input)
	err = runner.Invoke(ctx)
	asrt.NoError(err)
	txTrk.Merge(invokeTrk)

	balance, _ := runner.Proxy.QueryBalanceByAddr32(addr32)
	newVal := common.DecodeBalance(balance)
	asrt.EqualValues(4000, newVal)
	balance, _ = runner.Proxy.QueryBalanceByAddr32(sender.PublicKey().Bytes())
	newVal = common.DecodeBalance(balance)
	asrt.EqualValues(1100, newVal)
}
