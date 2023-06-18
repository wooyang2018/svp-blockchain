// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func newProposal(privKey *PrivateKey) *Proposal {
	pro := NewProposal().
		SetBlock(newBlock(privKey)).
		SetView(1).
		Sign(privKey)
	return pro
}

func TestProposal(t *testing.T) {
	asrt := assert.New(t)
	privKey := GenerateKey(nil)

	pro := newProposal(privKey)
	v := pro.Vote(privKey)
	qc := NewQuorumCert().Build(privKey, []*Vote{v})
	pro.SetQuorumCert(qc).Sign(privKey)

	asrt.Equal(privKey.PublicKey(), pro.Proposer())
	asrt.Equal(qc, pro.QuorumCert())

	vs := new(MockValidatorStore)
	vs.On("MajorityValidatorCount").Return(1)
	vs.On("IsValidator", privKey.PublicKey()).Return(true)
	vs.On("IsValidator", mock.Anything).Return(false)

	bOk, err := pro.Marshal()
	asrt.NoError(err)

	privKey1 := GenerateKey(nil)
	bInvalidValidator, _ := pro.Sign(privKey1).Marshal()

	pro.data.Hash = []byte("invalid hash")
	bInvalidHash, _ := pro.Marshal()

	// test validate
	tests := []struct {
		name    string
		b       []byte
		wantErr bool
	}{
		{"valid", bOk, false},
		{"invalid validator", bInvalidValidator, true},
		{"invalid hash", bInvalidHash, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			asrt := assert.New(t)

			pro := NewProposal()
			err := pro.Unmarshal(tt.b)
			asrt.NoError(err)

			err = pro.Validate(vs)

			if tt.wantErr {
				asrt.Error(err)
			} else {
				asrt.NoError(err)
			}
		})
	}
}

func TestProposal_Vote(t *testing.T) {
	asrt := assert.New(t)
	privKey := GenerateKey(nil)

	pro := newProposal(privKey)
	vote := pro.Vote(privKey)
	asrt.Equal(pro.Block().Hash(), vote.BlockHash())

	vs := new(MockValidatorStore)
	vs.On("IsValidator", privKey.PublicKey()).Return(true)

	err := vote.Validate(vs)
	asrt.NoError(err)
	vs.AssertExpectations(t)
}
