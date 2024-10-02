// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package main

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/wooyang2018/svp-blockchain/execution/common"
	"github.com/wooyang2018/svp-blockchain/native"
	"github.com/wooyang2018/svp-blockchain/native/taddr"
)

const (
	FlagAddr string = "addr"
)

var (
	address string
)

var queryCmd = &cobra.Command{
	Use:   "query",
	Short: "Query the converted address of the given address",
	Run: func(cmd *cobra.Command, args []string) {
		client := native.NewClient(false)
		destBytes, _ := common.AddressToBytes(address)
		input := &taddr.Input{
			Method: "query",
			Addr:   destBytes,
		}
		rawBytes, _ := json.Marshal(input)
		fmt.Println(common.AddressToString(client.QueryState(rawBytes)))
	},
}

func main() {
	err := native.RootCmd.Execute()
	common.Check2(err)
}

func init() {
	native.RootCmd.AddCommand(queryCmd)
	queryCmd.PersistentFlags().StringVar(&address, FlagAddr, address, "destination address")
	queryCmd.MarkPersistentFlagRequired(FlagAddr)
}
