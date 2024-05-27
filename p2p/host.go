// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package p2p

import (
	"context"
	"errors"
	"time"

	"github.com/libp2p/go-libp2p"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/libp2p/go-libp2p/p2p/net/connmgr"
	"github.com/multiformats/go-multiaddr"

	"github.com/wooyang2018/svp-blockchain/core"
	"github.com/wooyang2018/svp-blockchain/emitter"
	"github.com/wooyang2018/svp-blockchain/logger"
)

const protocolID = "/point2point"
const low, high = 2, 4 // low and high watermark

type Host struct {
	privKey   *core.PrivateKey
	peerStore *PeerStore
	peers     []*Peer

	pointAddr  multiaddr.Multiaddr
	pointHost  host.Host
	consLeader *Peer

	topicAddr multiaddr.Multiaddr
	topicHost host.Host
	chatRoom  *ChatRoom
}

func NewHost(privKey *core.PrivateKey, pointAddr, topicAddr multiaddr.Multiaddr) (*Host, error) {
	host := new(Host)
	host.privKey = privKey
	host.pointAddr = pointAddr
	host.topicAddr = topicAddr
	host.peerStore = NewPeerStore()

	pointHost, topicHost, err := host.newLibHost()
	if err != nil {
		return nil, err
	}
	pointHost.SetStreamHandler(protocolID, host.handleStream)
	host.pointHost = pointHost
	host.topicHost = topicHost

	return host, nil
}

func (host *Host) newLibHost() (host.Host, host.Host, error) {
	priv, err := crypto.UnmarshalEd25519PrivateKey(host.privKey.Bytes())
	pointHost, err := libp2p.New(
		libp2p.Identity(priv),
		libp2p.ListenAddrs(host.pointAddr),
	)
	if err != nil {
		return nil, nil, err
	}

	connmgr, err := connmgr.NewConnManager(low, high,
		connmgr.WithGracePeriod(5*time.Second),
	)
	topicHost, err := libp2p.New(
		libp2p.Identity(priv),
		libp2p.ListenAddrs(host.topicAddr),
		libp2p.ConnectionManager(connmgr),
	)
	return pointHost, topicHost, nil
}

func (host *Host) handleStream(s network.Stream) {
	pubKey, err := getRemotePublicKey(s)
	if err != nil {
		return
	}
	if peer := host.peerStore.Load(pubKey); peer != nil {
		if err = peer.setConnecting(); err == nil {
			peer.onConnected(s)
			return
		}
	}
	s.Close() // cannot find peer in the store (peer not allowed to connect)
}

func (host *Host) JoinChatRoom() error {
	ctx := context.Background()
	// create a new PubSub service using the GossipSub router
	ps, err := pubsub.NewGossipSub(ctx, host.topicHost)
	if err != nil {
		return err
	}
	// setup local mDNS discovery
	if err = setupDiscovery(host.topicHost, host.peerStore); err != nil {
		return err
	}
	// join the chatroom
	if host.chatRoom, err = JoinChatRoom(ctx, ps, host.topicHost.ID()); err != nil {
		return err
	}
	return nil
}

func (host *Host) Close() {
	if err := host.pointHost.Close(); err != nil {
		logger.I().Error(err)
	}
	if err := host.topicHost.Close(); err != nil {
		logger.I().Error(err)
	}
}

func (host *Host) SetPeers(peers []*Peer) {
	host.peers = peers
}

func (host *Host) SetLeader(idx int) {
	host.consLeader = host.peers[idx]
	if !host.consLeader.pubKey.Equal(host.privKey.PublicKey()) {
		host.ConnectLeader()
	}
}

func (host *Host) ConnectLeader() {
	leader := host.consLeader
	// prevent simultaneous connections from both hosts
	if err := leader.setConnecting(); err != nil {
		logger.I().Error(err)
		return
	}
	s, err := host.newStream(leader)
	if err != nil {
		leader.disconnect()
		logger.I().Errorw("failed to reconnect leader", "error", err)
		return
	}
	leader.onConnected(s)
	return
}

func (host *Host) SubscribeMsg() *emitter.Subscription {
	return host.chatRoom.emitter.Subscribe(20)
}

func (host *Host) newStream(peer *Peer) (network.Stream, error) {
	logger.I().Debugw("newing stream to peer", "pubkey", peer.PublicKey())
	id, err := getIDFromPublicKey(peer.PublicKey())
	if err != nil {
		return nil, err
	}
	host.pointHost.Peerstore().AddAddr(id, peer.PointAddr(), peerstore.PermanentAddrTTL)
	return host.pointHost.NewStream(context.Background(), id, protocolID)
}

func (host *Host) AddPeer(peer *Peer) {
	host.peerStore.Store(peer)
	peer.host = host
}

func (host *Host) PeerStore() *PeerStore {
	return host.peerStore
}

func getRemotePublicKey(s network.Stream) (*core.PublicKey, error) {
	if _, ok := s.Conn().RemotePublicKey().(*crypto.Ed25519PublicKey); !ok {
		return nil, errors.New("invalid pubKey type")
	}
	b, err := s.Conn().RemotePublicKey().Raw()
	if err != nil {
		return nil, err
	}
	return core.NewPublicKey(b)
}

func getIDFromPublicKey(pubKey *core.PublicKey) (peer.ID, error) {
	var id peer.ID
	if pubKey == nil {
		return id, errors.New("nil peer pubkey")
	}
	key, err := crypto.UnmarshalEd25519PublicKey(pubKey.Bytes())
	if err != nil {
		return id, err
	}
	return peer.IDFromPublicKey(key)
}
