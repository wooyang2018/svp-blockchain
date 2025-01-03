// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package execution

import (
	"sync/atomic"
	"time"

	"github.com/wooyang2018/svp-blockchain/core"
	"github.com/wooyang2018/svp-blockchain/emitter"
	"github.com/wooyang2018/svp-blockchain/execution/common"
	"github.com/wooyang2018/svp-blockchain/logger"
)

type blkExecutor struct {
	txTimeout       time.Duration
	concurrentLimit int

	codeRegistry codeRegistry
	state        common.StateStore
	blk          *core.Block
	txs          []*core.Transaction

	rootTrk   *common.StateTracker
	txCommits []*core.TxCommit

	mergeIdx     int32
	mergeEmitter *emitter.Emitter
}

/*
Transactions of a block are executed concurrently.

The state changes for the block is tracked throughput the execution.

For each tx, both state changes and state dependencies are tracked separately when it's executed.
Later tx's state changes are merged with the block's state changes.

Given the total transactions is N and concurrentLimit is L.

An (i)th tx is executed after (i - L)th tx is executed, and it's state changes are merged.
If (i) < (L) it's straight away executed.

An (i)th tx's state changes are merged only after (i - 1)th tx's state changes are merged.
Before merging, it's checked that whether the state dependencies of the (i)th tx
are already tracked as changed.
If so, the tx is executed again before merging.
*/
func (bexe *blkExecutor) execute() (*core.BlockCommit, []*core.TxCommit) {
	start := time.Now()
	bexe.mergeEmitter = emitter.New()
	bexe.rootTrk = common.NewStateTracker(bexe.state, nil)
	bexe.txCommits = make([]*core.TxCommit, len(bexe.txs))
	bexe.executeConcurrent()

	elapsed := time.Since(start)
	bcm := core.NewBlockCommit().
		SetHash(bexe.blk.Hash()).
		SetStateChanges(bexe.rootTrk.GetStateChanges()).
		SetElapsedExec(elapsed.Seconds())

	if len(bexe.txs) > 0 {
		logger.I().Debugw("batch execution",
			"txs", len(bexe.txs), "elapsed", elapsed)
	}
	return bcm, bexe.txCommits
}

func (bexe *blkExecutor) executeConcurrent() {
	if len(bexe.txs) == 0 {
		return
	}
	jobCh := make(chan int, bexe.concurrentLimit)
	defer close(jobCh)

	for i := 0; i < bexe.concurrentLimit; i++ {
		go bexe.worker(jobCh)
	}

	sub := bexe.mergeEmitter.Subscribe(len(bexe.txs))
	defer sub.Unsubscribe()

	for i := range bexe.txs {
		jobCh <- i
	}
	for e := range sub.Events() {
		mergeIdx := e.(int)
		if mergeIdx == len(bexe.txs) { // until the last tx will finish merge
			return
		}
	}
}

func (bexe *blkExecutor) worker(jobCh <-chan int) {
	for i := range jobCh {
		bexe.executeTxAndMerge(i)
	}
}

func (bexe *blkExecutor) executeTxAndMerge(i int) {
	texe := bexe.executeTx(i)
	bexe.waitToMerge(i)
	bexe.mergeTxStateChanges(i, texe)
}

func (bexe *blkExecutor) waitToMerge(i int) {
	sub := bexe.mergeEmitter.Subscribe(20)
	defer sub.Unsubscribe()

	if bexe.getMergeIdx() == i {
		return
	}
	for e := range sub.Events() {
		mergeIdx := e.(int)
		if mergeIdx == i {
			return
		}
	}
}

func (bexe *blkExecutor) mergeTxStateChanges(i int, texe *txExecutor) {
	defer bexe.increaseMergeIdx()
	if bexe.rootTrk.HasDependencyChanges(texe.txTrk) {
		// earlier txs changes the dependencies of this tx, execute tx again
		texe = bexe.executeTx(i)
	}
	if bexe.txCommits[i].Error() != "" {
		return // don't merge state
	}
	bexe.rootTrk.Merge(texe.txTrk)
}

func (bexe *blkExecutor) executeTx(i int) *txExecutor {
	texe := &txExecutor{
		codeRegistry: bexe.codeRegistry,
		timeout:      bexe.txTimeout,
		txTrk:        bexe.rootTrk.Spawn(nil),
		blk:          bexe.blk,
		tx:           bexe.txs[i],
	}
	bexe.txCommits[i] = texe.execute()
	return texe
}

func (bexe *blkExecutor) getMergeIdx() int {
	return int(atomic.LoadInt32(&bexe.mergeIdx))
}

func (bexe *blkExecutor) increaseMergeIdx() {
	i := atomic.AddInt32(&bexe.mergeIdx, 1)
	bexe.mergeEmitter.Emit(int(i))
}
