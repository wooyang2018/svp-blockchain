// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package xcoin

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"

	"github.com/wooyang2018/svp-blockchain/execution/common"
)

type Input struct {
	Method string `json:"method"`
	Dest   []byte `json:"dest"`
	Value  int64  `json:"value"`
}

// XCoin chaincode
type XCoin struct{}

var _ common.Chaincode = (*XCoin)(nil)

func (c *XCoin) Init(ctx common.CallContext) error {
	if ctx.BlockHeight() != 0 {
		return errors.New("xcoin must init at height 0")
	}
	m := make(map[string]int64)
	json.Unmarshal(ctx.Input(), &m)
	for k, v := range m {
		owner, err := base64.StdEncoding.DecodeString(k)
		if err != nil {
			return errors.New("init xcoin failed: " + err.Error())
		}
		ctx.SetState(owner, encodeBalance(v))
	}
	return nil
}

func (c *XCoin) Invoke(ctx common.CallContext) error {
	input, err := parseInput(ctx.Input())
	if err != nil {
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
	switch input.Method {
	case "balance":
		return queryBalance(ctx, input)
	default:
		return nil, errors.New("method not found")
	}
}

func invokeSet(ctx common.CallContext, input *Input) error {
	if !bytes.Equal(nil, ctx.Sender()) {
		return errors.New("set must be internal")
	}
	ctx.SetState(input.Dest, encodeBalance(input.Value))
	return nil
}

func invokeTransfer(ctx common.CallContext, input *Input) error {
	bsctx := decodeBalance(ctx.GetState(ctx.Sender()))
	if bsctx < input.Value {
		return errors.New("not enough balance")
	}
	bdes := decodeBalance(ctx.GetState(input.Dest))

	bsctx -= input.Value
	bdes += input.Value

	ctx.SetState(ctx.Sender(), encodeBalance(bsctx))
	ctx.SetState(input.Dest, encodeBalance(bdes))
	return nil
}

func queryBalance(ctx common.CallContext, input *Input) ([]byte, error) {
	return json.Marshal(decodeBalance(ctx.GetState(input.Dest)))
}

func decodeBalance(b []byte) int64 {
	if b == nil {
		return 0
	}
	return int64(binary.BigEndian.Uint64(b))
}

func encodeBalance(value int64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(value))
	return b
}

func parseInput(b []byte) (*Input, error) {
	input := new(Input)
	err := json.Unmarshal(b, input)
	if err != nil {
		return nil, errors.New("failed to parse input: " + err.Error())
	}
	return input, nil
}
