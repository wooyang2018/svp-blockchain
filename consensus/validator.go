// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package consensus

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"sync"

	"github.com/wooyang2018/posv-blockchain/core"
	"github.com/wooyang2018/posv-blockchain/logger"
)

type validator struct {
	resources   *Resources
	config      Config
	state       *state
	posvState   *posvState
	driver      *driver
	mtxProposal sync.Mutex
	stopCh      chan struct{}
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
	sub := vld.resources.MsgSvc.SubscribeProposal(100)
	defer sub.Unsubscribe()

	for {
		select {
		case <-vld.stopCh:
			return

		case e := <-sub.Events():
			if err := vld.onReceiveProposal(e.(*core.Proposal)); err != nil {
				logger.I().Warnf("on received proposal failed, %+v", err)
			}
		}
	}
}

func (vld *validator) voteLoop() {
	sub := vld.resources.MsgSvc.SubscribeVote(1000)
	defer sub.Unsubscribe()

	for {
		select {
		case <-vld.stopCh:
			return

		case e := <-sub.Events():
			if err := vld.onReceiveVote(e.(*core.Vote)); err != nil {
				logger.I().Warnf("received vote failed, %+v", err)
			}
		}
	}
}

func (vld *validator) newViewLoop() {
	sub := vld.resources.MsgSvc.SubscribeNewView(100)
	defer sub.Unsubscribe()

	for {
		select {
		case <-vld.stopCh:
			return

		case e := <-sub.Events():
			if err := vld.onReceiveNewView(e.(*core.QuorumCert)); err != nil {
				logger.I().Warnf("received new view failed, %+v", err)
			}
		}
	}
}

func (vld *validator) onReceiveProposal(pro *core.Proposal) error {
	vld.mtxProposal.Lock()
	defer vld.mtxProposal.Unlock()

	if err := pro.Validate(vld.resources.VldStore); err != nil {
		return err
	}
	pidx := vld.resources.VldStore.GetWorkerIndex(pro.Proposer())
	logger.I().Debugw("received proposal", "proposer", pidx, "height", pro.Block().Height(), "txs", len(pro.Block().Transactions()))
	parent, err := vld.getParentBlock(pro.Block())
	if err != nil {
		return err
	}
	return vld.updatePoSVAndVote(pro.Proposer(), pro, parent)
}

// TODO 本函数解释了所有同步区块的入口
func (vld *validator) getParentBlock(block *core.Block) (*core.Block, error) {
	parent := vld.state.getBlock(block.ParentHash())
	if parent != nil {
		return parent, nil
	}
	if err := vld.syncMissingCommittedBlocks(block); err != nil {
		return nil, err
	}
	return vld.syncMissingParentBlocksRecursive(block.Proposer(), block)
}

func (vld *validator) syncMissingCommittedBlocks(pro *core.Block) error {
	commitHeight := vld.resources.Storage.GetBlockHeight()
	if pro.ExecHeight() <= commitHeight {
		return nil // already sync committed blocks
	}
	// seems like I left behind. Lets check with qcRef to confirm
	// only qc is trusted and proposal is not
	if pro.IsGenesis() {
		return fmt.Errorf("genesis block proposal")
	}
	qcRef := vld.state.getBlock(pro.ParentHash())
	if qcRef == nil {
		var err error
		qcRef, err = vld.requestBlock(
			pro.Proposer(), pro.ParentHash())
		if err != nil {
			return err
		}
	}
	if qcRef.Height() < commitHeight {
		return fmt.Errorf("old qc ref %d", qcRef.Height())
	}
	return vld.syncForwardCommittedBlocks(
		pro.Proposer(), commitHeight+1, pro.ExecHeight())
}

func (vld *validator) syncForwardCommittedBlocks(peer *core.PublicKey, start, end uint64) error {
	var pro *core.Block
	for height := start; height < end; height++ { // end is exclusive
		var err error
		pro, err = vld.requestBlockByHeight(peer, height)
		if err != nil {
			return err
		}
		parent := vld.state.getBlock(pro.ParentHash())
		if parent == nil {
			return fmt.Errorf("cannot connect chain, parent not found")
		}
		err = vld.verifyWithParentAndUpdatePoSV(peer, pro, parent)
		if err != nil {
			return err
		}
	}
	return nil
}

func (vld *validator) syncMissingParentBlocksRecursive(
	peer *core.PublicKey, blk *core.Block) (*core.Block, error) {
	parent := vld.state.getBlock(blk.ParentHash())
	if parent != nil {
		return parent, nil // not missing
	}
	parent, err := vld.requestBlock(peer, blk.ParentHash())
	if err != nil {
		return nil, err
	}
	grandParent, err := vld.syncMissingParentBlocksRecursive(peer, parent)
	if err != nil {
		return nil, err
	}
	err = vld.verifyWithParentAndUpdatePoSV(peer, parent, grandParent)
	if err != nil {
		return nil, err
	}
	return parent, nil
}

func (vld *validator) requestBlock(peer *core.PublicKey, hash []byte) (*core.Block, error) {
	blk, err := vld.resources.MsgSvc.RequestBlock(peer, hash)
	if err != nil {
		return nil, fmt.Errorf("cannot request block %w", err)
	}
	if err := blk.Validate(vld.resources.VldStore); err != nil {
		return nil, fmt.Errorf("validate block error %w", err)
	}
	return blk, nil
}

func (vld *validator) requestBlockByHeight(peer *core.PublicKey, height uint64) (*core.Block, error) {
	blk, err := vld.resources.MsgSvc.RequestBlockByHeight(peer, height)
	if err != nil {
		return nil, fmt.Errorf("cannot get block by height %d, %w", height, err)
	}
	if err := blk.Validate(vld.resources.VldStore); err != nil {
		return nil, fmt.Errorf("validate block error %w", err)
	}
	return blk, nil
}

// TODO: 一种情况是正常提交另一种情况是同步区块
func (vld *validator) verifyWithParentAndUpdatePoSV(
	peer *core.PublicKey, blk, parent *core.Block) error {
	if blk.Height() != parent.Height()+1 {
		return fmt.Errorf("invalid block height %d, parent %d",
			blk.Height(), parent.Height())
	}
	if ExecuteTxFlag {
		// must sync transactions before updating block to posv
		if err := vld.resources.TxPool.SyncTxs(peer, blk.Transactions()); err != nil {
			return err
		}
	}
	vld.state.setBlock(blk)
	return vld.updatePoSV(blk)
}

func (vld *validator) updatePoSV(blk *core.Block) error {
	vld.state.mtxUpdate.Lock()
	defer vld.state.mtxUpdate.Unlock()
	vld.driver.CommitRecursive(newBlock(blk, vld.state))
	return nil
}

func (vld *validator) updatePoSVAndVote(peer *core.PublicKey, pro *core.Proposal, parent *core.Block) error {
	if pro.Block().Height() != parent.Height()+1 {
		return fmt.Errorf("invalid block height %d, parent %d",
			pro.Block().Height(), parent.Height())
	}
	if ExecuteTxFlag {
		// must sync transactions before updating block to posv
		if err := vld.resources.TxPool.SyncTxs(peer, pro.Block().Transactions()); err != nil {
			return err
		}
	}
	vld.state.setBlock(pro.Block())

	vld.state.mtxUpdate.Lock()
	defer vld.state.mtxUpdate.Unlock()

	if err := vld.verifyProposalToVote(pro); err != nil {
		vld.driver.Update(newProposal(pro, vld.state))
		return err
	}
	vld.driver.OnReceiveProposal(newProposal(pro, vld.state))
	return nil
}

func (vld *validator) verifyProposalToVote(pro *core.Proposal) error {
	if !vld.state.isLeader(pro.Proposer()) {
		pidx := vld.resources.VldStore.GetWorkerIndex(pro.Proposer())
		return fmt.Errorf("proposer %d is not leader", pidx)
	}
	// on node restart, not committed any blocks yet, don't check merkle root
	if vld.state.getCommittedHeight() != 0 {
		if err := vld.verifyMerkleRoot(pro); err != nil {
			return err
		}
	}
	return vld.verifyProposalTxs(pro)
}

func (vld *validator) verifyMerkleRoot(pro *core.Proposal) error {
	bh := vld.resources.Storage.GetBlockHeight()
	if bh != pro.Block().ExecHeight() {
		return fmt.Errorf("invalid exec height")
	}
	if ExecuteTxFlag {
		mr := vld.resources.Storage.GetMerkleRoot()
		if !bytes.Equal(mr, pro.Block().MerkleRoot()) {
			return fmt.Errorf("invalid merkle root")
		}
	}
	return nil
}

func (vld *validator) verifyProposalTxs(pro *core.Proposal) error {
	if ExecuteTxFlag {
		for _, hash := range pro.Block().Transactions() {
			if vld.resources.Storage.HasTx(hash) {
				return fmt.Errorf("already committed tx: %s", base64String(hash))
			}
			tx := vld.resources.TxPool.GetTx(hash)
			if tx == nil {
				return fmt.Errorf("tx not found: %s", base64String(hash))
			}
			if tx.Expiry() != 0 && tx.Expiry() < pro.Block().Height() {
				return fmt.Errorf("expired tx: %s", base64String(hash))
			}
		}
	}
	return nil
}

func (vld *validator) onReceiveVote(vote *core.Vote) error {
	if err := vote.Validate(vld.resources.VldStore); err != nil {
		return err
	}
	return vld.driver.OnReceiveVote(newVote(vote, vld.state))
}

func (vld *validator) onReceiveNewView(qc *core.QuorumCert) error {
	if err := qc.Validate(vld.resources.VldStore); err != nil {
		return err
	}
	vld.driver.posvState.UpdateQCHigh(newQC(qc, vld.state))
	return nil
}

func base64String(b []byte) string {
	return base64.StdEncoding.EncodeToString(b)
}
