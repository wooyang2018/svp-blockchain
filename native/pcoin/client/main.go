// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package main

import (
	"encoding/json"
	"fmt"

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

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy the pcoin native contract",
	Run: func(cmd *cobra.Command, args []string) {
		client := native.NewClient(true)
		tx := client.MakeDeploymentTx(common.DriverTypeNative, native.CodePCoin)
		client.SubmitTx(tx)
		common.DumpFile(tx.Hash(), native.DataPath, native.FileCodeDefault)
	},
}

var mintCmd = &cobra.Command{
	Use:   "mint",
	Short: "Minter mines tokens for the dest account",
	Run: func(cmd *cobra.Command, args []string) {
		client := native.NewClient(false)
		destBytes, _ := common.Address32ToBytes(dest)
		input := &xcoin.Input{
			Method: "mint",
			Dest:   destBytes,
			Value:  value,
		}
		rawBytes, _ := json.Marshal(input)
		tx := client.MakeTx(rawBytes)
		client.SubmitTx(tx)
	},
}

var transferCmd = &cobra.Command{
	Use:   "transfer",
	Short: "Transfer tokens to the given public key",
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

var minterCmd = &cobra.Command{
	Use:   "minter",
	Short: "Query the public key of minter",
	Run: func(cmd *cobra.Command, args []string) {
		client := native.NewClient(false)
		input := &xcoin.Input{
			Method: "minter",
		}
		rawBytes, _ := json.Marshal(input)
		fmt.Println(common.AddressToString(client.QueryState(rawBytes)))
	},
}

var totalCmd = &cobra.Command{
	Use:   "total",
	Short: "Query the total number of tokens",
	Run: func(cmd *cobra.Command, args []string) {
		client := native.NewClient(false)
		input := &xcoin.Input{
			Method: "total",
		}
		rawBytes, _ := json.Marshal(input)
		fmt.Println(common.DecodeBalance(client.QueryState(rawBytes)))
	},
}

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

func main() {
	err := native.RootCmd.Execute()
	common.Check(err)
}

func init() {
	native.RootCmd.AddCommand(deployCmd)
	native.RootCmd.AddCommand(mintCmd)
	native.RootCmd.AddCommand(transferCmd)
	native.RootCmd.AddCommand(minterCmd)
	native.RootCmd.AddCommand(totalCmd)
	native.RootCmd.AddCommand(balanceCmd)

	mintCmd.PersistentFlags().StringVar(&dest, FlagDest, dest, "destination public key")
	mintCmd.MarkPersistentFlagRequired(FlagDest)
	mintCmd.PersistentFlags().Uint64Var(&value, FlagValue, value, "value of tokens")
	mintCmd.MarkPersistentFlagRequired(FlagValue)

	transferCmd.PersistentFlags().StringVar(&dest, FlagDest, dest, "destination public key")
	transferCmd.MarkPersistentFlagRequired(FlagDest)
	transferCmd.PersistentFlags().Uint64Var(&value, FlagValue, value, "value of tokens")
	transferCmd.MarkPersistentFlagRequired(FlagValue)

	balanceCmd.PersistentFlags().StringVar(&dest, FlagDest, dest, "destination public key")
	balanceCmd.MarkPersistentFlagRequired(FlagDest)
}
