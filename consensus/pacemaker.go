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

	pro := pm.createProposal()
	pm.status.setBLeaf(pro.Block())
	pm.status.startProposal(pro, pro.Block())
	pm.driver.onNewProposal(pro)
	if err := pm.resources.MsgSvc.BroadcastProposal(pro); err != nil {
		logger.I().Errorf("broadcast proposal failed, %+v", err)
	}

	logger.I().Infow("proposed proposal",
		"view", pro.View(),
		"height", pro.Block().Height(),
		"exec", pro.Block().ExecHeight(),
		"qc", pm.driver.qcRefHeight(pro.QuorumCert()),
		"txs", len(pro.Block().Transactions()))
	vote := pro.Vote(pm.resources.Signer)
	pm.driver.onReceiveVote(vote)
	pm.driver.updateQCHigh(pro.QuorumCert())
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

func (pm *pacemaker) createProposal() *core.Proposal {
	var txs [][]byte
	if PreserveTxFlag {
		txs = pm.resources.TxPool.GetTxsFromQueue(pm.config.BlockTxLimit)
	} else {
		txs = pm.resources.TxPool.PopTxsFromQueue(pm.config.BlockTxLimit)
	}
	parent := pm.status.getBLeaf()
	blk := core.NewBlock().
		SetParentHash(parent.Hash()).
		SetHeight(parent.Height() + 1).
		SetTransactions(txs).
		SetExecHeight(pm.resources.Storage.GetBlockHeight()).
		SetMerkleRoot(pm.resources.Storage.GetMerkleRoot()).
		SetTimestamp(time.Now().UnixNano()).
		Sign(pm.resources.Signer)
	pm.state.setBlock(blk)
	pro := core.NewProposal().
		SetBlock(blk).
		SetQuorumCert(pm.status.getQCHigh()).
		SetView(pm.status.getView()).
		Sign(pm.resources.Signer)
	return pro
}
