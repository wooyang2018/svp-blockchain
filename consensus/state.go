// Copyright (C) 2021 Aung Maw
// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package consensus

import (
	"sync"
	"sync/atomic"

	"github.com/wooyang2018/ppov-blockchain/core"
)

type state struct {
	resources *Resources

	blocks    map[string]*core.Block
	mtxBlocks sync.RWMutex

	committed    map[string]struct{}
	mtxCommitted sync.RWMutex

	qcs    map[string]*core.QuorumCert // qc by block hash
	mtxQCs sync.RWMutex

	mtxUpdate sync.Mutex // lock for hotstuff update call

	leaderIndex int64

	// committed block height. on node restart, it's zero until a block is committed
	committedHeight uint64

	// committed tx count, since last node start
	committedTxCount uint64
}

func newState(resources *Resources) *state {
	return &state{
		resources: resources,
		blocks:    make(map[string]*core.Block),
		committed: make(map[string]struct{}),
		qcs:       make(map[string]*core.QuorumCert),
	}
}

func (state *state) getBlockPoolSize() int {
	state.mtxBlocks.RLock()
	defer state.mtxBlocks.RUnlock()
	return len(state.blocks)
}

func (state *state) setBlock(blk *core.Block) {
	state.mtxBlocks.Lock()
	defer state.mtxBlocks.Unlock()
	state.blocks[string(blk.Hash())] = blk
}

func (state *state) getBlock(hash []byte) *core.Block {
	blk := state.getBlockFromState(hash)
	if blk != nil {
		return blk
	}
	blk, _ = state.resources.Storage.GetBlock(hash)
	if blk == nil {
		return nil
	}
	state.setBlock(blk)
	return blk
}

func (state *state) getBlockFromState(hash []byte) *core.Block {
	state.mtxBlocks.RLock()
	defer state.mtxBlocks.RUnlock()
	return state.blocks[string(hash)]
}

func (state *state) deleteBlock(hash []byte) {
	state.mtxBlocks.Lock()
	defer state.mtxBlocks.Unlock()
	delete(state.blocks, string(hash))
}

func (state *state) getQCPoolSize() int {
	state.mtxQCs.RLock()
	defer state.mtxQCs.RUnlock()
	return len(state.qcs)
}

func (state *state) setQC(qc *core.QuorumCert) {
	state.mtxQCs.Lock()
	defer state.mtxQCs.Unlock()
	state.qcs[string(qc.BlockHash())] = qc
}

func (state *state) getQC(blkHash []byte) *core.QuorumCert {
	state.mtxQCs.RLock()
	defer state.mtxQCs.RUnlock()
	return state.qcs[string(blkHash)]
}

func (state *state) deleteQC(blkHash []byte) {
	state.mtxQCs.Lock()
	defer state.mtxQCs.Unlock()
	delete(state.qcs, string(blkHash))
}

func (state *state) setCommittedBlock(blk *core.Block) {
	state.mtxCommitted.Lock()
	defer state.mtxCommitted.Unlock()
	state.committed[string(blk.Hash())] = struct{}{}
	atomic.StoreUint64(&state.committedHeight, blk.Height())
}

func (state *state) deleteCommitted(blkhash []byte) {
	state.mtxCommitted.Lock()
	defer state.mtxCommitted.Unlock()
	delete(state.committed, string(blkhash))
}

func (state *state) getOlderBlocks(height uint64) []*core.Block {
	state.mtxBlocks.RLock()
	defer state.mtxBlocks.RUnlock()
	ret := make([]*core.Block, 0)
	for _, b := range state.blocks {
		if b.Height() < height {
			ret = append(ret, b)
		}
	}
	return ret
}

func (state *state) getUncommittedOlderBlocks(bexec *core.Block) []*core.Block {
	state.mtxBlocks.RLock()
	defer state.mtxBlocks.RUnlock()

	state.mtxCommitted.RLock()
	defer state.mtxCommitted.RUnlock()

	ret := make([]*core.Block, 0)
	for _, b := range state.blocks {
		if b.Height() < bexec.Height() {
			if _, committed := state.committed[string(b.Hash())]; !committed {
				ret = append(ret, b)
			}
		}
	}
	return ret
}

func (state *state) isThisNodeLeader() bool {
	return state.isLeader(state.resources.Signer.PublicKey())
}

func (state *state) isThisNodeWorker() bool {
	return state.isWorker(state.resources.Signer.PublicKey())
}

func (state *state) isThisNodeVoter() bool {
	return state.isVoter(state.resources.Signer.PublicKey())
}

func (state *state) isLeader(pubKey *core.PublicKey) bool {
	if !state.resources.VldStore.IsWorker(pubKey) {
		return false
	}
	return state.getLeaderIndex() == state.resources.VldStore.GetWorkerIndex(pubKey)
}

func (state *state) isWorker(pubKey *core.PublicKey) bool {
	return state.resources.VldStore.IsWorker(pubKey)
}

func (state *state) isVoter(pubKey *core.PublicKey) bool {
	return state.resources.VldStore.IsVoter(pubKey)
}

func (state *state) setLeaderIndex(idx int) {
	atomic.StoreInt64(&state.leaderIndex, int64(idx))
}

func (state *state) getLeaderIndex() int {
	return int(atomic.LoadInt64(&state.leaderIndex))
}

func (state *state) getFaultyCount() int {
	return state.resources.VldStore.ValidatorCount() - state.resources.VldStore.MajorityValidatorCount()
}

func (state *state) addCommittedTxCount(count int) {
	atomic.AddUint64(&state.committedTxCount, uint64(count))
}

func (state *state) getCommittedTxCount() int {
	return int(atomic.LoadUint64(&state.committedTxCount))
}

func (state *state) getCommittedHeight() uint64 {
	return atomic.LoadUint64(&state.committedHeight)
}
