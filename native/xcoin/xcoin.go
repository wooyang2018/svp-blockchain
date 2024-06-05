// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package xcoin

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

type InitInput struct {
	Values map[string]uint64 `json:"values"`
}

// XCoin chaincode
type XCoin struct{}

var _ common.Chaincode = (*XCoin)(nil)

func (c *XCoin) Init(ctx common.CallContext) error {
	if ctx.BlockHeight() != 0 {
		return errors.New("xcoin must init at genesis")
	}
	input, err := parseInitInput(ctx.Input())
	if err != nil {
		return err
	}
	for k, v := range input.Values {
		owner, err := common.Address32ToBytes(k)
		if err != nil {
			return fmt.Errorf("init xcoin failed: %w", err)
		}
		if err = common.AssertLength(owner, 32); err != nil {
			return err
		}
		ctx.SetState(owner, common.EncodeBalance(v))
	}
	return nil
}

func (c *XCoin) Invoke(ctx common.CallContext) error {
	input, err := parseInput(ctx.Input())
	if err != nil {
		return err
	}
	if err = common.AssertLength(input.Dest, 32); err != nil {
		return err
	}
	switch input.Method {
	case "set":
		return invokeSet(ctx, input)
	case "transfer":
		return invokeTransfer(ctx, input)
	default:
		return errors.New("method not found")
	}
}

func (c *XCoin) Query(ctx common.CallContext) ([]byte, error) {
	input, err := parseInput(ctx.Input())
	if err != nil {
		return nil, err
	}
	if err = common.AssertLength(input.Dest, 32); err != nil {
		return nil, err
	}
	switch input.Method {
	case "balance":
		return ctx.GetState(input.Dest), nil
	default:
		return nil, errors.New("method not found")
	}
}

func (c *XCoin) SetTxTrk(txTrk *common.StateTracker) {
	return
}

func invokeSet(ctx common.CallContext, input *Input) error {
	if !bytes.Equal(nil, ctx.Sender()) {
		return errors.New("method must be internal")
	}
	ctx.SetState(input.Dest, common.EncodeBalance(input.Value))
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

func parseInitInput(b []byte) (*InitInput, error) {
	input := new(InitInput)
	err := json.Unmarshal(b, input)
	if err != nil {
		return nil, fmt.Errorf("failed to parse init input: %w", err)
	}
	return input, nil
}
