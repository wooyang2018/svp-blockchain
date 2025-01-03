// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package bincc

type CallType int

const (
	CallTypeInit CallType = iota
	CallTypeInvoke
	CallTypeQuery
)

type CallData struct {
	Input           []byte
	Sender          []byte
	TransactionHash []byte
	BlockHash       []byte
	BlockHeight     uint64
	CallType        CallType
}

type UpStreamType int

const (
	UpStreamGetState UpStreamType = iota
	UpStreamSetState
	UpStreamResult
)

type UpStream struct {
	Key   []byte
	Value []byte
	Error string
	Type  UpStreamType
}

type DownStream struct {
	Value []byte
	Error string
}
