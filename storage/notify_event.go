// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package storage

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common/hexutil"

	"github.com/wooyang2018/svp-blockchain/evm/common"
)

// NotifyEventInfo describe smart contract event notify info struct
type NotifyEventInfo struct {
	ContractAddress common.Address
	States          interface{}

	IsEvm bool
}

type ExecuteNotify struct {
	TxHash      common.Uint256
	State       byte
	GasConsumed uint64 // in GWei
	Notify      []*NotifyEventInfo

	GasStepUsed     uint64
	TxIndex         uint32
	CreatedContract common.Address
}

func NotifyEventInfoFromEvmLog(log *common.StorageLog) *NotifyEventInfo {
	raw := common.SerializeToBytes(log)

	return &NotifyEventInfo{
		ContractAddress: common.Address(log.Address),
		States:          hexutil.Bytes(raw),
		IsEvm:           true,
	}
}

func NotifyEventInfoToEvmLog(n *NotifyEventInfo) (*common.StorageLog, error) {
	if !n.IsEvm {
		return nil, fmt.Errorf("not evm event")
	}

	var data []byte
	var err error
	switch val := n.States.(type) {
	case string:
		if data, err = hexutil.Decode(val); err != nil {
			return nil, err
		}
	case hexutil.Bytes:
		data = val
	default:
		return nil, fmt.Errorf("not support such states type")
	}

	source := common.NewZeroCopySource(data)
	var storageLog common.StorageLog
	if err = storageLog.Deserialization(source); err != nil {
		return nil, err
	}

	return &storageLog, nil
}
