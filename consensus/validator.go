// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package consensus

import (
	"bytes"
	"encoding/base64"
	"fmt"

	"github.com/wooyang2018/posv-blockchain/core"
	"github.com/wooyang2018/posv-blockchain/logger"
)

type validator struct {
	resources *Resources
	state     *state
	driver    *driver
	stopCh    chan struct{}
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
			if err := vld.onReceiveProposal(e.(*core.Proposal)); err != nil {
				logger.I().Warnf("on received proposal failed, %+v", err)
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
				logger.I().Warnf("received vote failed, %+v", err)
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
				logger.I().Warnf("received qc failed, %+v", err)
			}
		}
	}
}

func (vld *validator) onReceiveProposal(pro *core.Proposal) error {
	var err error
	if err = pro.Validate(vld.resources.RoleStore); err != nil {
		return err
	}

	hash := pro.QuorumCert().BlockHash()
	if pro.Block() != nil {
		hash = pro.Block().Hash()
	}
	blk, err := vld.syncBlockByHash(pro.Proposer(), hash)
	if err != nil {
		return err
	}

	vld.state.setQC(pro.QuorumCert())
	pidx := vld.resources.RoleStore.GetValidatorIndex(pro.Proposer())
	logger.I().Debugw("received proposal", "view", pro.View(), "proposer", pidx, "height", blk.Height(), "exec", blk.ExecHeight(), "txs", len(blk.Transactions()))

	return vld.updateQCHighAndVote(pro, blk)
}

func (vld *validator) syncBlockByHash(peer *core.PublicKey, hash []byte) (*core.Block, error) {
	var err error
	blk := vld.driver.getBlockByHash(hash)
	if blk == nil {
		if blk, err = vld.requestBlock(peer, hash); err != nil {
			return nil, err
		}
	}
	if _, err = vld.getParentBlock(blk); err != nil { // fetch parent block recursively
		return nil, err
	}
	if ExecuteTxFlag { // must sync transactions before updating block
		if err := vld.resources.TxPool.SyncTxs(peer, blk.Transactions()); err != nil {
			return nil, err
		}
	}
	vld.state.setBlock(blk)
	return blk, nil
}

func (vld *validator) getParentBlock(blk *core.Block) (*core.Block, error) {
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
	// seems like I left behind. Lets check with qcRef to confirm
	// only qc is trusted and proposal is not
	if blk.Height() == 0 {
		return fmt.Errorf("genesis block proposal")
	}
	qcRef := vld.driver.getBlockByHash(blk.ParentHash())
	if qcRef == nil {
		var err error
		qcRef, err = vld.requestBlock(blk.Proposer(), blk.ParentHash())
		if err != nil {
			return err
		}
	}
	if qcRef.Height() < commitHeight {
		return fmt.Errorf("old qc ref %d", qcRef.Height())
	}
	return vld.syncForwardCommittedBlocks(blk.Proposer(), commitHeight+1, blk.ExecHeight())
}

func (vld *validator) syncForwardCommittedBlocks(peer *core.PublicKey, start, end uint64) error {
	var blk *core.Block
	for height := start; height < end; height++ { // end is exclusive
		var err error
		blk, err = vld.requestBlockByHeight(peer, height)
		if err != nil {
			return err
		}
		parent := vld.driver.getBlockByHash(blk.ParentHash())
		if parent == nil {
			return fmt.Errorf("cannot connect chain, parent not found")
		}
		err = vld.verifyParentAndCommitRecursive(peer, blk, parent)
		if err != nil {
			return err
		}
	}
	return nil
}

func (vld *validator) requestBlockByHeight(peer *core.PublicKey, height uint64) (*core.Block, error) {
	blk, err := vld.resources.MsgSvc.RequestBlockByHeight(peer, height)
	if err != nil {
		return nil, fmt.Errorf("cannot get block by height %d, %w", height, err)
	}
	if err := blk.Validate(vld.resources.RoleStore); err != nil {
		return nil, fmt.Errorf("validate block error, %w", err)
	}
	return blk, nil
}

func (vld *validator) verifyParentAndCommitRecursive(peer *core.PublicKey, blk, parent *core.Block) error {
	if blk.Height() != parent.Height()+1 {
		return fmt.Errorf("invalid block height %d, parent %d",
			blk.Height(), parent.Height())
	}
	if ExecuteTxFlag { // must sync transactions before updating block
		if err := vld.resources.TxPool.SyncTxs(peer, blk.Transactions()); err != nil {
			return err
		}
	}
	vld.state.setBlock(blk)

	vld.driver.mtxUpdate.Lock()
	defer vld.driver.mtxUpdate.Unlock()
	vld.driver.CommitRecursive(blk)

	return nil
}

func (vld *validator) syncMissingParentRecursive(peer *core.PublicKey, blk *core.Block) (*core.Block, error) {
	parent := vld.driver.getBlockByHash(blk.ParentHash())
	if parent != nil {
		return parent, nil // not missing
	}
	parent, err := vld.requestBlock(peer, blk.ParentHash())
	if err != nil {
		return nil, err
	}
	if blk.Height() != parent.Height()+1 {
		return nil, fmt.Errorf("invalid block height %d, parent %d", blk.Height(), parent.Height())
	}
	vld.state.setBlock(parent)
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
		return nil, fmt.Errorf("cannot request block, %w", err)
	}
	if err := blk.Validate(vld.resources.RoleStore); err != nil {
		return nil, fmt.Errorf("validate block error, %w", err)
	}
	return blk, nil
}

func (vld *validator) updateQCHighAndVote(pro *core.Proposal, blk *core.Block) error {
	vld.driver.mtxUpdate.Lock()
	defer vld.driver.mtxUpdate.Unlock()

	vld.driver.proposalCh <- pro
	vld.driver.UpdateQCHigh(pro.QuorumCert())
	if !vld.driver.isLeader(pro.Proposer()) {
		pidx := vld.resources.RoleStore.GetValidatorIndex(pro.Proposer())
		return fmt.Errorf("proposer %d is not leader", pidx)
	}
	if vld.driver.getView() != pro.View() {
		return fmt.Errorf("not same view")
	}
	if pro.Block() != nil { //new view proposal
		if err := vld.verifyBlockToVote(blk); err != nil {
			return err
		}
	}
	vld.driver.VoteProposal(pro, blk)

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
			return fmt.Errorf("invalid merkle root")
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

func (vld *validator) onReceiveVote(vote *core.Vote) error {
	if err := vote.Validate(vld.resources.RoleStore); err != nil {
		return err
	}

	vld.driver.mtxUpdate.Lock()
	defer vld.driver.mtxUpdate.Unlock()

	return vld.driver.OnReceiveVote(vote)
}

func (vld *validator) onReceiveQC(qc *core.QuorumCert) error {
	var err error
	if err = qc.Validate(vld.resources.RoleStore); err != nil {
		return err
	}
	if _, err = vld.syncBlockByHash(qc.Proposer(), qc.BlockHash()); err != nil {
		return err
	}
	vld.state.setQC(qc)

	if qc.View() > vld.driver.getView() {
		vld.driver.setViewStart()
		vld.driver.setView(qc.View())
		leaderIdx := vld.driver.getView() % uint32(vld.resources.RoleStore.ValidatorCount())
		vld.driver.setLeaderIndex(leaderIdx)
		logger.I().Infow("view changed by higher qc", "view", vld.driver.getView(),
			"leader", vld.driver.getLeaderIndex(), "qc", vld.driver.qcRefHeight(qc))
	}

	vld.driver.mtxUpdate.Lock()
	defer vld.driver.mtxUpdate.Unlock()
	vld.driver.UpdateQCHigh(qc)

	return nil
}

func base64String(b []byte) string {
	return base64.StdEncoding.EncodeToString(b)
}
