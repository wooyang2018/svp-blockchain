// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package consensus

import (
	"os"
	"time"

	"github.com/wooyang2018/posv-blockchain/core"
	"github.com/wooyang2018/posv-blockchain/logger"
)

type Consensus struct {
	resources *Resources
	config    Config
	state     *state
	status    *status
	driver    *driver

	validator *validator
	pacemaker *pacemaker
	rotator   *rotator

	startTime int64
	logfile   *os.File
}

func New(resources *Resources, config Config) *Consensus {
	return &Consensus{
		resources: resources,
		config:    config,
	}
}

func (cons *Consensus) Start() {
	cons.start()
}

func (cons *Consensus) Stop() {
	cons.stop()
}

func (cons *Consensus) GetStatus() Status {
	return cons.getStatus()
}

func (cons *Consensus) GetBlock(hash []byte) *core.Block {
	return cons.state.getBlock(hash)
}

func (cons *Consensus) GetQC(blkHash []byte) *core.QuorumCert {
	return cons.state.getQC(blkHash)
}

func (cons *Consensus) start() {
	cons.startTime = time.Now().UnixNano()
	exec, leaf, qc := cons.getInitialBlockAndQC()
	cons.state = newState()
	cons.state.setBlock(leaf)
	cons.state.setQC(qc)
	cons.setupStatus(exec, leaf, qc)
	cons.setupDriver()
	cons.setupWindow(qc)
	cons.setupValidator()
	cons.setupPacemaker()
	cons.setupRotator()

	status := cons.GetStatus()
	logger.I().Infow("starting consensus",
		"view", status.View,
		"leader", status.LeaderIndex,
		"bLeaf", status.BLeaf,
		"qc", status.QCHigh)

	cons.validator.start()
	cons.pacemaker.start()
	cons.rotator.start()
}

func (cons *Consensus) stop() {
	cons.driver.waitCommit()
	cons.pacemaker.stop()
	cons.validator.stop()
	cons.rotator.stop()
	cons.resources.MsgSvc.BroadcastQC(cons.status.getQCHigh())
	if cons.logfile != nil {
		cons.logfile.Close()
	}
}

func (cons *Consensus) getInitialBlockAndQC() (exec *core.Block, leaf *core.Block, qc *core.QuorumCert) {
	var err error
	exec, err = cons.resources.Storage.GetLastBlock()
	if err == nil {
		qc, err = cons.resources.Storage.GetLastQC()
		if err != nil {
			panic("cannot get last qc")
		}
		leaf, err = cons.resources.Storage.GetBlock(qc.BlockHash())
		if err != nil {
			panic("cannot get block with qc")
		}
		return exec, leaf, qc
	}
	// not started blockchain yet, create genesis block
	genesis := &genesis{
		resources: cons.resources,
		chainID:   cons.config.ChainID,
	}
	exec, qc = genesis.run()
	return exec, exec, qc
}

func (cons *Consensus) setupStatus(exec *core.Block, leaf *core.Block, qc *core.QuorumCert) {
	cons.status = new(status)
	cons.status.setBLeaf(leaf)
	cons.status.setBExec(exec)
	cons.status.setQCHigh(qc)
	// proposer of q0 may not be leader, but it doesn't matter
	cons.status.setView(qc.View())
	leaderIdx := uint32(cons.resources.RoleStore.GetValidatorIndex(qc.Proposer()))
	cons.status.setLeaderIndex(leaderIdx)
}

func (cons *Consensus) setupDriver() {
	cons.driver = &driver{
		resources: cons.resources,
		config:    cons.config,
		state:     cons.state,
		status:    cons.status,
		proposeCh: make(chan struct{}),
		receiveCh: make(chan *core.Block),
	}
	if cons.config.BenchmarkPath != "" {
		var err error
		if cons.logfile, err = os.Create(cons.config.BenchmarkPath); err != nil {
			logger.I().Fatalf("cannot create benchmark log file, %+v", err)
		}
	}
	cons.driver.tester = newTester(cons.logfile)
}

func (cons *Consensus) setupWindow(qc *core.QuorumCert) {
	size := cons.resources.RoleStore.GetWindowSize()
	w := &window{
		qcQuotas:   make([]float64, size),
		voteLimit:  cons.resources.RoleStore.GetValidatorQuota(cons.resources.Signer.PublicKey()),
		voteQuotas: make([]float64, size),
		strategy:   cons.config.VoteStrategy,
		height:     cons.driver.qcRefHeight(qc),
		size:       size,
	}
	for i := size - 1; i >= 0 && qc != nil; i-- {
		w.qcQuotas[i] = qc.SumQuota()
		w.qcAcc += w.qcQuotas[i]
		w.voteQuotas[i] = qc.FindVote(cons.resources.Signer)
		w.voteAcc += w.voteQuotas[i]
		qc = cons.driver.getBlockByHash(qc.BlockHash()).QuorumCert()
	}
	cons.status.window = w
	logger.I().Infow("setup qc stake window", "quotas", w.qcQuotas, "height", w.height)
	logger.I().Infow("setup vote stake window", "quotas", w.voteQuotas, "limit", w.voteLimit)
}

func (cons *Consensus) setupValidator() {
	cons.validator = &validator{
		resources: cons.resources,
		config:    cons.config,
		state:     cons.state,
		status:    cons.status,
		driver:    cons.driver,
	}
}

func (cons *Consensus) setupPacemaker() {
	cons.pacemaker = &pacemaker{
		resources:  cons.resources,
		config:     cons.config,
		state:      cons.state,
		status:     cons.status,
		driver:     cons.driver,
		checkDelay: 100 * time.Millisecond,
	}
}

func (cons *Consensus) setupRotator() {
	cons.rotator = &rotator{
		resources: cons.resources,
		config:    cons.config,
		state:     cons.state,
		status:    cons.status,
		driver:    cons.driver,
		newViewCh: make(chan struct{}),
	}
}

func (cons *Consensus) getStatus() (status Status) {
	status.StartTime = cons.startTime

	status.CommittedTxCount = cons.state.getCommittedTxCount()
	status.BlockPoolSize = cons.state.getBlockPoolSize()
	status.QCPoolSize = cons.state.getQCPoolSize()

	status.BLeaf = cons.status.getBLeaf().Height()
	status.BExec = cons.status.getBExec().Height()
	status.View = cons.status.getView()
	status.LeaderIndex = cons.status.getLeaderIndex()
	status.ViewStart = cons.status.getViewStart()
	status.ViewChange = cons.status.getViewChange()
	status.QCHigh = cons.driver.qcRefHeight(cons.status.getQCHigh())

	return status
}
