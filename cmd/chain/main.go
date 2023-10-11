// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package main

import (
	"log"

	"github.com/spf13/cobra"

	"github.com/wooyang2018/svp-blockchain/consensus"

	"github.com/wooyang2018/svp-blockchain/node"
)

var nodeConfig = node.DefaultConfig

var rootCmd = &cobra.Command{
	Use:   "chain",
	Short: "svp blockchain",
	Run: func(cmd *cobra.Command, args []string) {
		node.Run(nodeConfig)
	},
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&nodeConfig.Debug,
		consensus.FlagDebug, false, "debug mode")

	rootCmd.PersistentFlags().StringVarP(&nodeConfig.Datadir,
		consensus.FlagDataDir, "d", "", "blockchain data directory")
	rootCmd.MarkPersistentFlagRequired(consensus.FlagDataDir)

	rootCmd.Flags().IntVarP(&nodeConfig.Port,
		consensus.FlagPort, "p", nodeConfig.Port, "p2p port")

	rootCmd.Flags().IntVarP(&nodeConfig.APIPort,
		consensus.FlagAPIPort, "P", nodeConfig.APIPort, "node api port")

	rootCmd.Flags().BoolVar(&nodeConfig.BroadcastTx,
		consensus.FlagBroadcastTx, false,
		"whether to broadcast transaction")

	rootCmd.Flags().Uint8Var(&nodeConfig.StorageConfig.MerkleBranchFactor,
		consensus.FlagMerkleBranchFactor, nodeConfig.StorageConfig.MerkleBranchFactor,
		"merkle tree branching factor")

	rootCmd.Flags().DurationVar(&nodeConfig.ExecutionConfig.TxExecTimeout,
		consensus.FlagTxExecTimeout, nodeConfig.ExecutionConfig.TxExecTimeout,
		"tx execution timeout")

	rootCmd.Flags().IntVar(&nodeConfig.ExecutionConfig.ConcurrentLimit,
		consensus.FlagExecConcurrentLimit, nodeConfig.ExecutionConfig.ConcurrentLimit,
		"concurrent tx execution limit")

	rootCmd.Flags().Int64Var(&nodeConfig.ConsensusConfig.ChainID,
		consensus.FlagChainID, nodeConfig.ConsensusConfig.ChainID,
		"chain id is used to create genesis block")

	rootCmd.Flags().IntVar(&nodeConfig.ConsensusConfig.BlockTxLimit,
		consensus.FlagBlockTxLimit, nodeConfig.ConsensusConfig.BlockTxLimit,
		"maximum tx count in a block")

	rootCmd.Flags().DurationVar(&nodeConfig.ConsensusConfig.TxWaitTime,
		consensus.FlagTxWaitTime, nodeConfig.ConsensusConfig.TxWaitTime,
		"proposal creation delay if no transactions in pool")

	rootCmd.Flags().DurationVar(&nodeConfig.ConsensusConfig.ViewWidth,
		consensus.FlagViewWidth, nodeConfig.ConsensusConfig.ViewWidth,
		"view duration for a leader")

	rootCmd.Flags().DurationVar(&nodeConfig.ConsensusConfig.LeaderTimeout,
		consensus.FlagLeaderTimeout, nodeConfig.ConsensusConfig.LeaderTimeout,
		"leader must create next qc in this duration")

	rootCmd.Flags().Uint8Var(&nodeConfig.ConsensusConfig.VoteStrategy,
		consensus.FlagVoteStrategy, nodeConfig.ConsensusConfig.VoteStrategy,
		"voting strategy adopted by validator")

	rootCmd.Flags().StringVar(&nodeConfig.ConsensusConfig.BenchmarkPath,
		consensus.FlagBenchmarkPath, nodeConfig.ConsensusConfig.BenchmarkPath,
		"path to save the benchmark log of the consensus algorithm")
}
