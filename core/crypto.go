// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package core

import (
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"io"

	"github.com/wooyang2018/svp-blockchain/pb"
)

var ErrInvalidKeySize = errors.New("invalid key size")

type Signer interface {
	Sign(msg []byte) *Signature
	PublicKey() *PublicKey
}

// PublicKey type
type PublicKey struct {
	key    ed25519.PublicKey
	keyStr string
}

// NewPublicKey creates PublicKey from bytes
func NewPublicKey(b []byte) (*PublicKey, error) {
	if len(b) != ed25519.PublicKeySize {
		return nil, ErrInvalidKeySize
	}
	return &PublicKey{
		key:    b,
		keyStr: base64.StdEncoding.EncodeToString(b),
	}, nil
}

// Equal checks whether pub and x has the same value
func (pub *PublicKey) Equal(x *PublicKey) bool {
	return pub.key.Equal(x.key)
}

// Bytes return raw bytes
func (pub *PublicKey) Bytes() []byte {
	return pub.key
}

func (pub *PublicKey) String() string {
	return pub.keyStr
}

// PrivateKey type
type PrivateKey struct {
	key    ed25519.PrivateKey
	pubKey *PublicKey
}

var _ Signer = (*PrivateKey)(nil)

// NewPrivateKey creates PrivateKey from bytes
func NewPrivateKey(b []byte) (*PrivateKey, error) {
	if len(b) != ed25519.PrivateKeySize {
		return nil, ErrInvalidKeySize
	}
	priv := &PrivateKey{
		key: b,
	}
	priv.pubKey, _ = NewPublicKey(priv.key.Public().(ed25519.PublicKey))
	return priv, nil
}

// Bytes return raw bytes
func (priv *PrivateKey) Bytes() []byte {
	return priv.key
}

// PublicKey returns corresponding public key
func (priv *PrivateKey) PublicKey() *PublicKey {
	return priv.pubKey
}

// Sign signs the message
func (priv *PrivateKey) Sign(msg []byte) *Signature {
	return &Signature{
		data: &pb.Signature{
			Value:  ed25519.Sign(priv.key, msg),
			PubKey: priv.pubKey.Bytes(),
		},
		pubKey: priv.pubKey,
	}
}

func GenerateKey(rand io.Reader) *PrivateKey {
	_, priv, _ := ed25519.GenerateKey(rand)
	privKey, _ := NewPrivateKey(priv)
	return privKey
}

// Signature type
type Signature struct {
	data   *pb.Signature
	pubKey *PublicKey
}

func newSignature(data *pb.Signature) (*Signature, error) {
	if data == nil {
		return nil, ErrNilSig
	}
	pubKey, err := NewPublicKey(data.PubKey)
	if err != nil {
		return nil, err
	}
	return &Signature{data, pubKey}, nil
}

// Verify verifies the signature
func (sig *Signature) Verify(msg []byte) bool {
	return ed25519.Verify(sig.pubKey.key, msg, sig.data.Value)
}

// PublicKey returns corresponding public key
func (sig *Signature) PublicKey() *PublicKey {
	return sig.pubKey
}

type sigList []*Signature

func newSigList(pbsigs []*pb.Signature) (sigList, error) {
	sigs := make([]*Signature, len(pbsigs))
	for i, data := range pbsigs {
		sig, err := newSignature(data)
		if err != nil {
			return nil, err
		}
		sigs[i] = sig
	}
	return sigs, nil
}

func (sigs sigList) hasInvalidValidator(rs RoleStore) bool {
	for _, sig := range sigs {
		if !rs.IsValidator(sig.PublicKey().String()) {
			return true
		}
	}
	return false
}
