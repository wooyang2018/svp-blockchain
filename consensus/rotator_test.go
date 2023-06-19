// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package consensus

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/wooyang2018/posv-blockchain/core"
	"github.com/wooyang2018/posv-blockchain/emitter"
)

func setupRotator() (*rotator, *core.Proposal) {
	key1 := core.GenerateKey(nil)
	key2 := core.GenerateKey(nil)
	validators := []string{
		key1.PublicKey().String(),
		key2.PublicKey().String(),
	}
	resources := &Resources{
		Signer:    key1,
		RoleStore: core.NewRoleStore(validators),
	}

	blk := core.NewBlock().Sign(key1)
	b0 := core.NewProposal().SetBlock(blk).SetView(1).Sign(key1)
	q0 := core.NewQuorumCert().Build(key1, []*core.Vote{b0.Vote(key1)})
	b0.SetQuorumCert(q0)

	state := newState()
	state.setBlock(b0.Block())
	driver := &driver{
		resources:  resources,
		state:      state,
		proEmitter: emitter.New(),
	}

	driver.setupInnerState(b0.Block(), q0)
	driver.tester = newTester(nil)

	return &rotator{
		resources: resources,
		config:    DefaultConfig,
		state:     state,
		driver:    driver,
	}, b0
}

func TestRotator_changeView(t *testing.T) {
	asrt := assert.New(t)

	rot, _ := setupRotator()
	rot.driver.setLeaderIndex(1)

	msgSvc := new(MockMsgService)
	msgSvc.On("BroadcastProposal", mock.Anything).Return(nil)
	rot.resources.MsgSvc = msgSvc

	rot.changeView()

	msgSvc.AssertExpectations(t)
	asrt.EqualValues(rot.driver.getViewChange(), 1)
	asrt.EqualValues(rot.driver.getLeaderIndex(), 0)
}

func Test_rotator_isNewViewApproval(t *testing.T) {
	asrt := assert.New(t)

	rot1, _ := setupRotator()
	rot2, _ := setupRotator()

	rot1.driver.setViewChange(1)
	rot2.driver.setViewChange(0)

	tests := []struct {
		name        string
		rot         *rotator
		proposerIdx uint32
		want        bool
	}{
		{"pending and same leader", rot1, 0, true},
		{"not pending and different leader", rot2, 1, true},
		{"pending and different leader", rot1, 1, false},
		{"not pending and same leader", rot2, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			asrt.EqualValues(tt.want, tt.rot.isNewViewApproval(1, tt.proposerIdx))
		})
	}
}

func TestRotator_resetViewTimer(t *testing.T) {
	asrt := assert.New(t)

	rot, _ := setupRotator()
	rot.driver.setViewChange(1)

	rot.approveViewLeader(1, 1)

	asrt.EqualValues(rot.driver.getViewChange(), 0)
	asrt.EqualValues(rot.driver.getLeaderIndex(), 1)
}
