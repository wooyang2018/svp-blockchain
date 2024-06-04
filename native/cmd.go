// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package native

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/wooyang2018/svp-blockchain/core"
	"github.com/wooyang2018/svp-blockchain/execution/common"
)

const (
	FileCodeXCoin   = "xcoin"
	FileCodeTAddr   = "taddr"
	FileCodeDefault = "default"
	FileNodekey     = "nodekey"
)

var (
	NodeUrl  = "http://127.0.0.1:9090"
	DataPath = "./"
	CodeFile = FileCodeDefault
)

var RootCmd = &cobra.Command{
	Use:   "client",
	Short: "The client of native contract",
}

var accountCmd = &cobra.Command{
	Use:   "account",
	Short: "Generate a nodekey file for new account",
	Run: func(cmd *cobra.Command, args []string) {
		owner := core.GenerateKey(nil)
		common.DumpFile(owner.Bytes(), DataPath, FileNodekey)
		fmt.Println(owner.PublicKey().String())
	},
}

func init() {
	RootCmd.AddCommand(accountCmd)
	RootCmd.PersistentFlags().StringVar(&NodeUrl, "node", NodeUrl, "blockchain node url")
	RootCmd.PersistentFlags().StringVar(&DataPath, "path", DataPath, "workspace data path")
	RootCmd.PersistentFlags().StringVar(&CodeFile, "code", CodeFile, "native chaincode address")
}
