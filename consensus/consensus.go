// Copyright (C) 2021 Aung Maw
// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package consensus

import (
	"os"
	"time"

	"github.com/wooyang2018/ppov-blockchain/core"
	"github.com/wooyang2018/ppov-blockchain/hotstuff"
	"github.com/wooyang2018/ppov-blockchain/logger"
)

type Consensus struct {
	resources *Resources
	config    Config
	startTime int64
	logfile   *os.File
	state     *state
	hsDriver  *hsDriver
	hotstuff  *hotstuff.Hotstuff
	validator *validator
	pacemaker *pacemaker
	rotator   *rotator
}

func New(resources *Resources, config Config) *Consensus {
	cons := &Consensus{
		resources: resources,
		config:    config,
	}
	return cons
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

func (cons *Consensus) start() {
	cons.startTime = time.Now().UnixNano()
	b0, q0 := cons.getInitialBlockAndQC()
	if hotstuff.TwoPhaseFlag {
		b1, _ := cons.resources.Storage.GetBlock(q0.BlockHash())
		cons.setupState(b1)
	} else {
		cons.setupState(b0)
	}

	if cons.config.BenchmarkPath != "" {
		var err error
		cons.logfile, err = os.Create(cons.config.BenchmarkPath)
		if err != nil {
			logger.I().Warnf("create benchmark log file failed, %+v", err.Error())
		}
	}

	cons.setupHsDriver()
	cons.setupHotstuff(b0, q0)
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

func (cons *Consensus) setupState(b0 *core.Block) {
	cons.state = newState(cons.resources)
	cons.state.setBlock(b0)
	cons.state.setLeaderIndex(cons.resources.VldStore.GetWorkerIndex(b0.Proposer()))
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

func (cons *Consensus) setupHsDriver() {
	cons.hsDriver = &hsDriver{
		resources:    cons.resources,
		config:       cons.config,
		checkTxDelay: 10 * time.Millisecond,
		state:        cons.state,
	}
}

func (cons *Consensus) setupHotstuff(b0 *core.Block, q0 *core.QuorumCert) {
	cons.hotstuff = hotstuff.New(
		cons.hsDriver,
		cons.logfile,
		newHsBlock(b0, cons.state),
		newHsQC(q0, cons.state),
	)
}

func (cons *Consensus) setupValidator() {
	cons.validator = &validator{
		resources: cons.resources,
		state:     cons.state,
		hotstuff:  cons.hotstuff,
	}
}

func (cons *Consensus) setupPacemaker() {
	cons.pacemaker = &pacemaker{
		resources: cons.resources,
		config:    cons.config,
		state:     cons.state,
		hotstuff:  cons.hotstuff,
	}
}

func (cons *Consensus) setupRotator() {
	cons.rotator = &rotator{
		resources: cons.resources,
		config:    cons.config,
		state:     cons.state,
		hotstuff:  cons.hotstuff,
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
	status.ViewStart = cons.rotator.getViewStart()
	status.PendingViewChange = cons.rotator.getPendingViewChange()

	status.BVote = cons.hotstuff.GetBVote().Height()
	status.BLeaf = cons.hotstuff.GetBLeaf().Height()
	status.BLock = cons.hotstuff.GetBLock().Height()
	status.BExec = cons.hotstuff.GetBExec().Height()
	status.QCHigh = qcRefHeight(cons.hotstuff.GetQCHigh())
	return status
}
