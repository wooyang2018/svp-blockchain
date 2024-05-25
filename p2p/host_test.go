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

	priv1 := core.GenerateKey(nil)
	priv2 := core.GenerateKey(nil)

	pointAddr1, _ := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/25001")
	pointAddr2, _ := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/25002")
	topicAddr1, _ := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/26001")
	topicAddr2, _ := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/26002")

	peer1 := NewPeer(priv1.PublicKey(), pointAddr1, topicAddr1)
	peer2 := NewPeer(priv2.PublicKey(), pointAddr2, topicAddr2)

	host1, err := NewHost(priv1, pointAddr1, topicAddr1)
	asrt.NoError(err)
	host2, err := NewHost(priv2, pointAddr2, topicAddr2)
	asrt.NoError(err)

	host1.AddPeer(peer2)
	host2.AddPeer(peer1)

	asrt.NoError(host1.ConnectPeer(peer2))

	asrt.NoError(host1.JoinChatRoom())
	asrt.NoError(host2.JoinChatRoom())

	time.Sleep(2 * time.Second)
	asrt.Equal(PeerStatusConnected, peer1.Status())
	asrt.Equal(PeerStatusConnected, peer2.Status())

	return host1, host2, peer1, peer2
}

func TestPointHost(t *testing.T) {
	asrt := assert.New(t)
	_, _, peer1, peer2 := setupTwoHost(t)

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
}

func TestAddPeer(t *testing.T) {
	asrt := assert.New(t)
	host1, host2, _, peer2 := setupTwoHost(t)

	priv3 := core.GenerateKey(nil)
	peer3 := NewPeer(priv3.PublicKey(), peer2.pointAddr, peer2.topicAddr)
	host1.AddPeer(peer3) // invalid key
	asrt.Error(host1.ConnectPeer(peer3))
	asrt.Equal(PeerStatusDisconnected, peer3.Status())

	pointAddr3, _ := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/25003")
	topicAddr3, _ := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/26003")
	peer3 = NewPeer(priv3.PublicKey(), pointAddr3, topicAddr3)
	host2.AddPeer(peer3) // not reachable host
	asrt.Error(host2.ConnectPeer(peer3))
	asrt.Equal(PeerStatusDisconnected, peer3.Status())
}
