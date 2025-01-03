// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package p2p

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"google.golang.org/protobuf/proto"

	"github.com/wooyang2018/svp-blockchain/core"
	"github.com/wooyang2018/svp-blockchain/emitter"
	"github.com/wooyang2018/svp-blockchain/logger"
	"github.com/wooyang2018/svp-blockchain/pb"
)

type MsgType byte

const (
	_ MsgType = iota
	MsgTypeProposal
	MsgTypeVote
	MsgTypeQC
	MsgTypeTxList
	MsgTypeRequest
	MsgTypeResponse
)

func (t MsgType) String() string {
	switch t {
	case MsgTypeProposal:
		return "Proposal"
	case MsgTypeVote:
		return "Vote"
	case MsgTypeQC:
		return "QC"
	case MsgTypeTxList:
		return "TxList"
	case MsgTypeRequest:
		return "Request"
	case MsgTypeResponse:
		return "Response"
	default:
		return "Unknown"
	}
}

type msgReceiver func(peer *Peer, data []byte)
type topicReceiver func(data []byte)

type MsgService struct {
	host           *Host
	pointReceivers map[MsgType]msgReceiver
	topicReceivers map[MsgType]topicReceiver

	proposalEmitter *emitter.Emitter
	voteEmitter     *emitter.Emitter
	qcEmitter       *emitter.Emitter
	txListEmitter   *emitter.Emitter

	reqHandlers  map[pb.Request_Type]ReqHandler
	reqClientSeq uint32
}

func NewMsgService(host *Host) *MsgService {
	svc := new(MsgService)
	svc.host = host
	go svc.listenPoint()
	go svc.listenTopic()

	svc.reqHandlers = make(map[pb.Request_Type]ReqHandler)
	svc.setEmitters()
	svc.setMsgReceivers()
	svc.setTopicReceivers()
	return svc
}

func (svc *MsgService) SubscribeProposal(buffer int) *emitter.Subscription {
	return svc.proposalEmitter.Subscribe(buffer)
}

func (svc *MsgService) SubscribeVote(buffer int) *emitter.Subscription {
	return svc.voteEmitter.Subscribe(buffer)
}

func (svc *MsgService) SubscribeQC(buffer int) *emitter.Subscription {
	return svc.qcEmitter.Subscribe(buffer)
}

func (svc *MsgService) SubscribeTxList(buffer int) *emitter.Subscription {
	return svc.txListEmitter.Subscribe(buffer)
}

func (svc *MsgService) BroadcastProposal(blk *core.Block) error {
	data, err := blk.Marshal()
	if err != nil {
		return err
	}
	return svc.broadcastData(MsgTypeProposal, data)
}

func (svc *MsgService) SendVote(pubKey *core.PublicKey, vote *core.Vote) error {
	data, err := vote.Marshal()
	if err != nil {
		return err
	}
	return svc.sendData(pubKey, MsgTypeVote, data)
}

func (svc *MsgService) SendQC(pubKey *core.PublicKey, qc *core.QuorumCert) error {
	data, err := qc.Marshal()
	if err != nil {
		return err
	}
	return svc.sendData(pubKey, MsgTypeQC, data)
}

func (svc *MsgService) BroadcastQC(qc *core.QuorumCert) error {
	data, err := qc.Marshal()
	if err != nil {
		return err
	}
	return svc.broadcastData(MsgTypeQC, data)
}

func (svc *MsgService) BroadcastTxList(txList *core.TxList) error {
	data, err := txList.Marshal()
	if err != nil {
		return err
	}
	return svc.broadcastData(MsgTypeTxList, data)
}

func (svc *MsgService) RequestBlock(pubKey *core.PublicKey, hash []byte) (*core.Block, error) {
	respData, err := svc.requestData(pubKey, pb.Request_Block, hash)
	if err != nil {
		return nil, err
	}
	blk := core.NewBlock()
	if err := blk.Unmarshal(respData); err != nil {
		return nil, err
	}
	return blk, nil
}

func (svc *MsgService) RequestBlockByHeight(pubKey *core.PublicKey, height uint64) (*core.Block, error) {
	buf := bytes.NewBuffer(nil)
	binary.Write(buf, binary.BigEndian, height)
	respData, err := svc.requestData(pubKey, pb.Request_BlockByHeight, buf.Bytes())
	if err != nil {
		return nil, err
	}
	blk := core.NewBlock()
	if err := blk.Unmarshal(respData); err != nil {
		return nil, err
	}
	return blk, nil
}

func (svc *MsgService) RequestQC(pubKey *core.PublicKey, blkHash []byte) (*core.QuorumCert, error) {
	respData, err := svc.requestData(pubKey, pb.Request_QC, blkHash)
	if err != nil {
		return nil, err
	}
	qc := core.NewQuorumCert()
	if err := qc.Unmarshal(respData); err != nil {
		return nil, err
	}
	return qc, nil
}

func (svc *MsgService) RequestTxList(pubKey *core.PublicKey, hashes [][]byte) (*core.TxList, error) {
	hl := new(pb.HashList)
	hl.List = hashes
	reqData, _ := proto.Marshal(hl)
	respData, err := svc.requestData(pubKey, pb.Request_TxList, reqData)
	if err != nil {
		return nil, err
	}
	txList := core.NewTxList()
	if err := txList.Unmarshal(respData); err != nil {
		return nil, err
	}
	return txList, nil
}

func (svc *MsgService) SetReqHandler(reqHandler ReqHandler) error {
	if _, found := svc.reqHandlers[reqHandler.Type()]; found {
		return fmt.Errorf("request handler already set %s", reqHandler.Type())
	}
	svc.reqHandlers[reqHandler.Type()] = reqHandler
	return nil
}

func (svc *MsgService) setEmitters() {
	svc.proposalEmitter = emitter.New()
	svc.voteEmitter = emitter.New()
	svc.qcEmitter = emitter.New()
	svc.txListEmitter = emitter.New()
}

func (svc *MsgService) setMsgReceivers() {
	svc.pointReceivers = make(map[MsgType]msgReceiver)
	svc.pointReceivers[MsgTypeProposal] = svc.onReceiveProposal
	svc.pointReceivers[MsgTypeVote] = svc.onReceiveVote
	svc.pointReceivers[MsgTypeQC] = svc.onReceiveQC
	svc.pointReceivers[MsgTypeTxList] = svc.onReceiveTxList
	svc.pointReceivers[MsgTypeRequest] = svc.onReceiveRequest
	svc.pointReceivers[MsgTypeResponse] = func(peer *Peer, data []byte) {}
}

func (svc *MsgService) setTopicReceivers() {
	svc.topicReceivers = make(map[MsgType]topicReceiver)
	svc.topicReceivers[MsgTypeProposal] = svc.onReceiveProposal2
	svc.topicReceivers[MsgTypeTxList] = svc.onReceiveTxList2
	svc.topicReceivers[MsgTypeQC] = svc.onReceiveQC2
}

func (svc *MsgService) listenPoint() {
	sub := svc.host.SubscribePointMsg()
	for e := range sub.Events() {
		msg := e.(*PointMsg)
		if len(msg.data) < 2 {
			continue // invalid message
		}
		if receiver, found := svc.pointReceivers[MsgType(msg.data[0])]; found {
			receiver(msg.peer, msg.data[1:])
		} else {
			logger.I().Errorw("msg service received invalid point message", "type", MsgType(msg.data[0]))
		}
	}
}

func (svc *MsgService) listenTopic() {
	sub := svc.host.SubscribeTopicMsg()
	for e := range sub.Events() {
		msg := e.(*pubsub.Message)
		if receiver, found := svc.topicReceivers[MsgType(msg.Data[0])]; found {
			receiver(msg.Data[1:])
		} else {
			logger.I().Errorw("msg service received invalid topic message", "type", MsgType(msg.Data[0]))
		}
	}
}

func (svc *MsgService) onReceiveProposal(peer *Peer, data []byte) {
	blk := core.NewBlock()
	if err := blk.Unmarshal(data); err != nil {
		logger.I().Errorw("msg service receive proposal failed", "error", err)
		return
	}
	svc.proposalEmitter.Emit(blk)
}

func (svc *MsgService) onReceiveProposal2(data []byte) {
	blk := core.NewBlock()
	if err := blk.Unmarshal(data); err != nil {
		logger.I().Errorw("msg service receive proposal2 failed", "error", err)
		return
	}
	svc.proposalEmitter.Emit(blk)
}

func (svc *MsgService) onReceiveVote(peer *Peer, data []byte) {
	vote := core.NewVote()
	if err := vote.Unmarshal(data); err != nil {
		logger.I().Errorw("msg service receive vote failed", "error", err)
		return
	}
	svc.voteEmitter.Emit(vote)
}

func (svc *MsgService) onReceiveQC(peer *Peer, data []byte) {
	qc := core.NewQuorumCert()
	if err := qc.Unmarshal(data); err != nil {
		logger.I().Errorw("msg service receive qc failed", "error", err)
		return
	}
	svc.qcEmitter.Emit(qc)
}

func (svc *MsgService) onReceiveQC2(data []byte) {
	qc := core.NewQuorumCert()
	if err := qc.Unmarshal(data); err != nil {
		logger.I().Errorw("msg service receive qc2 failed", "error", err)
		return
	}
	svc.qcEmitter.Emit(qc)
}

func (svc *MsgService) onReceiveTxList(peer *Peer, data []byte) {
	txList := core.NewTxList()
	if err := txList.Unmarshal(data); err != nil {
		logger.I().Errorw("msg service receive txs failed", "error", err)
		return
	}
	svc.txListEmitter.Emit(txList)
}

func (svc *MsgService) onReceiveTxList2(data []byte) {
	txList := core.NewTxList()
	if err := txList.Unmarshal(data); err != nil {
		logger.I().Errorw("msg service receive txs2 failed", "error", err)
		return
	}
	svc.txListEmitter.Emit(txList)
}

func (svc *MsgService) onReceiveRequest(peer *Peer, data []byte) {
	req := new(pb.Request)
	if err := proto.Unmarshal(data, req); err != nil {
		return
	}
	resp := new(pb.Response)
	resp.Seq = req.Seq

	if hdlr, found := svc.reqHandlers[req.Type]; found {
		data, err := hdlr.HandleReq(req.Data)
		if err != nil {
			resp.Error = err.Error()
		} else {
			resp.Data = data
		}
	} else {
		resp.Error = "no handler for request"
	}
	b, _ := proto.Marshal(resp)
	peer.WriteMsg(append([]byte{byte(MsgTypeResponse)}, b...))
}

func (svc *MsgService) broadcastData(msgType MsgType, data []byte) error {
	return svc.host.chatRoom.Publish(append([]byte{byte(msgType)}, data...))
}

func (svc *MsgService) sendData(pubKey *core.PublicKey, msgType MsgType, data []byte) error {
	peer := svc.host.RoleStore().Load(pubKey)
	if peer == nil {
		return errors.New("peer not found")
	}
	return peer.WriteMsg(append([]byte{byte(msgType)}, data...))
}

func (svc *MsgService) requestData(pubKey *core.PublicKey,
	reqType pb.Request_Type, reqData []byte) ([]byte, error) {
	peer := svc.host.RoleStore().Load(pubKey)
	if peer == nil {
		return nil, errors.New("peer not found")
	}
	req := new(pb.Request)
	req.Type = reqType
	req.Data = reqData
	req.Seq = atomic.AddUint32(&svc.reqClientSeq, 1)
	b, _ := proto.Marshal(req)

	sub := svc.host.SubscribePointMsg()
	defer sub.Unsubscribe()

	if err := peer.WriteMsg(append([]byte{byte(MsgTypeRequest)}, b...)); err != nil {
		return nil, err
	}
	return svc.waitResponse(sub, peer, req.Seq)
}

func (svc *MsgService) waitResponse(sub *emitter.Subscription, peer *Peer, seq uint32) ([]byte, error) {
	timeout := time.After(5 * time.Second)
	for {
		select {
		case <-timeout:
			return nil, errors.New("request timeout")
		case e := <-sub.Events():
			msg := e.(*PointMsg)
			if len(msg.data) < 2 || msg.peer != peer {
				continue
			}
			if MsgType(msg.data[0]) == MsgTypeResponse {
				resp := new(pb.Response)
				if err := proto.Unmarshal(msg.data[1:], resp); err != nil {
					continue
				}
				if resp.Seq == seq {
					if len(resp.Error) > 0 {
						return nil, errors.New(resp.Error)
					}
					return resp.Data, nil
				}
			}
		}
	}
}
