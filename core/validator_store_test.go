// Copyright (C) 2021 Aung Maw
// Licensed under the GNU General Public License v3.0

package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockValidatorStore struct {
	mock.Mock
}

var _ ValidatorStore = (*MockValidatorStore)(nil)

func (m *MockValidatorStore) VoterCount() int {
	args := m.Called()
	return args.Int(0)
}

func (m *MockValidatorStore) WorkerCount() int {
	args := m.Called()
	return args.Int(0)
}

func (m *MockValidatorStore) ValidatorCount() int {
	args := m.Called()
	return args.Int(0)
}

func (m *MockValidatorStore) MajorityVoterCount() int {
	//TODO implement me
	panic("implement me")
}

func (m *MockValidatorStore) MajorityValidatorCount() int {
	args := m.Called()
	return args.Int(0)
}

func (m *MockValidatorStore) IsVoter(pubKey *PublicKey) bool {
	args := m.Called(pubKey)
	return args.Bool(0)
}

func (m *MockValidatorStore) IsWorker(pubKey *PublicKey) bool {
	args := m.Called(pubKey)
	return args.Bool(0)
}

func (m *MockValidatorStore) GetVoter(idx int) *PublicKey {
	args := m.Called(idx)
	val := args.Get(0)
	if val == nil {
		return nil
	}
	return val.(*PublicKey)
}

func (m *MockValidatorStore) GetWorker(idx int) *PublicKey {
	args := m.Called(idx)
	val := args.Get(0)
	if val == nil {
		return nil
	}
	return val.(*PublicKey)
}

func (m *MockValidatorStore) GetVoterIndex(pubKey *PublicKey) int {
	args := m.Called(pubKey)
	return args.Int(0)
}

func (m *MockValidatorStore) GetWorkerIndex(pubKey *PublicKey) int {
	args := m.Called(pubKey)
	return args.Int(0)
}

func (m *MockValidatorStore) GetWorkerWeight(idx int) int {
	args := m.Called(idx)
	return args.Int(0)
}

func TestMajorityCount(t *testing.T) {
	type args struct {
		validatorCount int
	}
	tests := []struct {
		name string
		args args
		want int
	}{
		{"single node", args{1}, 1},
		{"exact factor", args{4}, 3},  // n = 3f+1, f=1
		{"exact factor", args{10}, 7}, // f=3, m=10-3
		{"middle", args{12}, 9},       // f=3, m=12-3
		{"middle", args{14}, 10},      // f=4, m=14-4
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MajorityCount(tt.args.validatorCount)
			assert.Equal(t, tt.want, got)
		})
	}
}
