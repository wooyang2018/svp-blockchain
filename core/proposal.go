// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package core

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"

	"github.com/wooyang2018/posv-blockchain/pb"
	"golang.org/x/crypto/sha3"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// errors
var (
	ErrInvalidProposalHash = errors.New("invalid proposal hash")
	ErrNilProposal         = errors.New("nil proposal")
)

// Proposal type
type Proposal struct {
	data       *pb.Proposal
	block      *Block
	signature  *Signature
	quorumCert *QuorumCert
}

var _ json.Marshaler = (*Proposal)(nil)
var _ json.Unmarshaler = (*Proposal)(nil)

func NewProposal() *Proposal {
	pro := &Proposal{data: new(pb.Proposal)}
	return pro
}

// Sum returns sha3 sum of proposal
func (pro *Proposal) Sum() []byte {
	h := sha3.New256()
	if pro.data.Block != nil {
		h.Write(pro.data.Block.Hash)
	}
	if pro.data.QuorumCert != nil {
		h.Write(pro.data.QuorumCert.BlockHash) // qc reference block hash
	}
	binary.Write(h, binary.BigEndian, pro.data.View)
	return h.Sum(nil)
}

// Validate proposal
func (pro *Proposal) Validate(vs ValidatorStore) error {
	if pro.data == nil {
		return ErrNilProposal
	}
	if pro.block != nil {
		if err := pro.block.Validate(vs); err != nil {
			return err
		}
	}
	if pro.quorumCert != nil { // skip quorum cert validation for genesis block
		if err := pro.quorumCert.Validate(vs); err != nil {
			return err
		}
	}
	if !bytes.Equal(pro.Sum(), pro.Hash()) {
		return ErrInvalidProposalHash
	}
	sig, err := newSignature(pro.data.Signature)
	if !vs.IsWorker(sig.PublicKey()) {
		return ErrInvalidValidator
	}
	if err != nil {
		return err
	}
	if !sig.Verify(pro.data.Hash) {
		return ErrInvalidSig
	}
	return nil
}

// Vote creates a vote for proposal
func (pro *Proposal) Vote(signer Signer) *Vote {
	vote := &pb.Vote{Quota: 1, View: pro.data.View}
	if pro.block != nil {
		vote.BlockHash = pro.block.Hash()
	} else {
		vote.BlockHash = pro.quorumCert.BlockHash()
	}
	vote.Signature = signer.Sign(vote.BlockHash).data
	res := NewVote()
	res.setData(vote)
	return res
}

func (pro *Proposal) setData(data *pb.Proposal) error {
	pro.data = data
	signature, err := newSignature(pro.data.Signature)
	if err != nil {
		return err
	}
	pro.signature = signature

	if pro.data.Block != nil {
		block := NewBlock()
		if err = block.setData(pro.data.Block); err != nil {
			return err
		}
		pro.block = block
	}

	if pro.data.QuorumCert != nil { // every block contains qc except for genesis
		pro.quorumCert = NewQuorumCert()
		if err := pro.quorumCert.setData(pro.data.QuorumCert); err != nil {
			return err
		}
	}
	return nil
}

func (pro *Proposal) SetBlock(val *Block) *Proposal {
	pro.block = val
	pro.data.Block = val.data
	return pro
}

func (pro *Proposal) SetQuorumCert(val *QuorumCert) *Proposal {
	pro.quorumCert = val
	pro.data.QuorumCert = val.data
	return pro
}

func (pro *Proposal) SetView(val uint32) *Proposal {
	pro.data.View = val
	return pro
}

func (pro *Proposal) Sign(signer Signer) *Proposal {
	pro.data.Hash = pro.Sum()
	pro.signature = signer.Sign(pro.data.Hash)
	pro.data.Signature = pro.signature.data
	return pro
}

func (pro *Proposal) Hash() []byte            { return pro.data.Hash }
func (pro *Proposal) Block() *Block           { return pro.block }
func (pro *Proposal) QuorumCert() *QuorumCert { return pro.quorumCert }
func (pro *Proposal) View() uint32            { return pro.data.View }
func (pro *Proposal) Proposer() *PublicKey    { return pro.signature.pubKey }

// Marshal encodes proposal as bytes
func (pro *Proposal) Marshal() ([]byte, error) {
	return proto.Marshal(pro.data)
}

// Unmarshal decodes proposal from bytes
func (pro *Proposal) Unmarshal(b []byte) error {
	data := new(pb.Proposal)
	if err := proto.Unmarshal(b, data); err != nil {
		return err
	}
	return pro.setData(data)
}

func (pro *Proposal) MarshalJSON() ([]byte, error) {
	return protojson.Marshal(pro.data)
}

func (pro *Proposal) UnmarshalJSON(b []byte) error {
	data := new(pb.Proposal)
	if err := protojson.Unmarshal(b, data); err != nil {
		return err
	}
	return pro.setData(data)
}
