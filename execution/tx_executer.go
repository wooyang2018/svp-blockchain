// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package execution

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/wooyang2018/svp-blockchain/core"
	"github.com/wooyang2018/svp-blockchain/execution/common"
	"github.com/wooyang2018/svp-blockchain/logger"
)

type txExecutor struct {
	codeRegistry *codeRegistry

	timeout time.Duration
	txTrk   *stateTracker

	blk *core.Block
	tx  *core.Transaction
}

func (txe *txExecutor) execute() *core.TxCommit {
	start := time.Now()
	txc := core.NewTxCommit().
		SetHash(txe.tx.Hash()).
		SetBlockHash(txe.blk.Hash()).
		SetBlockHeight(txe.blk.Height())
	if err := txe.executeWithTimeout(); err != nil {
		logger.I().Warnf("execute tx error %+v", err)
		txc.SetError(err.Error())
	}
	txc.SetElapsed(time.Since(start).Seconds())
	return txc
}

func (txe *txExecutor) executeWithTimeout() error {
	exeError := make(chan error)
	go func() {
		exeError <- txe.executeChaincode()
	}()

	select {
	case err := <-exeError:
		return err
	case <-time.After(txe.timeout):
		return errors.New("tx execution timeout")
	}
}

func (txe *txExecutor) executeChaincode() (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%+v", r)
		}
	}()
	if len(txe.tx.CodeAddr()) == 0 {
		return txe.executeDeployment()
	}
	return txe.executeInvoke()
}

func (txe *txExecutor) executeDeployment() error {
	input := new(common.DeploymentInput)
	err := json.Unmarshal(txe.tx.Input(), input)
	if err != nil {
		return err
	}

	regTrk := txe.txTrk.spawn(codeRegistryAddr)
	cc, err := txe.codeRegistry.deploy(txe.tx.Hash(), input, regTrk)
	if err != nil {
		return err
	}

	initTrk := txe.txTrk.spawn(txe.tx.Hash())
	err = cc.Init(txe.makeCallContext(initTrk, input.InitInput))
	if err != nil {
		return err
	}
	txe.txTrk.merge(regTrk)
	txe.txTrk.merge(initTrk)
	return nil
}

func (txe *txExecutor) executeInvoke() error {
	cc, err := txe.codeRegistry.getInstance(
		txe.tx.CodeAddr(), txe.txTrk.spawn(codeRegistryAddr))
	if err != nil {
		return err
	}
	invokeTrk := txe.txTrk.spawn(txe.tx.CodeAddr())
	err = cc.Invoke(txe.makeCallContext(invokeTrk, txe.tx.Input()))
	if err != nil {
		return err
	}
	txe.txTrk.merge(invokeTrk)
	return nil
}

func (txe *txExecutor) makeCallContext(st *stateTracker, input []byte) common.CallContext {
	return &callContextTx{
		blk:          txe.blk,
		tx:           txe.tx,
		input:        input,
		stateTracker: st,
	}
}
