// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package common

type CodeDriver interface {
	// Install is called when code deployment transaction is received
	// Example data field - download url for code binary
	// After successful Install, getInstance should give a Chaincode instance without error
	Install(codeID, data []byte) error
	GetInstance(codeID []byte) (Chaincode, error)
}

type CallContext interface {
	Sender() []byte
	BlockHash() []byte
	BlockHeight() uint64
	Input() []byte

	GetState(key []byte) []byte
	SetState(key, value []byte)
}

// Chaincode all chaincodes implements this interface
type Chaincode interface {
	// Init is called when chaincode is deployed
	Init(ctx CallContext) error
	Invoke(ctx CallContext) error
	Query(ctx CallContext) ([]byte, error)
}
