// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package execution

import "github.com/wooyang2018/svp-blockchain/execution/common"

// stateVerifier is used for state query calls
// it calls the VerifyState of state store instead of GetState
// to verify the state value with the merkle root
type stateVerifier struct {
	store     common.StateStore
	keyPrefix []byte
}

func newStateVerifier(store common.StateStore, prefix []byte) *stateVerifier {
	return &stateVerifier{
		store:     store,
		keyPrefix: prefix,
	}
}

func (sv *stateVerifier) GetState(key []byte) []byte {
	key = common.ConcatBytes(sv.keyPrefix, key)
	return sv.store.VerifyState(key)
}
