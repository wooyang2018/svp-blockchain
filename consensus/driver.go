// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package consensus

import (
	"bytes"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/wooyang2018/posv-blockchain/core"
	"github.com/wooyang2018/posv-blockchain/logger"
	"github.com/wooyang2018/posv-blockchain/storage"
)

type driver struct {
	resources *Resources
	config    Config
	state     *state
	status    *status
	tester    *tester

	isCommitting bool
	isExisted    bool
	proposeCh    chan struct{}
	receiveCh    chan *core.Block

	mtxUpdate sync.Mutex // lock for update call
}

func (d *driver) waitCommit() {
	for d.isCommitting {
		time.Sleep(20 * time.Millisecond)
	}
	d.isExisted = true
	logger.I().Info("stopped driver")
}

func (d *driver) isLeader(pubKey *core.PublicKey) bool {
	if !d.resources.RoleStore.IsValidator(pubKey) {
		return false
	}
	return d.status.getLeaderIndex() == uint32(d.resources.RoleStore.GetValidatorIndex(pubKey))
}

func (d *driver) qcRefHeight(qc *core.QuorumCert) uint64 {
	return d.getBlockByHash(qc.BlockHash()).Height()
}

func (d *driver) getBlockByHash(hash []byte) *core.Block {
	blk := d.state.getBlock(hash)
	if blk != nil {
		return blk
	}
	blk, _ = d.resources.Storage.GetBlock(hash)
	if blk == nil {
		return nil
	}
	d.state.setBlock(blk)
	return blk
}

// Deprecated
func (d *driver) getQCByBlockHash(blkHash []byte) *core.QuorumCert {
	qc := d.state.getQC(blkHash)
	if qc != nil {
		return qc
	}
	qc, _ = d.resources.Storage.GetQC(blkHash)
	if qc == nil {
		return nil
	}
	d.state.setQC(qc)
	return qc
}

func (d *driver) requestBlock(peer *core.PublicKey, hash []byte) (*core.Block, error) {
	blk, err := d.resources.MsgSvc.RequestBlock(peer, hash)
	if err != nil {
		return nil, fmt.Errorf("request block failed, %w", err)
	}
	if err := blk.Validate(d.resources.RoleStore); err != nil {
		return nil, fmt.Errorf("validate block failed, %w", err)
	}
	d.state.setBlock(blk)
	return blk, nil
}

func (d *driver) requestBlockByHeight(peer *core.PublicKey, height uint64) (*core.Block, error) {
	blk, err := d.resources.MsgSvc.RequestBlockByHeight(peer, height)
	if err != nil {
		return nil, fmt.Errorf("cannot get block, height %d, %w", height, err)
	}
	if err := blk.Validate(d.resources.RoleStore); err != nil {
		return nil, fmt.Errorf("validate block failed, %w", err)
	}
	d.state.setBlock(blk)
	return blk, nil
}

// cmpBlockHeight compares two blocks by height
func (d *driver) cmpBlockHeight(b1, b2 *core.Block) int {
	if b1 == nil || b2 == nil {
		panic("failed to compare nil block height")
	}
	if b1.Height() == b2.Height() {
		return 0
	} else if b1.Height() > b2.Height() {
		return 1
	}
	return -1
}

// cmpQCPriority compares two qc by view and height
func (d *driver) cmpQCPriority(qc1, qc2 *core.QuorumCert) int {
	if qc1 == nil || qc2 == nil {
		panic("failed to compare nil qc priority")
	}
	if qc1.View() > qc2.View() {
		return 1
	} else if qc1.View() < qc2.View() {
		return -1
	} else { //qc1.View() == qc2.View()
		if d.qcRefHeight(qc1) > d.qcRefHeight(qc2) {
			return 1
		} else if d.qcRefHeight(qc1) < d.qcRefHeight(qc2) {
			return -1
		}
	}
	return 0
}

func (d *driver) syncParentBlock(blk *core.Block) (*core.Block, error) {
	if blk.Height() == 0 { // genesis block
		return nil, nil
	}
	parent := d.getBlockByHash(blk.ParentHash())
	if parent != nil {
		return parent, nil
	}
	if err := d.syncMissingCommittedBlocks(blk); err != nil {
		return nil, err
	}
	return d.syncMissingParentRecursive(blk.Proposer(), blk)
}

func (d *driver) syncMissingCommittedBlocks(blk *core.Block) error {
	commitHeight := d.resources.Storage.GetBlockHeight()
	if blk.ExecHeight() <= commitHeight {
		return nil // already sync committed blocks
	}
	if blk.Height() == 0 {
		return errors.New("missing genesis block")
	}
	qcRef := d.getBlockByHash(blk.ParentHash())
	if qcRef == nil {
		var err error
		if qcRef, err = d.requestBlock(blk.Proposer(), blk.ParentHash()); err != nil {
			return err
		}
	}
	if qcRef.Height() < commitHeight {
		return fmt.Errorf("old qc ref %d", qcRef.Height())
	}
	return d.syncForwardCommittedBlocks(blk.Proposer(), commitHeight+1, blk.ExecHeight())
}

func (d *driver) syncForwardCommittedBlocks(peer *core.PublicKey, start, end uint64) error {
	for height := start; height < end; height++ { // end is exclusive
		blk, err := d.requestBlockByHeight(peer, height)
		if err != nil {
			return err
		}
		parent := d.getBlockByHash(blk.ParentHash())
		if parent == nil {
			return errors.New("cannot connect chain, parent not found")
		}
		if err := d.verifyParentAndCommitRecursive(peer, blk, parent); err != nil {
			return err
		}
	}
	return nil
}

func (d *driver) verifyParentAndCommitRecursive(peer *core.PublicKey, blk, parent *core.Block) error {
	if blk.Height() != parent.Height()+1 {
		return fmt.Errorf("invalid block, height %d, parent %d", blk.Height(), parent.Height())
	}
	if ExecuteTxFlag { // must sync transactions before updating block
		if err := d.resources.TxPool.SyncTxs(peer, blk.Transactions()); err != nil {
			return err
		}
	}

	d.mtxUpdate.Lock()
	defer d.mtxUpdate.Unlock()
	d.commitRecursive(blk)

	return nil
}

func (d *driver) syncMissingParentRecursive(peer *core.PublicKey, blk *core.Block) (*core.Block, error) {
	var err error
	parent := d.getBlockByHash(blk.ParentHash())
	if parent == nil {
		parent, err = d.requestBlock(peer, blk.ParentHash())
		if err != nil {
			return nil, err
		}
	}
	if blk.Height() == d.resources.Storage.GetBlockHeight() {
		return parent, nil
	}
	if blk.Height() != parent.Height()+1 {
		return nil, fmt.Errorf("invalid block, height %d, parent %d", blk.Height(), parent.Height())
	}
	if ExecuteTxFlag { // must sync transactions before updating block
		if err := d.resources.TxPool.SyncTxs(peer, parent.Transactions()); err != nil {
			return nil, err
		}
	}
	if _, err := d.syncMissingParentRecursive(peer, parent); err != nil {
		return nil, err
	}
	return parent, nil
}

func (d *driver) createProposal(view uint32, parent *core.Block, qcHigh *core.QuorumCert) *core.Block {
	var txs [][]byte
	if PreserveTxFlag {
		txs = d.resources.TxPool.GetTxsFromQueue(d.config.BlockTxLimit)
	} else {
		txs = d.resources.TxPool.PopTxsFromQueue(d.config.BlockTxLimit)
	}

	blk := core.NewBlock().
		SetView(view).
		SetParentHash(parent.Hash()).
		SetHeight(parent.Height() + 1).
		SetTransactions(txs).
		SetExecHeight(d.resources.Storage.GetBlockHeight()).
		SetMerkleRoot(d.resources.Storage.GetMerkleRoot()).
		SetTimestamp(time.Now().UnixNano()).
		SetQuorumCert(qcHigh).
		Sign(d.resources.Signer)

	d.state.setBlock(blk)
	d.status.setBLeaf(blk)
	d.status.startProposal(blk)
	d.receiveCh <- blk

	logger.I().Infow("proposed proposal",
		"view", blk.View(),
		"height", blk.Height(),
		"exec", blk.ExecHeight(),
		"qc", d.qcRefHeight(blk.QuorumCert()),
		"txs", len(blk.Transactions()))

	return blk
}

// onReceiveVote is called when received a vote
func (d *driver) onReceiveVote(vote *core.Vote) error {
	if err := d.status.addVote(vote); err != nil {
		return err
	}
	blk := d.getBlockByHash(vote.BlockHash())
	logger.I().Debugw("received vote",
		"view", vote.View(),
		"height", blk.Height(),
		"quota", vote.Quota())
	isVoteEnough := false
	if TwoPhaseBFTFlag {
		isVoteEnough = d.status.getVoteCount() >= d.resources.RoleStore.MajorityValidatorCount()
	} else {
		isVoteEnough = d.status.getVoteCount() >= d.resources.RoleStore.MajorityValidatorCount() &&
			d.status.getQuotaCount() >= d.resources.RoleStore.MajorityQuotaCount()
	}
	if isVoteEnough {
		qc := core.NewQuorumCert().Build(d.resources.Signer, d.status.getVotes())
		d.status.endProposal()
		d.updateQCHigh(qc)
		d.proposeCh <- struct{}{} // trigger propose rule
	}
	return nil
}

// updateQCHigh replaces high qc if the given qc is higher than it
func (d *driver) updateQCHigh(qc *core.QuorumCert) {
	if d.cmpQCPriority(qc, d.status.getQCHigh()) == 1 {
		blk1 := d.getBlockByHash(qc.BlockHash())
		blk0 := d.getBlockByHash(blk1.ParentHash())

		d.state.setQC(qc)
		d.status.setQCHigh(qc)
		d.status.setBLeaf(blk1)
		if !TwoPhaseBFTFlag {
			d.status.updateWindow(qc.SumQuota(), qc.FindVote(d.resources.Signer), blk1.Height())
		}
		d.resources.Storage.StoreQC(qc)
		d.resources.Storage.StoreBlock(blk1)

		if d.cmpBlockHeight(blk0, d.status.getBExec()) == 1 && blk0.View() == blk1.View() {
			d.commitRecursive(blk0)
		}

		logKVs := []interface{}{
			"view", qc.View(),
			"qc", blk1.Height(),
			"votes", len(qc.Quotas()),
		}
		if !TwoPhaseBFTFlag {
			logKVs = append(logKVs, "quota", qc.SumQuota(),
				"window", d.status.getQCWindow())
		}
		logger.I().Infow("updated high qc", logKVs...)
	}
}

func (d *driver) commitRecursive(blk *core.Block) {
	t1 := time.Now().UnixNano()
	d.onCommit(blk)
	d.status.setBExec(blk)
	t2 := time.Now().UnixNano()
	d.tester.saveItem(blk.Height(), blk.Timestamp(), t1, t2, len(blk.Transactions()))
}

func (d *driver) onCommit(blk *core.Block) {
	if d.cmpBlockHeight(blk, d.status.getBExec()) == 1 {
		d.onCommit(d.getBlockByHash(blk.ParentHash())) // commit parent blocks recursively
		d.commit(blk)
	} else if !bytes.Equal(d.status.getBExec().Hash(), blk.Hash()) {
		logger.I().Fatalw("safety breached",
			"height", blk.Height(),
			"hash", base64String(blk.Hash()))
	}
}

func (d *driver) commit(blk *core.Block) {
	start := time.Now()
	rawTxs := blk.Transactions()
	var txCount int
	var data *storage.CommitData
	if ExecuteTxFlag {
		txs, old := d.resources.TxPool.GetTxsToExecute(rawTxs)
		txCount = len(txs)
		logger.I().Debugw("executing block", "height", blk.Height(), "txs", txCount)
		bcm, txcs := d.resources.Execution.Execute(blk, txs)
		bcm.SetOldBlockTxs(old)
		data = &storage.CommitData{
			Block:        blk,
			Transactions: txs,
			BlockCommit:  bcm,
			TxCommits:    txcs,
		}
	} else {
		txCount = len(rawTxs)
		logger.I().Debugw("executing block", "height", blk.Height(), "txs", txCount)
		bcm, txcs := d.resources.Execution.MockExecute(blk)
		bcm.SetOldBlockTxs(rawTxs)
		data = &storage.CommitData{
			Block:        blk,
			Transactions: nil,
			BlockCommit:  bcm,
			TxCommits:    txcs,
		}
	}
	d.isCommitting = true
	if !d.isExisted {
		if err := d.resources.Storage.Commit(data); err != nil {
			logger.I().Fatalf("commit storage error, %+v", err)
		}
	}
	d.isCommitting = false
	d.state.addCommittedTxCount(txCount)
	d.cleanStateOnCommitted(blk)
	logger.I().Infow("committed bock",
		"height", blk.Height(),
		"txs", txCount,
		"elapsed", time.Since(start))
}

func (d *driver) cleanStateOnCommitted(blk *core.Block) {
	// qc for bexec is no longer needed here after committed to storage
	d.state.deleteQC(blk.Hash())
	if !PreserveTxFlag {
		d.resources.TxPool.RemoveTxs(blk.Transactions())
	}
	d.state.setCommittedBlock(blk)
	blocks := d.state.getUncommittedOlderBlocks(blk)
	for _, blk := range blocks {
		// put txs from forked block back to queue
		d.resources.TxPool.PutTxsToQueue(blk.Transactions())
		d.state.deleteBlock(blk.Hash())
		d.state.deleteQC(blk.Hash())
	}
	//delete committed older blocks
	height := blk.Height()
	if height < 20 {
		return
	}
	blks := d.state.getOlderBlocks(height)
	for _, blk := range blks {
		d.state.deleteBlock(blk.Hash())
		d.state.deleteCommitted(blk.Hash())
	}
}
