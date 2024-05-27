// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package taddr

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/wooyang2018/svp-blockchain/execution/common"
)

const suffix = "000000000000"

type Input struct {
	Method string `json:"method"`
	Addr   []byte `json:"addr"`
}

type InitInput struct {
	Values []string `json:"values"`
}

// TAddr chaincode to resolve the conversion between addr20 and addr32
type TAddr struct{}

var _ common.Chaincode = (*TAddr)(nil)

func (c *TAddr) Init(ctx common.CallContext) error {
	if ctx.BlockHeight() != 0 {
		return errors.New("taddr chaincode must init at genesis")
	}
	input, err := parseInitInput(ctx.Input())
	if err != nil {
		return err
	}
	for _, v := range input.Values {
		addr, err := common.Address32ToBytes(v)
		if err != nil {
			return fmt.Errorf("init taddr chaincode failed: %w", err)
		}
		if err = common.AssertLength(addr, 32); err != nil {
			return err
		}
		addr20 := toAddr20(addr)
		ctx.SetState(addr, addr20)
		ctx.SetState(addr20, addr)
	}
	return nil
}

func (c *TAddr) Invoke(ctx common.CallContext) error {
	input, err := parseInput(ctx.Input())
	if err != nil {
		return err
	}
	switch input.Method {
	case "store":
		return storeAddr(ctx, input)
	default:
		return errors.New("method not found")
	}
}

func (c *TAddr) Query(ctx common.CallContext) ([]byte, error) {
	input, err := parseInput(ctx.Input())
	if err != nil {
		return nil, err
	}
	switch input.Method {
	case "query":
		return queryAddr(ctx, input)
	default:
		return nil, errors.New("method not found")
	}
}

func storeAddr(ctx common.CallContext, input *Input) error {
	if !bytes.Equal(nil, ctx.Sender()) {
		return errors.New("method must be internal")
	}
	if err := common.ValidLength(input.Addr); err != nil {
		return err
	}
	var destAddr []byte
	if len(input.Addr) == 20 {
		destAddr = toAddr32(input.Addr)
	} else {
		destAddr = toAddr20(input.Addr)
	}
	ctx.SetState(input.Addr, destAddr)
	ctx.SetState(destAddr, input.Addr)
	return nil
}

func queryAddr(ctx common.CallContext, input *Input) ([]byte, error) {
	if err := common.ValidLength(input.Addr); err != nil {
		return nil, err
	}
	destAddr := ctx.GetState(input.Addr)
	if len(destAddr) == 0 {
		return nil, errors.New("address not found")
	}
	return destAddr, nil
}

func toAddr20(addr []byte) []byte {
	return addr[0:20]
}

func toAddr32(addr []byte) []byte {
	res := make([]byte, 32)
	copy(res, addr)
	copy(res[20:], suffix)
	return res
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
