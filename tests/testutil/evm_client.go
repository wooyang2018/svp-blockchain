// Copyright (C) 2023 Chenrui
// Licensed under the GNU General Public License v3.0

package testutil

import (
	"github.com/wooyang2018/svp-blockchain/core"
	"github.com/wooyang2018/svp-blockchain/tests/cluster"
)

type EVMClient struct {
}

var _ LoadClient = (*EVMClient)(nil)

func (E EVMClient) SetupOnCluster(cls *cluster.Cluster) error {
	panic("implement me")
}

func (E EVMClient) SubmitTx() (int, *core.Transaction, error) {
	panic("implement me")
}

func (E EVMClient) BatchSubmitTx(num int) (int, *core.TxList, error) {
	panic("implement me")
}

func (E EVMClient) SubmitTxAndWait() (int, error) {
	panic("implement me")
}
