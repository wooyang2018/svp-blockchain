// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package core

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTransaction(t *testing.T) {
	privKey := GenerateKey(nil)
	nonce := time.Now().UnixNano()
	tx := NewTransaction().
		SetNonce(nonce).
		SetCodeAddr([]byte{1}).
		SetInput([]byte{2}).
		Sign(privKey)

	asrt := assert.New(t)
	asrt.Equal(nonce, tx.Nonce())
	asrt.Equal([]byte{1}, tx.CodeAddr())
	asrt.Equal([]byte{2}, tx.Input())
	asrt.Equal(privKey.PublicKey(), tx.Sender())
	asrt.Equal(privKey.PublicKey().Bytes(), tx.data.Sender)
	asrt.NoError(tx.Validate())

	b, err := tx.Marshal()
	asrt.NoError(err)
	tx = NewTransaction()
	err = tx.Unmarshal(b)
	asrt.NoError(err)
	asrt.NoError(tx.Validate())

	b, err = json.Marshal(tx)
	asrt.NoError(err)
	tx = NewTransaction()
	err = json.Unmarshal(b, tx)
	asrt.NoError(err)
	asrt.NoError(tx.Validate())
}

func TestTxList(t *testing.T) {
	privKey := GenerateKey(nil)
	tx1 := NewTransaction().
		SetNonce(time.Now().UnixNano()).
		SetCodeAddr([]byte{1}).
		SetInput([]byte{1}).
		Sign(privKey)
	tx2 := NewTransaction().
		SetNonce(time.Now().UnixNano()).
		SetCodeAddr([]byte{2}).
		SetInput([]byte{2}).
		Sign(privKey)

	asrt := assert.New(t)
	txs := &TxList{tx1, tx2}
	b, err := txs.Marshal()
	asrt.NoError(err)

	txs = NewTxList()
	err = txs.Unmarshal(b)
	asrt.NoError(err)

	asrt.Equal(2, len(*txs))
	asrt.Equal(tx1.Sum(), (*txs)[0].Sum())
	asrt.Equal(tx2.Sum(), (*txs)[1].Sum())
}
