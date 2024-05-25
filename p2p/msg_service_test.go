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

func TestBroadcastProposal(t *testing.T) {
	asrt := assert.New(t)

	host1, host2, _, _ := setupTwoHost(t)
	svc1 := NewMsgService(host1)
	svc2 := NewMsgService(host2)
	sub := svc1.SubscribeProposal(5)

	var recvBlk *core.Block
	var recvCount int
	go func() {
		for e := range sub.Events() {
			recvCount++
			recvBlk = e.(*core.Block)
		}
	}()

	_, _, blk := newTestProposal(core.GenerateKey(nil))
	err := svc2.BroadcastProposal(blk)
	asrt.NoError(err)

	time.Sleep(10 * time.Millisecond)
	if asrt.Equal(1, recvCount) && asrt.NotNil(recvBlk) {
		asrt.Equal(blk.Height(), recvBlk.Height())
	}

	host1.Close()
	host2.Close()
}

func TestSendVote(t *testing.T) {
	asrt := assert.New(t)

	host1, host2, peer1, _ := setupTwoHost(t)
	svc1 := NewMsgService(host1)
	svc2 := NewMsgService(host2)
	sub := svc1.SubscribeVote(5)

	var recvVote *core.Vote
	go func() {
		for e := range sub.Events() {
			recvVote = e.(*core.Vote)
		}
	}()

	vote, _, _ := newTestProposal(core.GenerateKey(nil))
	err := svc2.SendVote(peer1.PublicKey(), vote)
	asrt.NoError(err)

	time.Sleep(10 * time.Millisecond)
	if asrt.NotNil(recvVote) {
		asrt.Equal(vote.BlockHash(), recvVote.BlockHash())
	}

	host1.Close()
	host2.Close()
}

func TestSendNewView(t *testing.T) {
	asrt := assert.New(t)

	host1, host2, peer1, _ := setupTwoHost(t)
	svc1 := NewMsgService(host1)
	svc2 := NewMsgService(host2)
	sub := svc1.SubscribeQC(5)

	var recvQC *core.QuorumCert
	go func() {
		for e := range sub.Events() {
			recvQC = e.(*core.QuorumCert)
		}
	}()

	_, qc, _ := newTestProposal(core.GenerateKey(nil))
	err := svc2.SendQC(peer1.PublicKey(), qc)
	asrt.NoError(err)

	time.Sleep(10 * time.Millisecond)
	if asrt.NotNil(recvQC) {
		asrt.Equal(qc.BlockHash(), recvQC.BlockHash())
	}

	host1.Close()
	host2.Close()
}

func TestBroadcastTxList(t *testing.T) {
	asrt := assert.New(t)

	host1, host2, _, _ := setupTwoHost(t)
	svc1 := NewMsgService(host1)
	svc2 := NewMsgService(host2)
	sub := svc1.SubscribeTxList(5)

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
	err := svc2.BroadcastTxList(txs)
	asrt.NoError(err)

	time.Sleep(10 * time.Millisecond)
	if asrt.Equal(1, recvCount) && asrt.NotNil(recvTxs) {
		asrt.Equal((*txs)[0].Nonce(), (*recvTxs)[0].Nonce())
		asrt.Equal((*txs)[1].Nonce(), (*recvTxs)[1].Nonce())
	}

	host1.Close()
	host2.Close()
}

func TestRequestBlock(t *testing.T) {
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

	host1, host2, peer1, _ := setupTwoHost(t)
	svc1 := NewMsgService(host1)
	svc2 := NewMsgService(host2)
	svc1.SetReqHandler(blkReqHandler)

	recvBlk, err := svc2.RequestBlock(peer1.PublicKey(), blk.Hash())
	if asrt.NoError(err) && asrt.NotNil(recvBlk) {
		asrt.Equal(blk.Height(), recvBlk.Height())
	}

	_, err = svc2.RequestBlock(peer1.PublicKey(), []byte{1})
	asrt.Error(err)

	host1.Close()
	host2.Close()
}

func TestRequestTxList(t *testing.T) {
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

	host1, host2, peer1, _ := setupTwoHost(t)
	svc1 := NewMsgService(host1)
	svc2 := NewMsgService(host2)
	svc1.SetReqHandler(txListReqHandler)

	recvTxs, err := svc2.RequestTxList(peer1.PublicKey(), [][]byte{{1}, {2}})
	if asrt.NoError(err) && asrt.NotNil(recvTxs) {
		asrt.Equal((*txs)[0].Nonce(), (*recvTxs)[0].Nonce())
		asrt.Equal((*txs)[1].Nonce(), (*recvTxs)[1].Nonce())
	}

	host1.Close()
	host2.Close()
}
