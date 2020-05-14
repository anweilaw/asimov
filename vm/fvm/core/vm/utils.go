package vm

import (
	"errors"
	"github.com/AsimovNetwork/asimov/common"
)

func FindAsset(fvm *FVM, coinId uint32, caller common.Address, gas uint64) (uint32, uint32, uint64, error) {
	registryCenterAddr, _, registryCenterABI := fvm.GetSystemContractInfo(common.RegistryCenter)
	findAsset := common.ContractRegistryCenter_FindAssetFunction()
	input, err := fvm.PackFunctionArgs(registryCenterABI, findAsset, coinId)
	if err != nil {
		return 0, 0, gas, err
	}
	result, leftOverGas, _, err := fvm.Call(AccountRef(caller), registryCenterAddr, input, gas, common.Big0, nil, true)
	if err != nil {
		return 0, 0, leftOverGas, err
	}

	returnData := []interface{}{new(uint32), new(uint32), new(bool)}
	err = fvm.UnPackFunctionResult(registryCenterABI, &returnData, findAsset, result)
	if err != nil {
		return 0, 0, leftOverGas, err
	}
	coinType := *(returnData[0]).(*uint32)
	orgId := *(returnData[1]).(*uint32)
	registered := *(returnData[2]).(*bool)
	if registered {
		return coinType, orgId, leftOverGas, nil
	} else {
		return 0, 0, leftOverGas, errors.New("organization is not registered")
	}
}

func GetOrganizationId(fvm *FVM, caller common.Address, gas uint64) (uint32, uint64, error) {
	registryCenterAddr, _, registryCenterABI := fvm.GetSystemContractInfo(common.RegistryCenter)
	getOrganizationId := common.ContractRegistryCenter_GetOrganizationIdFunction()
	input, err := fvm.PackFunctionArgs(registryCenterABI, getOrganizationId)
	if err != nil {
		return 0, gas, err
	}
	result, leftOverGas, _, err := fvm.Call(AccountRef(caller), registryCenterAddr, input, gas, common.Big0, nil, true)
	if err != nil {
		return 0, leftOverGas, err
	}

	outType := &[]interface{}{new(bool), new(uint32)}
	err = fvm.UnPackFunctionResult(registryCenterABI, outType, getOrganizationId, result)
	if err != nil {
		return 0, leftOverGas, err
	}
	registered := *((*outType)[0]).(*bool)
	organizationId := *((*outType)[1]).(*uint32)

	if registered {
		return organizationId, leftOverGas, nil
	} else {
		return 0, leftOverGas, errors.New("organization is not registered")
	}
}
