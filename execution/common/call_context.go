// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package common

import (
	"github.com/wooyang2018/svp-blockchain/core"
)

type CallContextTx struct {
	*StateTracker
	Block       *core.Block
	Transaction *core.Transaction
	RawInput    []byte
}

var _ CallContext = (*CallContextTx)(nil)

func (ctx *CallContextTx) Sender() []byte {
	if ctx.Transaction == nil {
		return nil
	}
	if ctx.Transaction.Sender() == nil {
		return nil
	}
	return ctx.Transaction.Sender().Bytes()
}

func (ctx *CallContextTx) BlockHash() []byte {
	if ctx.Block == nil {
		return nil
	}
	return ctx.Block.Hash()
}

func (ctx *CallContextTx) BlockHeight() uint64 {
	if ctx.Block == nil {
		return 0
	}
	return ctx.Block.Height()
}

func (ctx *CallContextTx) Input() []byte {
	return ctx.RawInput
}

type CallContextQuery struct {
	StateGetter
	RawInput []byte
}

var _ CallContext = (*CallContextQuery)(nil)

func (ctx *CallContextQuery) Input() []byte {
	return ctx.RawInput
}

func (ctx *CallContextQuery) Sender() []byte {
	return nil
}

func (ctx *CallContextQuery) BlockHash() []byte {
	return nil
}

func (ctx *CallContextQuery) BlockHeight() uint64 {
	return 0
}

func (ctx *CallContextQuery) SetState(key, value []byte) {
	// do nothing
}
