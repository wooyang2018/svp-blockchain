// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package p2p

import (
	"encoding/binary"

	"github.com/wooyang2018/ppov-blockchain/core"
	"github.com/wooyang2018/ppov-blockchain/pb"
	"google.golang.org/protobuf/proto"
)

type ReqHandler interface {
	Type() pb.Request_Type
	HandleReq(sender *core.PublicKey, data []byte) ([]byte, error)
}

type TxListReqHandler struct {
	GetTxList func(hashes [][]byte) (*core.TxList, error)
}

var _ ReqHandler = (*TxListReqHandler)(nil)

func (hdlr *TxListReqHandler) Type() pb.Request_Type {
	return pb.Request_TxList
}

func (hdlr *TxListReqHandler) HandleReq(sender *core.PublicKey, data []byte) ([]byte, error) {
	req := new(pb.HashList)
	if err := proto.Unmarshal(data, req); err != nil {
		return nil, err
	}
	txList, err := hdlr.GetTxList(req.List)
	if err != nil {
		return nil, err
	}
	return txList.Marshal()
}

type BlockReqHandler struct {
	GetBlock func(hash []byte) (*core.Block, error)
}

var _ ReqHandler = (*BlockReqHandler)(nil)

func (hdlr *BlockReqHandler) Type() pb.Request_Type {
	return pb.Request_Block
}

func (hdlr *BlockReqHandler) HandleReq(sender *core.PublicKey, data []byte) ([]byte, error) {
	block, err := hdlr.GetBlock(data)
	if err != nil {
		return nil, err
	}
	return block.Marshal()
}

type BlockByHeightReqHandler struct {
	GetBlockByHeight func(height uint64) (*core.Block, error)
}

var _ ReqHandler = (*BlockByHeightReqHandler)(nil)

func (hdlr *BlockByHeightReqHandler) Type() pb.Request_Type {
	return pb.Request_BlockByHeight
}

func (hdlr *BlockByHeightReqHandler) HandleReq(sender *core.PublicKey, data []byte) ([]byte, error) {
	height := binary.BigEndian.Uint64(data)
	block, err := hdlr.GetBlockByHeight(height)
	if err != nil {
		return nil, err
	}
	return block.Marshal()
}
