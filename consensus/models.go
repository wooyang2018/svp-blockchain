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

func newVote(vote *core.Vote, store blockStore) *innerVote {
	return &innerVote{
		vote:  vote,
		store: store,
	}
}

func (v *innerVote) Block() *innerBlock {
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

func newQC(coreQC *core.QuorumCert, store blockStore) *innerQC {
	return &innerQC{
		qc:    coreQC,
		store: store,
	}
}

func (q *innerQC) Block() *innerBlock {
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

func newBlock(coreBlock *core.Block, store blockStore) *innerBlock {
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

func (b *innerBlock) Parent() *innerBlock {
	blk := b.store.getBlock(b.block.ParentHash())
	if blk == nil {
		return nil
	}
	return newBlock(blk, b.store)
}

func (b *innerBlock) Equal(iBlk *innerBlock) bool {
	if iBlk == nil {
		return false
	}
	return bytes.Equal(b.block.Hash(), iBlk.block.Hash())
}

// CmpBlockHeight compares two blocks by height
func CmpBlockHeight(b1, b2 *innerBlock) int {
	if b1 == nil && b2 == nil {
		return 0
	}
	if b1 == nil {
		return -1
	}
	if b2 == nil {
		return 1
	}
	if b1.Height() == b2.Height() {
		return 0
	}
	if b1.Height() > b2.Height() {
		return 1
	}
	return -1
}

type innerProposal struct {
	proposal *core.Proposal
	store    blockStore
}

func newProposal(coreBlock *core.Proposal, store blockStore) *innerProposal {
	return &innerProposal{
		proposal: coreBlock,
		store:    store,
	}
}

func (p *innerProposal) Justify() *innerQC {
	if p.proposal.Block().IsGenesis() { // genesis block doesn't have qc
		return newQC(nil, p.store)
	}
	return newQC(p.proposal.QuorumCert(), p.store)
}

func (p *innerProposal) Block() *innerBlock {
	return newBlock(p.proposal.Block(), p.store)
}

func (p *innerProposal) View() uint32 {
	return p.proposal.ViewNum()
}

func qcRefHeight(qc *innerQC) (height uint64) {
	ref := qc.Block()
	if ref != nil {
		height = ref.Height()
	}
	return height
}

func qcRefProposer(qc *innerQC) *core.PublicKey {
	ref := qc.Block()
	if ref == nil {
		return nil
	}
	return ref.block.Proposer()
}
