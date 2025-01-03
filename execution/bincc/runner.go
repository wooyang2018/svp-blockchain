// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package bincc

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"time"

	"github.com/wooyang2018/svp-blockchain/execution/common"
	"github.com/wooyang2018/svp-blockchain/logger"
)

type Runner struct {
	codePath string
	timeout  time.Duration
	timer    *time.Timer

	cmd     *exec.Cmd
	rw      *readWriter
	callCtx common.CallContext
}

var _ common.Chaincode = (*Runner)(nil)

func (r *Runner) Init(ctx common.CallContext) error {
	r.callCtx = ctx
	_, err := r.runCode(CallTypeInit)
	return err
}

func (r *Runner) Invoke(ctx common.CallContext) error {
	r.callCtx = ctx
	_, err := r.runCode(CallTypeInvoke)
	return err
}

func (r *Runner) Query(ctx common.CallContext) ([]byte, error) {
	r.callCtx = ctx
	return r.runCode(CallTypeQuery)
}

func (r *Runner) SetTxTrk(txTrk *common.StateTracker) {
	return
}

func (r *Runner) runCode(callType CallType) ([]byte, error) {
	r.timer = time.NewTimer(r.timeout)
	defer r.timer.Stop()

	if err := r.startCode(CallTypeInit); err != nil {
		return nil, err
	}
	defer r.cmd.Process.Kill()

	if err := r.sendCallData(callType); err != nil {
		return nil, err
	}
	res, err := r.serveStateAndGetResult()
	if err != nil {
		return nil, err
	}
	return res, r.cmd.Wait()
}

func (r *Runner) startCode(callType CallType) error {
	if err := r.setupCmd(); err != nil {
		return err
	}
	err := r.cmd.Start()
	if err == nil {
		return nil
	}
	logger.I().Warnf("start code error:%+v", err)
	select {
	case <-r.timer.C:
		return errors.New("chaincode start timeout")
	default:
	}
	time.Sleep(5 * time.Millisecond)
	return r.startCode(callType)
}

func (r *Runner) setupCmd() error {
	r.cmd = exec.Command(r.codePath)
	var err error
	r.rw = new(readWriter)
	r.rw.writer, err = r.cmd.StdinPipe()
	if err != nil {
		return err
	}
	r.rw.reader, err = r.cmd.StderrPipe()
	return err
}

func (r *Runner) sendCallData(callType CallType) error {
	callData := &CallData{
		CallType:        callType,
		Input:           r.callCtx.Input(),
		Sender:          r.callCtx.Sender(),
		TransactionHash: r.callCtx.TransactionHash(),
		BlockHash:       r.callCtx.BlockHash(),
		BlockHeight:     r.callCtx.BlockHeight(),
	}
	b, _ := json.Marshal(callData)
	return r.rw.write(b)
}

func (r *Runner) serveStateAndGetResult() ([]byte, error) {
	for {
		select {
		case <-r.timer.C:
			return nil, errors.New("chaincode call timeout")
		default:
		}
		b, err := r.rw.read()
		if err != nil {
			return nil, fmt.Errorf("read upstream error, %w", err)
		}
		up := new(UpStream)
		if err = json.Unmarshal(b, up); err != nil {
			return nil, errors.New("cannot parse upstream data")
		}
		if up.Type == UpStreamResult {
			if len(up.Error) > 0 {
				return nil, errors.New(up.Error)
			}
			return up.Value, nil
		}
		if err = r.serveState(up); err != nil {
			return nil, err
		}
	}
}

func (r *Runner) serveState(up *UpStream) error {
	down := new(DownStream)
	switch up.Type {
	case UpStreamGetState:
		val := r.callCtx.GetState(up.Key)
		down.Value = val
	case UpStreamSetState:
		r.callCtx.SetState(up.Key, up.Value)
	}
	b, _ := json.Marshal(down)
	return r.rw.write(b)
}
