// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package node

import (
	"encoding/json"
	"fmt"
	"os"
	"path"

	"github.com/multiformats/go-multiaddr"

	"github.com/wooyang2018/svp-blockchain/core"
	"github.com/wooyang2018/svp-blockchain/p2p"
)

const (
	NodekeyFile = "nodekey"
	GenesisFile = "genesis.json"
	PeersFile   = "peers.json"
)

type Peer struct {
	PubKey    []byte
	PointAddr string
	TopicAddr string
}

type Genesis struct {
	Validators  []string
	StakeQuotas []uint64
	WindowSize  int
}

func ReadNodeKey(datadir string) (*core.PrivateKey, error) {
	b, err := os.ReadFile(path.Join(datadir, NodekeyFile))
	if err != nil {
		return nil, fmt.Errorf("cannot read %s, %w", NodekeyFile, err)
	}
	return core.NewPrivateKey(b)
}

func ReadGenesis(datadir string) (*Genesis, error) {
	f, err := os.Open(path.Join(datadir, GenesisFile))
	if err != nil {
		return nil, fmt.Errorf("cannot read %s, %w", GenesisFile, err)
	}
	defer f.Close()

	genesis := new(Genesis)
	if err := json.NewDecoder(f).Decode(&genesis); err != nil {
		return nil, fmt.Errorf("cannot parse %s, %w", GenesisFile, err)
	}

	return genesis, nil
}

func ReadPeers(datadir string) ([]*p2p.Peer, error) {
	f, err := os.Open(path.Join(datadir, PeersFile))
	if err != nil {
		return nil, fmt.Errorf("cannot read %s, %w", PeersFile, err)
	}
	defer f.Close()

	var raws []Peer
	if err = json.NewDecoder(f).Decode(&raws); err != nil {
		return nil, fmt.Errorf("cannot parse %s, %w", PeersFile, err)
	}

	peers := make([]*p2p.Peer, len(raws))

	for i, r := range raws {
		pubKey, err := core.NewPublicKey(r.PubKey)
		if err != nil {
			return nil, fmt.Errorf("invalid public key, %w", err)
		}
		pointAddr, err := multiaddr.NewMultiaddr(r.PointAddr)
		topicAddr, err := multiaddr.NewMultiaddr(r.TopicAddr)
		if err != nil {
			return nil, fmt.Errorf("invalid multiaddr, %w", err)
		}
		peers[i] = p2p.NewPeer(pubKey, pointAddr, topicAddr)
	}
	return peers, nil
}

func WriteNodeKey(datadir string, key *core.PrivateKey) error {
	f, err := os.Create(path.Join(datadir, NodekeyFile))
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(key.Bytes())
	return err
}

func WriteGenesisFile(datadir string, genesis *Genesis) error {
	f, err := os.Create(path.Join(datadir, GenesisFile))
	if err != nil {
		return err
	}
	defer f.Close()
	e := json.NewEncoder(f)
	e.SetIndent("", "  ")
	return e.Encode(genesis)
}

func WritePeersFile(datadir string, peers []Peer) error {
	f, err := os.Create(path.Join(datadir, PeersFile))
	if err != nil {
		return err
	}
	defer f.Close()
	e := json.NewEncoder(f)
	e.SetIndent("", "  ")
	return e.Encode(peers)
}
