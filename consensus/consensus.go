// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package consensus

import (
	"os"
	"time"

	"github.com/wooyang2018/posv-blockchain/core"
	"github.com/wooyang2018/posv-blockchain/emitter"
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
	b0, q0 := cons.getInitialBlockAndQC()
	cons.state = newState()
	cons.state.setBlock(b0)
	cons.state.setQC(q0)
	cons.setupStatus(b0, q0)
	cons.setupDriver()
	cons.setupValidator()
	cons.setupPacemaker()

	status := cons.GetStatus()
	logger.I().Infow("starting consensus",
		"view", status.View,
		"leader", status.LeaderIndex,
		"bLeaf", status.BLeaf,
		"qc", status.QCHigh)

	cons.driver.start()
	cons.validator.start()
	cons.pacemaker.start()
}

func (cons *Consensus) stop() {
	if cons.pacemaker == nil {
		return
	}
	cons.driver.stop()
	cons.pacemaker.stop()
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

func (cons *Consensus) setupStatus(b0 *core.Block, q0 *core.QuorumCert) {
	cons.status = new(status)
	cons.status.setBLeaf(b0)
	cons.status.setBExec(b0)
	cons.status.setQCHigh(q0)
	// proposer of q0 may not be leader, but it doesn't matter
	cons.status.setView(q0.View())
	cons.status.setLeaderIndex(uint32(cons.resources.RoleStore.GetValidatorIndex(q0.Proposer())))
}

func (cons *Consensus) setupDriver() {
	cons.driver = &driver{
		resources:  cons.resources,
		config:     cons.config,
		state:      cons.state,
		status:     cons.status,
		checkDelay: 100 * time.Millisecond,
		qcEmitter:  emitter.New(),
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

func (cons *Consensus) setupValidator() {
	cons.validator = &validator{
		resources: cons.resources,
		state:     cons.state,
		status:    cons.status,
		driver:    cons.driver,
	}
}

func (cons *Consensus) setupPacemaker() {
	cons.pacemaker = &pacemaker{
		resources: cons.resources,
		state:     cons.state,
		status:    cons.status,
		driver:    cons.driver,
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
