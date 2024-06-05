// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package empty

import (
	"github.com/wooyang2018/svp-blockchain/execution/common"
)

// Empty chaincode
type Empty struct{}

var _ common.Chaincode = (*Empty)(nil)

func (c *Empty) Init(ctx common.CallContext) error {
	return nil
}

func (c *Empty) Invoke(ctx common.CallContext) error {
	return nil
}

func (c *Empty) Query(ctx common.CallContext) ([]byte, error) {
	return nil, nil
}

func (c *Empty) SetTxTrk(txTrk *common.StateTracker) {
	return
}
