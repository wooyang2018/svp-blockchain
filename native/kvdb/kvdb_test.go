// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package kvdb

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/wooyang2018/svp-blockchain/core"
	"github.com/wooyang2018/svp-blockchain/execution/common"
)

func makeInitCtx() *common.MockCallContext {
	ctx := new(common.MockCallContext)
	ctx.MemStateStore = common.NewMemStateStore()
	ctx.MockSender = core.GenerateKey(nil).PublicKey().Bytes()
	return ctx
}

func TestInit(t *testing.T) {
	asrt := assert.New(t)
	jctx := new(KVDB)
	ctx := makeInitCtx()
	err := jctx.Init(ctx)
	asrt.NoError(err)

	input := &Input{Method: "owner"}
	ctx.MockInput, _ = json.Marshal(input)
	owner, err := jctx.Query(ctx)
	asrt.NoError(err)
	asrt.Equal(ctx.MockSender, owner, "deployer should be owner")
}

func TestSet(t *testing.T) {
	asrt := assert.New(t)
	jctx := new(KVDB)
	ctx := makeInitCtx()
	err := jctx.Init(ctx)
	asrt.NoError(err)

	input := &Input{
		Method: "set",
		Key:    []byte("key"),
		Value:  []byte("value"),
	}
	ctx.MockInput, _ = json.Marshal(input)
	err = jctx.Invoke(ctx)
	asrt.NoError(err)

	input = &Input{
		Method: "get",
		Key:    []byte("key"),
	}
	ctx.MockInput, _ = json.Marshal(input)
	ctx.MockSender = nil
	value, err := jctx.Query(ctx)
	asrt.NoError(err)
	asrt.EqualValues([]byte("value"), value)
}
