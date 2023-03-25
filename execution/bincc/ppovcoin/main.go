// Copyright (C) 2021 Aung Maw
// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package main

import (
	"github.com/wooyang2018/ppov-blockchain/chaincode/ppovcoin"
	"github.com/wooyang2018/ppov-blockchain/execution/bincc"
)

// bincc version of ppovcoin. User can compile and deploy it separately to the running ppov network

func main() {
	jcc := new(ppovcoin.PPoVCoin)
	bincc.RunChaincode(jcc)
}
