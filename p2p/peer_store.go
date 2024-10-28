// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package p2p

import (
	"sync"

	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/wooyang2018/svp-blockchain/core"
)

type RoleStore interface {
	Load(pubKey *core.PublicKey) *Peer
	Store(p *Peer) *Peer
	StoredPeers() []*Peer
	AllPeers() []*Peer
	IsValidID(id peer.ID) bool
}

type PeerStore struct {
	host  *Host
	peers map[string]*Peer
	idSet map[peer.ID]struct{}
	mtx   sync.RWMutex
}

func NewPeerStore(host *Host) *PeerStore {
	return &PeerStore{
		host:  host,
		peers: make(map[string]*Peer),
		idSet: make(map[peer.ID]struct{}),
	}
}

func (s *PeerStore) Load(pubKey *core.PublicKey) *Peer {
	s.mtx.RLock()
	defer s.mtx.RUnlock()
	return s.peers[pubKey.String()]
}

func (s *PeerStore) Store(p *Peer) *Peer {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	s.peers[p.PublicKey().String()] = p
	id, err := getIDFromPublicKey(p.PublicKey())
	if err != nil {
		panic(err)
	}
	s.idSet[id] = struct{}{}
	p.host = s.host
	return p
}

func (s *PeerStore) Delete(pubKey *core.PublicKey) *Peer {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	p := s.peers[pubKey.String()]
	delete(s.peers, pubKey.String())
	id, err := getIDFromPublicKey(pubKey)
	if err != nil {
		panic(err)
	}
	delete(s.idSet, id)
	return p
}

func (s *PeerStore) StoredPeers() []*Peer {
	s.mtx.RLock()
	defer s.mtx.RUnlock()
	peers := make([]*Peer, 0, len(s.peers))
	for _, p := range s.peers {
		peers = append(peers, p)
	}
	return peers
}

func (s *PeerStore) LoadOrStore(p *Peer) (actual *Peer, loaded bool) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	if actual, loaded = s.peers[p.PublicKey().String()]; loaded {
		return actual, loaded
	}
	s.peers[p.PublicKey().String()] = p
	id, err := getIDFromPublicKey(p.PublicKey())
	if err != nil {
		panic(err)
	}
	s.idSet[id] = struct{}{}
	return p, false
}

func (s *PeerStore) IsValidID(id peer.ID) bool {
	_, ok := s.idSet[id]
	return ok
}
