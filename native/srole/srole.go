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
	Point  string `json:"point"`
	Topic  string `json:"topic"`
}

type InitInput struct {
	Size  int     `json:"size"`
	Peers []*Peer `json:"peers"`
}

type Peer struct {
	Addr  []byte `json:"addr"`
	Quota uint64 `json:"quota"`
	Point string `json:"point"`
	Topic string `json:"topic"`
}

// SRole chaincode
type SRole struct{}

var _ common.Chaincode = (*SRole)(nil)

func (c *SRole) Init(ctx common.CallContext) error {
	if ctx.BlockHeight() != 0 {
		return errors.New("srole chaincode must init at genesis")
	}
	input, err := parseInitInput(ctx.Input())
	if err != nil {
		return err
	}
	core.SRole.SetWindowSize(input.Size)
	keyMap := make(map[string]struct{})
	for _, peer := range input.Peers {
		keyStr := common.AddressToString(peer.Addr)
		keyMap[keyStr] = struct{}{}
		if !core.SRole.IsValidator(keyStr) {
			err := core.SRole.AddValidator(keyStr, peer.Point, peer.Topic, peer.Quota)
			if err != nil {
				return err
			}
		}
	}
	count := core.SRole.ValidatorCount()
	var toDel []string
	for i := 0; i < count; i++ {
		key := core.SRole.GetValidator(i)
		if _, ok := keyMap[key.String()]; !ok {
			toDel = append(toDel, key.String())
		}
	}
	for _, key := range toDel {
		err := core.SRole.DeleteValidator(key)
		if err != nil {
			return err
		}
	}
	return nil
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
		keyStr := common.AddressToString(input.Addr)
		return core.SRole.DeleteValidator(keyStr)
	case "add":
		keyStr := common.AddressToString(input.Addr)
		return core.SRole.AddValidator(keyStr, input.Point, input.Topic, input.Quota)
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
	case "genesis":
		return []byte(core.SRole.GetGenesisFile()), nil
	case "peers":
		return []byte(core.SRole.GetPeersFile()), nil
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
