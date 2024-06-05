// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package kvdb

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/wooyang2018/svp-blockchain/execution/common"
)

var keyOwner = []byte("owner")

type Input struct {
	Method string `json:"method"`
	Key    []byte `json:"key"`
	Value  []byte `json:"value"`
}

// KVDB chaincode
type KVDB struct{}

var _ common.Chaincode = (*KVDB)(nil)

func (c *KVDB) Init(ctx common.CallContext) error {
	ctx.SetState(keyOwner, ctx.Sender())
	return nil
}

func (c *KVDB) Invoke(ctx common.CallContext) error {
	input, err := parseInput(ctx.Input())
	if err != nil {
		return err
	}
	switch input.Method {
	case "set":
		return invokeSet(ctx, input)
	default:
		return errors.New("method not found")
	}
}

func (c *KVDB) Query(ctx common.CallContext) ([]byte, error) {
	input, err := parseInput(ctx.Input())
	if err != nil {
		return nil, err
	}
	switch input.Method {
	case "owner":
		return ctx.GetState(keyOwner), nil
	case "get":
		return queryGet(ctx, input)
	default:
		return nil, errors.New("method not found")
	}
}

func (c *KVDB) SetTxTrk(txTrk *common.StateTracker) {
	return
}

func invokeSet(ctx common.CallContext, input *Input) error {
	owner := ctx.GetState(keyOwner)
	if !bytes.Equal(owner, ctx.Sender()) {
		return errors.New("sender must be owner")
	}
	if len(input.Key) == 0 || len(input.Value) == 0 {
		return errors.New("empty key or value")
	}
	ctx.SetState(input.Key, input.Value)
	return nil
}

func queryGet(ctx common.CallContext, input *Input) ([]byte, error) {
	if len(input.Key) == 0 {
		return nil, errors.New("empty key")
	}
	return ctx.GetState(input.Key), nil
}

func parseInput(b []byte) (*Input, error) {
	input := new(Input)
	err := json.Unmarshal(b, input)
	if err != nil {
		return nil, fmt.Errorf("failed to parse input: %w", err)
	}
	return input, nil
}
