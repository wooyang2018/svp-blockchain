// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package consensus

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/wooyang2018/posv-blockchain/core"
	"github.com/wooyang2018/posv-blockchain/logger"
)

type validator struct {
	resources *Resources
	config    Config
	state     *state
	status    *status
	driver    *driver

	stopCh chan struct{}
}

func (vld *validator) start() {
	if vld.stopCh != nil {
		return
	}
	vld.stopCh = make(chan struct{})
	go vld.proposalLoop()
	go vld.voteLoop()
	go vld.newViewLoop()
	logger.I().Info("started validator")
}

func (vld *validator) stop() {
	if vld.stopCh == nil {
		return // not started yet
	}
	select {
	case <-vld.stopCh: // already stopped
		return
	default:
	}
	close(vld.stopCh)
	logger.I().Info("stopped validator")
	vld.stopCh = nil
}

func (vld *validator) proposalLoop() {
	sub := vld.resources.MsgSvc.SubscribeProposal(1)
	defer sub.Unsubscribe()

	for {
		select {
		case <-vld.stopCh:
			return

		case e := <-sub.Events():
			if err := vld.onReceiveProposal(e.(*core.Block)); err != nil {
				logger.I().Warnf("receive proposal failed, %+v", err)
			}
		}
	}
}

func (vld *validator) voteLoop() {
	sub := vld.resources.MsgSvc.SubscribeVote(10)
	defer sub.Unsubscribe()

	for {
		select {
		case <-vld.stopCh:
			return

		case e := <-sub.Events():
			if err := vld.onReceiveVote(e.(*core.Vote)); err != nil {
				logger.I().Warnf("receive vote failed, %+v", err)
			}
		}
	}
}

func (vld *validator) newViewLoop() {
	sub := vld.resources.MsgSvc.SubscribeQC(10)
	defer sub.Unsubscribe()

	for {
		select {
		case <-vld.stopCh:
			return

		case e := <-sub.Events():
			if err := vld.onReceiveQC(e.(*core.QuorumCert)); err != nil {
				logger.I().Warnf("receive qc failed, %+v", err)
			}
		}
	}
}

func (vld *validator) onReceiveProposal(blk *core.Block) error {
	if err := blk.Validate(vld.resources.RoleStore); err != nil {
		return err
	}
	if _, err := vld.syncParentBlock(blk); err != nil { // fetch parent block recursively
		return err
	}
	if ExecuteTxFlag { // must sync transactions before updating block
		if err := vld.resources.TxPool.SyncTxs(blk.Proposer(), blk.Transactions()); err != nil {
			return fmt.Errorf("sync txs failed, %w", err)
		}
	}
	vld.state.setBlock(blk)
	pidx := vld.resources.RoleStore.GetValidatorIndex(blk.Proposer())
	logger.I().Infow("received proposal",
		"view", blk.View(),
		"proposer", pidx,
		"height", blk.Height(),
		"txs", len(blk.Transactions()))
	return vld.updateQCHighAndVote(blk)
}

func (vld *validator) syncParentBlock(blk *core.Block) (*core.Block, error) {
	if blk.Height() == 0 { // genesis block
		return nil, nil
	}
	parent := vld.driver.getBlockByHash(blk.ParentHash())
	if parent != nil {
		return parent, nil
	}
	if err := vld.syncMissingCommittedBlocks(blk); err != nil {
		return nil, err
	}
	return vld.syncMissingParentRecursive(blk.Proposer(), blk)
}

func (vld *validator) syncMissingCommittedBlocks(blk *core.Block) error {
	commitHeight := vld.resources.Storage.GetBlockHeight()
	if blk.ExecHeight() <= commitHeight {
		return nil // already sync committed blocks
	}
	if blk.Height() == 0 {
		return errors.New("missing genesis block")
	}
	qcRef := vld.driver.getBlockByHash(blk.ParentHash())
	if qcRef == nil {
		var err error
		if qcRef, err = vld.requestBlock(blk.Proposer(), blk.ParentHash()); err != nil {
			return err
		}
	}
	if qcRef.Height() < commitHeight {
		return fmt.Errorf("old qc ref %d", qcRef.Height())
	}
	return vld.syncForwardCommittedBlocks(blk.Proposer(), commitHeight+1, blk.ExecHeight())
}

func (vld *validator) syncForwardCommittedBlocks(peer *core.PublicKey, start, end uint64) error {
	for height := start; height < end; height++ { // end is exclusive
		blk, err := vld.requestBlockByHeight(peer, height)
		if err != nil {
			return err
		}
		parent := vld.driver.getBlockByHash(blk.ParentHash())
		if parent == nil {
			return errors.New("cannot connect chain, parent not found")
		}
		if err := vld.verifyParentAndCommitRecursive(peer, blk, parent); err != nil {
			return err
		}
	}
	return nil
}

func (vld *validator) requestBlockByHeight(peer *core.PublicKey, height uint64) (*core.Block, error) {
	blk, err := vld.resources.MsgSvc.RequestBlockByHeight(peer, height)
	if err != nil {
		return nil, fmt.Errorf("cannot get block, height %d, %w", height, err)
	}
	if err := blk.Validate(vld.resources.RoleStore); err != nil {
		return nil, fmt.Errorf("validate block failed, %w", err)
	}
	vld.state.setBlock(blk)
	return blk, nil
}

func (vld *validator) verifyParentAndCommitRecursive(peer *core.PublicKey, blk, parent *core.Block) error {
	if blk.Height() != parent.Height()+1 {
		return fmt.Errorf("invalid block, height %d, parent %d", blk.Height(), parent.Height())
	}
	if ExecuteTxFlag { // must sync transactions before updating block
		if err := vld.resources.TxPool.SyncTxs(peer, blk.Transactions()); err != nil {
			return err
		}
	}

	vld.driver.mtxUpdate.Lock()
	defer vld.driver.mtxUpdate.Unlock()
	vld.driver.commitRecursive(blk)

	return nil
}

func (vld *validator) syncMissingParentRecursive(peer *core.PublicKey, blk *core.Block) (*core.Block, error) {
	var err error
	parent := vld.driver.getBlockByHash(blk.ParentHash())
	if parent == nil {
		parent, err = vld.requestBlock(peer, blk.ParentHash())
		if err != nil {
			return nil, err
		}
	}
	if blk.Height() == vld.resources.Storage.GetBlockHeight() {
		return parent, nil
	}
	if blk.Height() != parent.Height()+1 {
		return nil, fmt.Errorf("invalid block, height %d, parent %d", blk.Height(), parent.Height())
	}
	if ExecuteTxFlag { // must sync transactions before updating block
		if err := vld.resources.TxPool.SyncTxs(peer, parent.Transactions()); err != nil {
			return nil, err
		}
	}
	if _, err := vld.syncMissingParentRecursive(peer, parent); err != nil {
		return nil, err
	}
	return parent, nil
}

func (vld *validator) requestBlock(peer *core.PublicKey, hash []byte) (*core.Block, error) {
	blk, err := vld.resources.MsgSvc.RequestBlock(peer, hash)
	if err != nil {
		return nil, fmt.Errorf("request block failed, %w", err)
	}
	if err := blk.Validate(vld.resources.RoleStore); err != nil {
		return nil, fmt.Errorf("validate block failed, %w", err)
	}
	vld.state.setBlock(blk)
	return blk, nil
}

func (vld *validator) updateQCHighAndVote(blk *core.Block) error {
	if vld.status.getView() > blk.View() {
		return errors.New("not same view")
	}

	vld.driver.mtxUpdate.Lock()
	defer vld.driver.mtxUpdate.Unlock()

	vld.driver.receiveCh <- blk
	if vld.driver.cmpQCPriority(blk.QuorumCert(), vld.status.getQCHigh()) < 0 {
		return fmt.Errorf("can not vote by lower qc, height %d", blk.Height())
	}
	vld.driver.updateQCHigh(blk.QuorumCert())

	proposer := vld.resources.RoleStore.GetValidatorIndex(blk.Proposer())
	if !vld.driver.isLeader(blk.Proposer()) {
		return fmt.Errorf("proposer %d is not leader", proposer)
	}

	if err := vld.verifyBlockToVote(blk); err != nil {
		return err
	}
	quota := vld.voteBlock(blk)

	logger.I().Infow("voted proposal",
		"view", blk.View(),
		"proposer", proposer,
		"height", blk.Height(),
		"qc", vld.driver.qcRefHeight(blk.QuorumCert()),
		"quota", quota,
	)
	return nil
}

func (vld *validator) verifyBlockToVote(blk *core.Block) error {
	// on node restart, not committed any blocks yet, don't check merkle root
	if vld.state.getCommittedHeight() != 0 {
		if err := vld.verifyExecHeight(blk); err != nil {
			return err
		}
		if err := vld.verifyMerkleRoot(blk); err != nil {
			return err
		}
	}
	return vld.verifyBlockTxs(blk)
}

func (vld *validator) verifyMerkleRoot(blk *core.Block) error {
	if ExecuteTxFlag {
		mr := vld.resources.Storage.GetMerkleRoot()
		if !bytes.Equal(mr, blk.MerkleRoot()) {
			return fmt.Errorf("invalid merkle root, height %d", blk.Height())
		}
	}
	return nil
}

func (vld *validator) verifyExecHeight(blk *core.Block) error {
	bh := vld.resources.Storage.GetBlockHeight()
	if bh != blk.ExecHeight() {
		return fmt.Errorf("invalid exec height, expected %d, got %d", bh, blk.ExecHeight())
	}
	return nil
}

func (vld *validator) verifyBlockTxs(blk *core.Block) error {
	if ExecuteTxFlag {
		for _, hash := range blk.Transactions() {
			if vld.resources.Storage.HasTx(hash) {
				return fmt.Errorf("tx %s already committed", base64String(hash))
			}
			tx := vld.resources.TxPool.GetTx(hash)
			if tx == nil {
				return fmt.Errorf("tx %s not found", base64String(hash))
			}
			if tx.Expiry() != 0 && tx.Expiry() < blk.Height() {
				return fmt.Errorf("tx %s expired", base64String(hash))
			}
		}
	}
	return nil
}

func (vld *validator) voteBlock(blk *core.Block) float64 {
	quota := vld.resources.RoleStore.GetValidatorQuota(vld.resources.Signer.PublicKey()) /
		float64(vld.resources.RoleStore.GetWindowSize())
	vote := blk.Vote(vld.resources.Signer, quota)
	if !PreserveTxFlag {
		vld.resources.TxPool.SetTxsPending(blk.Transactions())
	}
	if err := vld.resources.MsgSvc.SendVote(blk.Proposer(), vote); err != nil {
		logger.I().Errorf("send vote failed, %+v", err)
	}
	return quota
}

func (vld *validator) onReceiveVote(vote *core.Vote) error {
	if err := vote.Validate(vld.resources.RoleStore); err != nil {
		return err
	}

	vld.driver.mtxUpdate.Lock()
	defer vld.driver.mtxUpdate.Unlock()

	return vld.driver.onReceiveVote(vote)
}

func (vld *validator) onReceiveQC(qc *core.QuorumCert) error {
	var err error
	if err = qc.Validate(vld.resources.RoleStore); err != nil {
		return err
	}
	blk := vld.driver.getBlockByHash(qc.BlockHash())
	if blk == nil {
		if blk, err = vld.requestBlock(qc.Proposer(), qc.BlockHash()); err != nil {
			return err
		}
	}
	if _, err = vld.syncParentBlock(blk); err != nil { // fetch parent block recursively
		return err
	}
	if ExecuteTxFlag { // must sync transactions before updating block
		if err = vld.resources.TxPool.SyncTxs(qc.Proposer(), blk.Transactions()); err != nil {
			return fmt.Errorf("sync txs failed, %w", err)
		}
	}
	vld.driver.mtxUpdate.Lock()
	defer vld.driver.mtxUpdate.Unlock()
	vld.driver.updateQCHigh(qc)
	return nil
}

func base64String(b []byte) string {
	return base64.StdEncoding.EncodeToString(b)
}
