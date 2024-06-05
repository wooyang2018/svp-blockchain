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
	codeRegistry codeRegistry
	txTrk        *common.StateTracker
	blk          *core.Block
	tx           *core.Transaction
	timeout      time.Duration
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

	regTrk := txe.txTrk.Spawn(codeRegistryAddr)
	cc, err := txe.codeRegistry.deploy(txe.tx.Hash(), input, regTrk)
	if err != nil {
		return err
	}

	initTrk := txe.txTrk.Spawn(txe.tx.Hash())
	cc.SetTxTrk(txe.txTrk)
	err = cc.Init(txe.makeCallContext(initTrk, input.InitInput))
	if err != nil {
		return err
	}
	txe.txTrk.Merge(regTrk)
	txe.txTrk.Merge(initTrk)
	return nil
}

func (txe *txExecutor) executeInvoke() error {
	cc, err := txe.codeRegistry.getInstance(
		txe.tx.CodeAddr(), txe.txTrk.Spawn(codeRegistryAddr))
	if err != nil {
		return err
	}
	invokeTrk := txe.txTrk.Spawn(txe.tx.CodeAddr())
	cc.SetTxTrk(txe.txTrk)
	err = cc.Invoke(txe.makeCallContext(invokeTrk, txe.tx.Input()))
	if err != nil {
		return err
	}
	txe.txTrk.Merge(invokeTrk)
	return nil
}

func (txe *txExecutor) makeCallContext(st *common.StateTracker, input []byte) common.CallContext {
	return &common.CallContextTx{
		Block:        txe.blk,
		Transaction:  txe.tx,
		RawInput:     input,
		StateTracker: st,
	}
}
