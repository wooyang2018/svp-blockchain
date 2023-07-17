// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package consensus

import (
	"time"

	"github.com/wooyang2018/posv-blockchain/core"
	"github.com/wooyang2018/posv-blockchain/logger"
)

type pacemaker struct {
	resources  *Resources
	config     Config
	state      *state
	status     *status
	driver     *driver
	checkDelay time.Duration // latency to check tx num in pool
	stopCh     chan struct{}
}

func (pm *pacemaker) start() {
	if pm.stopCh != nil {
		return
	}
	pm.stopCh = make(chan struct{})
	go pm.run()
	if PreserveTxFlag {
		time.Sleep(2 * time.Second)
		pm.driver.proposeCh <- struct{}{}
	}
	logger.I().Info("started pacemaker")
}

func (pm *pacemaker) stop() {
	if pm.stopCh == nil {
		return // not started yet
	}
	select {
	case <-pm.stopCh: // already stopped
		return
	default:
	}
	close(pm.stopCh)
	logger.I().Info("stopped pacemaker")
	pm.stopCh = nil
}

func (pm *pacemaker) run() {
	for {
		select {
		case <-pm.stopCh:
			return
		case <-pm.driver.proposeCh:
			pm.newProposal()
		}
	}
}

func (pm *pacemaker) newProposal() {
	pm.delayProposeWhenNoTxs()

	pm.driver.mtxUpdate.Lock()
	defer pm.driver.mtxUpdate.Unlock()

	if pm.status.getViewChange() != 0 {
		logger.I().Warn("can not create proposal when view change")
		return
	}
	if !pm.driver.isLeader(pm.resources.Signer.PublicKey()) {
		return
	}

	blk := pm.createProposal()
	pm.status.setBLeaf(blk)
	pm.status.startProposal(blk)
	pm.driver.onNewProposal(blk)
	if err := pm.resources.MsgSvc.BroadcastProposal(blk); err != nil {
		logger.I().Errorf("broadcast proposal failed, %+v", err)
	}

	logger.I().Infow("proposed proposal",
		"view", blk.View(),
		"height", blk.Height(),
		"exec", blk.ExecHeight(),
		"qc", pm.driver.qcRefHeight(blk.QuorumCert()),
		"txs", len(blk.Transactions()))
	quota := pm.resources.RoleStore.GetValidatorQuota(pm.resources.Signer.PublicKey())
	vote := blk.Vote(pm.resources.Signer, quota/float64(pm.resources.RoleStore.GetWindowSize()))
	pm.driver.onReceiveVote(vote)
	pm.driver.updateQCHigh(blk.QuorumCert())
}

func (pm *pacemaker) delayProposeWhenNoTxs() {
	timer := time.NewTimer(pm.config.TxWaitTime)
	defer timer.Stop()
	for pm.resources.TxPool.GetStatus().Total == 0 {
		select {
		case <-timer.C:
			return
		case <-time.After(pm.checkDelay):
		}
	}
}

func (pm *pacemaker) createProposal() *core.Block {
	var txs [][]byte
	if PreserveTxFlag {
		txs = pm.resources.TxPool.GetTxsFromQueue(pm.config.BlockTxLimit)
	} else {
		txs = pm.resources.TxPool.PopTxsFromQueue(pm.config.BlockTxLimit)
	}
	parent := pm.status.getBLeaf()
	blk := core.NewBlock().
		SetView(pm.status.getView()).
		SetParentHash(parent.Hash()).
		SetHeight(parent.Height() + 1).
		SetTransactions(txs).
		SetExecHeight(pm.resources.Storage.GetBlockHeight()).
		SetMerkleRoot(pm.resources.Storage.GetMerkleRoot()).
		SetTimestamp(time.Now().UnixNano()).
		SetQuorumCert(pm.status.getQCHigh()).
		Sign(pm.resources.Signer)
	pm.state.setBlock(blk)
	return blk
}
