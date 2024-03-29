// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package p2p

import (
	"context"
	"errors"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/multiformats/go-multiaddr"

	"github.com/wooyang2018/svp-blockchain/core"
)

const protocolID = "/single_pid"

type Host struct {
	privKey   *core.PrivateKey
	localAddr multiaddr.Multiaddr
	peerStore *PeerStore
	libHost   host.Host
}

func NewHost(privKey *core.PrivateKey, localAddr multiaddr.Multiaddr) (*Host, error) {
	host := new(Host)
	host.privKey = privKey
	host.localAddr = localAddr
	host.peerStore = NewPeerStore()

	libHost, err := host.newLibHost()
	if err != nil {
		return nil, err
	}
	host.libHost = libHost
	host.libHost.SetStreamHandler(protocolID, host.handleStream)
	return host, nil
}

func (host *Host) newLibHost() (host.Host, error) {
	priv, err := crypto.UnmarshalEd25519PrivateKey(host.privKey.Bytes())
	if err != nil {
		return nil, err
	}
	return libp2p.New(
		libp2p.Identity(priv),
		libp2p.ListenAddrs(host.localAddr),
	)
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

func (host *Host) connectPeer(peer *Peer) {
	// prevent simultaneous connections from both hosts
	if err := peer.setConnecting(); err != nil {
		return
	}
	s, err := host.newStream(peer)
	if err != nil {
		peer.disconnect()
		return
	}
	peer.onConnected(s)
}

func (host *Host) newStream(peer *Peer) (network.Stream, error) {
	id, err := getIDFromPublicKey(peer.PublicKey())
	if err != nil {
		return nil, err
	}
	host.libHost.Peerstore().AddAddr(id, peer.Addr(), peerstore.PermanentAddrTTL)
	return host.libHost.NewStream(context.Background(), id, protocolID)
}

func (host *Host) AddPeer(peer *Peer) {
	peer.host = host
	peer, _ = host.peerStore.LoadOrStore(peer)
	go host.connectPeer(peer)
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
