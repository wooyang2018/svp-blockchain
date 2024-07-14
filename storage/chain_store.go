// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package storage

import (
	"bytes"
	"encoding/binary"

	"github.com/wooyang2018/svp-blockchain/core"
)

type chainStore struct {
	getter Getter
}

func (cs *chainStore) getLastBlock() (*core.Block, error) {
	height, err := cs.getBlockHeight()
	if err != nil {
		return nil, err
	}
	return cs.getBlockByHeight(height)
}

func (cs *chainStore) getBlockHeight() (uint64, error) {
	b, err := cs.getter.Get(convertPrefix(BLOCK_HEIGHT))
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint64(b), nil
}

func (cs *chainStore) getBlockByHeight(height uint64) (*core.Block, error) {
	hash, err := cs.getBlockHashByHeight(height)
	if err != nil {
		return nil, err
	}
	return cs.getBlock(hash)
}

func (cs *chainStore) getBlockHashByHeight(height uint64) ([]byte, error) {
	return cs.getter.Get(concatBytes(convertPrefix(BLOCK_HASH_BY_HEIGHT), uint64BEBytes(height)))
}

func (cs *chainStore) getBlock(hash []byte) (*core.Block, error) {
	b, err := cs.getter.Get(concatBytes(convertPrefix(BLOCK_BY_HASH), hash))
	if err != nil {
		return nil, err
	}
	blk := core.NewBlock()
	if err := blk.Unmarshal(b); err != nil {
		return nil, err
	}
	return blk, nil
}

func (cs *chainStore) getLastQC() (*core.QuorumCert, error) {
	hash, err := cs.getter.Get(convertPrefix(LAST_QC_BLOCK_HASH))
	if err != nil {
		return nil, err
	}
	return cs.getQCByBlockHash(hash)
}

func (cs *chainStore) getQCByBlockHash(hash []byte) (*core.QuorumCert, error) {
	data, err := cs.getter.Get(concatBytes(convertPrefix(QC_BY_BLOCK_HASH), hash))
	if err != nil {
		return nil, err
	}
	qc := core.NewQuorumCert()
	if err := qc.Unmarshal(data); err != nil {
		return nil, err
	}
	return qc, nil
}

func (cs *chainStore) getBlockCommit(hash []byte) (*core.BlockCommit, error) {
	b, err := cs.getter.Get(concatBytes(convertPrefix(BLOCK_COMMIT_BY_HASH), hash))
	if err != nil {
		return nil, err
	}
	bcm := core.NewBlockCommit()
	if err := bcm.Unmarshal(b); err != nil {
		return nil, err
	}
	return bcm, nil
}

func (cs *chainStore) getTx(hash []byte) (*core.Transaction, error) {
	b, err := cs.getter.Get(concatBytes(convertPrefix(TX_BY_HASH), hash))
	if err != nil {
		return nil, err
	}
	tx := core.NewTransaction()
	if err := tx.Unmarshal(b); err != nil {
		return nil, err
	}
	return tx, nil
}

func (cs *chainStore) hasTx(hash []byte) (bool, error) {
	return cs.getter.Has(concatBytes(convertPrefix(TX_BY_HASH), hash))
}

func (cs *chainStore) getTxCommit(hash []byte) (*core.TxCommit, error) {
	val, err := cs.getter.Get(concatBytes(convertPrefix(TX_COMMIT_BY_HASH), hash))
	if err != nil {
		return nil, err
	}
	txc := core.NewTxCommit()
	if err := txc.Unmarshal(val); err != nil {
		return nil, err
	}
	return txc, nil
}

func (cs *chainStore) setBlockHeight(height uint64) updateFunc {
	return func(setter Setter) error {
		return setter.Put(convertPrefix(BLOCK_HEIGHT), uint64BEBytes(height))
	}
}

func (cs *chainStore) setBlock(blk *core.Block) []updateFunc {
	ret := make([]updateFunc, 0)
	ret = append(ret, cs.setBlockByHash(blk))
	ret = append(ret, cs.setBlockHashByHeight(blk))
	return ret
}

func (cs *chainStore) setQC(qc *core.QuorumCert) []updateFunc {
	ret := make([]updateFunc, 0)
	ret = append(ret, cs.setQCByBlockHash(qc))
	ret = append(ret, cs.setLastQCBlockHash(qc.BlockHash()))
	return ret
}

func (cs *chainStore) setLastQCBlockHash(blkHash []byte) updateFunc {
	return func(setter Setter) error {
		return setter.Put(convertPrefix(LAST_QC_BLOCK_HASH), blkHash)
	}
}

func (cs *chainStore) setBlockByHash(blk *core.Block) updateFunc {
	return func(setter Setter) error {
		val, err := blk.Marshal()
		if err != nil {
			return err
		}
		return setter.Put(concatBytes(convertPrefix(BLOCK_BY_HASH), blk.Hash()), val)
	}
}

func (cs *chainStore) setBlockHashByHeight(blk *core.Block) updateFunc {
	return func(setter Setter) error {
		return setter.Put(
			concatBytes(convertPrefix(BLOCK_HASH_BY_HEIGHT), uint64BEBytes(blk.Height())),
			blk.Hash(),
		)
	}
}

func (cs *chainStore) setQCByBlockHash(qc *core.QuorumCert) updateFunc {
	return func(setter Setter) error {
		val, err := qc.Marshal()
		if err != nil {
			return err
		}
		return setter.Put(concatBytes(convertPrefix(QC_BY_BLOCK_HASH), qc.BlockHash()), val)
	}
}

func (cs *chainStore) setBlockCommit(bcm *core.BlockCommit) updateFunc {
	return func(setter Setter) error {
		val, err := bcm.Marshal()
		if err != nil {
			return err
		}
		return setter.Put(
			concatBytes(convertPrefix(BLOCK_COMMIT_BY_HASH), bcm.Hash()), val,
		)
	}
}

func (cs *chainStore) setTxs(txs []*core.Transaction) []updateFunc {
	ret := make([]updateFunc, len(txs))
	for i, tx := range txs {
		ret[i] = cs.setTx(tx)
	}
	return ret
}

func (cs *chainStore) setTxCommits(txCommits []*core.TxCommit) []updateFunc {
	ret := make([]updateFunc, len(txCommits))
	for i, txc := range txCommits {
		ret[i] = cs.setTxCommit(txc)
	}
	return ret
}

func (cs *chainStore) setTx(tx *core.Transaction) updateFunc {
	return func(setter Setter) error {
		val, err := tx.Marshal()
		if err != nil {
			return err
		}
		return setter.Put(
			concatBytes(convertPrefix(TX_BY_HASH), tx.Hash()), val,
		)
	}
}

func (cs *chainStore) setTxCommit(txc *core.TxCommit) updateFunc {
	return func(setter Setter) error {
		val, err := txc.Marshal()
		if err != nil {
			return err
		}
		return setter.Put(
			concatBytes(convertPrefix(TX_COMMIT_BY_HASH), txc.Hash()), val,
		)
	}
}

func uint64BEBytes(val uint64) []byte {
	buf := bytes.NewBuffer(nil)
	binary.Write(buf, binary.BigEndian, val)
	return buf.Bytes()
}
