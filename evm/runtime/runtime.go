// Copyright 2015 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package runtime

import (
	"math"
	"math/big"
	"time"

	ethcomm "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
	"github.com/holiman/uint256"

	"github.com/wooyang2018/svp-blockchain/evm"
	"github.com/wooyang2018/svp-blockchain/evm/statedb"
	"github.com/wooyang2018/svp-blockchain/storage"
)

// Config is a basic type specifying certain configuration flags for running the EVM.
type Config struct {
	ChainConfig *params.ChainConfig
	Difficulty  *big.Int
	Origin      ethcomm.Address
	Coinbase    ethcomm.Address
	BlockNumber *big.Int
	Time        *big.Int
	GasLimit    uint64
	GasPrice    *big.Int
	Value       *big.Int
	Debug       bool
	EVMConfig   evm.Config
	State       *statedb.StateDB
	GetHashFn   func(n uint64) ethcomm.Hash
}

// SetDefaults sets defaults on the config
func SetDefaults(cfg *Config) {
	if cfg.ChainConfig == nil {
		cfg.ChainConfig = &params.ChainConfig{
			ChainID:             big.NewInt(1),
			HomesteadBlock:      new(big.Int),
			DAOForkBlock:        new(big.Int),
			DAOForkSupport:      false,
			EIP150Block:         new(big.Int),
			EIP155Block:         new(big.Int),
			EIP158Block:         new(big.Int),
			ByzantiumBlock:      new(big.Int),
			ConstantinopleBlock: new(big.Int),
			PetersburgBlock:     new(big.Int),
			IstanbulBlock:       new(big.Int),
			MuirGlacierBlock:    new(big.Int),
		}
	}

	if cfg.Difficulty == nil {
		cfg.Difficulty = new(big.Int)
	}
	if cfg.Time == nil {
		cfg.Time = big.NewInt(time.Now().Unix())
	}
	if cfg.GasLimit == 0 {
		cfg.GasLimit = math.MaxUint64
	}
	if cfg.GasPrice == nil {
		cfg.GasPrice = new(big.Int)
	}
	if cfg.Value == nil {
		cfg.Value = new(big.Int)
	}
	if cfg.BlockNumber == nil {
		cfg.BlockNumber = new(big.Int)
	}
	if cfg.GetHashFn == nil {
		cfg.GetHashFn = func(n uint64) ethcomm.Hash {
			return ethcomm.BytesToHash(crypto.Keccak256([]byte(new(big.Int).SetUint64(n).String())))
		}
	}
}

// Execute executes the code using the input as call data during the execution.
// It returns the EVM's return value, the new state and an error if it failed.
//
// Execute sets up an in-memory, temporary, environment for the execution of
// the given code. It makes sure that it's restored to its original state afterwards.
func Execute(code, input []byte, cfg *Config) ([]byte, *statedb.StateDB, error) {
	if cfg == nil {
		cfg = new(Config)
	}
	SetDefaults(cfg)

	if cfg.State == nil {
		db := statedb.NewCacheDB(storage.NewMemLevelDBStore())
		cfg.State = statedb.NewStateDB(db, ethcomm.Hash{}, ethcomm.Hash{}, statedb.NewDummy())
	}
	var (
		address = ethcomm.BytesToAddress([]byte("contract"))
		vmenv   = NewEnv(cfg)
		sender  = evm.AccountRef(cfg.Origin)
	)

	cfg.State.CreateAccount(address)
	// set the receiver's (the executing contract) code for execution.
	cfg.State.SetCode(address, code)
	// Call the code with the given configuration.
	ret, _, err := vmenv.Call(
		sender,
		ethcomm.BytesToAddress([]byte("contract")),
		input,
		cfg.GasLimit,
		cfg.Value,
	)

	return ret, cfg.State, err
}

// Create executes the code using the EVM create method
func Create(input []byte, cfg *Config) ([]byte, ethcomm.Address, uint64, error) {
	if cfg == nil {
		cfg = new(Config)
	}
	SetDefaults(cfg)

	if cfg.State == nil {
		db := statedb.NewCacheDB(storage.NewMemLevelDBStore())
		cfg.State = statedb.NewStateDB(db, ethcomm.Hash{}, ethcomm.Hash{}, statedb.NewDummy())
	}
	var (
		vmenv  = NewEnv(cfg)
		sender = evm.AccountRef(cfg.Origin)
	)

	// Call the code with the given configuration.
	code, address, leftOverGas, err := vmenv.Create(
		sender,
		input,
		cfg.GasLimit,
		cfg.Value,
	)
	return code, address, leftOverGas, err
}

// Create2 executes the code using the EVM create2 method
func Create2(input []byte, cfg *Config, salt *uint256.Int) ([]byte, ethcomm.Address, uint64, error) {
	if cfg == nil {
		cfg = new(Config)
	}
	SetDefaults(cfg)

	if cfg.State == nil {
		db := statedb.NewCacheDB(storage.NewMemLevelDBStore())
		cfg.State = statedb.NewStateDB(db, ethcomm.Hash{}, ethcomm.Hash{}, statedb.NewDummy())
	}
	var (
		vmenv  = NewEnv(cfg)
		sender = evm.AccountRef(cfg.Origin)
	)

	// Call the code with the given configuration.
	code, address, leftOverGas, err := vmenv.Create2(
		sender,
		input,
		cfg.GasLimit,
		cfg.Value,
		salt,
	)
	return code, address, leftOverGas, err
}

// Call executes the code given by the contract's address. It will return the
// EVM's return value or an error if it failed.
//
// Call, unlike Execute, requires a config and also requires the State field to
// be set.
func Call(address ethcomm.Address, input []byte, cfg *Config) ([]byte, uint64, error) {
	SetDefaults(cfg)

	vmenv := NewEnv(cfg)
	sender := evm.AccountRef(cfg.Origin)

	// Call the code with the given configuration.
	ret, leftOverGas, err := vmenv.Call(
		sender,
		address,
		input,
		cfg.GasLimit,
		cfg.Value,
	)

	return ret, leftOverGas, err
}
