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
	"fmt"
	"math/big"
	"os/exec"
	"regexp"
	"strings"

	"github.com/ethereum/go-ethereum/common"

	"github.com/wooyang2018/svp-blockchain/evm"
)

func NewEnv(cfg *Config) *evm.EVM {
	txContext := evm.TxContext{
		Origin:   cfg.Origin,
		GasPrice: cfg.GasPrice,
	}
	blockContext := evm.BlockContext{
		CanTransfer: CanTransfer,
		Transfer:    Transfer,
		GetHash:     cfg.GetHashFn,
		Coinbase:    cfg.Coinbase,
		BlockNumber: cfg.BlockNumber,
		Time:        cfg.Time,
		Difficulty:  cfg.Difficulty,
		GasLimit:    cfg.GasLimit,
	}
	return evm.NewEVM(blockContext, txContext, cfg.State, cfg.ChainConfig, cfg.EVMConfig)
}

// CanTransfer checks whether there are enough funds in the address' account to make a transfer.
// This does not take the necessary gas in to account to make the transfer valid.
func CanTransfer(db evm.StateDB, addr common.Address, amount *big.Int) bool {
	return db.GetBalance(addr).Cmp(amount) >= 0
}

// Transfer subtracts amount from sender and adds amount to recipient using the given PersistStore
func Transfer(db evm.StateDB, sender, recipient common.Address, amount *big.Int) {
	db.SubBalance(sender, amount)
	db.AddBalance(recipient, amount)
	evm.MakeOngTransferLog(db, sender, recipient, amount)
}

// CompileCode compiles the solidity code for the specified path.
// See https://docs.soliditylang.org/zh/v0.8.17/installing-solidity.html#linux to install solc command
func CompileCode(path string) map[string][2]string {
	cmd := exec.Command("solc",
		"--evm-version", "paris",
		path,
		"--bin", "--abi", "--optimize")
	fmt.Println(cmd.String())
	output, err := cmd.Output()
	if err != nil {
		panic("compile code failed")
	}

	contracts := make(map[string][2]string)
	pattern := regexp.MustCompile(`^======= \S* =======$`)
	lines := strings.Split(string(output), "\n")
	for i := 0; i < len(lines); i++ {
		if !pattern.MatchString(lines[i]) {
			continue
		}
		nameRegex := regexp.MustCompile(`:\w+`)
		name := nameRegex.FindString(lines[i])[1:]
		i++
		content := [2]string{}
		if lines[i] == "Binary:" {
			i++
			content[0] = "0x" + lines[i]
		}
		i++
		if lines[i] == "Contract JSON ABI" {
			i++
			content[1] = lines[i]
		}
		contracts[name] = content
	}
	return contracts
}
