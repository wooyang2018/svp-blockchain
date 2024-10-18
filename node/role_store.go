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
	*p2p.PeerStore
	validatorMap map[string]int
	validators   []*core.PublicKey
	stakeQuotas  []uint64
	quotaCount   uint64
	windowSize   int
	peers        []*p2p.Peer
}

var _ core.RoleStore = (*roleStore)(nil)
var _ p2p.RoleStore = (*roleStore)(nil)

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

	for i, v := range peers {
		pubKey, err := core.NewPublicKey(v.PubKey)
		common.Check(err)
		pointAddr, err := multiaddr.NewMultiaddr(v.PointAddr)
		common.Check(err)
		topicAddr, err := multiaddr.NewMultiaddr(v.TopicAddr)
		common.Check(err)
		store.peers[i] = p2p.NewPeer(pubKey, pointAddr, topicAddr)
	}
	return store
}

func (store *roleStore) SetHost(host *p2p.Host) {
	store.PeerStore = p2p.NewPeerStore(host)
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

func (store *roleStore) IsValidator(keyStr string) bool {
	if keyStr == "" {
		return false
	}
	_, ok := store.validatorMap[keyStr]
	return ok
}

func (store *roleStore) SetWindowSize(size int) {
	store.windowSize = size
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

func (store *roleStore) GetValidatorIndex(keyStr string) int {
	if keyStr == "" {
		return 0
	}
	return store.validatorMap[keyStr]
}

func (store *roleStore) GetValidatorQuota(keyStr string) uint64 {
	if keyStr == "" {
		return 0
	}
	return store.stakeQuotas[store.GetValidatorIndex(keyStr)]
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

func (store *roleStore) GetPeersFile() string {
	peers := make([]*Peer, len(store.peers))
	for i, v := range store.peers {
		peers[i] = &Peer{
			PubKey:    v.PublicKey().Bytes(),
			PointAddr: v.PointAddr().String(),
			TopicAddr: v.TopicAddr().String(),
		}
	}
	buf := new(bytes.Buffer)
	e := json.NewEncoder(buf)
	e.SetIndent("", "  ")
	e.Encode(peers)
	return buf.String()
}

func (store *roleStore) GetValidatorAddr(keyStr string) (string, string) {
	for _, v := range store.peers {
		if v.PublicKey().String() == keyStr {
			return v.PointAddr().String(), v.TopicAddr().String()
		}
	}
	return "", ""
}

func (store *roleStore) DeleteValidator(keyStr string) error {
	if _, ok := store.validatorMap[keyStr]; !ok {
		return errors.New("validator not found")
	}
	for i, v := range store.validators {
		if v.String() == keyStr {
			store.quotaCount -= store.stakeQuotas[i]
			store.validators = append(store.validators[:i], store.validators[i+1:]...)
			store.stakeQuotas = append(store.stakeQuotas[:i], store.stakeQuotas[i+1:]...)
			delete(store.validatorMap, keyStr)
			break
		}
	}
	for i, v := range store.peers {
		if v.PublicKey().String() == keyStr {
			store.PeerStore.Delete(v.PublicKey())
			store.peers = append(store.peers[:i], store.peers[i+1:]...)
			break
		}
	}
	logger.I().Infow("role store delete validator", "key", keyStr)
	return nil
}

func (store *roleStore) AddValidator(keyStr, point, topic string, quota uint64) error {
	if _, ok := store.validatorMap[keyStr]; ok {
		return errors.New("validator is added repeatedly")
	}
	store.validatorMap[keyStr] = len(store.validators)
	store.validators = append(store.validators, StringToPubKey(keyStr))
	store.stakeQuotas = append(store.stakeQuotas, quota)
	store.quotaCount += quota

	pointAddr, err := multiaddr.NewMultiaddr(point)
	if err != nil {
		return err
	}
	topicAddr, err := multiaddr.NewMultiaddr(topic)
	if err != nil {
		return err
	}
	peer := p2p.NewPeer(StringToPubKey(keyStr), pointAddr, topicAddr)
	store.peers = append(store.peers, peer)
	store.PeerStore.Store(peer)

	logger.I().Infow("role store add validator", "key", keyStr, "point addr", point, "topic addr", topic, "stake quota", quota)
	return nil
}

func (store *roleStore) AllPeers() []*p2p.Peer {
	logger.I().Infow("got role store peers", "count", len(store.peers))
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
