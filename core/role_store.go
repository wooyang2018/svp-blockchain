// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package core

import (
	"encoding/base64"
	"math"

	"github.com/wooyang2018/svp-blockchain/logger"
)

type RoleStore interface {
	ValidatorCount() int
	MajorityValidatorCount() int
	MajorityQuotaCount() uint32
	IsValidator(pubKey *PublicKey) bool
	GetWindowSize() int
	GetValidator(idx int) *PublicKey
	GetValidatorIndex(pubKey *PublicKey) int
	GetValidatorQuota(pubKey *PublicKey) uint32
}

type roleStore struct {
	validatorMap map[string]int
	validators   []*PublicKey
	stakeQuotas  []uint32
	quotaCount   uint32
	windowSize   int
}

var _ RoleStore = (*roleStore)(nil)

func NewRoleStore(validators []string, quotas []uint32, size int) RoleStore {
	store := &roleStore{
		validatorMap: make(map[string]int, len(validators)),
		validators:   make([]*PublicKey, len(validators)),
		stakeQuotas:  quotas,
		windowSize:   size,
	}
	//assert len(validators) == len(quotas)
	for i, v := range validators {
		store.validators[i] = StringToPubKey(v)
		store.validatorMap[v] = i
		store.quotaCount += quotas[i]
	}
	return store
}

func (store *roleStore) ValidatorCount() int {
	return len(store.validators)
}

func (store *roleStore) MajorityValidatorCount() int {
	return MajorityCount(len(store.validators))
}

func (store *roleStore) MajorityQuotaCount() uint32 {
	return (store.quotaCount + 1) / 2
}

func (store *roleStore) IsValidator(pubKey *PublicKey) bool {
	if pubKey == nil {
		return false
	}
	_, ok := store.validatorMap[pubKey.String()]
	return ok
}

func (store *roleStore) GetWindowSize() int {
	return store.windowSize
}

func (store *roleStore) GetValidator(idx int) *PublicKey {
	if idx >= len(store.validators) || idx < 0 {
		return nil
	}
	return store.validators[idx]
}

func (store *roleStore) GetValidatorIndex(pubKey *PublicKey) int {
	if pubKey == nil {
		return 0
	}
	return store.validatorMap[pubKey.String()]
}

func (store *roleStore) GetValidatorQuota(pubKey *PublicKey) uint32 {
	if pubKey == nil {
		return 0
	}
	return store.stakeQuotas[store.GetValidatorIndex(pubKey)]
}

func StringToPubKey(v string) *PublicKey {
	key, err := base64.StdEncoding.DecodeString(v)
	pubKey, err := NewPublicKey(key)
	if err != nil {
		logger.I().Fatalw("parse voter failed", "error", err)
	}
	return pubKey
}

// MajorityCount returns 2f + 1 members
func MajorityCount(validatorCount int) int {
	// n=3f+1 -> f=floor((n-1)3) -> m=n-f -> m=ceil((2n+1)/3)
	return int(math.Ceil(float64(2*validatorCount+1) / 3))
}
