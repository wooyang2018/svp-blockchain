// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package xcoin

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/wooyang2018/svp-blockchain/core"
	"github.com/wooyang2018/svp-blockchain/execution/common"
)

func makeInitCtx() (*common.MockCallContext, []*core.PublicKey) {
	vlds := make([]*core.PublicKey, 4)
	input := make(map[string]int64)
	for i := range vlds {
		vlds[i] = core.GenerateKey(nil).PublicKey()
		input[vlds[i].String()] = 100
	}
	ctx := new(common.MockCallContext)
	ctx.MockState = common.NewMockState()
	ctx.MockSender = vlds[0].Bytes()
	ctx.MockInput, _ = json.Marshal(input)
	return ctx, vlds
}

func TestTransfer(t *testing.T) {
	asrt := assert.New(t)
	jctx := new(XCoin)
	ctx, vlds := makeInitCtx()
	err := jctx.Init(ctx)
	asrt.NoError(err)

	// transfer vlds[1] -> vlds[2], value = 110
	input := &Input{
		Method: "transfer",
		Dest:   vlds[2].Bytes(),
		Value:  110,
	}
	ctx.MockInput, _ = json.Marshal(input)
	ctx.MockSender = vlds[1].Bytes()
	err = jctx.Invoke(ctx)
	asrt.Error(err)

	// transfer vlds[1] -> vlds[2], value = 10
	input.Value = 10
	ctx.MockInput, _ = json.Marshal(input)
	err = jctx.Invoke(ctx)
	asrt.NoError(err)

	// query balance vlds[1]
	input.Method = "balance"
	input.Dest = vlds[1].Bytes()
	ctx.MockInput, _ = json.Marshal(input)
	b, _ := jctx.Query(ctx)
	balance := 0
	json.Unmarshal(b, &balance)
	asrt.EqualValues(90, balance)

	// query balance vlds[2]
	input.Dest = vlds[2].Bytes()
	ctx.MockInput, _ = json.Marshal(input)
	b, _ = jctx.Query(ctx)
	json.Unmarshal(b, &balance)
	asrt.EqualValues(110, balance)
}

func TestSetBalance(t *testing.T) {
	asrt := assert.New(t)
	jctx := new(XCoin)
	ctx, vlds := makeInitCtx()
	err := jctx.Init(ctx)
	asrt.NoError(err)

	// set balance of vlds[3] to 90
	input := &Input{
		Method: "set",
		Dest:   vlds[3].Bytes(),
		Value:  90,
	}
	ctx.MockInput, _ = json.Marshal(input)
	ctx.MockSender = vlds[3].Bytes()
	err = jctx.Invoke(ctx)
	asrt.Error(err)

	ctx.MockSender = nil
	err = jctx.Invoke(ctx)
	asrt.NoError(err)

	// query balance vlds[3]
	input.Method = "balance"
	input.Dest = vlds[3].Bytes()
	ctx.MockInput, _ = json.Marshal(input)
	b, _ := jctx.Query(ctx)
	balance := 0
	json.Unmarshal(b, &balance)
	asrt.EqualValues(90, balance)
}
