// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/wooyang2018/posv-blockchain/pb"
)

func TestVote(t *testing.T) {
	vote := &Vote{data: &pb.Vote{}}
	vNilSig, err := vote.Marshal()
	assert.NoError(t, err)

	proposer := GenerateKey(nil)
	validator := GenerateKey(nil)

	pro := newProposal(proposer)
	vote = pro.Vote(validator, 1)
	vOk, _ := vote.Marshal()

	vote.data.BlockHash = []byte("invalid hash")
	vInvalid, _ := vote.Marshal()

	// test validate
	tests := []struct {
		name         string
		b            []byte
		unmarshalErr bool
		validateErr  bool
	}{
		{"valid", vOk, false, false},
		{"nil vote", nil, true, true},
		{"nil signature", vNilSig, true, true},
		{"invalid", vInvalid, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			asrt := assert.New(t)

			vote := NewVote()
			err := vote.Unmarshal(tt.b)
			if tt.unmarshalErr {
				asrt.Error(err)
				return
			}
			asrt.NoError(err)

			vs := new(MockValidatorStore)
			vs.On("IsValidator", mock.Anything).Return(true)

			err = vote.Validate(vs)
			if tt.validateErr {
				asrt.Error(err)
			} else {
				asrt.NoError(err)
			}
		})
	}
}
