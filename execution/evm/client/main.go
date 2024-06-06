// Copyright (C) 2024 Wooyang2018
// Licensed under the GNU General Public License v3.0

package main

import (
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/spf13/cobra"

	"github.com/wooyang2018/svp-blockchain/execution/common"
	"github.com/wooyang2018/svp-blockchain/execution/evm"
	"github.com/wooyang2018/svp-blockchain/native"
)

const (
	FlagFilePath string = "contract"
	FlagInput    string = "input"
)

var (
	filePath string
	rawInput string
)

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy a specific evm contract",
	Run: func(cmd *cobra.Command, args []string) {
		// here test if the init input json is valid
		var input evm.InitInput
		err := json.Unmarshal([]byte(rawInput), &input)
		fmt.Printf("%+v\n", input)
		common.Check2(err)

		client := native.NewClient(true)
		codeID, err := client.UploadChainCode(common.DriverTypeEVM, filePath)
		common.Check2(err)
		tx := client.MakeDeploymentTx(common.DriverTypeEVM, codeID, []byte(rawInput))
		client.SubmitTx(tx)
		common.DumpFile(tx.Hash(), native.DataPath, native.FileCodeDefault)
	},
}

var invokeCmd = &cobra.Command{
	Use:   "invoke",
	Short: "Invoke evm contract with specific input json",
	Run: func(cmd *cobra.Command, args []string) {
		// here test if the input json is valid
		var input evm.Input
		err := json.Unmarshal([]byte(rawInput), &input)
		fmt.Printf("%+v\n", input)
		common.Check2(err)

		client := native.NewClient(false)
		tx := client.MakeTx([]byte(rawInput))
		client.SubmitTx(tx)
	},
}

var queryCmd = &cobra.Command{
	Use:   "query",
	Short: "Query evm contract with specific input json",
	Run: func(cmd *cobra.Command, args []string) {
		// here test if the input json is valid
		var input evm.Input
		err := json.Unmarshal([]byte(rawInput), &input)
		fmt.Printf("%+v\n", input)
		common.Check2(err)

		client := native.NewClient(false)
		ret := client.QueryState([]byte(rawInput))
		fmt.Println(big.NewInt(0).SetBytes(ret)) // TODO only for Storage.sol
	},
}

func main() {
	err := native.RootCmd.Execute()
	common.Check(err)
}

func init() {
	native.RootCmd.AddCommand(deployCmd)
	native.RootCmd.AddCommand(invokeCmd)
	native.RootCmd.AddCommand(queryCmd)

	deployCmd.PersistentFlags().StringVar(&filePath, FlagFilePath, filePath, "file path of evm contract")
	deployCmd.MarkPersistentFlagRequired(FlagFilePath)
	deployCmd.PersistentFlags().StringVar(&rawInput, FlagInput, rawInput, "init input json of evm contract")
	deployCmd.MarkPersistentFlagRequired(FlagInput)

	invokeCmd.PersistentFlags().StringVar(&rawInput, FlagInput, rawInput, "input json of evm contract")
	invokeCmd.MarkPersistentFlagRequired(FlagInput)

	queryCmd.PersistentFlags().StringVar(&rawInput, FlagInput, rawInput, "input json of evm contract")
	queryCmd.MarkPersistentFlagRequired(FlagInput)
}
