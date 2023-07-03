// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0
package consensus

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wooyang2018/posv-blockchain/core"
)

func setupTestPacemaker() *pacemaker {
	d := setupTestDriver()
	pm := &pacemaker{
		resources: d.resources,
		config:    d.config,
		status:    d.status,
		state:     d.state,
		driver:    d,
	}
	return pm
}

func TestCreateProposal(t *testing.T) {
	pm := setupTestPacemaker()
	parent := newTestBlock(pm.resources.Signer, 4, 3, nil, nil, nil)
	pm.state.setBlock(parent)
	pm.status.setBLeaf(parent)
	qc := core.NewQuorumCert()
	pm.status.setQCHigh(qc)

	txsInQ := [][]byte{[]byte("tx1"), []byte("tx2")}
	txPool := new(MockTxPool)
	if PreserveTxFlag {
		txPool.On("GetTxsFromQueue", pm.config.BlockTxLimit).Return(txsInQ)
	} else {
		txPool.On("PopTxsFromQueue", pm.config.BlockTxLimit).Return(txsInQ)
	}
	pm.resources.TxPool = txPool

	strg := new(MockStorage)
	strg.On("GetBlockHeight").Return(4) // driver should get bexec height from storage
	strg.On("GetMerkleRoot").Return([]byte("merkle-root"))
	pm.resources.Storage = strg

	pro := pm.createProposal()

	txPool.AssertExpectations(t)
	strg.AssertExpectations(t)

	asrt := assert.New(t)
	asrt.NotNil(pro)
	asrt.True(bytes.Equal(parent.Hash(), pro.Block().ParentHash()), "should link to parent")
	asrt.Equal(qc, pro.QuorumCert(), "should add qc")
	asrt.EqualValues(5, pro.Block().Height())

	blk := pro.Block()
	asrt.Equal(txsInQ, blk.Transactions())
	asrt.EqualValues(4, blk.ExecHeight())
	asrt.Equal([]byte("merkle-root"), blk.MerkleRoot())
	asrt.NotEmpty(blk.Timestamp(), "should add timestamp")
	asrt.NotNil(pm.state.getBlock(blk.Hash()), "should store leaf block in state")
}
