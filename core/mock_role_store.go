// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package core

import (
	"github.com/stretchr/testify/mock"
)

type RoleStore interface {
	ValidatorCount() int
	MajorityValidatorCount() int
	MajorityQuotaCount() uint64
	IsValidator(pubKey *PublicKey) bool
	GetWindowSize() int
	GetValidator(idx int) *PublicKey
	GetValidatorIndex(pubKey *PublicKey) int
	GetValidatorQuota(pubKey *PublicKey) uint64

	GetGenesisFile() string
	DeleteValidator(pubKey string) error
	AddValidator(pubKey string, quota uint64) error
}

var SRole RoleStore

func SetSRole(role RoleStore) {
	SRole = role
}

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

func (m *MockValidatorStore) MajorityQuotaCount() uint64 {
	args := m.Called()
	return args.Get(0).(uint64)
}

func (m *MockValidatorStore) IsValidator(pubKey *PublicKey) bool {
	args := m.Called(pubKey)
	return args.Bool(0)
}

func (m *MockValidatorStore) GetWindowSize() int {
	args := m.Called()
	return args.Int(0)
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

func (m *MockValidatorStore) GetValidatorQuota(pubKey *PublicKey) uint64 {
	args := m.Called(pubKey)
	return args.Get(0).(uint64)
}

func (m *MockValidatorStore) GetGenesisFile() string {
	args := m.Called()
	return args.Get(0).(string)
}

func (m *MockValidatorStore) DeleteValidator(pubKey string) error {
	args := m.Called(pubKey)
	return args.Get(0).(error)
}

func (m *MockValidatorStore) AddValidator(pubKey string, quota uint64) error {
	args := m.Called(pubKey, quota)
	return args.Get(0).(error)
}
