// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package p2p

import (
	"testing"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/multiformats/go-multiaddr"
	"github.com/stretchr/testify/assert"

	"github.com/wooyang2018/svp-blockchain/core"
)

func setupTwoHost(t *testing.T) (*Host, *Host, *Peer, *Peer) {
	asrt := assert.New(t)

	pointAddr1, _ := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/15151")
	topicAddr1, _ := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/16161")
	pointAddr2, _ := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/15152")
	topicAddr2, _ := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/16162")

	priv1 := core.GenerateKey(nil)
	peer1 := NewPeer(priv1.PublicKey(), pointAddr1, topicAddr1)
	host1, err := NewHost(priv1, pointAddr1, topicAddr1)
	asrt.NoError(err)

	priv2 := core.GenerateKey(nil)
	peer2 := NewPeer(priv2.PublicKey(), pointAddr2, topicAddr2)
	host2, err := NewHost(priv2, pointAddr2, topicAddr2)
	asrt.NoError(err)

	host1.AddPeer(peer2)
	host1.SetPeers([]*Peer{peer1, peer2})
	host2.AddPeer(peer1)
	host2.SetPeers([]*Peer{peer1, peer2})

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
	s1 := peer1.SubscribeMsg()
	var recv1 []byte
	go func() {
		for e := range s1.Events() {
			recv1 = e.([]byte)
		}
	}()

	// send message from host1
	msg := []byte("hello")
	peer2.WriteMsg(msg)

	time.Sleep(10 * time.Millisecond)
	asrt.Equal(msg, recv1)

	// wait message from host1
	s2 := peer2.SubscribeMsg()
	var recv2 []byte
	go func() {
		for e := range s2.Events() {
			recv2 = e.([]byte)
		}
	}()

	// send message from host2
	msg = []byte("world")
	peer1.WriteMsg(msg)

	time.Sleep(10 * time.Millisecond)
	asrt.Equal(msg, recv2)

	host1.Close()
	host2.Close()
}

func TestTopicHost(t *testing.T) {
	asrt := assert.New(t)
	host1, host2, _, _ := setupTwoHost(t)

	// wait message from host2
	s1 := host1.SubscribeMsg()
	var recv1 []byte
	go func() {
		for e := range s1.Events() {
			data := e.(*pubsub.Message)
			recv1 = data.Data
		}
	}()

	// send message from host1
	msg := []byte("hello")
	asrt.NoError(host2.chatRoom.Publish(msg))

	time.Sleep(10 * time.Millisecond)
	asrt.Equal(msg, recv1)

	// wait message from host1
	s2 := host2.SubscribeMsg()
	var recv2 []byte
	go func() {
		for e := range s2.Events() {
			data := e.(*pubsub.Message)
			recv2 = data.Data
		}
	}()

	// send message from host2
	msg = []byte("world")
	asrt.NoError(host1.chatRoom.Publish(msg))

	time.Sleep(10 * time.Millisecond)
	asrt.Equal(msg, recv2)

	host1.Close()
	host2.Close()
}
