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

type iVote struct {
	vote  *core.Vote
	store blockStore
}

func newVote(vote *core.Vote, store blockStore) *iVote {
	return &iVote{
		vote:  vote,
		store: store,
	}
}

func (v *iVote) View() uint32 {
	return v.vote.View()
}

func (v *iVote) Quota() float64 {
	return v.vote.Quota()
}

func (v *iVote) Block() *iBlock {
	blk := v.store.getBlock(v.vote.BlockHash())
	if blk == nil {
		return nil
	}
	return newBlock(blk, v.store)
}

func (v *iVote) Voter() string {
	voter := v.vote.Voter()
	if voter == nil {
		return ""
	}
	return voter.String()
}

type iQC struct {
	qc    *core.QuorumCert
	store blockStore
}

func newQC(qc *core.QuorumCert, store blockStore) *iQC {
	return &iQC{
		qc:    qc,
		store: store,
	}
}

func (q *iQC) Block() *iBlock {
	blk := q.store.getBlock(q.qc.BlockHash())
	if blk == nil {
		return nil
	}
	return newBlock(blk, q.store)
}

func (q *iQC) View() uint32 {
	return q.qc.View()
}

type iBlock struct {
	block *core.Block
	store blockStore
}

func newBlock(block *core.Block, store blockStore) *iBlock {
	return &iBlock{
		block: block,
		store: store,
	}
}

func (b *iBlock) Hash() []byte {
	return b.block.Hash()
}

func (b *iBlock) Height() uint64 {
	return b.block.Height()
}

func (b *iBlock) Timestamp() int64 {
	return b.block.Timestamp()
}

func (b *iBlock) Transactions() [][]byte {
	return b.block.Transactions()
}

func (b *iBlock) Parent() *iBlock {
	blk := b.store.getBlock(b.block.ParentHash())
	if blk == nil {
		return nil
	}
	return newBlock(blk, b.store)
}

func (b *iBlock) Proposer() string {
	proposer := b.block.Proposer()
	if proposer == nil {
		return ""
	}
	return proposer.String()
}

func (b *iBlock) Equal(blk *iBlock) bool {
	if blk == nil {
		return false
	}
	return bytes.Equal(b.block.Hash(), blk.block.Hash())
}

type iProposal struct {
	proposal *core.Proposal
	store    blockStore
}

func newProposal(pro *core.Proposal, store blockStore) *iProposal {
	return &iProposal{
		proposal: pro,
		store:    store,
	}
}

func (p *iProposal) Justify() *iQC {
	if p.proposal.Block().IsGenesis() { // genesis block doesn't have qc
		return newQC(nil, p.store)
	}
	return newQC(p.proposal.QuorumCert(), p.store)
}

func (p *iProposal) Block() *iBlock {
	return newBlock(p.proposal.Block(), p.store)
}

func (p *iProposal) View() uint32 {
	return p.proposal.View()
}

func (p *iProposal) Hash() []byte {
	return p.proposal.Hash()
}

func (p *iProposal) Proposer() string {
	proposer := p.proposal.Proposer()
	if proposer == nil {
		return ""
	}
	return proposer.String()
}

func qcRefHeight(qc *iQC) (height uint64) {
	ref := qc.Block()
	if ref != nil {
		height = ref.Height()
	}
	return height
}

func qcRefProposer(qc *iQC) *core.PublicKey {
	ref := qc.Block()
	if ref == nil {
		return nil
	}
	return ref.block.Proposer()
}

// cmpBlockHeight compares two blocks by height
func cmpBlockHeight(b1, b2 *iBlock) int {
	if b1 == nil || b2 == nil {
		panic("failed to compare nil block height")
	}
	if b1.Height() == b2.Height() {
		return 0
	} else if b1.Height() > b2.Height() {
		return 1
	}
	return -1
}

func cmpQCPriority(qc1, qc2 *iQC) int {
	if qc1 == nil || qc2 == nil {
		panic("failed to compare nil qc priority")
	}
	if qc1.View() > qc2.View() {
		return 1
	} else if qc1.View() < qc2.View() {
		return -1
	} else { //qc1.View() == qc2.View()
		if qcRefHeight(qc1) > qcRefHeight(qc2) {
			return 1
		} else if qcRefHeight(qc1) < qcRefHeight(qc2) {
			return -1
		}
		return 0
	}
}
