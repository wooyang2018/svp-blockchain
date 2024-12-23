// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package evm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/big"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"

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
var mtxExec sync.RWMutex

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

	mtxExec.Lock()
	_, address, leftOverGas, err := vmenv.Create(
		sender,
		deploy,
		r.config.GasLimit,
		r.config.Value,
	)
	err = r.config.State.Commit()
	common.Check(err)
	mtxExec.Unlock()

	ctx.SetState(keyAddr, address.Bytes())
	ctx.SetState(keyAbi, []byte(jsonABI))
	err = r.Proxy.storeAddr(address.Bytes())
	if err != nil {
		return err
	}

	fmt.Printf("deploy code at: %s, used gas: %d\n", address.String(), r.config.GasLimit-leftOverGas)
	r.Proxy.mergeTrks()
	return nil
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

	mtxExec.Lock()
	ret, leftOverGas, err := vmenv.Call(
		sender,
		address,
		invoke,
		r.config.GasLimit,
		r.config.Value,
	)
	err = r.config.State.Commit()
	common.Check(err)
	mtxExec.Unlock()

	retSlice, _ := contractAbi.Unpack(input.Method, ret)
	res, _ := json.Marshal(retSlice)
	fmt.Printf("invoke code at: %s, used gas: %d, return result: %s\n", address.String(),
		r.config.GasLimit-leftOverGas, res)
	r.Proxy.mergeTrks()
	return nil
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

	mtxExec.RLock()
	ret, leftOverGas, err := vmenv.StaticCall(
		sender,
		address,
		query,
		r.config.GasLimit,
	)
	mtxExec.RUnlock()

	retSlice, _ := contractAbi.Unpack(input.Method, ret)
	res, _ := json.Marshal(retSlice)
	fmt.Printf("query code at: %s, used gas: %d, return result: %s\n", address.String(),
		r.config.GasLimit-leftOverGas, res)
	return res, nil
}

func (r *Runner) SetTxTrk(txTrk *common.StateTracker) {
	r.Proxy.setTxTrk(txTrk)
}

func (r *Runner) setConfig(ctx common.CallContext) error {
	cfg := new(runtime.Config)
	runtime.SetDefaults(cfg)
	cfg.BlockNumber = big.NewInt(0).SetUint64(ctx.BlockHeight())

	addr20, err := r.Proxy.queryAddr(ctx.Sender())
	if err != nil {
		return err
	}
	cfg.Origin = ethcomm.BytesToAddress(addr20)

	r.StateDB.Prepare(ethcomm.BytesToHash(ctx.TransactionHash()),
		ethcomm.BytesToHash(ctx.BlockHash()))
	cfg.State = r.StateDB
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
	arrayRegex := regexp.MustCompile(`^(\w+)\[(\d*)\]$`)
	if arrayRegex.MatchString(paramType) {
		matches := arrayRegex.FindStringSubmatch(paramType)
		baseType := matches[1]
		baseSize := matches[2]
		elemType := getElemType(baseType)
		params := strings.Split(param, ",")
		if baseSize != "" {
			length, _ := strconv.Atoi(baseSize)
			arrayType := reflect.ArrayOf(length, elemType)
			arrayValue := reflect.New(arrayType).Elem()
			for i := 0; i < len(params); i++ {
				value := parseParam(baseType, strings.TrimSpace(params[i]))
				arrayValue.Index(i).Set(reflect.ValueOf(value))
			}
			return arrayValue.Interface()
		} else {
			arrayType := reflect.SliceOf(elemType)
			arrayValue := reflect.MakeSlice(arrayType, len(params), len(params))
			for i := 0; i < len(params); i++ {
				value := parseParam(baseType, strings.TrimSpace(params[i]))
				arrayValue.Index(i).Set(reflect.ValueOf(value))
			}
			return arrayValue.Interface()
		}
	}
	return getElemValue(paramType, param)
}

func getElemType(baseType string) reflect.Type {
	var elemType reflect.Type
	switch baseType {
	case "uint256":
		elemType = reflect.TypeOf(big.NewInt(0))
	case "string":
		elemType = reflect.TypeOf("")
	case "uint32":
		elemType = reflect.TypeOf(uint32(0))
	case "uint16":
		elemType = reflect.TypeOf(uint16(0))
	case "address":
		elemType = reflect.TypeOf(ethcomm.Address{})
	case "bytes":
		elemType = reflect.TypeOf([]byte{})
	case "bool":
		elemType = reflect.TypeOf(false)
	}
	return elemType
}

func getElemValue(paramType string, param string) interface{} {
	switch paramType {
	case "uint256":
		tmp, err := strconv.ParseInt(param, 10, 64)
		common.Check(err)
		return big.NewInt(tmp)
	case "string":
		return param
	case "uint32":
		tmp, err := strconv.ParseUint(param, 10, 32)
		common.Check(err)
		return uint32(tmp)
	case "uint16":
		tmp, err := strconv.ParseUint(param, 10, 16)
		common.Check(err)
		return uint16(tmp)
	case "address":
		return ethcomm.HexToAddress(param)
	case "bytes":
		tmp, err := common.Address32ToBytes(param)
		common.Check(err)
		return tmp
	case "bool":
		return param == "true"
	default:
		return nil
	}
}
