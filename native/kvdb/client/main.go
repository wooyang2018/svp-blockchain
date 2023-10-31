// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package main

import (
	"encoding/base64"
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"

	"github.com/wooyang2018/svp-blockchain/core"
	"github.com/wooyang2018/svp-blockchain/native/kvdb"
	"github.com/wooyang2018/svp-blockchain/node"
)

const (
	FlagKey    string = "key"
	FlagValue  string = "value"
	FlagAction string = "action"

	FileCodeAddr string = "codeaddr"
)

var (
	nodeUrl = fmt.Sprintf("http://127.0.0.1:%d", node.DefaultConfig.APIPort)
	keyPath = "./"
)

var (
	key    string
	value  string
	action string
)

var rootCmd = &cobra.Command{
	Use:   "client",
	Short: "The client of kvdb native contract",
}

var getCmd = &cobra.Command{
	Use: "get",
	Run: func(cmd *cobra.Command, args []string) {
		client := NewClient(keyPath, false)
		query := client.MakeQuery(&kvdb.Input{
			Method: "get",
			Key:    []byte(key),
		})
		fmt.Println(string(client.QueryState(query)))
	},
}

var setCmd = &cobra.Command{
	Use: "set",
	Run: func(cmd *cobra.Command, args []string) {
		client := NewClient(keyPath, false)
		tx := client.MakeTx(&kvdb.Input{
			Method: "set",
			Key:    []byte(key),
			Value:  []byte(value),
		})
		client.SubmitTx(tx)
	},
}

var setupCmd = &cobra.Command{
	Use: "setup",
	Run: func(cmd *cobra.Command, args []string) {
		switch action {
		case "owner":
			client := NewClient(keyPath, false)
			query := client.MakeQuery(&kvdb.Input{
				Method: "owner",
			})
			owner := client.QueryState(query)
			fmt.Println(base64.StdEncoding.EncodeToString(owner))
		case "account":
			owner := core.GenerateKey(nil)
			dumpFile(owner.Bytes(), node.NodekeyFile)
			fmt.Println(owner.PublicKey().String())
		case "deploy":
			client := NewClient(keyPath, true)
			tx := client.MakeDeploymentTx()
			client.SubmitTx(tx)
			dumpFile(tx.Hash(), FileCodeAddr)
		default:
			fmt.Println("no support action")
		}
	},
}

func dumpFile(data []byte, path string) {
	f, err := os.Create(path)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer f.Close()
	if _, err = f.Write(data); err != nil {
		fmt.Println(err)
	}
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func init() {
	rootCmd.AddCommand(getCmd)
	rootCmd.AddCommand(setCmd)
	rootCmd.AddCommand(setupCmd)

	rootCmd.PersistentFlags().StringVar(&nodeUrl,
		"node-url", nodeUrl, "blockchain node url")
	rootCmd.PersistentFlags().StringVar(&keyPath,
		"key-path", keyPath, "private key path")

	getCmd.PersistentFlags().StringVar(&key, FlagKey, key, "key of query")
	getCmd.MarkPersistentFlagRequired(FlagKey)

	setCmd.PersistentFlags().StringVar(&key, FlagKey, key, "key of set command")
	setCmd.MarkPersistentFlagRequired(FlagKey)
	setCmd.PersistentFlags().StringVar(&value, FlagValue, value, "value of set command")
	setCmd.MarkPersistentFlagRequired(FlagValue)

	setupCmd.PersistentFlags().StringVar(&action, FlagAction, action, "action of setup command")
	setupCmd.MarkPersistentFlagRequired(FlagAction)
}
