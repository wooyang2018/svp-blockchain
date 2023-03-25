// Copyright (C) 2021 Aung Maw
// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package core

import (
	"errors"

	"github.com/wooyang2018/ppov-blockchain/pb"
	"google.golang.org/protobuf/proto"
)

// errors
var (
	ErrNilQC            = errors.New("nil qc")
	ErrNilSig           = errors.New("nil signature")
	ErrNotEnoughSig     = errors.New("not enough signatures in qc")
	ErrDuplicateSig     = errors.New("duplicate signature in qc")
	ErrInvalidSig       = errors.New("invalid signature")
	ErrInvalidValidator = errors.New("voter is not a validator")
)

// QuorumCert type
type QuorumCert struct {
	data *pb.QuorumCert
	sigs sigList
}

func NewQuorumCert() *QuorumCert {
	return &QuorumCert{
		data: new(pb.QuorumCert),
	}
}

func (qc *QuorumCert) Validate(vs ValidatorStore) error {
	if qc.data == nil {
		return ErrNilQC
	}
	if len(qc.sigs) < vs.MajorityValidatorCount() {
		return ErrNotEnoughSig
	}
	if qc.sigs.hasDuplicate() {
		return ErrDuplicateSig
	}
	if qc.sigs.hasInvalidValidator(vs) {
		return ErrInvalidValidator
	}
	if qc.sigs.hasInvalidSig(qc.data.BlockHash) {
		return ErrInvalidSig
	}
	return nil
}

func (qc *QuorumCert) setData(data *pb.QuorumCert) error {
	qc.data = data
	sigs, err := newSigList(qc.data.Signatures)
	if err != nil {
		return err
	}
	qc.sigs = sigs
	return nil
}

func (qc *QuorumCert) Build(votes []*Vote) *QuorumCert {
	qc.data.Signatures = make([]*pb.Signature, len(votes))
	qc.sigs = make(sigList, len(votes))
	for i, vote := range votes {
		if qc.data.BlockHash == nil {
			qc.data.BlockHash = vote.data.BlockHash
		}
		qc.data.Signatures[i] = vote.data.Signature
		qc.sigs[i] = &Signature{
			data:   vote.data.Signature,
			pubKey: vote.voter,
		}
	}
	return qc
}

func (qc *QuorumCert) BlockHash() []byte        { return qc.data.BlockHash }
func (qc *QuorumCert) Signatures() []*Signature { return qc.sigs }

// Marshal encodes quorum cert as bytes
func (qc *QuorumCert) Marshal() ([]byte, error) {
	return proto.Marshal(qc.data)
}

// Unmarshal decodes quorum cert from bytes
func (qc *QuorumCert) Unmarshal(b []byte) error {
	data := new(pb.QuorumCert)
	if err := proto.Unmarshal(b, data); err != nil {
		return err
	}
	return qc.setData(data)
}
