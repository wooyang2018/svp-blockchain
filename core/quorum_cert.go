// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package core

import (
	"encoding/binary"
	"errors"
	"math"

	"github.com/wooyang2018/posv-blockchain/pb"
	"google.golang.org/protobuf/proto"
)

// errors
var (
	ErrNilQC            = errors.New("nil qc")
	ErrNilSig           = errors.New("nil signature")
	ErrNotEnoughSig     = errors.New("not enough signatures in qc")
	ErrDuplicateSig     = errors.New("duplicate signature in qc")
	ErrInvalidSig       = errors.New("invalid signature")
	ErrInvalidValidator = errors.New("invalid validator")
)

// QuorumCert type
type QuorumCert struct {
	data      *pb.QuorumCert
	sigs      sigList
	signature *Signature
}

func NewQuorumCert() *QuorumCert {
	return &QuorumCert{
		data: new(pb.QuorumCert),
	}
}

func (qc *QuorumCert) Validate(rs RoleStore) error {
	if qc.data == nil {
		return ErrNilQC
	}
	if len(qc.sigs) < rs.MajorityValidatorCount() {
		return ErrNotEnoughSig
	}
	if qc.sigs.hasDuplicate() {
		return ErrDuplicateSig
	}
	if qc.sigs.hasInvalidValidator(rs) {
		return ErrInvalidValidator
	}
	if qc.sigs.hasInvalidSig(appendUint32(qc.data.BlockHash, qc.data.View)) {
		return ErrInvalidSig
	}
	sig, err := newSignature(qc.data.Signature)
	if err != nil {
		return err
	}
	if !rs.IsValidator(sig.PublicKey()) {
		return ErrInvalidValidator
	}
	if !sig.Verify(appendFloat64(appendUint32(qc.data.BlockHash, qc.data.View), qc.data.Quota)) {
		return ErrInvalidSig
	}
	return nil
}

func (qc *QuorumCert) setData(data *pb.QuorumCert) error {
	qc.data = data
	signature, err := newSignature(qc.data.Signature)
	if err != nil {
		return err
	}
	qc.signature = signature

	sigs, err := newSigList(qc.data.Signatures)
	if err != nil {
		return err
	}
	qc.sigs = sigs
	return nil
}

func (qc *QuorumCert) Build(signer Signer, votes []*Vote) *QuorumCert {
	qc.data.Signatures = make([]*pb.Signature, len(votes))
	qc.sigs = make(sigList, len(votes))
	for i, vote := range votes {
		if qc.data.BlockHash == nil {
			qc.data.BlockHash = vote.data.BlockHash
			qc.data.View = vote.data.View
		}
		qc.data.Quota += vote.data.Quota
		qc.data.Signatures[i] = vote.data.Signature
		qc.sigs[i] = &Signature{
			data:   vote.data.Signature,
			pubKey: vote.voter.pubKey,
		}
	}
	qc.signature = signer.Sign(appendFloat64(appendUint32(qc.data.BlockHash, qc.data.View), qc.data.Quota))
	qc.data.Signature = qc.signature.data
	return qc
}

func (qc *QuorumCert) View() uint32             { return qc.data.View }
func (qc *QuorumCert) Quota() float64           { return qc.data.Quota }
func (qc *QuorumCert) BlockHash() []byte        { return qc.data.BlockHash }
func (qc *QuorumCert) Signatures() []*Signature { return qc.sigs }
func (qc *QuorumCert) Proposer() *PublicKey     { return qc.signature.pubKey }

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

func appendUint32(hash []byte, view uint32) []byte {
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, view)
	hash = append(hash, buf...)
	return hash
}

func appendFloat64(hash []byte, quota float64) []byte {
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf, math.Float64bits(quota))
	hash = append(hash, buf...)
	return hash
}
