// Copyright (C) 2024 Wooyang2018
// Licensed under the GNU General Public License v3.0

package main

import (
	"github.com/spf13/cobra"

	"github.com/wooyang2018/svp-blockchain/execution/common"
	"github.com/wooyang2018/svp-blockchain/native"
)

const (
	FlagFilePath string = "contract"
)

var (
	filePath string
)

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy a specific evm contract",
	Run: func(cmd *cobra.Command, args []string) {
		client := native.NewClient(true)
		codeID, err := client.UploadChainCode(common.DriverTypeEVM, filePath)
		common.Check(err)
		tx := client.MakeDeploymentTx(common.DriverTypeEVM, codeID)
		client.SubmitTx(tx)
		common.DumpFile(tx.Hash(), native.DataPath, native.FileCodeDefault)
	},
}

func main() {
	err := native.RootCmd.Execute()
	common.Check(err)
}

func init() {
	native.RootCmd.AddCommand(deployCmd)
	deployCmd.PersistentFlags().StringVar(&filePath, FlagFilePath, filePath, "file path of evm contract")
	deployCmd.MarkPersistentFlagRequired(FlagFilePath)
}
