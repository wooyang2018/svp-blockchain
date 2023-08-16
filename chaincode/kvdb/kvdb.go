// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package kvdb

import (
	"bytes"
	"encoding/json"
	"errors"

	"github.com/wooyang2018/posv-blockchain/execution/chaincode"
)

var (
	keyOwner = []byte("owner")
)

type Input struct {
	Method string `json:"method"`
	Key    []byte `json:"key"`
	Value  []byte `json:"value"`
}

// KVDB chaincode
type KVDB struct{}

var _ chaincode.Chaincode = (*KVDB)(nil)

func (c *KVDB) Init(ctx chaincode.CallContext) error {
	ctx.SetState(keyOwner, ctx.Sender())
	return nil
}

func (c *KVDB) Invoke(ctx chaincode.CallContext) error {
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

func (c *KVDB) Query(ctx chaincode.CallContext) ([]byte, error) {
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

func invokeSet(ctx chaincode.CallContext, input *Input) error {
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

func queryGet(ctx chaincode.CallContext, input *Input) ([]byte, error) {
	if len(input.Key) == 0 {
		return nil, errors.New("empty key")
	}
	return json.Marshal(ctx.GetState(input.Key))
}

func parseInput(b []byte) (*Input, error) {
	input := new(Input)
	err := json.Unmarshal(b, input)
	if err != nil {
		return nil, errors.New("failed to parse input: " + err.Error())
	}
	return input, nil
}
