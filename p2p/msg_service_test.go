// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package p2p

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/wooyang2018/svp-blockchain/core"
)

func setupMsgServiceWithLoopBackPeers() (*MsgService, [][]byte, []*Peer) {
	peers := make([]*Peer, 2)
	peers[0] = NewPeer(core.GenerateKey(nil).PublicKey(), nil)
	peers[1] = NewPeer(core.GenerateKey(nil).PublicKey(), nil)

	s1 := peers[0].SubscribeMsg()
	s2 := peers[1].SubscribeMsg()

	raws := make([][]byte, 2)

	go func() {
		for e := range s1.Events() {
			raws[0] = e.([]byte)
		}
	}()

	go func() {
		for e := range s2.Events() {
			raws[1] = e.([]byte)
		}
	}()

	host := new(Host)
	host.peerStore = NewPeerStore()

	peers[0].onConnected(newRWCLoopBack())
	peers[1].onConnected(newRWCLoopBack())
	host.peerStore.Store(peers[0])
	host.peerStore.Store(peers[1])

	svc := NewMsgService(host)
	time.Sleep(time.Millisecond)
	return svc, raws, peers
}

func newTestProposal(priv core.Signer) (*core.Vote, *core.QuorumCert, *core.Block) {
	blk0 := core.NewBlock().
		SetHeight(9).
		Sign(priv).
		Vote(priv, 1)
	qc := core.NewQuorumCert().Build(priv, []*core.Vote{blk0})
	blk := core.NewBlock().
		SetHeight(10).
		SetQuorumCert(qc).
		Sign(priv)
	return blk0, qc, blk
}

func TestMsgService_BroadcastProposal(t *testing.T) {
	asrt := assert.New(t)

	svc, raws, _ := setupMsgServiceWithLoopBackPeers()
	sub := svc.SubscribeProposal(5)
	var recvBlk *core.Block
	var recvCount int
	go func() {
		for e := range sub.Events() {
			recvCount++
			recvBlk = e.(*core.Block)
		}
	}()

	_, _, blk := newTestProposal(core.GenerateKey(nil))
	err := svc.BroadcastProposal(blk)

	if !asrt.NoError(err) {
		return
	}

	time.Sleep(time.Millisecond)

	asrt.NotNil(raws[0])
	asrt.Equal(raws[0], raws[1])

	asrt.EqualValues(MsgTypeProposal, raws[0][0])

	asrt.Equal(2, recvCount)
	if asrt.NotNil(recvBlk) {
		asrt.Equal(blk.Height(), recvBlk.Height())
	}
}

func TestMsgService_SendVote(t *testing.T) {
	asrt := assert.New(t)

	svc, raws, peers := setupMsgServiceWithLoopBackPeers()

	sub := svc.SubscribeVote(5)
	var recvVote *core.Vote
	go func() {
		for e := range sub.Events() {
			recvVote = e.(*core.Vote)
		}
	}()

	vote, _, _ := newTestProposal(core.GenerateKey(nil))
	err := svc.SendVote(peers[0].PublicKey(), vote)

	if !asrt.NoError(err) {
		return
	}

	time.Sleep(time.Millisecond)

	asrt.NotNil(raws[0])
	asrt.Nil(raws[1])
	asrt.EqualValues(MsgTypeVote, raws[0][0])

	if asrt.NotNil(recvVote) {
		asrt.Equal(vote.BlockHash(), recvVote.BlockHash())
	}
}

func TestMsgService_SendNewView(t *testing.T) {
	asrt := assert.New(t)

	svc, raws, peers := setupMsgServiceWithLoopBackPeers()

	sub := svc.SubscribeQC(5)
	var recvQC *core.QuorumCert
	go func() {
		for e := range sub.Events() {
			recvQC = e.(*core.QuorumCert)
		}
	}()

	_, qc, _ := newTestProposal(core.GenerateKey(nil))
	err := svc.SendQC(peers[0].PublicKey(), qc)

	if !asrt.NoError(err) {
		return
	}

	time.Sleep(time.Millisecond)

	asrt.NotNil(raws[0])
	asrt.Nil(raws[1])
	asrt.EqualValues(MsgTypeQC, raws[0][0])

	if asrt.NotNil(recvQC) {
		asrt.Equal(qc.BlockHash(), recvQC.BlockHash())
	}
}

func TestMsgService_BroadcastTxList(t *testing.T) {
	asrt := assert.New(t)

	svc, raws, _ := setupMsgServiceWithLoopBackPeers()
	sub := svc.SubscribeTxList(5)
	var recvTxs *core.TxList
	var recvCount int
	go func() {
		for e := range sub.Events() {
			recvCount++
			recvTxs = e.(*core.TxList)
		}
	}()

	txs := &core.TxList{
		core.NewTransaction().SetNonce(1).Sign(core.GenerateKey(nil)),
		core.NewTransaction().SetNonce(2).Sign(core.GenerateKey(nil)),
	}
	err := svc.BroadcastTxList(txs)

	if !asrt.NoError(err) {
		return
	}

	time.Sleep(time.Millisecond)

	asrt.NotNil(raws[0])
	asrt.Equal(raws[0], raws[1])
	asrt.EqualValues(MsgTypeTxList, raws[0][0])

	asrt.Equal(2, recvCount)
	if asrt.NotNil(recvTxs) {
		asrt.Equal((*txs)[0].Nonce(), (*recvTxs)[0].Nonce())
		asrt.Equal((*txs)[1].Nonce(), (*recvTxs)[1].Nonce())
	}
}

func TestMsgService_RequestBlock(t *testing.T) {
	asrt := assert.New(t)

	_, _, blk := newTestProposal(core.GenerateKey(nil))
	blkReqHandler := &BlockReqHandler{
		GetBlock: func(hash []byte) (*core.Block, error) {
			if bytes.Equal(blk.Hash(), hash) {
				return blk, nil
			}
			return nil, errors.New("block not found")
		},
	}
	svc, _, peers := setupMsgServiceWithLoopBackPeers()
	svc.SetReqHandler(blkReqHandler)

	recvBlk, err := svc.RequestBlock(peers[0].PublicKey(), blk.Hash())
	if asrt.NoError(err) && asrt.NotNil(recvBlk) {
		asrt.Equal(blk.Height(), recvBlk.Height())
	}

	_, err = svc.RequestBlock(peers[0].PublicKey(), []byte{1})
	asrt.Error(err)
}

func TestMsgService_RequestTxList(t *testing.T) {
	asrt := assert.New(t)

	var txs = &core.TxList{
		core.NewTransaction().SetNonce(1).Sign(core.GenerateKey(nil)),
		core.NewTransaction().SetNonce(2).Sign(core.GenerateKey(nil)),
	}

	txListReqHandler := &TxListReqHandler{
		GetTxList: func(hashes [][]byte) (*core.TxList, error) {
			return txs, nil
		},
	}
	svc, _, peers := setupMsgServiceWithLoopBackPeers()
	svc.SetReqHandler(txListReqHandler)

	recvTxs, err := svc.RequestTxList(peers[0].PublicKey(), [][]byte{{1}, {2}})
	if asrt.NoError(err) && asrt.NotNil(recvTxs) {
		asrt.Equal((*txs)[0].Nonce(), (*recvTxs)[0].Nonce())
		asrt.Equal((*txs)[1].Nonce(), (*recvTxs)[1].Nonce())
	}
}
