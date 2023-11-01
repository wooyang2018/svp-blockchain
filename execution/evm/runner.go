// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package evm

import (
	"github.com/wooyang2018/svp-blockchain/execution/common"
)

type Runner struct{}

var _ common.Chaincode = (*Runner)(nil)

func (r *Runner) Init(ctx common.CallContext) error {
	return nil
}

func (r *Runner) Invoke(ctx common.CallContext) error {
	return nil
}

func (r *Runner) Query(ctx common.CallContext) ([]byte, error) {
	return nil, nil
}
