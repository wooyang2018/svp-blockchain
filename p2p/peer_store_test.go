// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package p2p

import (
	"bytes"
	"crypto/ed25519"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wooyang2018/svp-blockchain/core"
)

func TestPeerStore(t *testing.T) {
	asrt := assert.New(t)
	s := NewPeerStore()

	// load or store
	pubKey, _ := core.NewPublicKey(bytes.Repeat([]byte{1}, ed25519.PublicKeySize))
	p := NewPeer(pubKey, nil)
	actual, loaded := s.LoadOrStore(p)
	asrt.False(loaded)
	asrt.Equal(p, actual)

	p1 := NewPeer(pubKey, nil)

	actual, loaded = s.LoadOrStore(p1)
	asrt.True(loaded)
	asrt.Equal(p, actual)

	// load
	asrt.Equal(p, s.Load(pubKey))

	// store
	asrt.Equal(p1, s.Store(p1))
	asrt.Equal(p1, s.Load(pubKey))

	// list
	asrt.Equal([]*Peer{p1}, s.List())

	// delete
	asrt.Equal(p1, s.Delete(pubKey))
	asrt.Nil(s.Load(pubKey))
	asrt.Equal([]*Peer{}, s.List())
}
