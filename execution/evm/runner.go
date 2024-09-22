// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package evm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	ethcomm "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"

	"github.com/wooyang2018/svp-blockchain/evm"
	"github.com/wooyang2018/svp-blockchain/evm/runtime"
	"github.com/wooyang2018/svp-blockchain/evm/statedb"
	"github.com/wooyang2018/svp-blockchain/execution/common"
	"github.com/wooyang2018/svp-blockchain/storage"
)

var keyAddr = []byte("address") // evm contract address
var keyAbi = []byte("abi")      // evm contract abi

type Input struct {
	Method string   `json:"method"`
	Params []string `json:"params"`
	Types  []string `json:"types"`
}

type InitInput struct {
	Class  string   `json:"class"`
	Params []string `json:"params"`
	Types  []string `json:"types"`
}

type Runner struct {
	config   *runtime.Config
	Proxy    *NativeProxy
	CodePath string
	Storage  storage.PersistStore
	StateDB  *statedb.StateDB
}

var _ common.Chaincode = (*Runner)(nil)

// Init deploys the solidity contract and binds the eth address.
func (r *Runner) Init(ctx common.CallContext) error {
	if err := r.setConfig(ctx); err != nil {
		return err
	}

	initInput, args := parseInitInput(ctx.Input())
	compiled := runtime.CompileCode(r.CodePath)
	hexCode, jsonABI := compiled[initInput.Class][0], compiled[initInput.Class][1]

	contractBin, _ := hexutil.Decode(hexCode)
	contractAbi, _ := abi.JSON(strings.NewReader(jsonABI))
	invoke, err := contractAbi.Pack("", args...)
	if err != nil {
		return err
	}

	deploy := append(contractBin, invoke...)
	vmenv := runtime.NewEnv(r.config)
	sender := evm.AccountRef(r.config.Origin)
	_, address, leftOverGas, err := vmenv.Create(
		sender,
		deploy,
		r.config.GasLimit,
		r.config.Value,
	)
	err = r.StateDB.Commit()
	if err != nil {
		return err
	}

	ctx.SetState(keyAddr, address.Bytes())
	ctx.SetState(keyAbi, []byte(jsonABI))
	err = r.Proxy.storeAddr(address.Bytes())
	if err != nil {
		return err
	}

	fmt.Printf("deploy code at: %s, used gas: %d\n", address.String(), r.config.GasLimit-leftOverGas)
	r.Proxy.mergeTrks()
	return err
}

func (r *Runner) Invoke(ctx common.CallContext) error {
	if err := r.setConfig(ctx); err != nil {
		return err
	}

	input, params := parseInput(ctx.Input())
	contractAbi, _ := abi.JSON(bytes.NewReader(ctx.GetState(keyAbi)))
	invoke, err := contractAbi.Pack(input.Method, params...)
	if err != nil {
		return err
	}

	vmenv := runtime.NewEnv(r.config)
	sender := evm.AccountRef(r.config.Origin)
	address := ethcomm.BytesToAddress(ctx.GetState(keyAddr))
	ret, leftOverGas, err := vmenv.Call(
		sender,
		address,
		invoke,
		r.config.GasLimit,
		r.config.Value,
	)
	err = r.StateDB.Commit()
	if err != nil {
		return err
	}
	retSlice, _ := contractAbi.Unpack(input.Method, ret)
	res, _ := json.Marshal(retSlice)

	fmt.Printf("invoke code at: %s, used gas: %d, return result: %s\n", address.String(),
		r.config.GasLimit-leftOverGas, res)
	r.Proxy.mergeTrks()
	return err
}

func (r *Runner) Query(ctx common.CallContext) ([]byte, error) {
	if err := r.setConfig(ctx); err != nil {
		return nil, err
	}

	input, params := parseInput(ctx.Input())
	contractAbi, _ := abi.JSON(bytes.NewReader(ctx.GetState(keyAbi)))
	query, err := contractAbi.Pack(input.Method, params...)
	if err != nil {
		return nil, err
	}

	vmenv := runtime.NewEnv(r.config)
	sender := evm.AccountRef(r.config.Origin)
	address := ethcomm.BytesToAddress(ctx.GetState(keyAddr))
	ret, leftOverGas, err := vmenv.StaticCall(
		sender,
		address,
		query,
		r.config.GasLimit,
	)
	retSlice, _ := contractAbi.Unpack(input.Method, ret)
	res, _ := json.Marshal(retSlice)

	fmt.Printf("query code at: %s, used gas: %d, return result: %s\n", address.String(),
		r.config.GasLimit-leftOverGas, res)
	return res, err
}

func (r *Runner) SetTxTrk(txTrk *common.StateTracker) {
	r.Proxy.setTxTrk(txTrk)
}

func (r *Runner) setConfig(ctx common.CallContext) error {
	cfg := new(runtime.Config)
	runtime.SetDefaults(cfg)
	addr20, err := r.Proxy.queryAddr(ctx.Sender())
	if err != nil {
		return err
	}
	cfg.Origin = ethcomm.BytesToAddress(addr20)
	cfg.State = r.StateDB
	cfg.BlockNumber = big.NewInt(0).SetUint64(ctx.BlockHeight())
	r.config = cfg
	return nil
}

func parseInput(raw []byte) (*Input, []interface{}) {
	input := new(Input)
	err := json.Unmarshal(raw, input)
	common.Check(err)
	var params []interface{}
	for i := 0; i < len(input.Params); i++ {
		params = append(params, parseParam(input.Types[i], input.Params[i]))
	}
	return input, params
}

func parseInitInput(raw []byte) (*InitInput, []interface{}) {
	input := new(InitInput)
	err := json.Unmarshal(raw, input)
	common.Check(err)
	var params []interface{}
	for i := 0; i < len(input.Params); i++ {
		params = append(params, parseParam(input.Types[i], input.Params[i]))
	}
	return input, params
}

func parseParam(paramType string, param string) interface{} {
	switch paramType {
	case "uint256":
		tmp, err := strconv.ParseInt(param, 10, 64)
		common.Check(err)
		return big.NewInt(tmp)
	default:
		return nil
	}
}
