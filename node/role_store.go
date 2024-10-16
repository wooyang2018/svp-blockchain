// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package node

import (
	"bytes"
	"encoding/json"
	"errors"
	"math"

	"github.com/multiformats/go-multiaddr"

	"github.com/wooyang2018/svp-blockchain/core"
	"github.com/wooyang2018/svp-blockchain/execution/common"
	"github.com/wooyang2018/svp-blockchain/logger"
	"github.com/wooyang2018/svp-blockchain/p2p"
)

type roleStore struct {
	validatorMap map[string]int
	validators   []*core.PublicKey
	stakeQuotas  []uint64
	quotaCount   uint64
	windowSize   int
	peers        []*p2p.Peer
}

var _ core.RoleStore = (*roleStore)(nil)

func NewRoleStore(genesis *Genesis, peers []*Peer) *roleStore {
	count := len(genesis.Validators)
	store := &roleStore{
		validatorMap: make(map[string]int, count),
		validators:   make([]*core.PublicKey, count),
		stakeQuotas:  genesis.StakeQuotas,
		windowSize:   genesis.WindowSize,
		peers:        make([]*p2p.Peer, count),
	}
	// assert len(validators) == len(quotas)
	for i, v := range genesis.Validators {
		store.validators[i] = StringToPubKey(v)
		store.validatorMap[v] = i
		store.quotaCount += genesis.StakeQuotas[i]
	}

	for i, r := range peers {
		pubKey, err := core.NewPublicKey(r.PubKey)
		common.Check(err)
		pointAddr, err := multiaddr.NewMultiaddr(r.PointAddr)
		common.Check(err)
		topicAddr, err := multiaddr.NewMultiaddr(r.TopicAddr)
		common.Check(err)
		store.peers[i] = p2p.NewPeer(pubKey, pointAddr, topicAddr)
	}
	return store
}

func (store *roleStore) ValidatorCount() int {
	return len(store.validators)
}

func (store *roleStore) MajorityValidatorCount() int {
	return MajorityCount(len(store.validators))
}

func (store *roleStore) MajorityQuotaCount() uint64 {
	return (store.quotaCount + 1) / 2
}

func (store *roleStore) IsValidator(pubKey *core.PublicKey) bool {
	if pubKey == nil {
		return false
	}
	_, ok := store.validatorMap[pubKey.String()]
	return ok
}

func (store *roleStore) GetWindowSize() int {
	return store.windowSize
}

func (store *roleStore) GetValidator(idx int) *core.PublicKey {
	if idx >= len(store.validators) || idx < 0 {
		return nil
	}
	return store.validators[idx]
}

func (store *roleStore) GetValidatorIndex(pubKey *core.PublicKey) int {
	if pubKey == nil {
		return 0
	}
	return store.validatorMap[pubKey.String()]
}

func (store *roleStore) GetValidatorQuota(pubKey *core.PublicKey) uint64 {
	if pubKey == nil {
		return 0
	}
	return store.stakeQuotas[store.GetValidatorIndex(pubKey)]
}

func (store *roleStore) GetGenesisFile() string {
	genesis := Genesis{
		Validators:  make([]string, len(store.validators)),
		StakeQuotas: store.stakeQuotas,
		WindowSize:  store.windowSize,
	}
	for i, v := range store.validators {
		genesis.Validators[i] = v.String()
	}
	buf := new(bytes.Buffer)
	e := json.NewEncoder(buf)
	e.SetIndent("", "  ")
	e.Encode(genesis)
	return buf.String()
}

func (store *roleStore) DeleteValidator(pubKey string) error {
	if _, ok := store.validatorMap[pubKey]; !ok {
		return errors.New("validator not found")
	}
	for i, v := range store.validators {
		if v.String() == pubKey {
			store.quotaCount -= store.stakeQuotas[i]
			store.validators = append(store.validators[:i], store.validators[i+1:]...)
			store.stakeQuotas = append(store.stakeQuotas[:i], store.stakeQuotas[i+1:]...)
			delete(store.validatorMap, pubKey)
			break
		}
	}
	return nil
}

func (store *roleStore) AddValidator(key string, quota uint64) error {
	if _, ok := store.validatorMap[key]; ok {
		return errors.New("validator is added repeatedly")
	}
	store.validatorMap[key] = len(store.validators)
	store.validators = append(store.validators, StringToPubKey(key))
	store.stakeQuotas = append(store.stakeQuotas, quota)
	store.quotaCount += quota
	return nil
}

func (store *roleStore) GetPeers() []*p2p.Peer {
	return store.peers
}

func StringToPubKey(v string) *core.PublicKey {
	key, err := common.Address32ToBytes(v)
	pubKey, err := core.NewPublicKey(key)
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
