package kvdb

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/wooyang2018/svp-blockchain/execution/chaincode"
)

func TestKVDB_Init(t *testing.T) {
	asrt := assert.New(t)
	state := chaincode.NewMockState()
	jctx := new(KVDB)

	ctx := new(chaincode.MockCallContext)
	ctx.MockState = state
	ctx.MockSender = []byte{1, 1, 1}
	err := jctx.Init(ctx)
	asrt.NoError(err)

	input := &Input{
		Method: "owner",
	}
	b, _ := json.Marshal(input)
	ctx.MockInput = b
	owner, err := jctx.Query(ctx)
	asrt.NoError(err)
	asrt.Equal(ctx.MockSender, owner, "deployer should be owner")
}

func TestKVDB_Set(t *testing.T) {
	asrt := assert.New(t)
	state := chaincode.NewMockState()
	jctx := new(KVDB)

	ctx := new(chaincode.MockCallContext)
	ctx.MockState = state
	ctx.MockSender = []byte{1, 1, 1}
	err := jctx.Init(ctx)
	asrt.NoError(err)

	input := &Input{
		Method: "set",
		Key:    []byte("key"),
		Value:  []byte("value"),
	}
	b, _ := json.Marshal(input)
	ctx.MockSender = []byte{3, 3, 3}
	ctx.MockInput = b
	err = jctx.Invoke(ctx)
	asrt.Error(err, "sender not owner error")

	ctx.MockSender = []byte{1, 1, 1}
	err = jctx.Invoke(ctx)
	asrt.NoError(err)

	input = &Input{
		Method: "get",
		Key:    []byte("key"),
	}
	b, _ = json.Marshal(input)
	ctx.MockInput = b
	b, err = jctx.Query(ctx)
	asrt.NoError(err)

	var value []byte
	json.Unmarshal(b, &value)
	asrt.EqualValues([]byte("value"), value)
}
