// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package pcoin

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wooyang2018/posv-blockchain/execution/chaincode"
)

func TestPCoin_Init(t *testing.T) {
	asrt := assert.New(t)
	state := chaincode.NewMockState()
	jctx := new(PCoin)

	ctx := new(chaincode.MockCallContext)
	ctx.MockState = state
	ctx.MockSender = []byte{1, 1, 1}
	err := jctx.Init(ctx)

	asrt.NoError(err)

	input := &Input{
		Method: "minter",
	}
	b, _ := json.Marshal(input)
	ctx.MockInput = b
	minter, err := jctx.Query(ctx)

	asrt.NoError(err)
	asrt.Equal(ctx.MockSender, minter, "deployer should be minter")

	input = &Input{
		Method: "balance",
	}
}

func TestPCoin_SetMinter(t *testing.T) {
	asrt := assert.New(t)
	state := chaincode.NewMockState()
	jctx := new(PCoin)

	ctx := new(chaincode.MockCallContext)
	ctx.MockState = state
	ctx.MockSender = []byte{1, 1, 1}
	jctx.Init(ctx)

	input := &Input{
		Method: "setMinter",
		Dest:   []byte{2, 2, 2},
	}
	b, _ := json.Marshal(input)
	ctx.MockSender = []byte{3, 3, 3}
	ctx.MockInput = b
	err := jctx.Invoke(ctx)
	asrt.Error(err, "sender not minter error")

	ctx.MockSender = []byte{1, 1, 1}
	err = jctx.Invoke(ctx)

	asrt.NoError(err)
	input = &Input{
		Method: "minter",
	}
	b, _ = json.Marshal(input)
	ctx.MockInput = b
	minter, err := jctx.Query(ctx)

	asrt.NoError(err)
	asrt.Equal([]byte{2, 2, 2}, minter)
}

func TestPCoin_Mint(t *testing.T) {
	asrt := assert.New(t)
	state := chaincode.NewMockState()
	jctx := new(PCoin)

	ctx := new(chaincode.MockCallContext)
	ctx.MockState = state
	ctx.MockSender = []byte{1, 1, 1}
	jctx.Init(ctx)

	input := &Input{
		Method: "mint",
		Dest:   []byte{2, 2, 2},
		Value:  100,
	}
	b, _ := json.Marshal(input)
	ctx.MockSender = []byte{3, 3, 3}
	ctx.MockInput = b
	err := jctx.Invoke(ctx)
	asrt.Error(err, "sender not minter error")

	ctx.MockSender = []byte{1, 1, 1}
	err = jctx.Invoke(ctx)

	asrt.NoError(err)

	input = &Input{
		Method: "total",
	}
	b, _ = json.Marshal(input)
	ctx.MockInput = b
	b, err = jctx.Query(ctx)

	asrt.NoError(err)

	var balance int64
	json.Unmarshal(b, &balance)

	asrt.EqualValues(100, balance)

	input = &Input{
		Method: "balance",
		Dest:   []byte{2, 2, 2},
	}
	b, _ = json.Marshal(input)
	ctx.MockInput = b
	b, err = jctx.Query(ctx)

	asrt.NoError(err)

	balance = 0
	json.Unmarshal(b, &balance)

	asrt.EqualValues(100, balance)
}

func TestPCoin_Transfer(t *testing.T) {
	asrt := assert.New(t)
	state := chaincode.NewMockState()
	jctx := new(PCoin)

	ctx := new(chaincode.MockCallContext)
	ctx.MockState = state
	ctx.MockSender = []byte{1, 1, 1}
	jctx.Init(ctx)

	input := &Input{
		Method: "mint",
		Dest:   []byte{2, 2, 2},
		Value:  100,
	}
	b, _ := json.Marshal(input)
	ctx.MockInput = b
	jctx.Invoke(ctx)

	// transfer 222 -> 333, value = 101
	input = &Input{
		Method: "transfer",
		Dest:   []byte{3, 3, 3},
		Value:  101,
	}
	b, _ = json.Marshal(input)
	ctx.MockSender = []byte{2, 2, 2}
	ctx.MockInput = b
	err := jctx.Invoke(ctx)

	asrt.Error(err, "not enough pcoin error")

	// transfer 222 -> 333, value = 100
	input.Value = 100
	b, _ = json.Marshal(input)
	ctx.MockInput = b
	err = jctx.Invoke(ctx)

	asrt.NoError(err)

	input.Method = "total"
	b, _ = json.Marshal(input)
	ctx.MockInput = b
	b, _ = jctx.Query(ctx)
	var balance int64
	json.Unmarshal(b, &balance)

	asrt.EqualValues(100, balance, "total should not change")

	input.Method = "balance"
	input.Dest = []byte{2, 2, 2}
	b, _ = json.Marshal(input)
	ctx.MockInput = b
	b, _ = jctx.Query(ctx)
	balance = 0
	json.Unmarshal(b, &balance)

	asrt.EqualValues(0, balance)

	input.Dest = []byte{3, 3, 3}
	b, _ = json.Marshal(input)
	ctx.MockInput = b
	b, _ = jctx.Query(ctx)
	balance = 0
	json.Unmarshal(b, &balance)

	asrt.EqualValues(100, balance)
}
