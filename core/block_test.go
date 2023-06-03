// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestBlock(t *testing.T) {
	asrt := assert.New(t)
	privKey := GenerateKey(nil)
	blk := NewBlock().
		SetHeight(4).
		SetParentHash([]byte{1}).
		SetExecHeight(0).
		SetMerkleRoot([]byte{1}).
		SetTransactions([][]byte{{1}}).
		Sign(privKey)

	asrt.Equal(uint64(4), blk.Height())
	asrt.Equal([]byte{1}, blk.ParentHash())
	asrt.Equal(privKey.PublicKey().Bytes(), blk.data.Signature.PubKey)
	asrt.Equal(uint64(0), blk.ExecHeight())
	asrt.Equal([]byte{1}, blk.MerkleRoot())
	asrt.Equal([][]byte{{1}}, blk.Transactions())

	vs := new(MockValidatorStore)
	vs.On("IsVoter", privKey.PublicKey()).Return(true)
	vs.On("IsVoter", mock.Anything).Return(false)
	vs.On("IsWorker", privKey.PublicKey()).Return(true)
	vs.On("IsWorker", mock.Anything).Return(false)

	bOk, err := blk.Marshal()
	asrt.NoError(err)

	privKey = GenerateKey(nil)
	bInvalidValidator, _ := blk.Sign(privKey).Marshal()

	blk.data.Hash = []byte("invalid hash")
	bInvalidHash, _ := blk.Marshal()

	// test validate
	tests := []struct {
		name    string
		data    []byte
		wantErr bool
	}{
		{"valid", bOk, false},
		{"invalid validator", bInvalidValidator, true},
		{"invalid hash", bInvalidHash, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			asrt := assert.New(t)

			blk := NewBlock()
			err := blk.Unmarshal(tt.data)
			asrt.NoError(err)

			err = blk.Validate(vs)
			if tt.wantErr {
				asrt.Error(err)
			} else {
				asrt.NoError(err)
			}
		})
	}
}
