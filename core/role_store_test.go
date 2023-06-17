// Copyright (C) 2023 Wooyang2018
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

var _ RoleStore = (*MockValidatorStore)(nil)

func (m *MockValidatorStore) ValidatorCount() int {
	args := m.Called()
	return args.Int(0)
}

func (m *MockValidatorStore) MajorityValidatorCount() int {
	args := m.Called()
	return args.Int(0)
}

func (m *MockValidatorStore) IsValidator(pubKey *PublicKey) bool {
	args := m.Called(pubKey)
	return args.Bool(0)
}

func (m *MockValidatorStore) GetValidator(idx int) *PublicKey {
	args := m.Called(idx)
	val := args.Get(0)
	if val == nil {
		return nil
	}
	return val.(*PublicKey)
}

func (m *MockValidatorStore) GetValidatorIndex(pubKey *PublicKey) int {
	args := m.Called(pubKey)
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
		{"exact factor", args{2}, 2},
		{"exact factor", args{3}, 2},
		{"middle", args{4}, 3},
		{"middle", args{5}, 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MajorityCount(tt.args.validatorCount)
			assert.Equal(t, tt.want, got)
		})
	}
}
