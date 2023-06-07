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
	startTime int64
	state     *state
	posvState *posvState
	logfile   *os.File
	driver    *driver
	validator *validator
	pacemaker *pacemaker
	rotator   *rotator
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
	b0, q0 := cons.getInitialBlockAndQC()
	cons.setupState(b0, q0)
	cons.setupDriver()
	cons.setupValidator()
	cons.setupPacemaker()
	cons.setupRotator()

	status := cons.GetStatus()
	logger.I().Infow("starting consensus", "leader", status.LeaderIndex, "bLeaf", status.BLeaf, "qc", status.QCHigh)

	cons.validator.start()
	cons.pacemaker.start()
	cons.rotator.start()
}

func (cons *Consensus) stop() {
	if cons.pacemaker == nil {
		return
	}
	cons.pacemaker.stop()
	cons.rotator.stop()
	cons.validator.stop()
	if cons.logfile != nil {
		cons.logfile.Close()
	}
}

func (cons *Consensus) getInitialBlockAndQC() (*core.Block, *core.QuorumCert) {
	b0, err := cons.resources.Storage.GetLastBlock()
	if err == nil {
		q0, err := cons.resources.Storage.GetLastQC()
		if err != nil {
			logger.I().Fatalf("cannot get last qc %d", b0.Height())
		}
		return b0, q0
	}
	// chain not started, create genesis block
	genesis := &genesis{
		resources: cons.resources,
		chainID:   cons.config.ChainID,
	}
	return genesis.run()
}

func (cons *Consensus) setupDriver() {
	cons.driver = &driver{
		resources:    cons.resources,
		config:       cons.config,
		checkTxDelay: 10 * time.Millisecond,
		state:        cons.state,
		posvState:    cons.posvState,
	}
	if cons.config.BenchmarkPath != "" {
		var err error
		cons.logfile, err = os.Create(cons.config.BenchmarkPath)
		if err != nil {
			logger.I().Errorw("create benchmark log file failed", "error", err)
		}
	}
	cons.driver.tester = newTester(cons.logfile)
}

func (cons *Consensus) setupState(b0 *core.Block, q0 *core.QuorumCert) {
	cons.state = newState(cons.resources)
	cons.state.setBlock(b0)
	cons.state.setLeaderIndex(cons.resources.VldStore.GetWorkerIndex(b0.Proposer()))
	cons.posvState = newInnerState(newBlock(b0, cons.state), newQC(q0, cons.state))
}

func (cons *Consensus) setupValidator() {
	cons.validator = &validator{
		resources: cons.resources,
		config:    cons.config,
		state:     cons.state,
		posvState: cons.posvState,
		driver:    cons.driver,
	}
}

func (cons *Consensus) setupPacemaker() {
	cons.pacemaker = &pacemaker{
		resources: cons.resources,
		config:    cons.config,
		state:     cons.state,
		posvState: cons.posvState,
		driver:    cons.driver,
	}
}

func (cons *Consensus) setupRotator() {
	cons.rotator = &rotator{
		resources: cons.resources,
		config:    cons.config,
		state:     cons.state,
		posvState: cons.posvState,
		driver:    cons.driver,
	}
}

func (cons *Consensus) getStatus() (status Status) {
	if cons.pacemaker == nil {
		return status
	}
	status.StartTime = cons.startTime
	status.CommittedTxCount = cons.state.getCommittedTxCount()
	status.BlockPoolSize = cons.state.getBlockPoolSize()
	status.QCPoolSize = cons.state.getQCPoolSize()
	status.LeaderIndex = cons.state.getLeaderIndex()
	status.ViewNum = cons.state.getViewNum()
	status.ViewStart = cons.rotator.getViewStart()
	status.PendingViewChange = cons.rotator.getPendingViewChange()

	status.BVote = cons.posvState.GetBVote().Height()
	status.BLeaf = cons.posvState.GetBLeaf().Height()
	status.BExec = cons.posvState.GetBExec().Height()
	status.QCHigh = qcRefHeight(cons.posvState.GetQCHigh())
	return status
}
