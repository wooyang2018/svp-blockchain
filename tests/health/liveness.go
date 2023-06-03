// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package health

import (
	"fmt"

	"github.com/wooyang2018/posv-blockchain/consensus"
	"github.com/wooyang2018/posv-blockchain/core"
	"github.com/wooyang2018/posv-blockchain/tests/testutil"
)

func (hc *checker) checkSafety() error {
	status, err := hc.shouldGetStatus()
	if err != nil {
		return err
	}
	height, err := hc.getMinimumBexec(status)
	if err != nil {
		return err
	}
	select {
	case <-hc.interrupt:
		return nil
	default:
	}
	blocks, err := hc.shouldGetBlockByHeight(height)
	if err != nil {
		return err
	}
	return hc.shouldEqualMerkleRoot(blocks)
}

func (hc *checker) getMinimumBexec(sMap map[int]*consensus.Status) (uint64, error) {
	var ret uint64
	for _, status := range sMap {
		if ret == 0 {
			ret = status.BExec
		} else if status.BExec < ret {
			ret = status.BExec
		}
	}
	return ret, nil
}

func (hc *checker) shouldGetBlockByHeight(height uint64) (map[int]*core.Proposal, error) {
	ret := testutil.GetBlockByHeightAll(hc.cluster, height)
	min := hc.minimumHealthyNode()
	if len(ret) < min {
		return nil, fmt.Errorf("failed to get block %d from %d nodes", height, min-len(ret))
	}
	return ret, nil
}

func (hc *checker) shouldEqualMerkleRoot(blocks map[int]*core.Proposal) error {
	var height uint64
	equalCount := make(map[string]int)
	for i, blk := range blocks {
		if !hc.cluster.EmptyChainCode && blk.Block().MerkleRoot() == nil {
			return fmt.Errorf("nil merkle root at node %d, block %d", i, blk.Block().Height())
		}
		equalCount[string(blk.Block().MerkleRoot())]++
		if height == 0 {
			height = blk.Block().Height()
		}
	}
	for _, count := range equalCount {
		if count >= hc.minimumHealthyNode() {
			fmt.Printf(" + Same merkle root at block %d\n", height)
			return nil
		}
	}
	return fmt.Errorf("different merkle root at block %d", height)
}
