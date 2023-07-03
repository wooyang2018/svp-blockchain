// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0
package consensus

import (
	"testing"

	"github.com/wooyang2018/posv-blockchain/core"
)

func setupTestValidator() *validator {
	d := setupTestDriver()
	vld := &validator{
		resources: d.resources,
		config:    d.config,
		status:    d.status,
		state:     d.state,
		driver:    d,
	}
	return vld
}

func TestVoteBlock(t *testing.T) {
	vld := setupTestValidator()
	blk1 := newTestBlock(vld.resources.Signer, 3, 2, nil, nil, nil)
	pro1 := core.NewProposal().SetBlock(blk1).Sign(vld.resources.Signer)
	vld.state.setBlock(blk1)
	votes := []*core.Vote{
		pro1.Vote(vld.resources.Signer),
		pro1.Vote(core.GenerateKey(nil)),
	}
	qc1 := core.NewQuorumCert().Build(vld.resources.Signer, votes)
	vld.status.setQCHigh(qc1)

	proposer := core.GenerateKey(nil)
	blk2 := newTestBlock(vld.resources.Signer, 4, 3, nil, nil, nil)
	pro2 := core.NewProposal().SetBlock(blk2).SetQuorumCert(qc1).Sign(proposer)
	validators := []string{pro2.Proposer().String()}
	vld.resources.RoleStore = core.NewRoleStore(validators)

	txPool := new(MockTxPool)
	if !PreserveTxFlag {
		txPool.On("SetTxsPending", pro2.Block().Transactions())
	}
	vld.resources.TxPool = txPool

	msgSvc := new(MockMsgService)
	msgSvc.On("SendVote", proposer.PublicKey(), pro2.Vote(vld.resources.Signer)).Return(nil)
	vld.resources.MsgSvc = msgSvc

	// should sign block and send vote
	vld.voteProposal(pro2, pro2.Block())

	txPool.AssertExpectations(t)
	msgSvc.AssertExpectations(t)
}
