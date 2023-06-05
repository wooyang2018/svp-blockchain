// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package consensus

// Block type
type Block interface { //TODO 删除
	Hash() []byte
	Height() uint64
	Parent() Block
	Equal(blk Block) bool
	Timestamp() int64
	Transactions() [][]byte
}

type Proposal interface { //TODO 删除
	Block() Block
	Justify() QC
	View() uint32
}

// QC type
type QC interface { //TODO 删除
	Block() Block
	View() uint32
}

// Vote type
type Vote interface { //TODO 删除
	Block() Block
	Voter() string
}

// Driver godoc
type Driver interface { //TODO 删除
	MajorityValidatorCount() int
	CreateProposal(parent Block, qc QC, height uint64, view uint32) Proposal
	CreateQC(votes []Vote) QC
	BroadcastProposal(blk Proposal)
	VoteProposal(pro Proposal)
	Commit(blk Block)
}

// CmpBlockHeight compares two blocks by height
func CmpBlockHeight(b1, b2 Block) int {
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
