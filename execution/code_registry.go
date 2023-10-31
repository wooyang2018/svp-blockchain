// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package execution

import (
	"bytes"
	"encoding/json"
	"errors"

	"github.com/wooyang2018/svp-blockchain/execution/common"
)

var codeRegistryAddr = bytes.Repeat([]byte{0}, 32)

type DriverType uint8

const (
	DriverTypeNative DriverType = iota + 1
	DriverTypeBincc
)

type CodeInfo struct {
	DriverType DriverType `json:"driverType"`
	CodeID     []byte     `json:"codeID"`
}

type codeRegistry struct {
	drivers map[DriverType]common.CodeDriver
}

func newCodeRegistry() *codeRegistry {
	reg := new(codeRegistry)
	reg.drivers = make(map[DriverType]common.CodeDriver)
	return reg
}

func (reg *codeRegistry) registerDriver(driverType DriverType, driver common.CodeDriver) error {
	if _, found := reg.drivers[driverType]; found {
		return errors.New("driver already registered")
	}
	reg.drivers[driverType] = driver
	return nil
}

func (reg *codeRegistry) install(input *DeploymentInput) error {
	driver, err := reg.getDriver(input.CodeInfo.DriverType)
	if err != nil {
		return err
	}
	return driver.Install(input.CodeInfo.CodeID, input.InstallData)
}

func (reg *codeRegistry) deploy(codeAddr []byte, input *DeploymentInput,
	st *stateTracker) (common.Chaincode, error) {
	driver, err := reg.getDriver(input.CodeInfo.DriverType)
	if err != nil {
		return nil, err
	}
	reg.setCodeInfo(codeAddr, &input.CodeInfo, st)
	return driver.GetInstance(input.CodeInfo.CodeID)
}

func (reg *codeRegistry) getInstance(codeAddr []byte, state stateGetter,
) (common.Chaincode, error) {
	info, err := reg.getCodeInfo(codeAddr, state)
	if err != nil {
		return nil, err
	}
	driver, err := reg.getDriver(info.DriverType)
	if err != nil {
		return nil, err
	}
	return driver.GetInstance(info.CodeID)
}

func (reg *codeRegistry) getDriver(driverType DriverType) (common.CodeDriver, error) {
	driver, ok := reg.drivers[driverType]
	if !ok {
		return nil, errors.New("unknown chaincode driver type")
	}
	return driver, nil
}

func (reg *codeRegistry) setCodeInfo(codeAddr []byte, codeInfo *CodeInfo, st *stateTracker) error {
	b, err := json.Marshal(codeInfo)
	if err != nil {
		return err
	}
	st.SetState(codeAddr, b)
	return nil
}

func (reg *codeRegistry) getCodeInfo(codeAddr []byte, state stateGetter) (*CodeInfo, error) {
	b := state.GetState(codeAddr)
	info := new(CodeInfo)
	err := json.Unmarshal(b, info)
	if err != nil {
		return nil, err
	}
	return info, nil
}
