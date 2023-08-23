// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package consensus

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/wooyang2018/svp-blockchain/core"
	"github.com/wooyang2018/svp-blockchain/logger"
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
	sub := vld.resources.MsgSvc.SubscribeProposal(10)
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
	sub := vld.resources.MsgSvc.SubscribeVote(100)
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

func (vld *validator) onReceiveProposal(blk *core.Block) error {
	if err := blk.Validate(vld.resources.RoleStore); err != nil {
		return err
	}
	if _, err := vld.driver.syncParentBlock(blk); err != nil { // fetch parent block recursively
		return err
	}
	if ExecuteTxFlag { // must sync transactions before updating block
		if err := vld.resources.TxPool.SyncTxs(blk.Proposer(), blk.Transactions()); err != nil {
			return fmt.Errorf("sync txs failed, %w", err)
		}
	}

	vld.driver.mtxUpdate.Lock()
	defer vld.driver.mtxUpdate.Unlock()

	vld.state.setBlock(blk)
	vld.driver.rotator.onReceiveProposal(blk)

	logger.I().Infow("received proposal",
		"view", blk.View(),
		"proposer", vld.resources.RoleStore.GetValidatorIndex(blk.Proposer()),
		"height", blk.Height(),
		"txs", len(blk.Transactions()))

	if vld.status.getView() > blk.View() {
		return errors.New("not same view")
	}
	if vld.driver.cmpQCPriority(blk.QuorumCert(), vld.status.getQCHigh()) < 0 {
		return fmt.Errorf("can not vote by lower qc")
	}

	vld.driver.updateQCHigh(blk.QuorumCert())
	return vld.voteProposal(blk)
}

func (vld *validator) voteProposal(blk *core.Block) error {
	if !vld.driver.isLeader(blk.Proposer()) {
		proposer := vld.resources.RoleStore.GetValidatorIndex(blk.Proposer())
		return fmt.Errorf("proposer %d is not leader", proposer)
	}
	if err := vld.verifyBlockToVote(blk); err != nil {
		return err
	}
	var quota uint32 = 1
	if !TwoPhaseBFTFlag {
		var err error
		if quota, err = vld.status.getVoteQuota(); err != nil {
			return err
		}
	}
	vote := blk.Vote(vld.resources.Signer, quota)
	if !PreserveTxFlag {
		vld.resources.TxPool.SetTxsPending(blk.Transactions())
	}
	if err := vld.resources.MsgSvc.SendVote(blk.Proposer(), vote); err != nil {
		logger.I().Errorf("send vote failed, %+v", err)
	}

	logKVs := []interface{}{
		"view", blk.View(),
		"height", blk.Height(),
		"qc", vld.driver.qcRefHeight(blk.QuorumCert()),
	}
	if !TwoPhaseBFTFlag {
		logKVs = append(logKVs, "quota", vote.Quota(),
			"window", vld.status.getVoteWindow())
	}
	logger.I().Infow("voted proposal", logKVs...)
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

func (vld *validator) onReceiveVote(vote *core.Vote) error {
	if err := vote.Validate(vld.resources.RoleStore); err != nil {
		return err
	}

	vld.driver.mtxUpdate.Lock()
	defer vld.driver.mtxUpdate.Unlock()

	if vld.status.getViewChange() != 0 {
		return fmt.Errorf("discard vote when view change")
	}
	return vld.driver.onReceiveVote(vote)
}

func base64String(b []byte) string {
	return base64.StdEncoding.EncodeToString(b)
}
