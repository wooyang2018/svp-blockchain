// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package execution

import (
	"encoding/json"
	"math/big"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/wooyang2018/svp-blockchain/core"
	ethrunt "github.com/wooyang2018/svp-blockchain/evm/runtime"
	"github.com/wooyang2018/svp-blockchain/execution/common"
	"github.com/wooyang2018/svp-blockchain/execution/evm"
	"github.com/wooyang2018/svp-blockchain/native"
	"github.com/wooyang2018/svp-blockchain/native/pcoin"
	"github.com/wooyang2018/svp-blockchain/native/taddr"
	"github.com/wooyang2018/svp-blockchain/native/xcoin"
	"github.com/wooyang2018/svp-blockchain/storage"
)

func newTestExecution() (*common.MapStateStore, codeRegistry, *Execution) {
	state := common.NewMapStateStore()
	reg := newCodeRegistry()
	reg.registerDriver(common.DriverTypeNative, native.NewCodeDriver())

	execution := &Execution{
		stateStore:   state,
		codeRegistry: reg,
		config:       DefaultConfig,
	}
	execution.config.TxExecTimeout = 1 * time.Second
	return state, reg, execution
}

func TestExecution(t *testing.T) {
	asrt := assert.New(t)

	priv := core.GenerateKey(nil)
	blk := core.NewBlock().SetHeight(10).Sign(priv)

	cinfo := common.CodeInfo{
		DriverType: common.DriverTypeNative,
		CodeID:     native.CodePCoin,
	}
	cinfo2 := common.CodeInfo{
		DriverType: common.DriverTypeNative,
		CodeID:     []byte{2, 2, 2}, // invalid code id
	}

	depInput := &common.DeploymentInput{CodeInfo: cinfo}
	b, _ := json.Marshal(depInput)

	depInput.CodeInfo = cinfo2
	b2, _ := json.Marshal(depInput)

	tx1 := core.NewTransaction().SetNonce(time.Now().Unix()).SetInput(b).Sign(priv)
	tx2 := core.NewTransaction().SetNonce(time.Now().Unix()).SetInput(b2).Sign(priv)
	tx3 := core.NewTransaction().SetNonce(time.Now().Unix()).SetInput(b).Sign(priv)

	state, reg, execution := newTestExecution()
	bcm, txcs := execution.Execute(blk, []*core.Transaction{tx1, tx2, tx3})

	asrt.Equal(blk.Hash(), bcm.Hash())
	asrt.EqualValues(3, len(txcs))
	asrt.NotEmpty(bcm.StateChanges())

	asrt.Equal(tx1.Hash(), txcs[0].Hash())
	asrt.Equal(tx2.Hash(), txcs[1].Hash())
	asrt.Equal(tx3.Hash(), txcs[2].Hash())

	for _, sc := range bcm.StateChanges() {
		state.SetState(sc.Key(), sc.Value())
	}

	regTrk := common.NewStateTracker(state, codeRegistryAddr)
	resci, err := reg.getCodeInfo(tx1.Hash(), regTrk)
	asrt.NoError(err)
	asrt.Equal(&cinfo, resci)

	resci, err = reg.getCodeInfo(tx2.Hash(), regTrk)
	asrt.Error(err)
	asrt.Nil(resci)

	resci, err = reg.getCodeInfo(tx3.Hash(), regTrk)
	asrt.NoError(err)
	asrt.Equal(&cinfo, resci)

	ccInput, _ := json.Marshal(pcoin.Input{Method: "minter"})
	minter, err := execution.Query(&common.QueryData{tx1.Hash(), ccInput})
	asrt.NoError(err)
	asrt.Equal(priv.PublicKey().Bytes(), minter)

	minter, err = execution.Query(&common.QueryData{tx2.Hash(), ccInput})
	asrt.Error(err)
	asrt.Nil(minter)

	minter, err = execution.Query(&common.QueryData{tx3.Hash(), ccInput})
	asrt.NoError(err)
	asrt.Equal(priv.PublicKey().Bytes(), minter)
}

func genesisTxs() []*core.Transaction {
	count := 4
	values0 := make(map[string]uint64)
	values1 := make([]string, count)
	priKeys := make([]*core.PrivateKey, count)
	for i := 0; i < count; i++ {
		priKeys[i] = core.GenerateKey(nil)
		pubKey := priKeys[i].PublicKey()
		values0[pubKey.String()] = 100
		values1[i] = pubKey.String()
	}

	input0 := &common.DeploymentInput{
		CodeInfo: common.CodeInfo{
			DriverType: common.DriverTypeNative,
			CodeID:     native.CodeXCoin,
		},
	}
	input0.InitInput, _ = json.Marshal(xcoin.InitInput{values0})
	b0, _ := json.Marshal(input0)
	tx0 := core.NewTransaction().
		SetNonce(time.Now().UnixNano()).
		SetInput(b0).
		Sign(priKeys[0])

	input1 := &common.DeploymentInput{
		CodeInfo: common.CodeInfo{
			DriverType: common.DriverTypeNative,
			CodeID:     native.CodeTAddr,
		},
	}
	input1.InitInput, _ = json.Marshal(taddr.InitInput{values1})
	b1, _ := json.Marshal(input1)
	tx1 := core.NewTransaction().
		SetNonce(time.Now().UnixNano()).
		SetInput(b1).
		Sign(priKeys[0])

	common.RegisterCode(native.FileCodeXCoin, tx0.Hash())
	common.RegisterCode(native.FileCodeTAddr, tx1.Hash())

	return []*core.Transaction{tx0, tx1}
}

func makeInitCtx() *common.MockCallContext {
	ctx := new(common.MockCallContext)
	ctx.MockState = common.NewMockState()
	ctx.MockSender = core.GenerateKey(nil).PublicKey().Bytes()
	ctx.MockInput, _ = json.Marshal(nil)
	return ctx
}

func TestEVMInvoke(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("compiling contracts on windows is not supported")
	}
	asrt := assert.New(t)
	compiled := ethrunt.CompileCode("../evm/testdata/contracts/Storage.sol")

	ctx := makeInitCtx()

	txs := genesisTxs()
	state, _, execution := newTestExecution()

	priv := core.GenerateKey(nil)
	blk := core.NewBlock().SetHeight(10).Sign(priv)
	bcm, txcs := execution.Execute(blk, txs)

	asrt.Equal(blk.Hash(), bcm.Hash())
	asrt.EqualValues(2, len(txcs))

	runner := evm.NewRunner(native.NewCodeDriver(), storage.NewMemLevelDBStore(), common.NewStateTracker(state, nil))
	runner.Build(compiled["Storage"][1], compiled["Storage"][0], ctx)

	err := runner.Init(ctx)
	asrt.NoError(err)
	// runner.jsonABI, runner.hexCode = "", "" // subsequent invokes do not need to pass in jsonABI and hexCode

	input := &evm.Input{
		Method: "store",
		Params: []interface{}{1024},
		Types:  []string{"uint256"},
	}
	ctx.MockInput, _ = json.Marshal(input)
	err = runner.Invoke(ctx)
	asrt.NoError(err)

	input = &evm.Input{
		Method: "retrieve",
	}
	ctx.MockInput, _ = json.Marshal(input)
	ret, err := runner.Query(ctx)
	asrt.NoError(err)
	asrt.EqualValues(big.NewInt(1024), big.NewInt(0).SetBytes(ret))
}
