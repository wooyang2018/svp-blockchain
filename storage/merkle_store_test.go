// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package storage

import (
	"math/big"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wooyang2018/posv-blockchain/merkle"
)

func TestMerkleStore(t *testing.T) {
	asrt := assert.New(t)

	dir, _ := os.MkdirTemp("", "db")
	rawDB, _ := NewLevelDB(dir)
	db := &levelDB{rawDB}
	ms := &merkleStore{db}

	asrt.Equal(uint8(0), ms.GetHeight())
	asrt.Equal(big.NewInt(0), ms.GetLeafCount())
	asrt.Nil(ms.GetNode(merkle.NewPosition(0, big.NewInt(0))))

	upd := &merkle.UpdateResult{
		LeafCount: big.NewInt(2),
		Height:    2,
		Leaves: []*merkle.Node{
			{Position: merkle.NewPosition(0, big.NewInt(0)), Data: []byte{1, 1}},
			{Position: merkle.NewPosition(0, big.NewInt(1)), Data: []byte{2, 2}},
		},
		Branches: []*merkle.Node{
			{Position: merkle.NewPosition(1, big.NewInt(0)), Data: []byte{3, 3}},
		},
	}

	updateLevelDB(db, ms.commitUpdate(upd))

	asrt.Equal(upd.Height, ms.GetHeight())
	asrt.Equal(upd.LeafCount, ms.GetLeafCount())
	asrt.Equal([]byte{1, 1}, ms.GetNode(merkle.NewPosition(0, big.NewInt(0))))
	asrt.Equal([]byte{2, 2}, ms.GetNode(merkle.NewPosition(0, big.NewInt(1))))
	asrt.Equal([]byte{3, 3}, ms.GetNode(merkle.NewPosition(1, big.NewInt(0))))
}
