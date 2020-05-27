// Copyright (c) 2018-2020 The asimov developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.
package fvm

import (
	"github.com/AsimovNetwork/asimov/asiutil"
	"github.com/AsimovNetwork/asimov/common"
	"github.com/AsimovNetwork/asimov/vm/fvm/abi"
	fvm "github.com/AsimovNetwork/asimov/vm/fvm/core/vm"
	"github.com/AsimovNetwork/asimov/vm/fvm/params"
	"math/big"
	"strings"
	"reflect"
	"fmt"
)

/*
	Call a readonly function of a contract
*/
func CallReadOnlyFunction(
	from common.Address,
	block *asiutil.Block,
	chain ChainContext,
	stateDB fvm.StateDB,
	chainConfig *params.ChainConfig,
	gasLimit uint64,
	contractAddr common.Address,
	input []byte) (ret []byte, leftOverGas uint64, err error) {
	context := NewFVMContext(from, new(big.Int).SetInt64(1), block, chain, nil,nil)
	vmInstance := fvm.NewFVM(context, stateDB, chainConfig, *chain.GetVmConfig())
	ret, leftOverGas, _, err = vmInstance.Call(
		fvm.AccountRef(from), contractAddr, input, gasLimit, common.Big0, nil, false)
	return
}

/*
	Pack constructor with arguments
*/
func PackConstructorArgs(abiStr string, args ...interface{}) ([]byte, error) {
	return PackFunctionArgs(abiStr, "", args...)
}

/*
	Pack function with arguments
*/
func PackFunctionArgs(abiStr string, funcName string, args ...interface{}) ([]byte, error) {
	definition, err := abi.JSON(strings.NewReader(abiStr))
	if err != nil {
		return nil, err
	}
	out, err := definition.Pack(funcName, args...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

/*
	Unpack function execution result
*/
func UnPackFunctionResult(abiStr string, v interface{}, funcName string, output []byte) error {
	definition, err := abi.JSON(strings.NewReader(abiStr))
	if err != nil {
		return err
	}
	err = definition.Unpack(v, funcName, output)
	return err
}

/*
	Unpack readonly function execution result
 */
func UnPackReadOnlyResult(abiStr string, funcName string, output []byte) (interface{}, error) {
	definition, err := abi.JSON(strings.NewReader(abiStr))

	if err != nil {
		return nil, err
	}
	if method, ok := definition.Methods[funcName]; ok {
		result, err := method.Outputs.UnpackValues(output)

		if err != nil {
			return nil, err
		}

		if len(method.Outputs) == 1 && method.Outputs[0].Type.T != abi.BytesTy {
			return reflect.ValueOf(result).Index(0).Interface(), nil
		} else {
			return result, nil
		}
	} else {
		return fmt.Errorf("abi: method name is not defined in abi"), nil
	}

}

/*
	Unpack event
 */
func UnpackEvent(abiStr string, eventName string, output []byte) (interface{}, error) {
	definition, err := abi.JSON(strings.NewReader(abiStr))

	if err != nil {
		return nil, err
	}

	if event, ok := definition.Events[eventName]; ok {
		result, err := event.Inputs.UnpackValues(output)

		if err != nil {
			return nil, err
		}

		if len(event.Inputs) == 1 && event.Inputs[0].Type.T != abi.BytesTy {
			return reflect.ValueOf(result).Index(0).Interface(), nil
		} else {
			return result, nil
		}
	} else {
		return fmt.Errorf("abi: event name is not defined in abi"), nil
	}
}
