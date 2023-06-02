// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package main

import (
	"github.com/wooyang2018/posv-blockchain/chaincode/pcoin"
	"github.com/wooyang2018/posv-blockchain/execution/bincc"
)

// bincc version of pcoin. User can compile and deploy it separately to the running posv network

func main() {
	jcc := new(pcoin.PCoin)
	bincc.RunChaincode(jcc)
}
