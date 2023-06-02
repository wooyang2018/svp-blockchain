// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package execution

import (
	"bytes"
	"errors"

	"github.com/wooyang2018/posv-blockchain/chaincode/empty"
	"github.com/wooyang2018/posv-blockchain/chaincode/pcoin"
	"github.com/wooyang2018/posv-blockchain/execution/chaincode"
)

var (
	NativeCodeIDEmpty = bytes.Repeat([]byte{1}, 32)
	NativeCodeIDPCoin = bytes.Repeat([]byte{2}, 32)
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
	case string(NativeCodeIDEmpty):
		return new(empty.Empty), nil
	case string(NativeCodeIDPCoin):
		return new(pcoin.PCoin), nil
	default:
		return nil, errors.New("unknown native chaincode id")
	}
}
