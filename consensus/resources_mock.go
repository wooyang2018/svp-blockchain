// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package consensus

import (
	"github.com/stretchr/testify/mock"

	"github.com/wooyang2018/svp-blockchain/core"
	"github.com/wooyang2018/svp-blockchain/emitter"
	"github.com/wooyang2018/svp-blockchain/storage"
	"github.com/wooyang2018/svp-blockchain/txpool"
)

type MockTxPool struct {
	mock.Mock
}

var _ TxPool = (*MockTxPool)(nil)

func (m *MockTxPool) SubmitTx(tx *core.Transaction) error {
	args := m.Called(tx)
	return args.Error(0)
}

func (m *MockTxPool) StoreTxs(txs *core.TxList) error {
	args := m.Called(txs)
	return args.Error(0)
}

func (m *MockTxPool) PopTxsFromQueue(max int) [][]byte {
	args := m.Called(max)
	return castBytesBytes(args.Get(0))
}

func (m *MockTxPool) GetTxsFromQueue(max int) [][]byte {
	args := m.Called(max)
	return castBytesBytes(args.Get(0))
}

func (m *MockTxPool) SetTxsPending(hashes [][]byte) {
	m.Called(hashes)
}

func (m *MockTxPool) GetTxsToExecute(hashes [][]byte) ([]*core.Transaction, [][]byte) {
	args := m.Called(hashes)
	return castTransactions(args.Get(0)), castBytesBytes(args.Get(1))
}

func (m *MockTxPool) RemoveTxs(hashes [][]byte) {
	m.Called(hashes)
}

func (m *MockTxPool) PutTxsToQueue(hashes [][]byte) {
	m.Called(hashes)
}

func (m *MockTxPool) SyncTxs(peer *core.PublicKey, hashes [][]byte) error {
	args := m.Called(peer, hashes)
	return args.Error(0)
}

func (m *MockTxPool) GetTx(hash []byte) *core.Transaction {
	args := m.Called(hash)
	return castTransaction(args.Get(0))
}

func (m *MockTxPool) GetTxStatus(hash []byte) txpool.TxStatus {
	args := m.Called(hash)
	return args.Get(0).(txpool.TxStatus)
}

func (m *MockTxPool) GetStatus() txpool.Status {
	args := m.Called()
	return args.Get(0).(txpool.Status)
}

type MockStorage struct {
	mock.Mock
}

var _ Storage = (*MockStorage)(nil)

func (m *MockStorage) GetMerkleRoot() []byte {
	args := m.Called()
	return castBytes(args.Get(0))
}

func (m *MockStorage) Commit(data *storage.CommitData) error {
	args := m.Called(data)
	return args.Error(0)
}

func (m *MockStorage) StoreBlock(blk *core.Block) error {
	args := m.Called(blk)
	return args.Error(0)
}

func (m *MockStorage) GetBlock(hash []byte) (*core.Block, error) {
	args := m.Called(hash)
	return castCoreBlock(args.Get(0)), args.Error(1)
}

func (m *MockStorage) GetLastBlock() (*core.Block, error) {
	args := m.Called()
	return castCoreBlock(args.Get(0)), args.Error(1)
}

func (m *MockStorage) StoreQC(qc *core.QuorumCert) error {
	args := m.Called(qc)
	return args.Error(0)
}

func (m *MockStorage) GetQC(blkHash []byte) (*core.QuorumCert, error) {
	args := m.Called(blkHash)
	return castCoreQC(args.Get(0)), args.Error(1)
}

func (m *MockStorage) GetLastQC() (*core.QuorumCert, error) {
	args := m.Called()
	return castCoreQC(args.Get(0)), args.Error(1)
}

func (m *MockStorage) GetBlockHeight() uint64 {
	args := m.Called()
	return uint64(args.Int(0))
}

func (m *MockStorage) HasTx(hash []byte) bool {
	args := m.Called(hash)
	return args.Bool(0)
}

type MockMsgService struct {
	mock.Mock
}

var _ MsgService = (*MockMsgService)(nil)

func (m *MockMsgService) BroadcastProposal(blk *core.Block) error {
	args := m.Called(blk)
	return args.Error(0)
}

func (m *MockMsgService) BroadcastQC(qc *core.QuorumCert) error {
	args := m.Called(qc)
	return args.Error(0)
}

func (m *MockMsgService) SendVote(pubKey *core.PublicKey, vote *core.Vote) error {
	args := m.Called(pubKey, vote)
	return args.Error(0)
}

func (m *MockMsgService) RequestBlock(pubKey *core.PublicKey, hash []byte) (*core.Block, error) {
	args := m.Called(pubKey, hash)
	return castCoreBlock(args.Get(0)), args.Error(1)
}

func (m *MockMsgService) RequestQC(pubKey *core.PublicKey, blkHash []byte) (*core.QuorumCert, error) {
	args := m.Called(pubKey, blkHash)
	return castCoreQC(args.Get(0)), args.Error(1)
}

func (m *MockMsgService) RequestBlockByHeight(pubKey *core.PublicKey, height uint64) (*core.Block, error) {
	args := m.Called(pubKey, height)
	return castCoreBlock(args.Get(0)), args.Error(1)
}

func (m *MockMsgService) SendQC(pubKey *core.PublicKey, qc *core.QuorumCert) error {
	args := m.Called(pubKey, qc)
	return args.Error(0)
}

func (m *MockMsgService) SubscribeProposal(buffer int) *emitter.Subscription {
	args := m.Called(buffer)
	return castSubscription(args.Get(0))
}

func (m *MockMsgService) SubscribeVote(buffer int) *emitter.Subscription {
	args := m.Called(buffer)
	return castSubscription(args.Get(0))
}

func (m *MockMsgService) SubscribeQC(buffer int) *emitter.Subscription {
	args := m.Called(buffer)
	return castSubscription(args.Get(0))
}

type MockExecution struct {
	mock.Mock
}

var _ Execution = (*MockExecution)(nil)

func (m *MockExecution) Execute(blk *core.Block, txs []*core.Transaction) (*core.BlockCommit, []*core.TxCommit) {
	args := m.Called(blk, txs)
	return castBlockCommit(args.Get(0)), castTxCommits(args.Get(1))
}

func (m *MockExecution) MockExecute(blk *core.Block) (*core.BlockCommit, []*core.TxCommit) {
	args := m.Called(blk)
	return castBlockCommit(args.Get(0)), castTxCommits(args.Get(1))
}

func castBytes(val interface{}) []byte {
	if val == nil {
		return nil
	}
	return val.([]byte)
}

func castBytesBytes(val interface{}) [][]byte {
	if val == nil {
		return nil
	}
	return val.([][]byte)
}

func castCoreBlock(val interface{}) *core.Block {
	if val == nil {
		return nil
	}
	return val.(*core.Block)
}

func castCoreQC(val interface{}) *core.QuorumCert {
	if val == nil {
		return nil
	}
	return val.(*core.QuorumCert)
}

func castTransaction(val interface{}) *core.Transaction {
	if val == nil {
		return nil
	}
	return val.(*core.Transaction)
}

func castTransactions(val interface{}) []*core.Transaction {
	if val == nil {
		return nil
	}
	return val.([]*core.Transaction)
}

func castSubscription(val interface{}) *emitter.Subscription {
	if val == nil {
		return nil
	}
	return val.(*emitter.Subscription)
}

func castBlockCommit(val interface{}) *core.BlockCommit {
	if val == nil {
		return nil
	}
	return val.(*core.BlockCommit)
}

func castTxCommits(val interface{}) []*core.TxCommit {
	if val == nil {
		return nil
	}
	return val.([]*core.TxCommit)
}
