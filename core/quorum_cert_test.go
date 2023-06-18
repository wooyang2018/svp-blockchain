// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
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

	votes := make([]*Vote, len(privKeys))
	pro := newProposal(privKeys[1])
	for i, priv := range privKeys {
		votes[i] = pro.Vote(priv)
	}

	qc := NewQuorumCert().Build(privKeys[1], []*Vote{votes[3], votes[2], votes[1]})
	qcValid, err := qc.Marshal()
	asrt.NoError(err)

	qc = NewQuorumCert().Build(privKeys[1], []*Vote{votes[5], votes[4], votes[3], votes[2], votes[1]})
	qcValidFull, _ := qc.Marshal()

	qc = NewQuorumCert().Build(privKeys[1], []*Vote{votes[2], votes[1]})
	qcNotEnoughSig, _ := qc.Marshal()

	qc = NewQuorumCert().Build(privKeys[1], []*Vote{votes[3], votes[3], votes[2], votes[1]})
	qcDuplicateKey, _ := qc.Marshal()

	qc = NewQuorumCert().Build(privKeys[1], []*Vote{votes[3], votes[2], votes[1], votes[0]})
	qcInvalidValidator, _ := qc.Marshal()

	// test validate
	tests := []struct {
		name         string
		b            []byte
		unmarshalErr bool
		validateErr  bool
	}{
		{"valid", qcValid, false, false},
		{"valid full", qcValidFull, false, false},
		{"nil qc", nil, true, true},
		{"not enough sig", qcNotEnoughSig, false, true},
		{"duplicate key", qcDuplicateKey, false, true},
		{"invalid validator", qcInvalidValidator, false, true},
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
