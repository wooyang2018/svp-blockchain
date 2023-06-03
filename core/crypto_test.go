// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
