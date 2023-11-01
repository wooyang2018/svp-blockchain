// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package pcoin

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/wooyang2018/svp-blockchain/core"
	"github.com/wooyang2018/svp-blockchain/execution/common"
)

func makeInitCtx() (*common.MockCallContext, []*core.PublicKey) {
	vlds := make([]*core.PublicKey, 4)
	for i := range vlds {
		vlds[i] = core.GenerateKey(nil).PublicKey()
	}
	ctx := new(common.MockCallContext)
	ctx.MockState = common.NewMockState()
	ctx.MockSender = vlds[0].Bytes()
	return ctx, vlds
}

func TestInit(t *testing.T) {
	asrt := assert.New(t)
	jctx := new(PCoin)
	ctx, _ := makeInitCtx()
	err := jctx.Init(ctx)
	asrt.NoError(err)

	input := &Input{Method: "minter"}
	ctx.MockInput, _ = json.Marshal(input)
	minter, err := jctx.Query(ctx)
	asrt.NoError(err)
	asrt.Equal(ctx.MockSender, minter, "deployer should be minter")
}

func TestMint(t *testing.T) {
	asrt := assert.New(t)
	jctx := new(PCoin)
	ctx, vlds := makeInitCtx()
	err := jctx.Init(ctx)
	asrt.NoError(err)

	input := &Input{
		Method: "mint",
		Dest:   vlds[1].Bytes(),
		Value:  100,
	}
	ctx.MockSender = vlds[1].Bytes()
	ctx.MockInput, _ = json.Marshal(input)
	err = jctx.Invoke(ctx)
	asrt.Error(err, "sender should be minter")

	ctx.MockSender = vlds[0].Bytes()
	err = jctx.Invoke(ctx)
	asrt.NoError(err)

	input = &Input{Method: "total"}
	ctx.MockInput, _ = json.Marshal(input)
	b, err := jctx.Query(ctx)
	asrt.NoError(err)
	var balance int64
	json.Unmarshal(b, &balance)
	asrt.EqualValues(100, balance)

	input = &Input{
		Method: "balance",
		Dest:   vlds[1].Bytes(),
	}
	ctx.MockInput, _ = json.Marshal(input)
	b, err = jctx.Query(ctx)
	asrt.NoError(err)
	balance = 0
	json.Unmarshal(b, &balance)
	asrt.EqualValues(100, balance)
}

func TestTransfer(t *testing.T) {
	asrt := assert.New(t)
	jctx := new(PCoin)
	ctx, vlds := makeInitCtx()
	err := jctx.Init(ctx)
	asrt.NoError(err)

	input := &Input{
		Method: "mint",
		Dest:   vlds[2].Bytes(),
		Value:  100,
	}
	ctx.MockInput, _ = json.Marshal(input)
	jctx.Invoke(ctx)

	// transfer vlds[2] -> vlds[3], value = 110
	input = &Input{
		Method: "transfer",
		Dest:   vlds[3].Bytes(),
		Value:  110,
	}
	ctx.MockSender = vlds[2].Bytes()
	ctx.MockInput, _ = json.Marshal(input)
	err = jctx.Invoke(ctx)
	asrt.Error(err, "not enough balance")

	// transfer vlds[2] -> vlds[3], value = 10
	input.Value = 10
	ctx.MockInput, _ = json.Marshal(input)
	err = jctx.Invoke(ctx)
	asrt.NoError(err)

	input.Method = "total"
	ctx.MockInput, _ = json.Marshal(input)
	b, _ := jctx.Query(ctx)
	var balance int64
	json.Unmarshal(b, &balance)
	asrt.EqualValues(100, balance, "total should not change")

	input.Method = "balance"
	input.Dest = vlds[2].Bytes()
	ctx.MockInput, _ = json.Marshal(input)
	b, _ = jctx.Query(ctx)
	balance = 0
	json.Unmarshal(b, &balance)
	asrt.EqualValues(90, balance)

	input.Dest = vlds[3].Bytes()
	ctx.MockInput, _ = json.Marshal(input)
	b, _ = jctx.Query(ctx)
	balance = 0
	json.Unmarshal(b, &balance)
	asrt.EqualValues(10, balance)
}
