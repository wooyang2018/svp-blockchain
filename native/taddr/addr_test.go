// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package taddr

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/wooyang2018/svp-blockchain/core"
	"github.com/wooyang2018/svp-blockchain/execution/common"
)

func makeInitCtx() (*common.MockCallContext, []*core.PublicKey) {
	vlds := make([]*core.PublicKey, 4)
	input := make(map[string]struct{})
	for i := range vlds {
		vlds[i] = core.GenerateKey(nil).PublicKey()
		input[vlds[i].String()] = struct{}{}
	}
	ctx := new(common.MockCallContext)
	ctx.MockState = common.NewMockState()
	ctx.MockSender = vlds[0].Bytes()
	ctx.MockInput, _ = json.Marshal(input)
	return ctx, vlds
}

func TestQuery(t *testing.T) {
	asrt := assert.New(t)
	jctx := new(TAddr)
	ctx, vlds := makeInitCtx()
	err := jctx.Init(ctx)
	asrt.NoError(err)

	expectedAddr32 := vlds[1].Bytes()
	input := &Input{
		Method: "query",
		Addr:   expectedAddr32,
	}
	ctx.MockInput, _ = json.Marshal(input)
	addr20, err := jctx.Query(ctx)
	asrt.NoError(err)
	asrt.EqualValues(expectedAddr32[0:20], addr20)

	input = &Input{
		Method: "query",
		Addr:   addr20,
	}
	ctx.MockInput, _ = json.Marshal(input)
	addr32, err := jctx.Query(ctx)
	asrt.NoError(err)
	asrt.EqualValues(expectedAddr32, addr32)
}

func TestStore(t *testing.T) {
	asrt := assert.New(t)
	jctx := new(TAddr)
	ctx, vlds := makeInitCtx()
	err := jctx.Init(ctx)
	asrt.NoError(err)

	mockAddr20 := []byte("6d8edfaaa3ca9d68032c")
	expectedAddr32 := toAddr32(mockAddr20)

	input := &Input{
		Method: "store",
		Addr:   mockAddr20,
	}
	ctx.MockInput, _ = json.Marshal(input)
	ctx.MockSender = nil
	err = jctx.Invoke(ctx)
	asrt.NoError(err)

	input = &Input{
		Method: "query",
		Addr:   mockAddr20,
	}
	ctx.MockInput, _ = json.Marshal(input)
	ctx.MockSender = vlds[0].Bytes()
	addr32, err := jctx.Query(ctx)
	asrt.NoError(err)
	asrt.EqualValues(expectedAddr32, addr32)
}
