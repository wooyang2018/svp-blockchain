// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package health

import (
	"fmt"

	"github.com/wooyang2018/svp-blockchain/core"
	"github.com/wooyang2018/svp-blockchain/tests/testutil"
)

func (hc *checker) checkSafety() error {
	status, err := hc.shouldGetStatus()
	if err != nil {
		return err
	}

	// get minimum bexec block
	var height uint64
	for _, s := range status {
		if height == 0 {
			height = s.BExec
		} else if s.BExec < height {
			height = s.BExec
		}
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

func (hc *checker) shouldGetBlockByHeight(height uint64) (map[int]*core.Block, error) {
	ret := testutil.GetBlockByHeightAll(hc.cluster, height)
	min := hc.minimumHealthyNode()
	if len(ret) < min {
		return nil, fmt.Errorf("failed to get block %d from %d nodes", height, min-len(ret))
	}
	return ret, nil
}

func (hc *checker) shouldEqualMerkleRoot(blocks map[int]*core.Block) error {
	var height uint64
	equalCount := make(map[string]int)
	for i, blk := range blocks {
		if !hc.cluster.EmptyChainCode && blk.MerkleRoot() == nil {
			return fmt.Errorf("nil merkle root at node %d, height %d", i, blk.Height())
		}
		equalCount[string(blk.MerkleRoot())]++
		if height == 0 {
			height = blk.Height()
		}
	}
	for _, count := range equalCount {
		if count >= hc.minimumHealthyNode() {
			fmt.Printf(" + Same merkle root at height %d\n", height)
			return nil
		}
	}
	return fmt.Errorf("different merkle root at height %d", height)
}
