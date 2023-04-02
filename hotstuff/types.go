// Copyright (C) 2021 Aung Maw
// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package hotstuff

const TwoPhaseFlag = true //true: 二阶段提交 false: 三阶段提交

// Block type
type Block interface {
	Height() uint64
	Parent() Block
	Equal(blk Block) bool
	Justify() QC
	Timestamp() int64
	Transactions() [][]byte
}

// QC type
type QC interface {
	Block() Block
}

// Vote type
type Vote interface {
	Block() Block
	Voter() string
}

// Driver godoc
type Driver interface {
	MajorityValidatorCount() int
	CreateLeaf(parent Block, qc QC, height uint64) Block
	CreateQC(votes []Vote) QC
	BroadcastProposal(blk Block)
	VoteBlock(blk Block)
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
