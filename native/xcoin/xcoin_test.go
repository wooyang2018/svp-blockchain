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
	values := make(map[string]uint64)
	for i := range vlds {
		vlds[i] = core.GenerateKey(nil).PublicKey()
		values[vlds[i].String()] = 100
	}
	ctx := new(common.MockCallContext)
	ctx.MemStateStore = common.NewMemStateStore()
	ctx.MockSender = vlds[0].Bytes()
	input := InitInput{values}
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
	asrt.EqualValues(90, common.DecodeBalance(b))

	// query balance vlds[2]
	input.Dest = vlds[2].Bytes()
	ctx.MockInput, _ = json.Marshal(input)
	b, _ = jctx.Query(ctx)
	asrt.EqualValues(110, common.DecodeBalance(b))
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
	asrt.EqualValues(90, common.DecodeBalance(b))
}
