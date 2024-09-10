// Copyright 2017 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package common

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math/big"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
)

const jsondata = `
[
	{ "type" : "function", "name" : ""},
	{ "type" : "function", "name" : "balance", "stateMutability" : "view" },
	{ "type" : "function", "name" : "send", "inputs" : [ { "name" : "amount", "type" : "uint256" } ] },
	{ "type" : "function", "name" : "test", "inputs" : [ { "name" : "number", "type" : "uint32" } ] },
	{ "type" : "function", "name" : "string", "inputs" : [ { "name" : "inputs", "type" : "string" } ] },
	{ "type" : "function", "name" : "bool", "inputs" : [ { "name" : "inputs", "type" : "bool" } ] },
	{ "type" : "function", "name" : "address", "inputs" : [ { "name" : "inputs", "type" : "address" } ] },
	{ "type" : "function", "name" : "uint64[2]", "inputs" : [ { "name" : "inputs", "type" : "uint64[2]" } ] },
	{ "type" : "function", "name" : "uint64[]", "inputs" : [ { "name" : "inputs", "type" : "uint64[]" } ] },
	{ "type" : "function", "name" : "int8", "inputs" : [ { "name" : "inputs", "type" : "int8" } ] },
	{ "type" : "function", "name" : "bytes32", "inputs" : [ { "name" : "inputs", "type" : "bytes32" } ] },
	{ "type" : "function", "name" : "foo", "inputs" : [ { "name" : "inputs", "type" : "uint32" } ] },
	{ "type" : "function", "name" : "bar", "inputs" : [ { "name" : "inputs", "type" : "uint32" }, { "name" : "string", "type" : "uint16" } ] },
	{ "type" : "function", "name" : "slice", "inputs" : [ { "name" : "inputs", "type" : "uint32[2]" } ] },
	{ "type" : "function", "name" : "slice256", "inputs" : [ { "name" : "inputs", "type" : "uint256[2]" } ] },
	{ "type" : "function", "name" : "sliceAddress", "inputs" : [ { "name" : "inputs", "type" : "address[]" } ] },
	{ "type" : "function", "name" : "sliceMultiAddress", "inputs" : [ { "name" : "a", "type" : "address[]" }, { "name" : "b", "type" : "address[]" } ] },
	{ "type" : "function", "name" : "nestedArray", "inputs" : [ { "name" : "a", "type" : "uint256[2][2]" }, { "name" : "b", "type" : "address[]" } ] },
	{ "type" : "function", "name" : "nestedArray2", "inputs" : [ { "name" : "a", "type" : "uint8[][2]" } ] },
	{ "type" : "function", "name" : "nestedSlice", "inputs" : [ { "name" : "a", "type" : "uint8[][]" } ] },
	{ "type" : "function", "name" : "receive", "inputs" : [ { "name" : "memo", "type" : "bytes" }], "outputs" : [], "payable" : true, "stateMutability" : "payable" },
	{ "type" : "function", "name" : "fixedArrStr", "stateMutability" : "view", "inputs" : [ { "name" : "str", "type" : "string" }, { "name" : "fixedArr", "type" : "uint256[2]" } ] },
	{ "type" : "function", "name" : "fixedArrBytes", "stateMutability" : "view", "inputs" : [ { "name" : "bytes", "type" : "bytes" }, { "name" : "fixedArr", "type" : "uint256[2]" } ] },
	{ "type" : "function", "name" : "mixedArrStr", "stateMutability" : "view", "inputs" : [ { "name" : "str", "type" : "string" }, { "name" : "fixedArr", "type" : "uint256[2]" }, { "name" : "dynArr", "type" : "uint256[]" } ] },
	{ "type" : "function", "name" : "doubleFixedArrStr", "stateMutability" : "view", "inputs" : [ { "name" : "str", "type" : "string" }, { "name" : "fixedArr1", "type" : "uint256[2]" }, { "name" : "fixedArr2", "type" : "uint256[3]" } ] },
	{ "type" : "function", "name" : "multipleMixedArrStr", "stateMutability" : "view", "inputs" : [ { "name" : "str", "type" : "string" }, { "name" : "fixedArr1", "type" : "uint256[2]" }, { "name" : "dynArr", "type" : "uint256[]" }, { "name" : "fixedArr2", "type" : "uint256[3]" } ] },
	{ "type" : "function", "name" : "overloadedNames", "stateMutability" : "view", "inputs": [ { "components": [ { "internalType": "uint256", "name": "_f",	"type": "uint256" }, { "internalType": "uint256", "name": "__f", "type": "uint256"}, { "internalType": "uint256", "name": "f", "type": "uint256"}],"internalType": "struct Overloader.F", "name": "f","type": "tuple"}]}
]`

// TestPack tests the general pack/unpack tests in packing_test.go
func TestPack(t *testing.T) {
	for i, test := range packUnpackTests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			encb, err := hex.DecodeString(test.packed)
			if err != nil {
				t.Fatalf("invalid hex %s: %v", test.packed, err)
			}
			inDef := fmt.Sprintf(`[{ "name" : "method", "type": "function", "inputs": %s}]`, test.def)
			inAbi, err := abi.JSON(strings.NewReader(inDef))
			if err != nil {
				t.Fatalf("invalid ABI definition %s, %v", inDef, err)
			}
			var packed []byte
			packed, err = inAbi.Pack("method", test.unpacked)

			if err != nil {
				t.Fatalf("test %d (%v) failed: %v", i, test.def, err)
			}
			if !reflect.DeepEqual(packed[4:], encb) {
				t.Errorf("test %d (%v) failed: expected %v, got %v", i, test.def, encb, packed[4:])
			}
		})
	}
}

func TestMethodPack(t *testing.T) {
	abi, err := abi.JSON(strings.NewReader(jsondata))
	if err != nil {
		t.Fatal(err)
	}

	sig := abi.Methods["slice"].ID
	sig = append(sig, common.LeftPadBytes([]byte{1}, 32)...)
	sig = append(sig, common.LeftPadBytes([]byte{2}, 32)...)

	packed, err := abi.Pack("slice", []uint32{1, 2})
	if err != nil {
		t.Error(err)
	}

	if !bytes.Equal(packed, sig) {
		t.Errorf("expected %x got %x", sig, packed)
	}

	var addrA, addrB = common.Address{1}, common.Address{2}
	sig = abi.Methods["sliceAddress"].ID
	sig = append(sig, common.LeftPadBytes([]byte{32}, 32)...)
	sig = append(sig, common.LeftPadBytes([]byte{2}, 32)...)
	sig = append(sig, common.LeftPadBytes(addrA[:], 32)...)
	sig = append(sig, common.LeftPadBytes(addrB[:], 32)...)

	packed, err = abi.Pack("sliceAddress", []common.Address{addrA, addrB})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(packed, sig) {
		t.Errorf("expected %x got %x", sig, packed)
	}

	var addrC, addrD = common.Address{3}, common.Address{4}
	sig = abi.Methods["sliceMultiAddress"].ID
	sig = append(sig, common.LeftPadBytes([]byte{64}, 32)...)
	sig = append(sig, common.LeftPadBytes([]byte{160}, 32)...)
	sig = append(sig, common.LeftPadBytes([]byte{2}, 32)...)
	sig = append(sig, common.LeftPadBytes(addrA[:], 32)...)
	sig = append(sig, common.LeftPadBytes(addrB[:], 32)...)
	sig = append(sig, common.LeftPadBytes([]byte{2}, 32)...)
	sig = append(sig, common.LeftPadBytes(addrC[:], 32)...)
	sig = append(sig, common.LeftPadBytes(addrD[:], 32)...)

	packed, err = abi.Pack("sliceMultiAddress", []common.Address{addrA, addrB}, []common.Address{addrC, addrD})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(packed, sig) {
		t.Errorf("expected %x got %x", sig, packed)
	}

	sig = abi.Methods["slice256"].ID
	sig = append(sig, common.LeftPadBytes([]byte{1}, 32)...)
	sig = append(sig, common.LeftPadBytes([]byte{2}, 32)...)

	packed, err = abi.Pack("slice256", []*big.Int{big.NewInt(1), big.NewInt(2)})
	if err != nil {
		t.Error(err)
	}

	if !bytes.Equal(packed, sig) {
		t.Errorf("expected %x got %x", sig, packed)
	}

	a := [2][2]*big.Int{{big.NewInt(1), big.NewInt(1)}, {big.NewInt(2), big.NewInt(0)}}
	sig = abi.Methods["nestedArray"].ID
	sig = append(sig, common.LeftPadBytes([]byte{1}, 32)...)
	sig = append(sig, common.LeftPadBytes([]byte{1}, 32)...)
	sig = append(sig, common.LeftPadBytes([]byte{2}, 32)...)
	sig = append(sig, common.LeftPadBytes([]byte{0}, 32)...)
	sig = append(sig, common.LeftPadBytes([]byte{0xa0}, 32)...)
	sig = append(sig, common.LeftPadBytes([]byte{2}, 32)...)
	sig = append(sig, common.LeftPadBytes(addrC[:], 32)...)
	sig = append(sig, common.LeftPadBytes(addrD[:], 32)...)
	packed, err = abi.Pack("nestedArray", a, []common.Address{addrC, addrD})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(packed, sig) {
		t.Errorf("expected %x got %x", sig, packed)
	}

	sig = abi.Methods["nestedArray2"].ID
	sig = append(sig, common.LeftPadBytes([]byte{0x20}, 32)...)
	sig = append(sig, common.LeftPadBytes([]byte{0x40}, 32)...)
	sig = append(sig, common.LeftPadBytes([]byte{0x80}, 32)...)
	sig = append(sig, common.LeftPadBytes([]byte{1}, 32)...)
	sig = append(sig, common.LeftPadBytes([]byte{1}, 32)...)
	sig = append(sig, common.LeftPadBytes([]byte{1}, 32)...)
	sig = append(sig, common.LeftPadBytes([]byte{1}, 32)...)
	packed, err = abi.Pack("nestedArray2", [2][]uint8{{1}, {1}})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(packed, sig) {
		t.Errorf("expected %x got %x", sig, packed)
	}

	sig = abi.Methods["nestedSlice"].ID
	sig = append(sig, common.LeftPadBytes([]byte{0x20}, 32)...)
	sig = append(sig, common.LeftPadBytes([]byte{0x02}, 32)...)
	sig = append(sig, common.LeftPadBytes([]byte{0x40}, 32)...)
	sig = append(sig, common.LeftPadBytes([]byte{0xa0}, 32)...)
	sig = append(sig, common.LeftPadBytes([]byte{2}, 32)...)
	sig = append(sig, common.LeftPadBytes([]byte{1}, 32)...)
	sig = append(sig, common.LeftPadBytes([]byte{2}, 32)...)
	sig = append(sig, common.LeftPadBytes([]byte{2}, 32)...)
	sig = append(sig, common.LeftPadBytes([]byte{1}, 32)...)
	sig = append(sig, common.LeftPadBytes([]byte{2}, 32)...)
	packed, err = abi.Pack("nestedSlice", [][]uint8{{1, 2}, {1, 2}})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(packed, sig) {
		t.Errorf("expected %x got %x", sig, packed)
	}
}
