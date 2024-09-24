// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/wooyang2018/svp-blockchain/execution/common"
	"github.com/wooyang2018/svp-blockchain/native"
	"github.com/wooyang2018/svp-blockchain/native/kvdb"
)

const (
	FlagKey   string = "key"
	FlagValue string = "value"
)

var (
	key   string
	value string
)

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy the kvdb native contract",
	Run: func(cmd *cobra.Command, args []string) {
		client := native.NewClient(true)
		tx := client.MakeDeploymentTx(common.DriverTypeNative, native.CodeKVDB, nil)
		client.SubmitTx(tx)
		common.DumpFile(tx.Hash(), native.GetDataPath(), native.FileCodeDefault)
	},
}

var ownerCmd = &cobra.Command{
	Use:   "owner",
	Short: "Query the owner of the deployed contract",
	Run: func(cmd *cobra.Command, args []string) {
		client := native.NewClient(false)
		input := &kvdb.Input{Method: "owner"}
		rawBytes, _ := json.Marshal(input)
		owner := client.QueryState(rawBytes)
		fmt.Println(base64.StdEncoding.EncodeToString(owner))
	},
}

var getCmd = &cobra.Command{
	Use:   "get",
	Short: "Query the value of the given key",
	Run: func(cmd *cobra.Command, args []string) {
		client := native.NewClient(false)
		input := &kvdb.Input{
			Method: "get",
			Key:    []byte(key),
		}
		rawBytes, _ := json.Marshal(input)
		fmt.Println(string(client.QueryState(rawBytes)))
	},
}

var setCmd = &cobra.Command{
	Use:   "set",
	Short: "Set the given key to the given value",
	Run: func(cmd *cobra.Command, args []string) {
		client := native.NewClient(false)
		input := &kvdb.Input{
			Method: "set",
			Key:    []byte(key),
			Value:  []byte(value),
		}
		rawBytes, _ := json.Marshal(input)
		tx := client.MakeTx(rawBytes)
		client.SubmitTx(tx)
	},
}

func main() {
	err := native.RootCmd.Execute()
	common.Check(err)
}

func init() {
	native.RootCmd.AddCommand(deployCmd)
	native.RootCmd.AddCommand(ownerCmd)
	native.RootCmd.AddCommand(getCmd)
	native.RootCmd.AddCommand(setCmd)

	getCmd.PersistentFlags().StringVar(&key, FlagKey, key, "key of query")
	getCmd.MarkPersistentFlagRequired(FlagKey)

	setCmd.PersistentFlags().StringVar(&key, FlagKey, key, "key of set command")
	setCmd.MarkPersistentFlagRequired(FlagKey)
	setCmd.PersistentFlags().StringVar(&value, FlagValue, value, "value of set command")
	setCmd.MarkPersistentFlagRequired(FlagValue)
}
