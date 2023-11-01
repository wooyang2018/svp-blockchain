// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package evm

import (
	"github.com/wooyang2018/svp-blockchain/execution/common"
)

type CodeDriver struct {
}

var _ common.CodeDriver = (*CodeDriver)(nil)

func NewCodeDriver() *CodeDriver {
	return &CodeDriver{}
}

func (c CodeDriver) Install(codeID, data []byte) error {
	return nil
}

func (c CodeDriver) GetInstance(codeID []byte) (common.Chaincode, error) {
	return &Runner{}, nil
}
