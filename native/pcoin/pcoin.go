// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package pcoin

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/wooyang2018/svp-blockchain/execution/common"
)

type Input struct {
	Method string `json:"method"`
	Dest   []byte `json:"dest"`
	Value  uint64 `json:"value"`
}

var (
	keyMinter = []byte("minter")
	keyTotal  = []byte("total")
)

// PCoin chaincode
type PCoin struct{}

var _ common.Chaincode = (*PCoin)(nil)

func (c *PCoin) Init(ctx common.CallContext) error {
	ctx.SetState(keyMinter, ctx.Sender())
	return nil
}

func (c *PCoin) Invoke(ctx common.CallContext) error {
	input, err := parseInput(ctx.Input())
	if err != nil {
		return err
	}
	if err = common.AssertLength(input.Dest, 32); err != nil {
		return err
	}
	switch input.Method {
	case "mint":
		return invokeMint(ctx, input)
	case "transfer":
		return invokeTransfer(ctx, input)
	default:
		return errors.New("method not found")
	}
}

func (c *PCoin) Query(ctx common.CallContext) ([]byte, error) {
	input, err := parseInput(ctx.Input())
	if err != nil {
		return nil, err
	}
	if err = common.AssertLength(input.Dest, 32); err != nil {
		return nil, err
	}
	switch input.Method {
	case "minter":
		return ctx.GetState(keyMinter), nil
	case "total":
		return ctx.GetState(keyTotal), nil
	case "balance":
		return ctx.GetState(input.Dest), nil
	default:
		return nil, errors.New("method not found")
	}
}

func (c *PCoin) SetTxTrk(txTrk *common.StateTracker) {
	return
}

func invokeMint(ctx common.CallContext, input *Input) error {
	minter := ctx.GetState(keyMinter)
	if !bytes.Equal(minter, ctx.Sender()) {
		return errors.New("sender must be minter")
	}
	total := common.DecodeBalance(ctx.GetState(keyTotal))
	balance := common.DecodeBalance(ctx.GetState(input.Dest))

	total += input.Value
	balance += input.Value

	ctx.SetState(keyTotal, common.EncodeBalance(total))
	ctx.SetState(input.Dest, common.EncodeBalance(balance))
	return nil
}

func invokeTransfer(ctx common.CallContext, input *Input) error {
	if bytes.Equal(ctx.Sender(), input.Dest) {
		return errors.New("self-transfer is prohibited")
	}

	bsctx := common.DecodeBalance(ctx.GetState(ctx.Sender()))
	if bsctx < input.Value {
		return errors.New("not enough balance")
	}
	bdes := common.DecodeBalance(ctx.GetState(input.Dest))

	bsctx -= input.Value
	bdes += input.Value

	ctx.SetState(ctx.Sender(), common.EncodeBalance(bsctx))
	ctx.SetState(input.Dest, common.EncodeBalance(bdes))
	return nil
}

func parseInput(b []byte) (*Input, error) {
	input := new(Input)
	err := json.Unmarshal(b, input)
	if err != nil {
		return nil, fmt.Errorf("failed to parse input: %w", err)
	}
	return input, nil
}
