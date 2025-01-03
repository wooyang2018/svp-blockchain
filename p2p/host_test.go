// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package p2p

import (
	"sync"
	"testing"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/multiformats/go-multiaddr"
	"github.com/stretchr/testify/assert"

	"github.com/wooyang2018/svp-blockchain/core"
)

// SafeBytes is a thread-safe []byte encapsule
type SafeBytes struct {
	data []byte
	mu   sync.RWMutex
}

func (s *SafeBytes) Set(b []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data = b
}

func (s *SafeBytes) Get() []byte {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data
}

type mockRoleStore struct {
	*PeerStore
	peers []*Peer
}

func (s *mockRoleStore) AllPeers() []*Peer {
	return s.peers
}

func newMockRoleStore(host *Host, peers ...*Peer) RoleStore {
	return &mockRoleStore{
		PeerStore: NewPeerStore(host),
		peers:     peers,
	}
}

func setupTwoHost(t *testing.T) (*Host, *Host, *Peer, *Peer) {
	asrt := assert.New(t)

	pointAddr1, _ := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/15151")
	topicAddr1, _ := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/16161")
	pointAddr2, _ := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/15152")
	topicAddr2, _ := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/16162")

	priv1 := core.GenerateKey(nil)
	peer1 := NewPeer(priv1.PublicKey(), pointAddr1, topicAddr1)
	priv2 := core.GenerateKey(nil)
	peer2 := NewPeer(priv2.PublicKey(), pointAddr2, topicAddr2)

	host1, err := NewHost(priv1, pointAddr1, topicAddr1)
	asrt.NoError(err)
	host1.SetRoleStore(newMockRoleStore(host1, peer1, peer2))
	host1.roleStore.Store(peer2)

	host2, err := NewHost(priv2, pointAddr2, topicAddr2)
	asrt.NoError(err)
	host2.SetRoleStore(newMockRoleStore(host2, peer1, peer2))
	host2.roleStore.Store(peer1)

	host1.SetLeader(0)
	host2.SetLeader(0)
	asrt.NoError(host1.JoinChatRoom())
	asrt.NoError(host2.JoinChatRoom())

	time.Sleep(1 * time.Second)
	asrt.Equal(PeerStatusConnected, peer1.Status())
	asrt.Equal(PeerStatusConnected, peer2.Status())

	return host1, host2, peer1, peer2
}

func TestPointHost(t *testing.T) {
	asrt := assert.New(t)
	host1, host2, peer1, peer2 := setupTwoHost(t)

	// wait message from host2
	s1 := host2.SubscribePointMsg()
	var recv1 SafeBytes
	go func() {
		for e := range s1.Events() {
			recv1.Set(e.(*PointMsg).data)
		}
	}()

	// send message from host1
	msg := []byte("hello")
	peer2.WriteMsg(msg)

	time.Sleep(10 * time.Millisecond)
	asrt.Equal(msg, recv1.Get())

	// wait message from host1
	s2 := host1.SubscribePointMsg()
	var recv2 SafeBytes
	go func() {
		for e := range s2.Events() {
			recv2.Set(e.(*PointMsg).data)
		}
	}()

	// send message from host2
	msg = []byte("world")
	peer1.WriteMsg(msg)

	time.Sleep(10 * time.Millisecond)
	asrt.Equal(msg, recv2.Get())

	host1.Close()
	host2.Close()
}

func TestTopicHost(t *testing.T) {
	asrt := assert.New(t)
	host1, host2, _, _ := setupTwoHost(t)

	// wait message from host2
	s1 := host1.SubscribeTopicMsg()
	var recv1 SafeBytes
	go func() {
		for e := range s1.Events() {
			data := e.(*pubsub.Message)
			recv1.Set(data.Data)
		}
	}()

	// send message from host1
	msg := []byte("hello")
	asrt.NoError(host2.chatRoom.Publish(msg))

	time.Sleep(10 * time.Millisecond)
	asrt.Equal(msg, recv1.Get())

	// wait message from host1
	s2 := host2.SubscribeTopicMsg()
	var recv2 SafeBytes
	go func() {
		for e := range s2.Events() {
			data := e.(*pubsub.Message)
			recv2.Set(data.Data)
		}
	}()

	// send message from host2
	msg = []byte("world")
	asrt.NoError(host1.chatRoom.Publish(msg))

	time.Sleep(10 * time.Millisecond)
	asrt.Equal(msg, recv2.Get())

	host1.Close()
	host2.Close()
}
