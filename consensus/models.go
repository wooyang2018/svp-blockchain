// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package consensus

import (
	"bytes"

	"github.com/wooyang2018/posv-blockchain/core"
)

type blockStore interface {
	getBlock(hash []byte) *core.Proposal
}

type innerVote struct {
	vote  *core.Vote
	store blockStore
}

var _ Vote = (*innerVote)(nil)

func newVote(vote *core.Vote, store blockStore) Vote {
	return &innerVote{
		vote:  vote,
		store: store,
	}
}

func (v *innerVote) Block() Block {
	blk := v.store.getBlock(v.vote.BlockHash())
	if blk == nil {
		return nil
	}
	return newBlock(blk, v.store)
}

func (v *innerVote) Voter() string {
	voter := v.vote.Voter()
	if voter == nil {
		return ""
	}
	return voter.String()
}

type innerQC struct {
	qc    *core.QuorumCert
	store blockStore
}

func newQC(coreQC *core.QuorumCert, store blockStore) QC {
	return &innerQC{
		qc:    coreQC,
		store: store,
	}
}

func (q *innerQC) Block() Block {
	if q.qc == nil {
		return nil
	}
	blk := q.store.getBlock(q.qc.BlockHash())
	if blk == nil {
		return nil
	}
	return newBlock(blk, q.store)
}

type innerBlock struct {
	block *core.Proposal
	store blockStore
}

var _ Block = (*innerBlock)(nil)

func newBlock(coreBlock *core.Proposal, store blockStore) Block {
	return &innerBlock{
		block: coreBlock,
		store: store,
	}
}

func (b *innerBlock) Height() uint64 {
	return b.block.Block().Height()
}

func (b *innerBlock) Timestamp() int64 {
	return b.block.Block().Timestamp()
}

func (b *innerBlock) Transactions() [][]byte {
	return b.block.Block().Transactions()
}

func (b *innerBlock) Parent() Block {
	blk := b.store.getBlock(b.block.Block().ParentHash())
	if blk == nil {
		return nil
	}
	return newBlock(blk, b.store)
}

func (b *innerBlock) Equal(iBlk Block) bool {
	if iBlk == nil {
		return false
	}
	b2 := iBlk.(*innerBlock)
	return bytes.Equal(b.block.Hash(), b2.block.Hash())
}

func (b *innerBlock) Justify() QC {
	if b.block.Block().IsGenesis() { // genesis block doesn't have qc
		return newQC(nil, b.store)
	}
	return newQC(b.block.QuorumCert(), b.store)
}

func qcRefHeight(qc QC) (height uint64) {
	ref := qc.Block()
	if ref != nil {
		height = ref.Height()
	}
	return height
}

func qcRefProposer(qc QC) *core.PublicKey {
	ref := qc.Block()
	if ref == nil {
		return nil
	}
	return ref.(*innerBlock).block.Proposer()
}
