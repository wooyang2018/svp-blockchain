// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package evm

import (
	"encoding/json"
	"math/big"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/wooyang2018/svp-blockchain/core"
	ethrunt "github.com/wooyang2018/svp-blockchain/evm/runtime"
	"github.com/wooyang2018/svp-blockchain/execution/common"
	"github.com/wooyang2018/svp-blockchain/storage"
)

func makeInitCtx() *common.MockCallContext {
	ctx := new(common.MockCallContext)
	ctx.MockState = common.NewMockState()
	ctx.MockSender = core.GenerateKey(nil).PublicKey().Bytes()
	ctx.MockInput, _ = json.Marshal(nil)
	return ctx
}

func TestInvoke(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("compiling contracts on windows is not supported")
	}
	asrt := assert.New(t)
	compiled := ethrunt.CompileCode("../../evm/testdata/contracts/Storage.sol")

	ctx := makeInitCtx()
	runner := NewRunner(compiled["Storage"][1], compiled["Storage"][0], storage.NewMemLevelDBStore(), ctx)
	err := runner.Init(ctx)
	asrt.NoError(err)
	runner.jsonABI, runner.hexCode = "", "" // subsequent invokes do not need to pass in jsonABI and hexCode

	input := &Input{
		Method: "store",
		Params: []interface{}{1024},
		Types:  []string{"uint256"},
	}
	ctx.MockInput, _ = json.Marshal(input)
	err = runner.Invoke(ctx)
	asrt.NoError(err)

	input = &Input{
		Method: "retrieve",
	}
	ctx.MockInput, _ = json.Marshal(input)
	ret, err := runner.Query(ctx)
	asrt.NoError(err)
	asrt.EqualValues(big.NewInt(1024), big.NewInt(0).SetBytes(ret))
}
