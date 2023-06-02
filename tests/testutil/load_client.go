// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package testutil

import (
	"github.com/wooyang2018/posv-blockchain/core"
	"github.com/wooyang2018/posv-blockchain/tests/cluster"
)

type LoadClient interface {
	SetupOnCluster(cls *cluster.Cluster) error
	SubmitTx() (int, *core.Transaction, error)
	BatchSubmitTx(num int) (int, *core.TxList, error)
	SubmitTxAndWait() (int, error)
}
