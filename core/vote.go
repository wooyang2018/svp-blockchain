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
	ErrNilVote = errors.New("nil vote")
)

// Vote type
type Vote struct {
	data  *pb.Vote
	voter *PublicKey
}

func NewVote() *Vote {
	return &Vote{
		data: new(pb.Vote),
	}
}

// Validate vote
func (vote *Vote) Validate(vs ValidatorStore) error {
	if vote.data == nil {
		return ErrNilVote
	}
	sig, err := newSignature(vote.data.Signature)
	if err != nil {
		return err
	}
	if !(vs.IsVoter(sig.PublicKey()) || vs.IsWorker(sig.PublicKey())) {
		return ErrInvalidValidator
	}
	if !sig.Verify(vote.data.BlockHash) {
		return ErrInvalidSig
	}
	return nil
}

func (vote *Vote) setData(data *pb.Vote) error {
	vote.data = data
	sig, err := newSignature(vote.data.Signature)
	if err != nil {
		return err
	}
	vote.voter = sig.pubKey
	return nil
}

func (vote *Vote) BlockHash() []byte { return vote.data.BlockHash }
func (vote *Vote) Voter() *PublicKey { return vote.voter }

// Marshal encodes vote as bytes
func (vote *Vote) Marshal() ([]byte, error) {
	return proto.Marshal(vote.data)
}

// Unmarshal decodes vote from bytes
func (vote *Vote) Unmarshal(b []byte) error {
	data := new(pb.Vote)
	if err := proto.Unmarshal(b, data); err != nil {
		return err
	}
	return vote.setData(data)
}
