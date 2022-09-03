// Copyright (C) 2021 Aung Maw
// Licensed under the GNU General Public License v3.0

package core

import (
	"math"
)

// ValidatorStore godoc
type ValidatorStore interface {
	VoterCount() int
	WorkerCount() int
	MajorityCount() int
	IsVoter(pubKey *PublicKey) bool
	IsWorker(pubKey *PublicKey) bool
	GetVoter(idx int) *PublicKey
	GetWorker(idx int) *PublicKey
	GetVoterIndex(pubKey *PublicKey) int
	GetWorkerIndex(pubKey *PublicKey) int
	GetWorkerWeight(idx int) int
}

type ppovValidatorStore struct {
	voters  []*PublicKey //投票节点列表
	workers []*PublicKey //记账节点列表
	weights []int        //记账节点权重列表
	vcount  int          //投票节点收集区块的阈值

	vMap     map[string]int
	wMap     map[string]int
	majority int
}

var _ ValidatorStore = (*ppovValidatorStore)(nil)

func NewValidatorStore(workers []*PublicKey, weights []int, voters []*PublicKey) ValidatorStore {
	store := &ppovValidatorStore{
		voters:  voters,
		workers: workers,
		weights: weights,
	}
	store.vcount = len(workers) //TODO
	store.vMap = make(map[string]int, len(store.voters))
	for i, v := range store.voters {
		store.vMap[v.String()] = i
	}
	store.wMap = make(map[string]int, len(store.workers))
	for i, v := range store.workers {
		store.wMap[v.String()] = i
	}
	store.majority = MajorityCount(len(voters))
	return store
}

func (store *ppovValidatorStore) VoterCount() int {
	return len(store.voters)
}

func (store *ppovValidatorStore) WorkerCount() int {
	return len(store.workers)
}

func (store *ppovValidatorStore) MajorityCount() int {
	return store.majority
}

func (store *ppovValidatorStore) IsVoter(pubKey *PublicKey) bool {
	if pubKey == nil {
		return false
	}
	_, ok := store.vMap[pubKey.String()]
	return ok
}

func (store *ppovValidatorStore) IsWorker(pubKey *PublicKey) bool {
	if pubKey == nil {
		return false
	}
	_, ok := store.wMap[pubKey.String()]
	return ok
}

func (store *ppovValidatorStore) GetVoter(idx int) *PublicKey {
	if idx >= len(store.voters) || idx < 0 {
		return nil
	}
	return store.voters[idx]
}

func (store *ppovValidatorStore) GetWorker(idx int) *PublicKey {
	if idx >= len(store.workers) || idx < 0 {
		return nil
	}
	return store.workers[idx]
}

func (store *ppovValidatorStore) GetVoterIndex(pubKey *PublicKey) int {
	if pubKey == nil {
		return 0
	}
	return store.vMap[pubKey.String()]
}

func (store *ppovValidatorStore) GetWorkerIndex(pubKey *PublicKey) int {
	if pubKey == nil {
		return 0
	}
	return store.wMap[pubKey.String()]
}

func (store *ppovValidatorStore) GetWorkerWeight(idx int) int {
	if idx >= len(store.weights) || idx < 0 {
		return -1
	}
	return store.weights[idx]
}

// MajorityCount returns 2f + 1 members
func MajorityCount(validatorCount int) int {
	// n=3f+1 -> f=floor((n-1)3) -> m=n-f -> m=ceil((2n+1)/3)
	return int(math.Ceil(float64(2*validatorCount+1) / 3))
}
