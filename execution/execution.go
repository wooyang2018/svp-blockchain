// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package execution

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/wooyang2018/svp-blockchain/core"
	"github.com/wooyang2018/svp-blockchain/execution/bincc"
	"github.com/wooyang2018/svp-blockchain/execution/common"
	"github.com/wooyang2018/svp-blockchain/execution/evm"
	"github.com/wooyang2018/svp-blockchain/logger"
	"github.com/wooyang2018/svp-blockchain/native"
	"github.com/wooyang2018/svp-blockchain/storage"
)

type Config struct {
	BinccDir        string
	ContractDir     string
	TxExecTimeout   time.Duration
	ConcurrentLimit int
}

var DefaultConfig = Config{
	TxExecTimeout:   10 * time.Second,
	ConcurrentLimit: 20,
}

type Execution struct {
	stateStore   common.StateStore
	config       Config
	codeRegistry codeRegistry
}

func New(stateStore *storage.Storage, config Config) *Execution {
	exec := &Execution{
		stateStore: stateStore,
		config:     config,
	}
	exec.codeRegistry = newCodeRegistry()
	exec.codeRegistry.registerDriver(common.DriverTypeNative, native.NewCodeDriver())
	exec.codeRegistry.registerDriver(common.DriverTypeBincc,
		bincc.NewCodeDriver(exec.config.BinccDir, exec.config.TxExecTimeout))
	exec.codeRegistry.registerDriver(common.DriverTypeEVM,
		evm.NewCodeDriver(exec.config.ContractDir))
	common.Storage = stateStore
	common.Drivers = exec.codeRegistry
	return exec
}

func (exec *Execution) Drivers() map[common.DriverType]common.CodeDriver {
	return exec.codeRegistry
}

func (exec *Execution) StateStore() common.StateStore {
	return exec.stateStore
}

func (exec *Execution) Execute(blk *core.Block, txs []*core.Transaction) (
	*core.BlockCommit, []*core.TxCommit) {
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

func (exec *Execution) Query(query *common.QueryData) (val []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%+v", r)
		}
	}()
	cc, err := exec.codeRegistry.getInstance(
		query.CodeAddr, newStateVerifier(exec.stateStore, codeRegistryAddr))
	if err != nil {
		return nil, err
	}
	return cc.Query(&callContextQuery{
		input:       query.Input,
		StateGetter: newStateVerifier(exec.stateStore, query.CodeAddr),
	})
}

func (exec *Execution) VerifyTx(tx *core.Transaction) error {
	if len(tx.CodeAddr()) != 0 { // invoke tx
		return nil
	}
	// deployment tx
	input := new(common.DeploymentInput)
	if err := json.Unmarshal(tx.Input(), input); err != nil {
		return err
	}
	return exec.codeRegistry.install(input)
}
