// Copyright (C) 2021 Aung Maw
// Licensed under the GNU General Public License v3.0

package execution

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/wooyang2018/ppov-blockchain/core"
	"github.com/wooyang2018/ppov-blockchain/execution/bincc"
	"github.com/wooyang2018/ppov-blockchain/logger"
)

type Config struct {
	BinccDir        string
	TxExecTimeout   time.Duration
	ConcurrentLimit int
}

var DefaultConfig = Config{
	TxExecTimeout:   10 * time.Second,
	ConcurrentLimit: 20,
}

type Execution struct {
	stateStore StateStore
	config     Config

	codeRegistry *codeRegistry
}

type StateStore interface {
	VerifyState(key []byte) []byte
	GetState(key []byte) []byte
}

func New(stateStore StateStore, config Config) *Execution {
	exec := &Execution{
		stateStore: stateStore,
		config:     config,
	}
	exec.codeRegistry = newCodeRegistry()
	exec.codeRegistry.registerDriver(DriverTypeNative, newNativeCodeDriver())
	exec.codeRegistry.registerDriver(DriverTypeBincc,
		bincc.NewCodeDriver(exec.config.BinccDir, exec.config.TxExecTimeout))
	return exec
}

func (exec *Execution) Execute(blk *core.Block, txs []*core.Transaction) (
	*core.BlockCommit, []*core.TxCommit,
) {
	bexe := &blkExecutor{
		txTimeout:       exec.config.TxExecTimeout,
		concurrentLimit: exec.config.ConcurrentLimit,
		codeRegistry:    exec.codeRegistry,
		state:           exec.stateStore,
		blk:             blk,
		txs:             txs,
	}
	return bexe.execute()
}

func (exec *Execution) MockExecute(blk *core.Block) (*core.BlockCommit, []*core.TxCommit) {
	bexe := &blkExecutor{
		state: exec.stateStore,
		blk:   blk,
	}
	start := time.Now()
	txCount := len(bexe.blk.Transactions())
	bexe.rootTrk = newStateTracker(bexe.state, nil)
	bexe.txCommits = make([]*core.TxCommit, txCount)
	for i := 0; i < txCount; i++ {
		bexe.txCommits[i] = core.NewTxCommit().
			SetHash(bexe.blk.Transactions()[i]).
			SetBlockHash(bexe.blk.Hash()).
			SetBlockHeight(bexe.blk.Height())
	}
	elapsed := time.Since(start)
	bcm := core.NewBlockCommit().
		SetHash(bexe.blk.Hash()).
		SetStateChanges(bexe.rootTrk.getStateChanges()).
		SetElapsedExec(elapsed.Seconds())
	if txCount > 0 {
		logger.I().Debugw("batch execution",
			"txs", txCount, "elapsed", elapsed)
	}
	return bcm, bexe.txCommits
}

type QueryData struct {
	CodeAddr []byte
	Input    []byte
}

func (exec *Execution) Query(query *QueryData) (val []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%v", r)
		}
	}()
	cc, err := exec.codeRegistry.getInstance(
		query.CodeAddr, newStateVerifier(exec.stateStore, codeRegistryAddr))
	if err != nil {
		return nil, err
	}
	return cc.Query(&callContextQuery{
		input:       query.Input,
		stateGetter: newStateVerifier(exec.stateStore, query.CodeAddr),
	})
}

func (exec *Execution) VerifyTx(tx *core.Transaction) error {
	if len(tx.CodeAddr()) != 0 { // invoke tx
		return nil
	}
	// deployment tx
	input := new(DeploymentInput)
	err := json.Unmarshal(tx.Input(), input)
	if err != nil {
		return err
	}
	return exec.codeRegistry.install(input)
}
