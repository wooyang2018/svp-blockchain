// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package consensus

import (
	"github.com/wooyang2018/svp-blockchain/core"
	"github.com/wooyang2018/svp-blockchain/emitter"
	"github.com/wooyang2018/svp-blockchain/execution"
	"github.com/wooyang2018/svp-blockchain/p2p"
	"github.com/wooyang2018/svp-blockchain/storage"
	"github.com/wooyang2018/svp-blockchain/txpool"
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

var _ TxPool = (*txpool.TxPool)(nil)

type Storage interface {
	GetMerkleRoot() []byte
	Commit(data *storage.CommitData) error
	StoreBlock(blk *core.Block) error
	GetBlock(hash []byte) (*core.Block, error)
	GetLastBlock() (*core.Block, error)
	StoreQC(qc *core.QuorumCert) error
	GetQC(blkHash []byte) (*core.QuorumCert, error)
	GetLastQC() (*core.QuorumCert, error)
	GetBlockHeight() uint64
	HasTx(hash []byte) bool
}

var _ Storage = (*storage.Storage)(nil)

type MsgService interface {
	BroadcastProposal(blk *core.Block) error
	BroadcastQC(qc *core.QuorumCert) error
	SendVote(pubKey *core.PublicKey, vote *core.Vote) error
	RequestBlock(pubKey *core.PublicKey, hash []byte) (*core.Block, error)
	RequestBlockByHeight(pubKey *core.PublicKey, height uint64) (*core.Block, error)
	RequestQC(pubKey *core.PublicKey, blkHash []byte) (*core.QuorumCert, error)
	SendQC(pubKey *core.PublicKey, qc *core.QuorumCert) error
	SubscribeProposal(buffer int) *emitter.Subscription
	SubscribeVote(buffer int) *emitter.Subscription
	SubscribeQC(buffer int) *emitter.Subscription
}

var _ MsgService = (*p2p.MsgService)(nil)

type Execution interface {
	Execute(blk *core.Block, txs []*core.Transaction) (*core.BlockCommit, []*core.TxCommit)
	MockExecute(blk *core.Block) (*core.BlockCommit, []*core.TxCommit)
}

var _ Execution = (*execution.Execution)(nil)

type Resources struct {
	Signer    core.Signer
	RoleStore core.RoleStore
	Storage   Storage
	MsgSvc    MsgService
	Host      *p2p.Host
	TxPool    TxPool
	Execution Execution
}
