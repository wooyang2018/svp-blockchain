// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package evm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/big"
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

var keyAddr = []byte("address") // TODO evm contract address
var keyAbi = []byte("abi")      // evm contract abi

type Input struct {
	Method string        `json:"method"`
	Params []interface{} `json:"params"`
	Types  []string      `json:"types"`
}

type Runner struct {
	jsonABI string
	hexCode string
	config  *runtime.Config
}

var _ common.Chaincode = (*Runner)(nil)

func NewRunner(jsonABI, hexCode string, store storage.PersistStore, ctx common.CallContext) *Runner {
	cfg := new(runtime.Config)
	runtime.SetDefaults(cfg)                          // TODO convert other call context to config
	cfg.Origin = ethcomm.BytesToAddress(ctx.Sender()) // TODO be cropped from the left

	cache := statedb.NewCacheDB(store) // TODO implement OngBalanceHandle
	cfg.State = statedb.NewStateDB(cache, ethcomm.Hash{}, ethcomm.Hash{}, statedb.NewDummy())

	return &Runner{jsonABI: jsonABI, hexCode: hexCode, config: cfg}
}

func (r *Runner) parseInput(raw []byte) (string, []interface{}) {
	var input Input
	err := json.Unmarshal(raw, &input)
	if err != nil {
		panic(err)
	}
	var params []interface{}
	for i := 0; i < len(input.Params); i++ {
		switch input.Types[i] {
		case "uint256":
			params = append(params, big.NewInt(int64(input.Params[i].(float64))))
		}
	}
	return input.Method, params
}

// Init deploys the solidity contract and binds the eth address.
func (r *Runner) Init(ctx common.CallContext) error {
	vmenv := runtime.NewEnv(r.config)
	sender := evm.AccountRef(r.config.Origin)
	contractBin, _ := hexutil.Decode(r.hexCode)
	contractAbi, _ := abi.JSON(strings.NewReader(r.jsonABI))

	_, args := r.parseInput(ctx.Input())
	invoke, err := contractAbi.Pack("", args...)
	if err != nil {
		return err
	}
	deploy := append(contractBin, invoke...)

	_, address, leftOverGas, err := vmenv.Create(
		sender,
		deploy,
		r.config.GasLimit,
		r.config.Value,
	)
	ctx.SetState(keyAddr, address.Bytes())
	ctx.SetState(keyAbi, []byte(r.jsonABI))

	fmt.Printf("deploy code at: %s, used gas: %d\n", address.String(), r.config.GasLimit-leftOverGas)
	return err
}

func (r *Runner) Invoke(ctx common.CallContext) error {
	vmenv := runtime.NewEnv(r.config)
	sender := evm.AccountRef(r.config.Origin)
	address := ethcomm.BytesToAddress(ctx.GetState(keyAddr))
	contractAbi, _ := abi.JSON(bytes.NewReader(ctx.GetState(keyAbi)))

	method, params := r.parseInput(ctx.Input())
	invoke, err := contractAbi.Pack(method, params...)
	if err != nil {
		return err
	}

	ret, leftOverGas, err := vmenv.Call(
		sender,
		address,
		invoke,
		r.config.GasLimit,
		r.config.Value,
	)

	fmt.Printf("invoke code at: %s, used gas: %d, return result: %s\n", address.String(),
		r.config.GasLimit-leftOverGas, big.NewInt(0).SetBytes(ret)) // TODO use logger.I()
	return err
}

func (r *Runner) Query(ctx common.CallContext) ([]byte, error) {
	vmenv := runtime.NewEnv(r.config)
	sender := evm.AccountRef(r.config.Origin)
	address := ethcomm.BytesToAddress(ctx.GetState(keyAddr))
	contractAbi, _ := abi.JSON(bytes.NewReader(ctx.GetState(keyAbi)))

	method, params := r.parseInput(ctx.Input())
	query, err := contractAbi.Pack(method, params...)
	if err != nil {
		return nil, err
	}

	ret, leftOverGas, err := vmenv.StaticCall(
		sender,
		address,
		query,
		r.config.GasLimit,
	)

	fmt.Printf("query code at: %s, used gas: %d, return result: %s\n", address.String(),
		r.config.GasLimit-leftOverGas, big.NewInt(0).SetBytes(ret)) // TODO use logger.I()
	return ret, err
}
