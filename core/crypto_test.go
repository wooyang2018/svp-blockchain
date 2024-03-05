// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/kyber/v3/pairing/bn256"
	"go.dedis.ch/kyber/v3/sign/bls"
	"go.dedis.ch/kyber/v3/util/random"
)

func TestSignVerify(t *testing.T) {
	asrt := assert.New(t)
	privKey := GenerateKey(nil)
	asrt.Equal(privKey.PublicKey(), privKey.PublicKey())
	msg := []byte("message to be signed")
	sig := privKey.Sign(msg)
	asrt.NotNil(sig)
	asrt.True(sig.Verify(msg))
	asrt.False(sig.Verify([]byte("tampered message")))
	asrt.Equal(privKey.PublicKey(), sig.PublicKey())
}

func TestBLSBatchVerify(t *testing.T) {
	msg1 := []byte("Hello Boneh-Lynn-Shacham")
	msg2 := []byte("Hello Dedis & Boneh-Lynn-Shacham")
	suite := bn256.NewSuite()
	private1, public1 := bls.NewKeyPair(suite, random.New())
	private2, public2 := bls.NewKeyPair(suite, random.New())
	sig1, err := bls.Sign(suite, private1, msg1)
	require.Nil(t, err)
	sig2, err := bls.Sign(suite, private2, msg2)
	require.Nil(t, err)
	aggregatedSig, err := bls.AggregateSignatures(suite, sig1, sig2)
	require.Nil(t, err)
	err = bls.BatchVerify(suite, []kyber.Point{public1, public2}, [][]byte{msg1, msg2}, aggregatedSig)
	require.Nil(t, err)
}

func TestBLSAggregateSignatures(t *testing.T) {
	msg := []byte("Hello Boneh-Lynn-Shacham")
	suite := bn256.NewSuite()
	private1, public1 := bls.NewKeyPair(suite, random.New())
	private2, public2 := bls.NewKeyPair(suite, random.New())
	sig1, err := bls.Sign(suite, private1, msg)
	require.Nil(t, err)
	sig2, err := bls.Sign(suite, private2, msg)
	require.Nil(t, err)
	aggregatedSig, err := bls.AggregateSignatures(suite, sig1, sig2)
	require.Nil(t, err)
	aggregatedKey := bls.AggregatePublicKeys(suite, public1, public2)
	err = bls.Verify(suite, aggregatedKey, msg, aggregatedSig)
	require.Nil(t, err)
}

func BenchmarkBLSAggregateSigs(b *testing.B) {
	suite := bn256.NewSuite()
	private1, _ := bls.NewKeyPair(suite, random.New())
	private2, _ := bls.NewKeyPair(suite, random.New())
	msg := []byte("Hello many times Boneh-Lynn-Shacham")
	sig1, err := bls.Sign(suite, private1, msg)
	require.Nil(b, err)
	sig2, err := bls.Sign(suite, private2, msg)
	require.Nil(b, err)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bls.AggregateSignatures(suite, sig1, sig2)
	}
}

func BenchmarkBLSVerifyAggregate(b *testing.B) {
	suite := bn256.NewSuite()
	private1, public1 := bls.NewKeyPair(suite, random.New())
	private2, public2 := bls.NewKeyPair(suite, random.New())
	msg := []byte("Hello many times Boneh-Lynn-Shacham")
	sig1, err := bls.Sign(suite, private1, msg)
	require.Nil(b, err)
	sig2, err := bls.Sign(suite, private2, msg)
	require.Nil(b, err)
	sig, err := bls.AggregateSignatures(suite, sig1, sig2)
	key := bls.AggregatePublicKeys(suite, public1, public2)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bls.Verify(suite, key, msg, sig)
	}
}
