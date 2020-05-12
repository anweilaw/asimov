// Copyright (c) 2018-2020. The asimov developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.
package syscontract

import (
	"fmt"
	"github.com/AsimovNetwork/asimov/ainterface"
	"github.com/AsimovNetwork/asimov/asiutil"
	"github.com/AsimovNetwork/asimov/blockchain"
	"github.com/AsimovNetwork/asimov/chaincfg"
	"github.com/AsimovNetwork/asimov/common"
	"github.com/AsimovNetwork/asimov/vm/fvm"
	"github.com/AsimovNetwork/asimov/vm/fvm/core/vm"
	"github.com/AsimovNetwork/asimov/vm/fvm/params"
	"math/big"
)

var templateWarehouseAddress = vm.ConvertSystemContractAddress(common.TemplateWarehouse)

// GetTemplates returns all submitted templates by calling system contract of template_warehouse
func (m *Manager) GetTemplates(
	block *asiutil.Block,
	gas uint64,
	stateDB vm.StateDB,
	chainConfig *params.ChainConfig,
	getCountFunc string,
	getTemplatesFunc string,
	category uint16,
	pageNo int,
	pageSize int) (int, []ainterface.TemplateWarehouseContent, error, uint64) {

	officialAddr := chaincfg.OfficialAddress
	contract := m.GetActiveContractByHeight(block.Height(), common.TemplateWarehouse)
	if contract == nil {
		errStr := fmt.Sprintf("Failed to get active contract %s, %d", common.TemplateWarehouse, block.Height())
		log.Error(errStr)
		panic(errStr)
	}

	// get number of templates.
	getTemplatesCount := getCountFunc
	runCode, err := fvm.PackFunctionArgs(contract.AbiInfo, getTemplatesCount, category)
	result, leftOverGas, err := fvm.CallReadOnlyFunction(officialAddr, block, m.chain, stateDB, chainConfig, gas,
		templateWarehouseAddress, runCode)
	if err != nil {
		log.Errorf("Get contract templates failed, error: %s", err)
		return 0, nil, err, leftOverGas
	}

	var outInt *big.Int
	err = fvm.UnPackFunctionResult(contract.AbiInfo, &outInt, getTemplatesCount, result)
	if err != nil {
		log.Errorf("Get contract templates failed, error: %s", err)
		return 0, nil, err, leftOverGas
	}
	if outInt.Int64() == 0 {
		return 0, nil, nil, leftOverGas
	}

	// get information details
	template := make([]ainterface.TemplateWarehouseContent, 0)
	getTemplate := getTemplatesFunc

	// settings of Pagination
	fromIndex := int(outInt.Int64()) - pageNo*pageSize - 1
	if fromIndex < 0 {
		return 0, nil, nil, leftOverGas
	}
	endIndex := fromIndex + 1 - pageSize
	if endIndex < 0 {
		endIndex = 0
	}

	for i := fromIndex; i >= endIndex; i-- {
		runCode, err := fvm.PackFunctionArgs(contract.AbiInfo, getTemplate, category, big.NewInt(int64(i)))
		if err != nil {
			log.Errorf("Get contract templates failed, error: %s", err)
			return 0, nil, err, leftOverGas
		}
		ret, leftOverGas, err := fvm.CallReadOnlyFunction(officialAddr, block, m.chain, stateDB, chainConfig,
			leftOverGas, templateWarehouseAddress, runCode)
		if err != nil {
			log.Errorf("Get contract templates failed, error: %s", err)
			return 0, nil, err, leftOverGas
		}

		cTime := new(big.Int)
		var keyType [32]byte
		outType := &[]interface{}{new(string), &keyType, &cTime, new(uint8), new(uint8), new(uint8), new(uint8)}
		err = fvm.UnPackFunctionResult(contract.AbiInfo, outType, getTemplate, ret)
		if err != nil {
			log.Errorf("Get contract template failed, index is %d, error: %s", i, err)
			continue
		}

		name := *((*outType)[0]).(*string)
		key := common.Bytes2Hex(keyType[:])
		createTime := cTime.Int64()
		approveCount := *((*outType)[3]).(*uint8)
		rejectCount := *((*outType)[4]).(*uint8)
		reviewers := *((*outType)[5]).(*uint8)
		status := *((*outType)[6]).(*uint8)
		if status != blockchain.TEMPLATE_STATUS_NOTEXIST {
			template = append(template, ainterface.TemplateWarehouseContent{
				Name: name, Key: key, CreateTime: createTime, ApproveCount: approveCount,
				RejectCount: rejectCount, Reviewers: reviewers, Status: status})
		}
	}

	return int(outInt.Int64()), template, nil, leftOverGas
}

// GetTemplate returns template detail by its template name
func (m *Manager) GetTemplate(
	block *asiutil.Block,
	gas uint64,
	stateDB vm.StateDB,
	chainConfig *params.ChainConfig,
	category uint16,
	name string) (ainterface.TemplateWarehouseContent, bool, uint64) {

	officialAddr := chaincfg.OfficialAddress
	contract := m.GetActiveContractByHeight(block.Height(), common.TemplateWarehouse)
	if contract == nil {
		errStr := fmt.Sprintf("Failed to get active contract %s, %d", common.TemplateWarehouse, block.Height())
		log.Error(errStr)
		panic(errStr)
	}

	// get functions of contract.
	getTemplate := common.ContractTemplateWarehouse_GetTemplateFunction()
	runCode, err := fvm.PackFunctionArgs(contract.AbiInfo, getTemplate, category, name)
	if err != nil {
		log.Errorf("Get contract template failed, error: %s", err)
		return ainterface.TemplateWarehouseContent{}, false, gas
	}
	ret, leftOverGas, err := fvm.CallReadOnlyFunction(officialAddr, block, m.chain, stateDB, chainConfig,
		gas, templateWarehouseAddress, runCode)
	if err != nil {
		log.Errorf("Get contract template failed, error: %s", err)
		return ainterface.TemplateWarehouseContent{}, false, leftOverGas
	}

	cTime := new(big.Int)
	var keyType [32]byte
	outType := &[]interface{}{new(string), &keyType, &cTime, new(uint8), new(uint8), new(uint8), new(uint8)}

	err = fvm.UnPackFunctionResult(contract.AbiInfo, outType, getTemplate, ret)
	if err != nil {
		log.Errorf("Get contract template failed, error: %s", err)
		return ainterface.TemplateWarehouseContent{}, false, leftOverGas
	}

	key := common.Bytes2Hex(keyType[:])
	createTime := cTime.Int64()
	approveCount := *((*outType)[3]).(*uint8)
	rejectCount := *((*outType)[4]).(*uint8)
	reviewers := *((*outType)[5]).(*uint8)
	status := *((*outType)[6]).(*uint8)
	if status == blockchain.TEMPLATE_STATUS_NOTEXIST {
		return ainterface.TemplateWarehouseContent{}, false, leftOverGas
	}

	return ainterface.TemplateWarehouseContent{Name: name, Key: key,
			CreateTime: createTime, ApproveCount: approveCount,
			RejectCount: rejectCount, Reviewers: reviewers,
			Status: status},
		true, leftOverGas
}