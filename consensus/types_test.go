// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package consensus

import (
	"testing"

	"github.com/stretchr/testify/mock"
)

func castProposal(val interface{}) Proposal {
	if b, ok := val.(Proposal); ok {
		return b
	}
	return nil
}

type MockProposal struct {
	mock.Mock
}

var _ Proposal = (*MockProposal)(nil)

func (m *MockProposal) Block() Block {
	args := m.Called()
	return castBlock(args.Get(0))
}

func (m *MockProposal) Justify() QC {
	args := m.Called()
	return castQC(args.Get(0))
}

func (m *MockProposal) View() uint32 {
	args := m.Called()
	return uint32(args.Int(0))
}

func castBlock(val interface{}) Block {
	if b, ok := val.(Block); ok {
		return b
	}
	return nil
}

func castQC(val interface{}) QC {
	if b, ok := val.(QC); ok {
		return b
	}
	return nil
}

type MockBlock struct {
	mock.Mock
}

var _ Block = (*MockBlock)(nil)

func (m *MockBlock) Hash() []byte {
	args := m.Called()
	return castBytes(args.Get(0))
}

func (m *MockBlock) Proposer() string {
	args := m.Called()
	return string(args.String(0))
}

func (m *MockBlock) Height() uint64 {
	args := m.Called()
	return uint64(args.Int(0))
}

func (m *MockBlock) Parent() Block {
	args := m.Called()
	return castBlock(args.Get(0))
}

func (m *MockBlock) Equal(blk Block) bool {
	args := m.Called(blk)
	return args.Bool(0)
}

func (m *MockBlock) Justify() QC {
	args := m.Called()
	return args.Get(0).(QC)
}

func (m *MockBlock) Timestamp() int64 {
	args := m.Called()
	return int64(args.Int(0))
}

func (m *MockBlock) Transactions() [][]byte {
	args := m.Called()
	return args.Get(0).([][]byte)
}

type MockQC struct {
	mock.Mock
}

var _ QC = (*MockQC)(nil)

func (m *MockQC) Block() Block {
	args := m.Called()
	return castBlock(args.Get(0))
}

func (m *MockQC) View() uint32 {
	args := m.Called()
	return uint32(args.Int(0))
}

type MockVote struct {
	mock.Mock
}

var _ Vote = (*MockVote)(nil)

func (m *MockVote) Block() Block {
	args := m.Called()
	return castBlock(args.Get(0))
}

func (m *MockVote) Voter() string {
	args := m.Called()
	return args.String(0)
}

func newMockBlock(height int, parent Block, qc QC) *MockBlock {
	b := new(MockBlock)
	b.On("Height").Return(height)
	b.On("Parent").Return(parent)
	b.On("Equal", b).Return(true)
	b.On("Equal", mock.Anything).Return(false)
	b.On("Justify").Return(qc)
	b.On("Timestamp").Return(0)
	b.On("Transactions").Return([][]byte{})
	return b
}

func newMockQC(blk Block) *MockQC {
	qc := new(MockQC)
	qc.On("Block").Return(blk)
	return qc
}

func newMockVote(blk Block, voter string) *MockVote {
	vote := new(MockVote)
	vote.On("Block").Return(blk)
	vote.On("Voter").Return(voter)
	return vote
}

func TestCmpBlockHeight(t *testing.T) {
	type args struct {
		b1 Block
		b2 Block
	}

	bh4 := newMockBlock(4, nil, nil)
	bh5 := newMockBlock(5, nil, nil)

	tests := []struct {
		name string
		args args
		want int
	}{
		{"nil blocks", args{nil, nil}, 0},
		{"b1 is nil", args{nil, new(MockBlock)}, -1},
		{"b2 is nil", args{new(MockBlock), nil}, 1},
		{"b1 is higher", args{bh5, bh4}, 1},
		{"b2 is higher", args{bh4, bh5}, -1},
		{"same height", args{bh4, bh4}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CmpBlockHeight(tt.args.b1, tt.args.b2); got != tt.want {
				t.Errorf("CmpBlockHeight() = %v, want %v", got, tt.want)
			}
		})
	}
}
