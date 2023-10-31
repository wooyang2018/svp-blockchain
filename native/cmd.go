// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package native

import (
	"fmt"
	"log"
	"os"
	"path"

	"github.com/spf13/cobra"

	"github.com/wooyang2018/svp-blockchain/core"
)

const (
	FileCodeAddr = "codeaddr"
	FileNodekey  = "nodekey"
)

var (
	NodeUrl  = "http://127.0.0.1:9040"
	DataPath = "./"
)

var RootCmd = &cobra.Command{
	Use:   "client",
	Short: "The client of native contract",
}

var accountCmd = &cobra.Command{
	Use:   "account",
	Short: "Generate the nodekey file for the account",
	Run: func(cmd *cobra.Command, args []string) {
		owner := core.GenerateKey(nil)
		DumpFile(owner.Bytes(), FileNodekey)
		fmt.Println(owner.PublicKey().String())
	},
}

func DumpFile(data []byte, file string) {
	if !Exists(DataPath) {
		check(os.MkdirAll(DataPath, os.ModePerm))
	}
	f, err := os.Create(path.Join(DataPath, file))
	check(err)
	defer f.Close()
	_, err = f.Write(data)
	check(err)
}

func Exists(path string) bool {
	_, err := os.Stat(path)
	if err != nil {
		if os.IsExist(err) {
			return true
		}
		return false
	}
	return true
}

func check(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func init() {
	RootCmd.AddCommand(accountCmd)
	RootCmd.PersistentFlags().StringVar(&NodeUrl,
		"node", NodeUrl, "blockchain node url")
	RootCmd.PersistentFlags().StringVar(&DataPath,
		"path", DataPath, "workspace data path")
}
