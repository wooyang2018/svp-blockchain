// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package execution

import (
	"bytes"
	"errors"

	"github.com/wooyang2018/svp-blockchain/chaincode/empty"
	"github.com/wooyang2018/svp-blockchain/chaincode/kvdb"
	"github.com/wooyang2018/svp-blockchain/chaincode/pcoin"
	"github.com/wooyang2018/svp-blockchain/execution/chaincode"
)

var (
	NativeCodeEmpty = bytes.Repeat([]byte{1}, 32)
	NativeCodePCoin = bytes.Repeat([]byte{2}, 32)
	NativeCodeKVDB  = bytes.Repeat([]byte{3}, 32)
)

type nativeCodeDriver struct{}

var _ CodeDriver = (*nativeCodeDriver)(nil)

func newNativeCodeDriver() *nativeCodeDriver {
	return new(nativeCodeDriver)
}

func (drv *nativeCodeDriver) Install(codeID, data []byte) error {
	_, err := drv.GetInstance(codeID)
	return err
}

func (drv *nativeCodeDriver) GetInstance(codeID []byte) (chaincode.Chaincode, error) {
	switch string(codeID) {
	case string(NativeCodeEmpty):
		return new(empty.Empty), nil
	case string(NativeCodePCoin):
		return new(pcoin.PCoin), nil
	case string(NativeCodeKVDB):
		return new(kvdb.KVDB), nil
	default:
		return nil, errors.New("unknown native chaincode id")
	}
}
