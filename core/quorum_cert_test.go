// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/wooyang2018/posv-blockchain/pb"
)

func TestQuorumCert(t *testing.T) {
	asrt := assert.New(t)
	privKeys := make([]*PrivateKey, 6)

	vs := new(MockValidatorStore)
	vs.On("ValidatorCount").Return(5)
	vs.On("MajorityValidatorCount").Return(3)

	for i := range privKeys {
		privKeys[i] = GenerateKey(nil)
		if i > 0 {
			vs.On("IsValidator", privKeys[i].pubKey).Return(true)
		}
	}
	vs.On("IsValidator", mock.Anything).Return(false)

	blockHash := []byte{1}
	votes := make([]*Vote, len(privKeys))
	for i, priv := range privKeys {
		vote := NewVote()
		vote.setData(&pb.Vote{
			BlockHash: blockHash,
			Signature: priv.Sign(blockHash).data,
		})
		votes[i] = vote
	}

	invalidSigVote := NewVote()
	invalidSigVote.setData(&pb.Vote{
		BlockHash: blockHash,
		Signature: privKeys[1].Sign([]byte("wrong data")).data,
	})

	qc := NewQuorumCert().Build([]*Vote{votes[3], votes[2], votes[1]})
	qcValid, err := qc.Marshal()
	asrt.NoError(err)

	qc = NewQuorumCert().Build([]*Vote{votes[5], votes[4], votes[3], votes[2], votes[1]})
	qcValidFull, _ := qc.Marshal()

	qc = NewQuorumCert().Build([]*Vote{votes[2], votes[1]})
	qcNotEnoughSig, _ := qc.Marshal()

	qc = NewQuorumCert().Build([]*Vote{votes[3], votes[3], votes[2], votes[1]})
	qcDuplicateKey, _ := qc.Marshal()

	qc = NewQuorumCert().Build([]*Vote{votes[3], votes[2], votes[1], votes[0]})
	qcInvalidValidator, _ := qc.Marshal()

	qc = NewQuorumCert().Build([]*Vote{votes[3], votes[2], votes[1], invalidSigVote})
	qcInvalidSig, _ := qc.Marshal()

	// test validate
	tests := []struct {
		name         string
		b            []byte
		unmarshalErr bool
		validateErr  bool
	}{
		{"valid", qcValid, false, false},
		{"valid full", qcValidFull, false, false},
		{"nil qc", nil, false, true},
		{"not enough sig", qcNotEnoughSig, false, true},
		{"duplicate key", qcDuplicateKey, false, true},
		{"invalid validator", qcInvalidValidator, false, true},
		{"invalid sig", qcInvalidSig, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			asrt := assert.New(t)

			qc := NewQuorumCert()
			err := qc.Unmarshal(tt.b)
			if tt.unmarshalErr {
				asrt.Error(err)
				return
			}
			asrt.NoError(err)
			err = qc.Validate(vs)
			if tt.validateErr {
				asrt.Error(err)
			} else {
				asrt.NoError(err)
			}
		})
	}
}
