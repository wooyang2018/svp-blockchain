// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package consensus

import (
	"bytes"

	"github.com/wooyang2018/posv-blockchain/core"
)

type blockStore interface {
	getBlock(hash []byte) *core.Block
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

var _ QC = (*innerQC)(nil)

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

func (q *innerQC) View() uint32 {
	return q.qc.ViewNum()
}

type innerBlock struct {
	block *core.Block
	store blockStore
}

var _ Block = (*innerBlock)(nil)

func newBlock(coreBlock *core.Block, store blockStore) Block {
	return &innerBlock{
		block: coreBlock,
		store: store,
	}
}

func (b *innerBlock) Hash() []byte {
	return b.block.Hash()
}

func (b *innerBlock) Height() uint64 {
	return b.block.Height()
}

func (b *innerBlock) Timestamp() int64 {
	return b.block.Timestamp()
}

func (b *innerBlock) Transactions() [][]byte {
	return b.block.Transactions()
}

func (b *innerBlock) Parent() Block {
	blk := b.store.getBlock(b.block.ParentHash())
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

type innerProposal struct {
	proposal *core.Proposal
	store    blockStore
}

var _ Proposal = (*innerProposal)(nil)

func newProposal(coreBlock *core.Proposal, store blockStore) Proposal {
	return &innerProposal{
		proposal: coreBlock,
		store:    store,
	}
}

func (p *innerProposal) Justify() QC {
	if p.proposal.Block().IsGenesis() { // genesis block doesn't have qc
		return newQC(nil, p.store)
	}
	return newQC(p.proposal.QuorumCert(), p.store)
}

func (p *innerProposal) Block() Block {
	return newBlock(p.proposal.Block(), p.store)
}

func (p *innerProposal) View() uint32 {
	return p.proposal.ViewNum()
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
