// Copyright (c) 2018-2020. The asimov developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.
package syscontract

import (
	"encoding/binary"
	"fmt"
	"github.com/AsimovNetwork/asimov/ainterface"
	"github.com/AsimovNetwork/asimov/asiutil"
	"github.com/AsimovNetwork/asimov/chaincfg"
	"github.com/AsimovNetwork/asimov/common"
	"github.com/AsimovNetwork/asimov/protos"
	"github.com/AsimovNetwork/asimov/vm/fvm"
	"github.com/AsimovNetwork/asimov/vm/fvm/core/vm"
	"github.com/AsimovNetwork/asimov/vm/fvm/params"
	"math/big"
)


// GetContractAddressByAsset returns organization addresses by calling system contract of registry
// according to parameter assets
func (m *Manager) GetContractAddressByAsset(
	gas uint64,
	block *asiutil.Block,
	stateDB vm.StateDB,
	chainConfig *params.ChainConfig,
	assets []string) ([]common.Address, bool, uint64) {

	officialAddr := chaincfg.OfficialAddress
	contract := m.GetActiveContractByHeight(block.Height(), common.RegistryCenter)
	if contract == nil {
		errStr := fmt.Sprintf("Failed to get active contract %s, %d", common.RegistryCenter, block.Height())
		log.Error(errStr)
		panic(errStr)
	}
	proxyAddr, abi := vm.ConvertSystemContractAddress(common.RegistryCenter), contract.AbiInfo

	// function name of contract registry
	funcName := common.ContractRegistryCenter_GetOrganizationAddressesByAssetsFunction()
	assetId := make([]uint64, 0, 1)
	for _, asset := range assets {
		assetBytes := common.Hex2Bytes(asset)
		assetId = append(assetId, binary.BigEndian.Uint64(assetBytes[4:]))

	}

	// pack function params, ready for calling
	runCode, err := fvm.PackFunctionArgs(abi, funcName, assetId)

	// call function
	result, leftOverGas, err := fvm.CallReadOnlyFunction(officialAddr, block, m.chain, stateDB, chainConfig,
		gas, proxyAddr, runCode)
	if err != nil {
		log.Errorf("Get  contract address of asset failed, error: %s", err)
		return nil, false, leftOverGas
	}

	var outType []common.Address

	// unpack results
	err = fvm.UnPackFunctionResult(abi, &outType, funcName, result)
	if err != nil {
		log.Errorf("Get contract address of asset failed, error: %s", err)
		return nil, false, leftOverGas
	}

	return outType, true, leftOverGas
}

// GetAssetInfoByAssetId returns asset information by parameter asset id
// asset id is consist of organization id and asset index
func (m *Manager) GetAssetInfoByAssetId(
	block *asiutil.Block,
	stateDB vm.StateDB,
	chainConfig *params.ChainConfig,
	assets []string) ([]ainterface.AssetInfo, error) {

	officialAddr := chaincfg.OfficialAddress
	contract := m.GetActiveContractByHeight(block.Height(), common.RegistryCenter)
	if contract == nil {
		errStr := fmt.Sprintf("Failed to get active contract %s, %d", common.RegistryCenter, block.Height())
		log.Error(errStr)
		panic(errStr)
	}
	proxyAddr, abi := vm.ConvertSystemContractAddress(common.RegistryCenter), contract.AbiInfo

	// function name of contract registry
	funcName := common.ContractRegistryCenter_GetAssetInfoByAssetIdFunction()

	results := make([]ainterface.AssetInfo, 0)
	var err error
	for _, asset := range assets {
		assetBytes := common.Hex2Bytes(asset)
		_, orgId, assetIndex := protos.AssetDetailFromBytes(assetBytes)

		runCode, err := fvm.PackFunctionArgs(abi, funcName, orgId, assetIndex)
		result, _, err := fvm.CallReadOnlyFunction(officialAddr, block, m.chain, stateDB, chainConfig,
			common.SystemContractReadOnlyGas, proxyAddr, runCode)
		if err != nil {
			log.Errorf("Get  asset inf failed, error: %s", err)
		}

		var outType = &[]interface{}{new(bool), new(string), new(string), new(string), new(*big.Int), new([]*big.Int)}

		err = fvm.UnPackFunctionResult(abi, outType, funcName, result)
		if err != nil {
			log.Errorf("Unpack asset info result failed, error: %s", err)
		}

		ret := ainterface.AssetInfo{
			Exist:       *((*outType)[0]).(*bool),
			Name:        *((*outType)[1]).(*string),
			Symbol:      *((*outType)[2]).(*string),
			Description: *((*outType)[3]).(*string),
			Total:       (*((*outType)[4]).(**big.Int)).Uint64(),
		}
		history := *((*outType)[5]).(*[]*big.Int)

		for _, h := range history {
			ret.History = append(ret.History, h.Uint64())
		}
		results = append(results, ret)
	}

	return results, err
}

// IsLimit returns a number of int type by calling system contract of registry
// the number represents if an asset is restricted
func (m *Manager) IsLimit(block *asiutil.Block,
	stateDB vm.StateDB, assets *protos.Assets) int {
	officialAddr := chaincfg.OfficialAddress
	_, organizationId, assetIndex := assets.AssetsFields()
	contract := m.GetActiveContractByHeight(block.Height(), common.RegistryCenter)
	if contract == nil {
		errStr := fmt.Sprintf("Failed to get active contract %s, %d", common.RegistryCenter, block.Height())
		log.Error(errStr)
		panic(errStr)
	}
	proxyAddr, abi := vm.ConvertSystemContractAddress(common.RegistryCenter), contract.AbiInfo
	funcName := common.ContractRegistryCenter_IsRestrictedAssetFunction()
	input, err := fvm.PackFunctionArgs(abi, funcName, organizationId, assetIndex)
	if err != nil {
		return -1
	}

	result, _, err := fvm.CallReadOnlyFunction(officialAddr, block, m.chain,
		stateDB, chaincfg.ActiveNetParams.FvmParam,
		common.ReadOnlyGas, proxyAddr, input)
	if err != nil {
		log.Error(err)
		return -1
	}

	var outType = &[]interface{}{new(bool), new(bool)}
	err = fvm.UnPackFunctionResult(abi, outType, funcName, result)
	if err != nil {
		log.Error(err)
		return -1
	}
	if !*((*outType)[0]).(*bool) {
		return -1
	}
	if *((*outType)[1]).(*bool) {
		return 1
	}
	return 0
}

// IsSupport returns a bool result, which represents if a restricted asset can be transfer
func (m *Manager) IsSupport(block *asiutil.Block,
	stateDB vm.StateDB, gasLimit uint64, assets *protos.Assets, address []byte) (bool, uint64) {

	if gasLimit < common.SupportCheckGas {
		return false, 0
	}

	// step1: prepare parameters for calling system contract to get organization address
	_, organizationId, assetIndex := assets.AssetsFields()
	caller := chaincfg.OfficialAddress

	contract := m.GetActiveContractByHeight(block.Height(), common.RegistryCenter)
	if contract == nil {
		errStr := fmt.Sprintf("Failed to get active contract %s, %d", common.RegistryCenter, block.Height())
		log.Error(errStr)
		panic(errStr)
	}
	proxyAddr, abi := vm.ConvertSystemContractAddress(common.RegistryCenter), contract.AbiInfo
	funcName := common.ContractRegistryCenter_GetOrganizationAddressByIdFunction()
	input, err := fvm.PackFunctionArgs(abi, funcName, organizationId, assetIndex)
	if err != nil {
		return false, gasLimit
	}

	result, leftOverGas, err := fvm.CallReadOnlyFunction(caller, block, m.chain,
		stateDB, chaincfg.ActiveNetParams.FvmParam,
		common.SupportCheckGas, proxyAddr, input)

	if err != nil {
		log.Error(err)
		return false, gasLimit - common.SupportCheckGas + leftOverGas
	}

	var outType common.Address
	err = fvm.UnPackFunctionResult(abi, &outType, funcName, result)
	if err != nil {
		log.Error(err)
	}

	// check if return valid organization address
	if common.EmptyAddressValue == outType.String() {
		return false, gasLimit - common.SupportCheckGas + leftOverGas
	}

	// step2: call canTransfer method to check if the asset can be transfer
	transferAddress := common.BytesToAddress(address)
	transferInput := common.PackCanTransferInput(transferAddress, assetIndex)

	result2, leftOverGas2, _ := fvm.CallReadOnlyFunction(caller, block, m.chain,
		stateDB, chaincfg.ActiveNetParams.FvmParam,
		common.ReadOnlyGas, outType, transferInput)

	support, err := common.UnPackBoolResult(result2)
	if err != nil {
		log.Error(err)
	}

	return support, gasLimit - common.SupportCheckGas + leftOverGas + leftOverGas2 - common.ReadOnlyGas
}
