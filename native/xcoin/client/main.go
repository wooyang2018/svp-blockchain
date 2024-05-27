// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package main

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/spf13/cobra"

	"github.com/wooyang2018/svp-blockchain/execution/common"
	"github.com/wooyang2018/svp-blockchain/native"
	"github.com/wooyang2018/svp-blockchain/native/xcoin"
)

const (
	FlagDest  string = "dest"
	FlagValue string = "value"
)

var (
	dest  string
	value uint64
)

var balanceCmd = &cobra.Command{
	Use:   "balance",
	Short: "Query the balance of the given public key",
	Run: func(cmd *cobra.Command, args []string) {
		client := native.NewClient(false)
		destBytes, _ := common.Address32ToBytes(dest)
		input := &xcoin.Input{
			Method: "balance",
			Dest:   destBytes,
		}
		rawBytes, _ := json.Marshal(input)
		fmt.Println(common.DecodeBalance(client.QueryState(rawBytes)))
	},
}

var transferCmd = &cobra.Command{
	Use:   "transfer",
	Short: "Transfer token to the given public key",
	Run: func(cmd *cobra.Command, args []string) {
		client := native.NewClient(false)
		destBytes, _ := common.Address32ToBytes(dest)
		input := &xcoin.Input{
			Method: "transfer",
			Dest:   destBytes,
			Value:  value,
		}
		rawBytes, _ := json.Marshal(input)
		tx := client.MakeTx(rawBytes)
		client.SubmitTx(tx)
	},
}

func main() {
	if err := native.RootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func init() {
	native.RootCmd.AddCommand(balanceCmd)
	native.RootCmd.AddCommand(transferCmd)

	balanceCmd.PersistentFlags().StringVar(&dest, FlagDest, dest, "destination public key")
	balanceCmd.MarkPersistentFlagRequired(FlagDest)

	transferCmd.PersistentFlags().StringVar(&dest, FlagDest, dest, "destination public key")
	transferCmd.MarkPersistentFlagRequired(FlagDest)
	transferCmd.PersistentFlags().Uint64Var(&value, FlagValue, value, "value of transfer command")
	transferCmd.MarkPersistentFlagRequired(FlagValue)
}
