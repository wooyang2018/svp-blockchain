// Copyright (C) 2024 Wooyang2018
// Licensed under the GNU General Public License v3.0

package main

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/wooyang2018/svp-blockchain/execution/common"
	"github.com/wooyang2018/svp-blockchain/native"
	"github.com/wooyang2018/svp-blockchain/native/srole"
)

const (
	FlagAddr  string = "addr"
	FlagQuota string = "quota"
)

var (
	address string
	quota   uint64
)

var printCmd = &cobra.Command{
	Use:   "print",
	Short: "Print the content of genesis file",
	Run: func(cmd *cobra.Command, args []string) {
		client := native.NewClient(false)
		input := &srole.Input{
			Method: "print",
		}
		rawBytes, _ := json.Marshal(input)
		fmt.Println(string(client.QueryState(rawBytes)))
	},
}

var deleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a validator from role store",
	Run: func(cmd *cobra.Command, args []string) {
		client := native.NewClient(false)
		destBytes, _ := common.AddressToBytes(address)
		input := &srole.Input{
			Method: "delete",
			Addr:   destBytes,
		}
		rawBytes, _ := json.Marshal(input)
		tx := client.MakeTx(rawBytes)
		client.SubmitTx(tx)
	},
}

var addCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a validator to role store",
	Run: func(cmd *cobra.Command, args []string) {
		client := native.NewClient(false)
		destBytes, _ := common.AddressToBytes(address)
		input := &srole.Input{
			Method: "add",
			Addr:   destBytes,
			Quota:  quota,
		}
		rawBytes, _ := json.Marshal(input)
		tx := client.MakeTx(rawBytes)
		client.SubmitTx(tx)
	},
}

func main() {
	err := native.RootCmd.Execute()
	common.Check2(err)
}

func init() {
	native.RootCmd.AddCommand(printCmd)

	native.RootCmd.AddCommand(deleteCmd)
	deleteCmd.PersistentFlags().StringVar(&address, FlagAddr, address, "destination address")
	deleteCmd.MarkPersistentFlagRequired(FlagAddr)

	native.RootCmd.AddCommand(addCmd)
	addCmd.PersistentFlags().StringVar(&address, FlagAddr, address, "destination address")
	addCmd.MarkPersistentFlagRequired(FlagAddr)
	addCmd.PersistentFlags().Uint64Var(&quota, FlagQuota, quota, "value of validator quota")
	addCmd.MarkPersistentFlagRequired(FlagQuota)
}
