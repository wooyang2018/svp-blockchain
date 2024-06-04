// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package bincc

import (
	"errors"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/wooyang2018/svp-blockchain/execution/common"
)

func setupRunnerAndClient() (*Runner, *Client) {
	downR, downW := io.Pipe()
	upR, upW := io.Pipe()
	r := &Runner{
		rw: &readWriter{ // read up, write down
			reader: upR,
			writer: downW,
		},
	}
	c := &Client{
		rw: &readWriter{ // write up, read down
			reader: downR,
			writer: upW,
		},
	}
	r.timer = time.NewTimer(1 * time.Minute)
	return r, c
}

func TestCallData(t *testing.T) {
	r, c := setupRunnerAndClient()

	mctx := new(common.MockCallContext)
	mctx.MockInput = []byte("input")
	mctx.MockSender = []byte("sender")
	mctx.MockBlockHash = []byte("blockHash")
	mctx.MockBlockHeight = 10
	r.callContext = mctx

	go r.serveStateAndGetResult()
	go r.sendCallData(CallTypeInit)
	c.loadCallData()

	asrt := assert.New(t)
	asrt.Equal(CallTypeInit, c.callData.CallType)
	asrt.Equal(mctx.Input(), c.Input())
	asrt.Equal(mctx.Sender(), c.Sender())
	asrt.Equal(mctx.BlockHash(), c.BlockHash())
	asrt.Equal(mctx.BlockHeight(), c.BlockHeight())
}

func TestGetState(t *testing.T) {
	r, c := setupRunnerAndClient()
	mctx := new(common.MockCallContext)
	mctx.MemStateStore = common.NewMemStateStore()
	r.callContext = mctx

	key := []byte("somekey")
	value := []byte("somevalue")
	mctx.SetState(key, value)

	go r.serveStateAndGetResult()
	res := c.GetState(key)

	assert.Equal(t, value, res)
}

func TestSetState(t *testing.T) {
	r, c := setupRunnerAndClient()
	mctx := new(common.MockCallContext)
	mctx.MemStateStore = common.NewMemStateStore()
	r.callContext = mctx

	key := []byte("somekey")
	value := []byte("somevalue")

	go r.serveStateAndGetResult()
	c.SetState(key, value)

	assert.Equal(t, value, mctx.GetState(key))
}

func TestResult(t *testing.T) {
	r, c := setupRunnerAndClient()

	value := []byte("somevalue")

	go c.sendResult(value, nil)
	res, err := r.serveStateAndGetResult()

	asrt := assert.New(t)
	asrt.NoError(err)
	asrt.Equal(value, res)

	resErr := errors.New("run chaincode error")
	go c.sendResult(value, resErr)
	res, err = r.serveStateAndGetResult()

	asrt.Error(err)
	asrt.Equal(resErr.Error(), err.Error())
	asrt.Nil(res)
}
