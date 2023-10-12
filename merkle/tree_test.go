// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package merkle

import (
	"crypto"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewTree(t *testing.T) {
	tests := []struct {
		name    string
		bfactor uint8
		want    uint8
	}{
		{"branch factor < 2", 1, 2},
		{"branch factor >= 2", 4, 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree := NewTree(nil, Config{Hash: crypto.SHA1, BranchFactor: tt.bfactor})
			assert.Equal(t, tt.want, tree.config.BranchFactor)
		})
	}
}

func TestRoot(t *testing.T) {
	store := NewMapStore()
	tree := NewTree(store, Config{Hash: crypto.SHA1, BranchFactor: 2})

	asrt := assert.New(t)
	asrt.Nil(tree.Root())

	upd := &UpdateResult{
		LeafCount: big.NewInt(2),
		Height:    3,
		Leaves: []*Node{
			{NewPosition(0, big.NewInt(0)), []byte{1}},
			{NewPosition(0, big.NewInt(1)), []byte{2}},
			{NewPosition(0, big.NewInt(2)), []byte{3}},
		},
		Branches: []*Node{
			{NewPosition(1, big.NewInt(0)), []byte{4}},
			{NewPosition(1, big.NewInt(1)), []byte{5}},
			{NewPosition(2, big.NewInt(0)), []byte{6}},
		},
	}
	store.CommitUpdate(upd)

	asrt.Equal(upd.Branches[2], tree.Root())
}

func TestUpdate(t *testing.T) {
	asrt := assert.New(t)

	store := NewMapStore()
	tree := NewTree(store, Config{Hash: crypto.SHA1, BranchFactor: 3})

	leaves := make([]*Node, 7)
	for i := range leaves {
		leaves[i] = &Node{NewPosition(0, big.NewInt(int64(i))), []byte{uint8(i)}}
	}

	res := tree.Update(leaves, big.NewInt(7))
	asrt.EqualValues(3, res.Height)

	store.CommitUpdate(res)

	n10 := sha1Sum([]byte{0, 1, 2}) // level 0, index 1
	n11 := sha1Sum([]byte{3, 4, 5})
	n12 := sha1Sum([]byte{6})
	n20 := sha1Sum(append(n10, append(n11, n12...)...))

	asrt.Equal(11, len(store.nodes))
	asrt.Equal(n10, store.GetNode(NewPosition(1, big.NewInt(0))))
	asrt.Equal(n11, store.GetNode(NewPosition(1, big.NewInt(1))))
	asrt.Equal(n20, store.GetNode(NewPosition(2, big.NewInt(0))))

	upd := []*Node{
		{NewPosition(0, big.NewInt(2)), []byte{1}},
		{NewPosition(0, big.NewInt(5)), []byte{1}},
		{NewPosition(0, big.NewInt(7)), []byte{1}},
		{NewPosition(0, big.NewInt(8)), []byte{1}},
		{NewPosition(0, big.NewInt(9)), []byte{1}},
	}
	res = tree.Update(upd, big.NewInt(10))
	asrt.EqualValues(4, res.Height)

	store.CommitUpdate(res)

	n10 = sha1Sum([]byte{0, 1, 1})
	n11 = sha1Sum([]byte{3, 4, 1})
	n12 = sha1Sum([]byte{6, 1, 1})
	n13 := sha1Sum([]byte{1})
	n20 = sha1Sum(append(n10, append(n11, n12...)...))
	n21 := sha1Sum(n13)
	n30 := sha1Sum(append(n20, n21...))

	asrt.Equal(17, len(store.nodes))
	asrt.Equal(n10, store.GetNode(NewPosition(1, big.NewInt(0))))
	asrt.Equal(n11, store.GetNode(NewPosition(1, big.NewInt(1))))
	asrt.Equal(n12, store.GetNode(NewPosition(1, big.NewInt(2))))
	asrt.Equal(n13, store.GetNode(NewPosition(1, big.NewInt(3))))
	asrt.Equal(n20, store.GetNode(NewPosition(2, big.NewInt(0))))
	asrt.Equal(n21, store.GetNode(NewPosition(2, big.NewInt(1))))
	asrt.Equal(n30, store.GetNode(NewPosition(3, big.NewInt(0))))

	upd = []*Node{
		{NewPosition(0, big.NewInt(6)), []byte{2}},
	}

	// delete last 3 nodes
	res = tree.Update(upd, big.NewInt(7))
	asrt.EqualValues(3, res.Height)

	store.CommitUpdate(res)

	n12 = sha1Sum([]byte{2})
	n20 = sha1Sum(append(n10, append(n11, n12...)...))

	asrt.Equal(n12, store.GetNode(NewPosition(1, big.NewInt(2))))
	asrt.Equal(n20, store.GetNode(NewPosition(2, big.NewInt(0))))
}

func TestVerify(t *testing.T) {
	store := NewMapStore()
	tree := NewTree(store, Config{Hash: crypto.SHA1, BranchFactor: 3})

	leaves := make([]*Node, 7)
	for i := range leaves {
		leaves[i] = &Node{NewPosition(0, big.NewInt(int64(i))), []byte{uint8(i)}}
	}

	asrt := assert.New(t)
	asrt.False(tree.Verify(leaves)) // no root in tree

	res := tree.Update(leaves, big.NewInt(7))
	store.CommitUpdate(res)

	asrt.False(tree.Verify([]*Node{})) // no leaves to verify
	asrt.False(tree.Verify([]*Node{
		{NewPosition(1, big.NewInt(0)), []byte{1}}, // invalid level
	}))
	asrt.False(tree.Verify([]*Node{
		{NewPosition(0, big.NewInt(7)), []byte{7}}, // unbounded leaf
	}))
	asrt.True(tree.Verify(leaves)) // verify all leaves
	asrt.True(tree.Verify([]*Node{leaves[2]}))
	asrt.True(tree.Verify([]*Node{leaves[1], leaves[5]}))
	asrt.False(tree.Verify([]*Node{
		{leaves[1].Position, []byte{4}}, // one node invalid
		leaves[5],
	}))
	asrt.False(tree.Verify([]*Node{ // multiple node invalid
		{leaves[1].Position, []byte{4}},
		{leaves[5].Position, []byte{1}},
	}))
}
