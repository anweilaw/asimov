// Copyright (c) 2018-2020. The asimov developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.
package common

import (
	"encoding/binary"
	"errors"
)

const (
	// A standard template contract must contains
	// two functions, getTemplateInfo and initTemplate
	// Following two fields are name for these functions.
	GetTemplateInfoFunc = "getTemplateInfo"

	InitTemplateFunc = "initTemplate"

	// empty value of an address type, formatted with string
	EmptyAddressValue = "0x000000000000000000000000000000000000000000"

	// This is the standard template's abi.
	TemplateABI = "[{\"constant\": true, \"inputs\": [], \"name\": \"getTemplateInfo\", \"outputs\": [{\"name\": \"\", \"type\": \"uint16\"}, {\"name\": \"\", \"type\": \"string\"} ], \"payable\": false, \"stateMutability\": \"view\", \"type\": \"function\"}, {\"constant\": false, \"inputs\": [{\"name\": \"_category\", \"type\": \"uint16\"}, {\"name\": \"_templateName\", \"type\": \"string\"} ], \"name\": \"initTemplate\", \"outputs\": [], \"payable\": false, \"stateMutability\": \"nonpayable\", \"type\": \"function\"} ]"
)

var (
	// callCode for `getTemplateInfo` function.
	GetTemplateInfoCallCode = Hex2Bytes("2fb97c1d")

	// callCode for `canTransfer` function of organization
	CanTransferFuncByte = Hex2Bytes("fc588476")

	// callCode for `isRestrictedAsset` function.
	IsRestrictedAssetByte = Hex2Bytes("f0d94a13")

	// callCode for `getOrganizationAddressById` function.
	GetOrganizationAddressByIdByte = Hex2Bytes("c0bb5900")

	// callCode for `getApprovedTemplatesCount` function.
	GetApprovedTemplatesCountByte = Hex2Bytes("ff2e9b4d")

	// callCode for `getSubmittedTemplatesCount` function.
	GetSubmittedTemplatesCountByte = Hex2Bytes("2c47086a")

	// callCode for `getApprovedTemplate` function.
	GetApprovedTemplateByte = Hex2Bytes("214b38d9")

	// callCode for `getSubmittedTemplate` function.
	GetSubmittedTemplateByte = Hex2Bytes("c17168bc")

	// callCode for `getTemplate` function.
	GetTemplateByte = Hex2Bytes("e9e89524")
)

// PackCanTransferInput returns a byte slice according to given parameters
func PackCanTransferInput(transferAddress []byte, assetIndex uint32) []byte {
	input := make([]byte, 68)
	copy(input[0:4], CanTransferFuncByte)
	copy(input[15:36], transferAddress)
	binary.BigEndian.PutUint32(input[64:], assetIndex)

	return input[:]
}

// PackIsRestrictedAssetInput returns a byte slice according to given parameters
func PackIsRestrictedAssetInput(organizationId uint32, assetIndex uint32) []byte {
	input := make([]byte, 68)
	copy(input[0:4], IsRestrictedAssetByte)
	binary.BigEndian.PutUint32(input[32:36], organizationId)
	binary.BigEndian.PutUint32(input[64:], assetIndex)

	return input[:]
}

// PackGetOrganizationAddressByIdInput returns a byte slice according to given parameters
func PackGetOrganizationAddressByIdInput(organizationId uint32, assetIndex uint32) []byte {
	input := make([]byte, 68)
	copy(input[0:4], GetOrganizationAddressByIdByte)
	binary.BigEndian.PutUint32(input[32:36], organizationId)
	binary.BigEndian.PutUint32(input[64:], assetIndex)

	return input[:]
}

// PackGetTemplateCountInput returns a byte slice according to given parameters
func PackGetTemplateCountInput(funcName string, category uint16) []byte {
	input := make([]byte, 36)
	if ContractTemplateWarehouse_GetApprovedTemplatesCountFunction() == funcName {
		copy(input[0:4], GetApprovedTemplatesCountByte)
	} else if ContractTemplateWarehouse_GetSubmittedTemplatesCountFunction() == funcName {
		copy(input[0:4], GetSubmittedTemplatesCountByte)
	}
	binary.BigEndian.PutUint16(input[34:], category)

	return input[:]
}

// PackGetTemplateDetailInput returns a byte slice according to given parameters
func PackGetTemplateDetailInput(funcName string, category uint16, index uint64) []byte {
	input := make([]byte, 68)
	if ContractTemplateWarehouse_GetApprovedTemplateFunction() == funcName {
		copy(input[0:4], GetApprovedTemplateByte)
	} else if ContractTemplateWarehouse_GetSubmittedTemplateFunction() == funcName {
		copy(input[0:4], GetSubmittedTemplateByte)
	}
	binary.BigEndian.PutUint16(input[34:36], category)
	binary.BigEndian.PutUint64(input[60:], index)

	return input[:]
}

// PackGetTemplateInput returns a byte slice according to given parameters
func PackGetTemplateInput(category uint16, name string) []byte {
	nameLength := len(name)
	input := make([]byte, 4+32+32+32+(nameLength+31)/32*32)
	copy(input[0:4], GetTemplateByte)
	binary.BigEndian.PutUint16(input[34:36], category)
	copy(input[67:68], []byte{byte(64)})
	binary.BigEndian.PutUint64(input[92:], uint64(nameLength))
	copy(input[100:], RightPadBytes([]byte(name), (nameLength+31)/32*32))

	return input[:]
}

// UnPackBoolResult returns bool value by unpacking given byte slice
func UnPackBoolResult(ret []byte) (bool, error) {
	if len(ret) != 32 {
		return false, errors.New("invalid length of ret of canTransfer func")
	}
	return ret[31] == 1, nil
}

// UnPackBoolResult returns bool value by unpacking given byte slice
func UnPackIsRestrictedAssetResult(ret []byte) (bool, bool, error) {
	if len(ret) != 64 {
		return false, false, errors.New("invalid length of ret of isRestrictedAsset func")
	}
	existed, support := ret[31] == 1, ret[63] == 1
	return existed, support, nil
}

// UnPackGetTemplatesCountResult returns template number by unpacking given byte slice
func UnPackGetTemplatesCountResult(ret []byte) (uint64, error) {
	if len(ret) != 32 {
		return 0, errors.New("invalid length of ret of getTemplatesCount func")
	}

	return binary.BigEndian.Uint64(ret[24:]), nil
}

// UnPackGetTemplateDetailResult returns template detail by unpacking given byte slice
func UnPackGetTemplateDetailResult(ret []byte) (string, []byte, int64, uint8, uint8, uint8, uint8, error) {
	name := string(ret[256:])
	key := ret[32:64]
	createTime := int64(binary.BigEndian.Uint64(ret[64:96][len(ret[64:96])-8:]))
	approveCount := ret[96:128][len(ret[96:128])-1]
	rejectCount := ret[128:160][len(ret[128:160])-1]
	reviewers := ret[160:192][len(ret[160:192])-1]
	status := ret[192:224][len(ret[192:224])-1]

	return name, key, createTime, approveCount, rejectCount, reviewers, status, nil
}

