// Copyright (C) 2024 Wooyang2018
// Licensed under the GNU General Public License v3.0

package srole

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/wooyang2018/svp-blockchain/core"
	"github.com/wooyang2018/svp-blockchain/execution/common"
)

type Input struct {
	Method string `json:"method"`
	Addr   []byte `json:"addr"`
	Quota  uint64 `json:"quota"`
}

type InitInput struct{}

// SRole chaincode
type SRole struct{}

var _ common.Chaincode = (*SRole)(nil)

func (c *SRole) Init(ctx common.CallContext) error {
	if ctx.BlockHeight() != 0 {
		return errors.New("srole chaincode must init at genesis")
	}
	_, err := parseInitInput(ctx.Input())
	return err
}

func (c *SRole) Invoke(ctx common.CallContext) error {
	input, err := parseInput(ctx.Input())
	if err != nil {
		return err
	}
	if err := common.ValidLength(input.Addr); err != nil {
		return err
	}
	switch input.Method {
	case "delete":
		addrStr := common.AddressToString(input.Addr)
		return core.SRole.DeleteValidator(addrStr)
	case "add":
		addrStr := common.AddressToString(input.Addr)
		return core.SRole.AddValidator(addrStr, input.Quota)
	default:
		return errors.New("method not found")
	}
}

func (c *SRole) Query(ctx common.CallContext) ([]byte, error) {
	input, err := parseInput(ctx.Input())
	if err != nil {
		return nil, err
	}
	switch input.Method {
	case "print":
		return []byte(core.SRole.GetGenesisFile()), nil
	default:
		return nil, errors.New("method not found")
	}
}

func (c *SRole) SetTxTrk(txTrk *common.StateTracker) {
	return
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
