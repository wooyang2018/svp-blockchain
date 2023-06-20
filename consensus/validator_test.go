// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package consensus

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/wooyang2018/posv-blockchain/core"
)

func TestValidator_verifyProposalToVote(t *testing.T) {
	_, priv1, resources := setupTestResources()

	mStrg := new(MockStorage)
	mRoot := []byte("merkle-root")
	mStrg.On("GetBlockHeight").Return(10)
	mStrg.On("GetMerkleRoot").Return(mRoot)
	// valid tx
	tx1 := core.NewTransaction().SetExpiry(12).Sign(core.GenerateKey(nil))
	mStrg.On("HasTx", tx1.Hash()).Return(false)
	// committed tx
	tx2 := core.NewTransaction().SetExpiry(9).Sign(core.GenerateKey(nil))
	mStrg.On("HasTx", tx2.Hash()).Return(true)
	// expired tx
	tx3 := core.NewTransaction().SetExpiry(10).Sign(core.GenerateKey(nil))
	mStrg.On("HasTx", tx3.Hash()).Return(false)
	// no expiry tx (should only used for test)
	tx4 := core.NewTransaction().Sign(core.GenerateKey(nil))
	mStrg.On("HasTx", tx4.Hash()).Return(false)
	// not found tx
	// This should not happen at run time.
	// Not found tx means sync txs failed. If sync failed, cannot vote already
	tx5 := core.NewTransaction().SetExpiry(12).Sign(core.GenerateKey(nil))
	mStrg.On("HasTx", tx5.Hash()).Return(false)
	resources.Storage = mStrg

	mTxPool := new(MockTxPool)
	mTxPool.On("GetTx", tx1.Hash()).Return(tx1)
	mTxPool.On("GetTx", tx3.Hash()).Return(tx3)
	mTxPool.On("GetTx", tx4.Hash()).Return(tx4)
	mTxPool.On("GetTx", tx5.Hash()).Return(nil)
	resources.TxPool = mTxPool

	vld := &validator{
		resources: resources,
		state:     newState(),
		driver:    new(driver),
	}
	vld.state.committedHeight = mStrg.GetBlockHeight()

	type testCase struct {
		name  string
		valid bool
		block *core.Block
	}
	tests := []testCase{
		{"valid", true, newTestBlock(priv1, 11, 10,
			nil, mRoot, [][]byte{tx1.Hash(), tx4.Hash()})},
		{"different exec height", false, newTestBlock(priv1, 11, 9,
			nil, mRoot, [][]byte{tx1.Hash(), tx4.Hash()})},
	}
	if ExecuteTxFlag {
		tests = append(tests, []testCase{
			{"different merkle root", false, newTestBlock(priv1, 11, 10,
				nil, []byte("different"), [][]byte{tx1.Hash(), tx4.Hash()})},
			{"committed tx", false, newTestBlock(priv1, 11, 10,
				nil, mRoot, [][]byte{tx1.Hash(), tx2.Hash(), tx4.Hash()})},
			{"expired tx", false, newTestBlock(priv1, 11, 10,
				nil, mRoot, [][]byte{tx1.Hash(), tx3.Hash(), tx4.Hash()})},
			{"not found tx", false, newTestBlock(priv1, 11, 10,
				nil, mRoot, [][]byte{tx1.Hash(), tx5.Hash(), tx4.Hash()})},
		}...)
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			asrt := assert.New(t)
			if tt.valid {
				asrt.NoError(vld.verifyBlockToVote(tt.block))
			} else {
				asrt.Error(vld.verifyBlockToVote(tt.block))
			}
		})
	}
}
