// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package native

import (
	"bytes"
	"errors"

	"github.com/wooyang2018/svp-blockchain/execution/common"
	"github.com/wooyang2018/svp-blockchain/native/empty"
	"github.com/wooyang2018/svp-blockchain/native/kvdb"
	"github.com/wooyang2018/svp-blockchain/native/pcoin"
	"github.com/wooyang2018/svp-blockchain/native/xcoin"
)

var (
	CodeEmpty = bytes.Repeat([]byte{1}, 32)
	CodeKVDB  = bytes.Repeat([]byte{2}, 32)
	CodePCoin = bytes.Repeat([]byte{3}, 32)
	CodeXCoin = bytes.Repeat([]byte{4}, 32)
)

type CodeDriver struct{}

var _ common.CodeDriver = (*CodeDriver)(nil)

func NewCodeDriver() *CodeDriver {
	return new(CodeDriver)
}

func (drv *CodeDriver) Install(codeID, data []byte) error {
	_, err := drv.GetInstance(codeID)
	return err
}

func (drv *CodeDriver) GetInstance(codeID []byte) (common.Chaincode, error) {
	switch string(codeID) {
	case string(CodeEmpty):
		return new(empty.Empty), nil
	case string(CodeKVDB):
		return new(kvdb.KVDB), nil
	case string(CodePCoin):
		return new(pcoin.PCoin), nil
	case string(CodeXCoin):
		return new(xcoin.XCoin), nil
	default:
		return nil, errors.New("unknown native chaincode id")
	}
}
