// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package core

import (
	"errors"

	"google.golang.org/protobuf/proto"

	"github.com/wooyang2018/svp-blockchain/pb"
)

// errors
var (
	ErrNilVote = errors.New("nil vote")
)

// Vote type
type Vote struct {
	data  *pb.Vote
	voter *Signature
}

func NewVote() *Vote {
	return &Vote{
		data: new(pb.Vote),
	}
}

// Validate vote
func (vote *Vote) Validate(rs RoleStore) error {
	if vote.data == nil {
		return ErrNilVote
	}
	sig, err := newSignature(vote.data.Signature)
	if err != nil {
		return err
	}
	if !rs.IsValidator(sig.PublicKey().String()) {
		return ErrInvalidValidator
	}
	if !sig.Verify(appendUint64(appendUint32(vote.data.BlockHash, vote.data.View), vote.data.Quota)) {
		return ErrInvalidSig
	}
	return nil
}

func (vote *Vote) setData(data *pb.Vote) error {
	vote.data = data
	sig, err := newSignature(data.Signature)
	if err != nil {
		return err
	}
	vote.voter = sig
	return nil
}

func (vote *Vote) View() uint32      { return vote.data.View }
func (vote *Vote) Quota() uint64     { return vote.data.Quota }
func (vote *Vote) BlockHash() []byte { return vote.data.BlockHash }
func (vote *Vote) Voter() *PublicKey { return vote.voter.pubKey }

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
