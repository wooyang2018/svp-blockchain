// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package consensus

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wooyang2018/posv-blockchain/core"
)

func setupRotator() (*rotator, *core.Proposal) {
	key1 := core.GenerateKey(nil)
	key2 := core.GenerateKey(nil)
	workers := []string{
		key1.PublicKey().String(),
		key2.PublicKey().String(),
	}
	weights := []int{1, 1}
	resources := &Resources{
		VldStore: core.NewValidatorStore(workers, weights, workers),
	}

	b0 := core.NewProposal().Sign(key1)
	q0 := core.NewQuorumCert().Build([]*core.Vote{b0.Vote(key1)})
	b0.SetQuorumCert(q0)

	state := newState(resources)
	state.setBlock(b0.Block())
	driver := &driver{
		resources: resources,
		state:     state,
	}
	posvState := newInnerState(newBlock(b0.Block(), state), newQC(q0, state))
	tester := newTester(nil)
	driver.posvState = posvState
	driver.tester = tester

	return &rotator{
		resources: resources,
		config:    DefaultConfig,
		state:     state,
		driver:    driver,
	}, b0
}

func TestRotator_changeView(t *testing.T) {
	asrt := assert.New(t)

	rot, b0 := setupRotator()
	rot.state.setLeaderIndex(1)

	msgSvc := new(MockMsgService)
	msgSvc.On("SendNewView", rot.resources.VldStore.GetWorker(0), b0.QuorumCert()).Return(nil)
	rot.resources.MsgSvc = msgSvc

	rot.changeView()

	msgSvc.AssertExpectations(t)
	asrt.True(rot.getPendingViewChange())
	asrt.EqualValues(rot.state.getLeaderIndex(), 0)
}

func Test_rotator_isNewViewApproval(t *testing.T) {
	asrt := assert.New(t)

	rot1, _ := setupRotator()
	rot2, _ := setupRotator()

	rot1.setPendingViewChange(true)
	rot2.setPendingViewChange(false)

	tests := []struct {
		name        string
		rot         *rotator
		proposerIdx int
		want        bool
	}{
		{"pending and same leader", rot1, 0, true},
		{"not pending and different leader", rot2, 1, true},
		{"pending and different leader", rot1, 1, false},
		{"not pending and same leader", rot2, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			asrt.EqualValues(tt.want, tt.rot.isNewViewApproval(tt.proposerIdx))
		})
	}
}

func TestRotator_resetViewTimer(t *testing.T) {
	asrt := assert.New(t)

	rot, _ := setupRotator()
	rot.setPendingViewChange(true)

	rot.approveViewLeader(1)

	asrt.False(rot.getPendingViewChange())
	asrt.EqualValues(rot.state.getLeaderIndex(), 1)
}
