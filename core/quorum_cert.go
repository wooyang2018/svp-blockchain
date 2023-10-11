// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package core

import (
	"encoding/binary"
	"errors"

	"google.golang.org/protobuf/proto"

	"github.com/wooyang2018/svp-blockchain/pb"
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
	quota     uint32
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
	dmap := make(map[string]struct{}, len(qc.sigs))
	for i, sig := range qc.sigs {
		key := sig.PublicKey().String()
		if _, found := dmap[key]; found {
			return ErrDuplicateSig
		}
		dmap[key] = struct{}{}
		if !rs.IsValidator(sig.PublicKey()) {
			return ErrInvalidValidator
		}
		if !sig.Verify(appendUint32(qc.data.BlockHash, qc.data.View, qc.data.Quotas[i])) {
			return ErrInvalidSig
		}
	}
	sig, err := newSignature(qc.data.Signature)
	if err != nil {
		return err
	}
	if !rs.IsValidator(sig.PublicKey()) {
		return ErrInvalidValidator
	}
	if !sig.Verify(appendUint32(qc.data.BlockHash, qc.data.View)) {
		return ErrInvalidSig
	}
	return nil
}

func (qc *QuorumCert) setData(data *pb.QuorumCert) error {
	qc.data = data
	qc.quota = 0
	for _, v := range qc.data.Quotas {
		qc.quota += v
	}
	sigs, err := newSigList(qc.data.Signatures)
	if err != nil {
		return err
	}
	qc.sigs = sigs
	signature, err := newSignature(qc.data.Signature)
	if err != nil {
		return err
	}
	qc.signature = signature
	return nil
}

func (qc *QuorumCert) Build(signer Signer, votes []*Vote) *QuorumCert {
	qc.data.Quotas = make([]uint32, len(votes))
	qc.data.Signatures = make([]*pb.Signature, len(votes))
	qc.sigs = make(sigList, len(votes))
	qc.quota = 0
	for i, vote := range votes {
		if qc.data.BlockHash == nil {
			qc.data.View = vote.data.View
			qc.data.BlockHash = vote.data.BlockHash
		}
		qc.quota += vote.data.Quota
		qc.data.Quotas[i] = vote.data.Quota
		qc.data.Signatures[i] = vote.data.Signature
		qc.sigs[i] = &Signature{
			data:   vote.data.Signature,
			pubKey: vote.voter.pubKey,
		}
	}
	qc.signature = signer.Sign(appendUint32(qc.data.BlockHash, qc.data.View))
	qc.data.Signature = qc.signature.data
	return qc
}

func (qc *QuorumCert) FindVote(signer Signer) uint32 {
	dst := signer.PublicKey().String()
	for i, v := range qc.sigs {
		if v.PublicKey().String() == dst {
			return qc.data.Quotas[i]
		}
	}
	return 0
}

func (qc *QuorumCert) View() uint32             { return qc.data.View }
func (qc *QuorumCert) SumQuota() uint32         { return qc.quota }
func (qc *QuorumCert) BlockHash() []byte        { return qc.data.BlockHash }
func (qc *QuorumCert) Quotas() []uint32         { return qc.data.Quotas }
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

func appendUint32(hash []byte, data ...uint32) []byte {
	n := len(data)
	buf := make([]byte, 4*n)
	for i := 0; i < n; i++ {
		binary.LittleEndian.PutUint32(buf, data[i])
	}
	hash = append(hash, buf...)
	return hash
}
