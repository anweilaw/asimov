// Copyright (c) 2018-2020. The asimov developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.
package blockchain

import (
	"github.com/AsimovNetwork/asimov/asiutil"
	"github.com/AsimovNetwork/asimov/chaincfg"
	"github.com/AsimovNetwork/asimov/common"
	"github.com/AsimovNetwork/asimov/protos"
	"github.com/AsimovNetwork/asimov/vm/fvm"
	"github.com/AsimovNetwork/asimov/vm/fvm/core/vm"
	"github.com/AsimovNetwork/asimov/vm/fvm/params"
)

// Get template info which is stored in system registry centor
func (b *BlockChain) GetTemplateInfo(
	contractAddr []byte,
	gas uint64,
	block *asiutil.Block,
	stateDB vm.StateDB,
	chainConfig *params.ChainConfig) (uint16, string, uint64) {
	officialAddr := chaincfg.OfficialAddress
	result, leftOverGas, err := fvm.CallReadOnlyFunction(officialAddr, block, b, stateDB,
		chainConfig, gas, common.BytesToAddress(contractAddr), common.GetTemplateInfoCallCode)
	if err != nil {
		log.Errorf("get template failed, error: %s", err)
		return 0, "", leftOverGas
	}

	outType := &[]interface{}{new(uint16), new(string)}
	err = fvm.UnPackFunctionResult(common.TemplateABI, outType, common.GetTemplateInfoFunc, result)
	if err != nil {
		log.Errorf("unpack template result failed, error: %s", err)
		return 0, "", leftOverGas
	}
	return *((*outType)[0]).(*uint16), *((*outType)[1]).(*string), leftOverGas
}

// init template, this method is invoked after a create contract is executed.
func (b *BlockChain) InitTemplate(
	category uint16,
	templateName string,
	contractAddr common.Address,
	gas uint64,
	asset *protos.Asset,
	vmenv *vm.FVM) (error, uint64) {
	officialAddr := chaincfg.OfficialAddress
	runCode, err := fvm.PackFunctionArgs(common.TemplateABI, common.InitTemplateFunc, category, templateName)
	if err != nil {
		return err, gas
	}
	_, leftOverGas, _, err := vmenv.Call(vm.AccountRef(officialAddr), contractAddr, runCode, gas, common.Big0, asset, false)
	return err, leftOverGas
}
