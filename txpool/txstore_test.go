// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package txpool

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/wooyang2018/svp-blockchain/core"
)

func TestTxStore_addNewTx(t *testing.T) {
	asrt := assert.New(t)

	tx := core.NewTransaction().Sign(core.GenerateKey(nil))
	store := newTxStore()
	store.addNewTx(tx)

	asrt.Equal(1, store.getStatus().Total)
	asrt.Equal(1, store.getStatus().Queue)
	asrt.Equal(0, store.getStatus().Pending)

	txItem := store.txItems[string(tx.Hash())]

	asrt.Equal(0, txItem.index)

	// add the same tx again and should not accept
	store.addNewTx(tx)

	asrt.Nil(store.getTx([]byte("notexist")))
	asrt.NotNil(store.getTx(tx.Hash()))
	asrt.Equal(1, store.getStatus().Total)
	asrt.Equal(1, store.getStatus().Queue)
	asrt.Equal(0, store.getStatus().Pending)

	txItem1 := store.txItems[string(tx.Hash())]

	asrt.Equal(txItem, txItem1)
	asrt.Equal(txItem.receivedTime, txItem1.receivedTime)
}

func TestTxStore_popTxsFromQueue(t *testing.T) {
	asrt := assert.New(t)

	priv := core.GenerateKey(nil)
	tx1 := core.NewTransaction().SetNonce(4).Sign(priv)
	tx2 := core.NewTransaction().SetNonce(3).Sign(priv)
	tx3 := core.NewTransaction().SetNonce(6).Sign(priv)
	tx4 := core.NewTransaction().SetNonce(2).Sign(priv)

	store := newTxStore()

	store.addNewTx(tx1)
	time.Sleep(1 * time.Microsecond)
	store.addNewTx(tx2)
	time.Sleep(1 * time.Microsecond)
	store.addNewTx(tx3)
	time.Sleep(1 * time.Microsecond)
	store.addNewTx(tx4)

	hashes := store.popTxsFromQueue(2)

	asrt.Equal(2, len(hashes))
	asrt.Equal(tx1.Hash(), hashes[0])
	asrt.Equal(tx2.Hash(), hashes[1])

	asrt.False(store.txItems[string(tx1.Hash())].inQueue())
	asrt.False(store.txItems[string(tx2.Hash())].inQueue())

	asrt.Equal(4, store.getStatus().Total)
	asrt.Equal(2, store.getStatus().Queue)
	asrt.Equal(2, store.getStatus().Pending)

	hashes = store.popTxsFromQueue(3)

	asrt.False(store.txItems[string(tx3.Hash())].inQueue())
	asrt.False(store.txItems[string(tx4.Hash())].inQueue())

	asrt.Equal(2, len(hashes))
	asrt.Equal(tx3.Hash(), hashes[0])
	asrt.Equal(tx4.Hash(), hashes[1])

	asrt.Equal(4, store.getStatus().Total)
	asrt.Equal(0, store.getStatus().Queue)
	asrt.Equal(4, store.getStatus().Pending)

	hashes = store.popTxsFromQueue(2)
	asrt.Nil(hashes)
}

func TestTxStore_putTxsToQueue(t *testing.T) {
	asrt := assert.New(t)

	priv := core.GenerateKey(nil)
	tx1 := core.NewTransaction().SetNonce(4).Sign(priv)
	tx2 := core.NewTransaction().SetNonce(3).Sign(priv)
	tx3 := core.NewTransaction().SetNonce(6).Sign(priv)
	tx4 := core.NewTransaction().SetNonce(2).Sign(priv)

	store := newTxStore()

	store.addNewTx(tx1)
	time.Sleep(1 * time.Microsecond)
	store.addNewTx(tx2)
	time.Sleep(1 * time.Microsecond)
	store.addNewTx(tx3)
	time.Sleep(1 * time.Microsecond)
	store.addNewTx(tx4)

	store.popTxsFromQueue(3)

	store.putTxsToQueue([][]byte{tx2.Hash(), tx3.Hash()})

	asrt.Equal(3, store.getStatus().Queue)

	hashes := store.popTxsFromQueue(2)

	asrt.Equal(tx2.Hash(), hashes[0])
	asrt.Equal(tx3.Hash(), hashes[1])

	store.putTxsToQueue([][]byte{tx1.Hash()})

	asrt.Equal(2, store.getStatus().Queue)

	hashes = store.popTxsFromQueue(2)

	asrt.Equal(tx1.Hash(), hashes[0])
	asrt.Equal(tx4.Hash(), hashes[1])
}

func TestTxStore_setTxsPending(t *testing.T) {
	asrt := assert.New(t)

	priv := core.GenerateKey(nil)
	tx1 := core.NewTransaction().SetNonce(4).Sign(priv)
	tx2 := core.NewTransaction().SetNonce(3).Sign(priv)
	tx3 := core.NewTransaction().SetNonce(6).Sign(priv)
	tx4 := core.NewTransaction().SetNonce(2).Sign(priv)

	store := newTxStore()

	store.addNewTx(tx1)
	time.Sleep(1 * time.Microsecond)
	store.addNewTx(tx2)
	time.Sleep(1 * time.Microsecond)
	store.addNewTx(tx3)
	time.Sleep(1 * time.Microsecond)
	store.addNewTx(tx4)

	store.setTxsPending([][]byte{tx2.Hash(), tx4.Hash()})

	asrt.Equal(2, store.getStatus().Pending)
	asrt.Equal(2, store.getStatus().Queue)

	asrt.False(store.txItems[string(tx2.Hash())].inQueue())
	asrt.False(store.txItems[string(tx4.Hash())].inQueue())

	hashes := store.popTxsFromQueue(3)

	asrt.Equal(2, len(hashes))
	asrt.Equal(tx1.Hash(), hashes[0])
	asrt.Equal(tx3.Hash(), hashes[1])
}

func TestTxStore_removeTxs(t *testing.T) {
	asrt := assert.New(t)

	priv := core.GenerateKey(nil)
	tx1 := core.NewTransaction().SetNonce(4).Sign(priv)
	tx2 := core.NewTransaction().SetNonce(3).Sign(priv)
	tx3 := core.NewTransaction().SetNonce(6).Sign(priv)
	tx4 := core.NewTransaction().SetNonce(2).Sign(priv)

	store := newTxStore()

	store.addNewTx(tx1)
	time.Sleep(1 * time.Microsecond)
	store.addNewTx(tx2)
	time.Sleep(1 * time.Microsecond)
	store.addNewTx(tx3)
	time.Sleep(1 * time.Microsecond)
	store.addNewTx(tx4)

	store.popTxsFromQueue(2)

	store.removeTxs([][]byte{tx2.Hash(), tx4.Hash()})

	asrt.Equal(2, store.getStatus().Total)
	asrt.Equal(1, store.getStatus().Queue)
	asrt.Equal(1, store.getStatus().Pending)

	hashes := store.popTxsFromQueue(3)

	asrt.Equal(1, len(hashes))
	asrt.Equal(tx3.Hash(), hashes[0])
}
