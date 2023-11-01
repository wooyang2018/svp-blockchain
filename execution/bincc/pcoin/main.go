// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package main

import (
	"github.com/wooyang2018/svp-blockchain/execution/bincc"
	"github.com/wooyang2018/svp-blockchain/native/pcoin"
)

// bincc version of pcoin.
// user can compile and deploy it separately to the running network

func main() {
	jcc := new(pcoin.PCoin)
	bincc.RunChaincode(jcc)
}
