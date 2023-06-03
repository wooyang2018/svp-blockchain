// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package consensus

import (
	"github.com/wooyang2018/posv-blockchain/core"
	"github.com/wooyang2018/posv-blockchain/emitter"
	"github.com/wooyang2018/posv-blockchain/storage"
	"github.com/wooyang2018/posv-blockchain/txpool"
)

type TxPool interface {
	SubmitTx(tx *core.Transaction) error
	StoreTxs(txs *core.TxList) error
	PopTxsFromQueue(max int) [][]byte
	GetTxsFromQueue(max int) [][]byte
	SetTxsPending(hashes [][]byte)
	GetTxsToExecute(hashes [][]byte) ([]*core.Transaction, [][]byte)
	RemoveTxs(hashes [][]byte)
	PutTxsToQueue(hashes [][]byte)
	SyncTxs(peer *core.PublicKey, hashes [][]byte) error
	GetTx(hash []byte) *core.Transaction
	GetStatus() txpool.Status
	GetTxStatus(hash []byte) txpool.TxStatus
}

type Storage interface {
	GetMerkleRoot() []byte
	Commit(data *storage.CommitData) error
	GetBlock(hash []byte) (*core.Proposal, error)
	GetLastBlock() (*core.Proposal, error)
	GetLastQC() (*core.QuorumCert, error)
	GetBlockHeight() uint64
	HasTx(hash []byte) bool
}

type MsgService interface {
	BroadcastProposal(blk *core.Proposal) error
	BroadcastNewView(qc *core.QuorumCert) error
	SendVote(pubKey *core.PublicKey, vote *core.Vote) error
	RequestBlock(pubKey *core.PublicKey, hash []byte) (*core.Proposal, error)
	RequestBlockByHeight(pubKey *core.PublicKey, height uint64) (*core.Proposal, error)
	SendNewView(pubKey *core.PublicKey, qc *core.QuorumCert) error

	SubscribeProposal(buffer int) *emitter.Subscription
	SubscribeVote(buffer int) *emitter.Subscription
	SubscribeNewView(buffer int) *emitter.Subscription
}

type Execution interface {
	Execute(blk *core.Block, txs []*core.Transaction) (*core.BlockCommit, []*core.TxCommit)
	MockExecute(blk *core.Block) (*core.BlockCommit, []*core.TxCommit)
}

type Resources struct {
	Signer    core.Signer
	VldStore  core.ValidatorStore
	Storage   Storage
	MsgSvc    MsgService
	TxPool    TxPool
	Execution Execution
}
