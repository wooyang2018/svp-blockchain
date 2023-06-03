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
	ErrInvalidBlockHash = errors.New("invalid block hash")
	ErrNilBlock         = errors.New("nil block")
)

// Block type
type Block struct {
	data      *pb.Block
	signature *Signature
}

var _ json.Marshaler = (*Block)(nil)
var _ json.Unmarshaler = (*Block)(nil)

func NewBlock() *Block {
	return &Block{
		data: new(pb.Block),
	}
}

// Sum returns sha3 sum of block
func (blk *Block) Sum() []byte {
	h := sha3.New256()
	binary.Write(h, binary.BigEndian, blk.data.Height)
	h.Write(blk.data.ParentHash)
	binary.Write(h, binary.BigEndian, blk.data.ExecHeight)
	h.Write(blk.data.MerkleRoot)
	binary.Write(h, binary.BigEndian, blk.data.Timestamp)
	for _, txHash := range blk.data.Transactions {
		h.Write(txHash)
	}
	return h.Sum(nil)
}

// Validate block
func (blk *Block) Validate(vs ValidatorStore) error {
	if blk.data == nil {
		return ErrNilBlock
	}
	if !bytes.Equal(blk.Sum(), blk.Hash()) {
		return ErrInvalidBlockHash
	}
	sig, err := newSignature(blk.data.Signature)
	if err != nil {
		return err
	}
	if !vs.IsVoter(sig.PublicKey()) && !vs.IsWorker(sig.PublicKey()) {
		return ErrInvalidValidator
	}
	if !sig.Verify(blk.data.Hash) {
		return ErrInvalidSig
	}
	return nil
}

func (blk *Block) setData(data *pb.Block) error {
	blk.data = data
	sig, err := newSignature(blk.data.Signature)
	if err != nil {
		return err
	}
	blk.signature = sig
	return nil
}

func (blk *Block) SetHeight(val uint64) *Block {
	blk.data.Height = val
	return blk
}

func (blk *Block) SetParentHash(val []byte) *Block {
	blk.data.ParentHash = val
	return blk
}

func (blk *Block) SetExecHeight(val uint64) *Block {
	blk.data.ExecHeight = val
	return blk
}

func (blk *Block) SetMerkleRoot(val []byte) *Block {
	blk.data.MerkleRoot = val
	return blk
}

func (blk *Block) SetTimestamp(val int64) *Block {
	blk.data.Timestamp = val
	return blk
}

func (blk *Block) SetTransactions(val [][]byte) *Block {
	blk.data.Transactions = val
	return blk
}

func (blk *Block) Sign(signer Signer) *Block {
	blk.data.Hash = blk.Sum()
	blk.signature = signer.Sign(blk.data.Hash)
	blk.data.Signature = blk.signature.data
	return blk
}

func (blk *Block) Hash() []byte           { return blk.data.Hash }
func (blk *Block) Height() uint64         { return blk.data.Height }
func (blk *Block) ParentHash() []byte     { return blk.data.ParentHash }
func (blk *Block) ExecHeight() uint64     { return blk.data.ExecHeight }
func (blk *Block) MerkleRoot() []byte     { return blk.data.MerkleRoot }
func (blk *Block) Timestamp() int64       { return blk.data.Timestamp }
func (blk *Block) Transactions() [][]byte { return blk.data.Transactions }
func (blk *Block) Proposer() *PublicKey   { return blk.signature.pubKey }
func (blk *Block) IsGenesis() bool        { return blk.Height() == 0 }

// Marshal encodes block as bytes
func (blk *Block) Marshal() ([]byte, error) {
	return proto.Marshal(blk.data)
}

// Unmarshal decodes block from bytes
func (blk *Block) Unmarshal(b []byte) error {
	data := new(pb.Block)
	if err := proto.Unmarshal(b, data); err != nil {
		return err
	}
	return blk.setData(data)
}

func (blk *Block) MarshalJSON() ([]byte, error) {
	return protojson.Marshal(blk.data)
}

func (blk *Block) UnmarshalJSON(b []byte) error {
	data := new(pb.Block)
	if err := protojson.Unmarshal(b, data); err != nil {
		return err
	}
	return blk.setData(data)
}
