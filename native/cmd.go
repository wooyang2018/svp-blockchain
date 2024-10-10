// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package native

import (
	"fmt"
	"path"

	"github.com/spf13/cobra"

	"github.com/wooyang2018/svp-blockchain/core"
	"github.com/wooyang2018/svp-blockchain/execution/common"
)

const (
	FileCodeXCoin   = "xcoin"
	FileCodeTAddr   = "taddr"
	FileCodeSRole   = "srole"
	FileCodeDefault = "default"
	FileNodekey     = "nodekey"
	CodePathDefault = "./builtin"
)

var (
	nodeUrl  = "http://127.0.0.1:9090"
	dataPath = "./"
	codeFile = FileCodeDefault
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
		common.DumpFile(owner.Bytes(), dataPath, FileNodekey)
		fmt.Println(owner.PublicKey().String())
	},
}

func SetNodeUrl(url string) {
	nodeUrl = url
}

func GetCodePath() string {
	return path.Join(dataPath, CodePathDefault)
}

func init() {
	RootCmd.AddCommand(accountCmd)
	RootCmd.PersistentFlags().StringVar(&nodeUrl, "node", nodeUrl, "blockchain node url")
	RootCmd.PersistentFlags().StringVar(&dataPath, "path", dataPath, "workspace data path")
	RootCmd.PersistentFlags().StringVar(&codeFile, "code", codeFile, "native chaincode address")
}
