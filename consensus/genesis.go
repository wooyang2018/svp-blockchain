// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package consensus

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"sync"
	"time"

	"golang.org/x/crypto/sha3"

	"github.com/wooyang2018/svp-blockchain/core"
	"github.com/wooyang2018/svp-blockchain/execution/common"
	"github.com/wooyang2018/svp-blockchain/logger"
	"github.com/wooyang2018/svp-blockchain/native"
	"github.com/wooyang2018/svp-blockchain/native/srole"
	"github.com/wooyang2018/svp-blockchain/native/taddr"
	"github.com/wooyang2018/svp-blockchain/native/xcoin"
	"github.com/wooyang2018/svp-blockchain/storage"
)

type genesis struct {
	resources *Resources
	config    Config

	// collect votes for genesis block from all validators instead of majority
	votes   map[string]*core.Vote
	mtxVote sync.Mutex

	b0    *core.Block
	mtxB0 sync.RWMutex
	q0    *core.QuorumCert
	mtxQ0 sync.RWMutex

	done chan struct{}
	once sync.Once // guarantee channel is closed only once
}

func (gns *genesis) run() (*core.Block, *core.QuorumCert) {
	gns.done = make(chan struct{})
	go gns.proposalLoop()
	go gns.voteLoop()
	go gns.newViewLoop()
	gns.propose()
	<-gns.done
	logger.I().Info("got genesis block and qc")
	gns.commit()
	return gns.getB0(), gns.getQ0()
}

func (gns *genesis) commit() {
	gns.resources.Storage.StoreQC(gns.getQ0())
	b0 := gns.getB0()
	if err := gns.resources.TxPool.SyncTxs(b0.Proposer(), b0.Transactions()); err != nil {
		logger.I().Fatalf("sync txs of genesis block failed, %+v", err)
	}
	txs, old := gns.resources.TxPool.GetTxsToExecute(b0.Transactions())
	if len(txs) != native.BuiltinCount {
		logger.I().Fatalf("genesis block contains %d txs", len(txs))
	}
	logger.I().Debugw("executing genesis block...")
	bcm, txcs := gns.resources.Execution.Execute(b0, txs)
	gns.dumpCodeFile(txs)
	bcm.SetOldBlockTxs(old)
	data := &storage.CommitData{
		Block:        b0,
		Transactions: txs,
		BlockCommit:  bcm,
		TxCommits:    txcs,
	}
	if err := gns.resources.Storage.Commit(data); err != nil {
		logger.I().Fatalf("commit storage failed, %+v", err)
	}
	logger.I().Info("committed genesis bock")
	gns.resources.TxPool.RemoveTxs(b0.Transactions())
}

func (gns *genesis) dumpCodeFile(txs []*core.Transaction) {
	codePath := path.Join(gns.config.DataDir, native.CodePathDefault)
	common.DumpFile(txs[0].Hash(), codePath, native.FileCodeXCoin)
	common.DumpFile(txs[1].Hash(), codePath, native.FileCodeTAddr)
	common.DumpFile(txs[2].Hash(), codePath, native.FileCodeSRole)
	common.RegisterCode(native.FileCodeXCoin, txs[0].Hash())
	common.RegisterCode(native.FileCodeTAddr, txs[1].Hash())
	common.RegisterCode(native.FileCodeSRole, txs[1].Hash())
}

func (gns *genesis) propose() {
	if !gns.isLeader(gns.resources.Signer.PublicKey()) {
		return
	}
	gns.votes = make(map[string]*core.Vote, gns.resources.RoleStore.MajorityValidatorCount())
	blk := core.NewBlock().
		SetView(0).
		SetHeight(0).
		SetTransactions(gns.genesisTxs()).
		SetParentHash(hashChainID(gns.config.ChainID)).
		SetTimestamp(time.Now().UnixNano()).
		Sign(gns.resources.Signer)
	gns.setB0(blk)
	logger.I().Info("created genesis block, broadcasting...")
	go gns.broadcastProposalLoop()
	quota := gns.resources.RoleStore.GetValidatorQuota(gns.resources.Signer.PublicKey().String())
	gns.onReceiveVote(blk.Vote(gns.resources.Signer, (quota+1)/2))
}

func (gns *genesis) genesisTxs() [][]byte {
	count := gns.resources.RoleStore.ValidatorCount()
	values0 := make(map[string]uint64)
	values1 := make([]string, count)
	for i := 0; i < count; i++ {
		pubKey := gns.resources.RoleStore.GetValidator(i)
		quota := gns.resources.RoleStore.GetValidatorQuota(pubKey.String())
		values0[pubKey.String()] = quota
		values1[i] = pubKey.String()
	}

	input0 := &common.DeploymentInput{
		CodeInfo: common.CodeInfo{
			DriverType: common.DriverTypeNative,
			CodeID:     native.CodeXCoin,
		},
	}
	input0.InitInput, _ = json.Marshal(xcoin.InitInput{values0})
	b0, _ := json.Marshal(input0)
	tx0 := core.NewTransaction().
		SetNonce(time.Now().UnixNano()).
		SetInput(b0).
		Sign(gns.resources.Signer)

	input1 := &common.DeploymentInput{
		CodeInfo: common.CodeInfo{
			DriverType: common.DriverTypeNative,
			CodeID:     native.CodeTAddr,
		},
	}
	input1.InitInput, _ = json.Marshal(taddr.InitInput{values1})
	b1, _ := json.Marshal(input1)
	tx1 := core.NewTransaction().
		SetNonce(time.Now().UnixNano()).
		SetInput(b1).
		Sign(gns.resources.Signer)

	input2 := &common.DeploymentInput{
		CodeInfo: common.CodeInfo{
			DriverType: common.DriverTypeNative,
			CodeID:     native.CodeSRole,
		},
	}
	peers := make([]*srole.Peer, 0)
	for i := 0; i < gns.resources.RoleStore.ValidatorCount(); i++ {
		pubKey := gns.resources.RoleStore.GetValidator(i)
		pointAddr, topicAddr := gns.resources.RoleStore.GetValidatorAddr(pubKey.String())
		quota := gns.resources.RoleStore.GetValidatorQuota(pubKey.String())
		peers = append(peers, &srole.Peer{pubKey.Bytes(), quota, pointAddr, topicAddr})
	}
	input2.InitInput, _ = json.Marshal(srole.InitInput{
		Size:  gns.resources.RoleStore.GetWindowSize(),
		Peers: peers,
	})
	b2, _ := json.Marshal(input2)
	tx2 := core.NewTransaction().
		SetNonce(time.Now().UnixNano()).
		SetInput(b2).
		Sign(gns.resources.Signer)

	if err := gns.resources.TxPool.StoreTxs(&core.TxList{tx0, tx1, tx2}); err != nil {
		logger.I().Fatal(err)
	}
	return [][]byte{tx0.Hash(), tx1.Hash(), tx2.Hash()}
}

func (gns *genesis) broadcastProposalLoop() {
	for {
		select {
		case <-gns.done:
			return
		default:
		}

		if gns.getQ0() == nil {
			if err := gns.resources.MsgSvc.BroadcastProposal(gns.getB0()); err != nil {
				logger.I().Errorf("broadcast proposal failed, %+v", err)
			}
		}
		time.Sleep(2 * time.Second)
	}
}

func (gns *genesis) proposalLoop() {
	sub := gns.resources.MsgSvc.SubscribeProposal(1)
	defer sub.Unsubscribe()

	for {
		select {
		case <-gns.done:
			return

		case e := <-sub.Events():
			if err := gns.onReceiveProposal(e.(*core.Block)); err != nil {
				logger.I().Warnf("receive proposal failed, %+v", err)
			}
		}
	}
}

func (gns *genesis) voteLoop() {
	sub := gns.resources.MsgSvc.SubscribeVote(10)
	defer sub.Unsubscribe()

	for {
		select {
		case <-gns.done:
			return

		case e := <-sub.Events():
			if err := gns.onReceiveVote(e.(*core.Vote)); err != nil {
				logger.I().Warnf("receive vote failed, %+v", err)
			}
		}
	}
}

func (gns *genesis) newViewLoop() {
	sub := gns.resources.MsgSvc.SubscribeQC(1)
	defer sub.Unsubscribe()

	for {
		select {
		case <-gns.done:
			return

		case e := <-sub.Events():
			if err := gns.onReceiveQC(e.(*core.QuorumCert)); err != nil {
				logger.I().Warnf("receive qc failed, %+v", err)
			}
		}
	}
}

func (gns *genesis) onReceiveProposal(blk *core.Block) error {
	if err := blk.Validate(gns.resources.RoleStore); err != nil {
		return err
	}
	if blk.Height() != 0 {
		logger.I().Info("left behind, fetching genesis block...")
		return gns.fetchGenesisBlockAndQC(blk.Proposer())
	}
	if !bytes.Equal(hashChainID(gns.config.ChainID), blk.ParentHash()) {
		return errors.New("different chain id")
	}
	if !gns.isLeader(blk.Proposer()) {
		return errors.New("proposer is not leader")
	}
	if len(blk.Transactions()) != native.BuiltinCount {
		return fmt.Errorf("genesis block contains %d txs", len(blk.Transactions()))
	}
	gns.setB0(blk)
	logger.I().Info("got genesis block, voting...")
	quota := gns.resources.RoleStore.GetValidatorQuota(gns.resources.Signer.PublicKey().String())
	return gns.resources.MsgSvc.SendVote(blk.Proposer(), blk.Vote(gns.resources.Signer, (quota+1)/2))
}

func (gns *genesis) fetchGenesisBlockAndQC(peer *core.PublicKey) error {
	b0, err := gns.requestBlockByHeight(peer, 0)
	if err != nil {
		return err
	}
	if b0.Height() != 0 {
		return errors.New("not genesis block")
	}
	qc, err := gns.requestQC(peer, b0.Hash())
	if err != nil {
		return err
	}
	if !bytes.Equal(b0.Hash(), qc.BlockHash()) {
		return errors.New("qc ref is not b0")
	}
	gns.setB0(b0)
	gns.setQ0(qc)
	gns.once.Do(func() {
		close(gns.done)
	})
	return nil
}

func (gns *genesis) requestBlockByHeight(peer *core.PublicKey, height uint64) (*core.Block, error) {
	blk, err := gns.resources.MsgSvc.RequestBlockByHeight(peer, height)
	if err != nil {
		return nil, fmt.Errorf("request block failed, height %d, %w", height, err)
	}
	if err = blk.Validate(gns.resources.RoleStore); err != nil {
		return nil, fmt.Errorf("validate block failed, height %d, %w", height, err)
	}
	return blk, nil
}

func (gns *genesis) requestQC(peer *core.PublicKey, blkHash []byte) (*core.QuorumCert, error) {
	qc, err := gns.resources.MsgSvc.RequestQC(peer, blkHash)
	if err != nil {
		return nil, fmt.Errorf("request qc failed, %w", err)
	}
	// bug fix
	if err = qc.Validate(gns.resources.RoleStore); err != nil {
		return nil, fmt.Errorf("validate qc failed, %w", err)
	}
	return qc, nil
}

func (gns *genesis) onReceiveVote(vote *core.Vote) error {
	if gns.votes == nil {
		return errors.New("not accepting votes")
	}
	if err := vote.Validate(gns.resources.RoleStore); err != nil {
		return err
	}
	gns.acceptVote(vote)
	return nil
}

func (gns *genesis) acceptVote(vote *core.Vote) {
	gns.mtxVote.Lock()
	defer gns.mtxVote.Unlock()

	gns.votes[vote.Voter().String()] = vote
	if len(gns.votes) < gns.resources.RoleStore.ValidatorCount() {
		return
	}
	vlist := make([]*core.Vote, 0, len(gns.votes))
	for _, v := range gns.votes {
		vlist = append(vlist, v)
	}
	gns.setQ0(core.NewQuorumCert().Build(gns.resources.Signer, vlist))
	logger.I().Info("created qc, broadcasting...")
	gns.broadcastQC()
}

func (gns *genesis) broadcastQC() {
	for {
		select {
		case <-gns.done:
			return
		default:
		}

		if err := gns.resources.MsgSvc.BroadcastQC(gns.getQ0()); err != nil {
			logger.I().Errorf("broadcast qc failed ,%+v", err)
		}
		time.Sleep(1 * time.Second)
	}
}

func (gns *genesis) onReceiveQC(qc *core.QuorumCert) error {
	if err := qc.Validate(gns.resources.RoleStore); err != nil {
		return err
	}
	b0 := gns.getB0()
	if b0 == nil {
		return errors.New("no received genesis block yet")
	}
	if !bytes.Equal(b0.Hash(), qc.BlockHash()) {
		return errors.New("invalid qc ref")
	}
	gns.acceptQC(qc)
	return nil
}

func (gns *genesis) acceptQC(qc *core.QuorumCert) {
	select {
	case <-gns.done: // genesis already done
		return
	default:
	}

	gns.setQ0(qc)
	if !gns.isLeader(gns.resources.Signer.PublicKey()) {
		gns.resources.MsgSvc.SendQC(gns.getB0().Proposer(), qc)
	}
	close(gns.done) // when qc is accepted, genesis creation is done
}

func (gns *genesis) setB0(blk *core.Block) {
	gns.mtxB0.Lock()
	defer gns.mtxB0.Unlock()

	gns.b0 = blk
}

func (gns *genesis) setQ0(qc *core.QuorumCert) {
	gns.mtxQ0.Lock()
	defer gns.mtxQ0.Unlock()

	gns.q0 = qc
}

func (gns *genesis) getB0() *core.Block {
	gns.mtxB0.RLock()
	defer gns.mtxB0.RUnlock()

	return gns.b0
}

func (gns *genesis) getQ0() *core.QuorumCert {
	gns.mtxQ0.RLock()
	defer gns.mtxQ0.RUnlock()

	return gns.q0
}

func (gns *genesis) isLeader(pubKey *core.PublicKey) bool {
	if !gns.resources.RoleStore.IsValidator(pubKey.String()) {
		return false
	}
	return gns.resources.RoleStore.GetValidatorIndex(pubKey.String()) == 0
}

func hashChainID(chainID int64) []byte {
	h := sha3.New256()
	binary.Write(h, binary.BigEndian, chainID)
	return h.Sum(nil)
}
