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
	IsValidator(keyStr string) bool
	SetWindowSize(size int)
	GetWindowSize() int
	GetValidator(idx int) *PublicKey
	GetValidatorIndex(keyStr string) int
	GetValidatorQuota(keyStr string) uint64

	GetGenesisFile() string
	GetPeersFile() string
	GetValidatorAddr(keyStr string) (string, string)
	DeleteValidator(keyStr string) error
	AddValidator(keyStr, point, topic string, quota uint64) error
	DecAndGetGrace() int
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

func (m *MockValidatorStore) IsValidator(keyStr string) bool {
	args := m.Called(keyStr)
	return args.Bool(0)
}

func (m *MockValidatorStore) SetWindowSize(size int) {
	m.Called(size)
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

func (m *MockValidatorStore) GetValidatorIndex(keyStr string) int {
	args := m.Called(keyStr)
	return args.Int(0)
}

func (m *MockValidatorStore) GetValidatorQuota(keyStr string) uint64 {
	args := m.Called(keyStr)
	return args.Get(0).(uint64)
}

func (m *MockValidatorStore) GetGenesisFile() string {
	args := m.Called()
	return args.Get(0).(string)
}

func (m *MockValidatorStore) GetPeersFile() string {
	args := m.Called()
	return args.Get(0).(string)
}

func (m *MockValidatorStore) GetValidatorAddr(keyStr string) (string, string) {
	args := m.Called(keyStr)
	return args.Get(0).(string), args.Get(1).(string)
}

func (m *MockValidatorStore) DeleteValidator(keyStr string) error {
	args := m.Called(keyStr)
	return args.Get(0).(error)
}

func (m *MockValidatorStore) AddValidator(keyStr, point, topic string, quota uint64) error {
	args := m.Called(keyStr, point, topic, quota)
	return args.Get(0).(error)
}

func (m *MockValidatorStore) DecAndGetGrace() int {
	args := m.Called()
	return args.Int(0)
}
