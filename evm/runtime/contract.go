// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package runtime

import (
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/holiman/uint256"

	"github.com/wooyang2018/svp-blockchain/logger"
)

type Contract struct {
	Abi        abi.ABI
	Cfg        *Config
	Address    common.Address
	AutoCommit bool
}

func Ensure(err error) {
	if err != nil {
		panic(err)
	}
}

func Create2Contract(cfg *Config, jsonABI, hexCode string, salt uint64, params ...interface{}) *Contract {
	contractBin, err := hexutil.Decode(hexCode)
	Ensure(err)

	cabi, err := abi.JSON(strings.NewReader(jsonABI))
	Ensure(err)
	var p []byte
	if len(params) != 0 {
		p, err = cabi.Pack("", params...)
		Ensure(err)
	}
	deploy := append(contractBin, p...)

	_, ctAddr, leftGas, err := Create2(deploy, cfg, uint256.NewInt(0).SetUint64(salt))
	Ensure(err)

	logger.I().Infof("deploy code at: %s, used gas: %d", ctAddr.String(), cfg.GasLimit-leftGas)

	return &Contract{
		Abi:     cabi,
		Cfg:     cfg,
		Address: ctAddr,
	}
}

// CreateContract create and deploy a contract
func CreateContract(cfg *Config, jsonABI string, hexCode string, params ...interface{}) *Contract {
	contractBin, err := hexutil.Decode(hexCode)
	Ensure(err)

	cabi, err := abi.JSON(strings.NewReader(jsonABI))
	Ensure(err)
	var p []byte
	if len(params) != 0 {
		p, err = cabi.Pack("", params...)
		Ensure(err)
	}
	deploy := append(contractBin, p...)

	_, ctAddr, leftGas, err := Create(deploy, cfg)
	Ensure(err)

	logger.I().Infof("deploy code at: %s, used gas: %d", ctAddr.String(), cfg.GasLimit-leftGas)

	return &Contract{
		Abi:     cabi,
		Cfg:     cfg,
		Address: ctAddr,
	}
}

func (self *Contract) Call(method string, params ...interface{}) ([]byte, uint64, error) {
	input, err := self.Abi.Pack(method, params...)
	Ensure(err)

	ret, gas, err := Call(self.Address, input, self.Cfg)
	if self.AutoCommit {
		err := self.Cfg.State.Commit()
		Ensure(err)
	}
	return ret, self.Cfg.GasLimit - gas, err
}

func (self *Contract) Balance() *big.Int {
	return self.Cfg.State.GetBalance(self.Address)
}

func (self *Contract) BalanceOf(addr common.Address) *big.Int {
	return self.Cfg.State.GetBalance(addr)
}
