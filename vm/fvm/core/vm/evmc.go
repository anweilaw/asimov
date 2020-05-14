// Copyright 2018 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.
// +build evmc

package vm

import (
	"errors"
	"fmt"
	"github.com/AsimovNetwork/asimov/common"
	"github.com/AsimovNetwork/asimov/protos"
	"github.com/AsimovNetwork/asimov/vm/evmc"
	"github.com/AsimovNetwork/asimov/vm/fvm/core/types"
	"github.com/AsimovNetwork/asimov/vm/fvm/log"
	"github.com/AsimovNetwork/asimov/vm/fvm/params"
	"math/big"
	"strings"
	"sync"
)

type EVMC struct {
	instance *evmc.Instance
	env      *FVM
	readOnly bool // TODO: The readOnly flag should not be here.
}

var (
	createMu     sync.Mutex
	evmcConfig   string // The configuration the instance was created with.
	evmcInstance *evmc.Instance
)

func createVM(config string) *evmc.Instance {
	createMu.Lock()
	defer createMu.Unlock()

	if evmcInstance == nil {
		options := strings.Split(config, ",")
		path := options[0]

		if path == "" {
			panic("EVMC VM path not provided, set --vm.(evm|ewasm)=/path/to/vm")
		}

		var err error
		evmcInstance, err = evmc.Load(path)
		if err != nil {
			panic(err.Error())
		}
		log.Info("EVMC VM loaded", "name", evmcInstance.Name(), "version", evmcInstance.Version(), "path", path)

		for _, option := range options[1:] {
			if idx := strings.Index(option, "="); idx >= 0 {
				name := option[:idx]
				value := option[idx+1:]
				err := evmcInstance.SetOption(name, value)
				if err == nil {
					log.Info("EVMC VM option set", "name", name, "value", value)
				} else {
					log.Warn("EVMC VM option setting failed", "name", name, "error", err)
				}
			}
		}

		evm1Cap := evmcInstance.HasCapability(evmc.CapabilityEVM1)
		ewasmCap := evmcInstance.HasCapability(evmc.CapabilityEWASM)
		log.Info("EVMC VM capabilities", "evm1", evm1Cap, "ewasm", ewasmCap)

		evmcConfig = config // Remember the config.

		log.Info("debug::evmc config: ", config)
	} else if evmcConfig != config {
		log.Error("New EVMC VM requested", "newconfig", config, "oldconfig", evmcConfig)
	}
	return evmcInstance
}

func NewEVMC(options string, env *FVM) *EVMC {
	return &EVMC{createVM(options), env, false}
}

// Implements evmc.HostContext interface.
type HostContext struct {
	env      *FVM
	contract *Contract
}

func (host *HostContext) CreateAsset(assetType, assetIndex uint32, amount *big.Int) error {
	if host == nil || host.env == nil {
		return errors.New("host is nil")
	}

	if amount == nil {
		return errors.New("amount is nil")
	}

	if amount.Cmp(common.Big0) <= 0 {
		return errors.New("invalid amount")
	}

	fmt.Printf("assetType:%v, assetIndex:%v, amount:%v\n", assetType, assetIndex, amount)

	// call a common method to create asset
	err := FlowCreateAsset(amount, host.contract, host.env, assetType, assetIndex)

	if err == errExecutionReverted {
		err = evmc.Revert
	} else if err != nil {
		fmt.Printf("execute custom instruction CreateAsset error: %s\n", err.Error())
		err = evmc.Failure
	}

	return err
}

func (host *HostContext) MintAsset(assetIndex uint32, amount *big.Int) error {
	if host == nil || host.env == nil {
		return errors.New("host is nil")
	}

	if amount == nil {
		return errors.New("amount is nil")
	}

	if amount.Cmp(common.Big0) <= 0 {
		return errors.New("invalid amount")
	}

	fmt.Printf("assetIndex:%v, amount:%v\n", assetIndex, amount)

	// call a common method to mint asset
	err := FlowMintAsset(amount, host.contract, host.env, assetIndex)

	if err == errExecutionReverted {
		err = evmc.Revert
	} else if err != nil {
		fmt.Printf("execute custom instruction MintAsset error:%s\n", err.Error())
		err = evmc.Failure
	}

	return err
}

func (host *HostContext) Transfer() {
	fmt.Printf("Transfer\n")
}

func (host *HostContext) DeployContract(
	category uint16,
	templateName string,
	args []byte) (contractAddr *common.Address, err error) {

	if host == nil || host.env == nil {
		return nil, errors.New("host is nil")
	}

	fmt.Printf("DeployContract category is %d\n", category)
	fmt.Printf("DeployContract templateName is %s\n", templateName)
	fmt.Printf("DeployContract args is %v\n", args)

	// call a common method to deploy contract
	addr, err := FlowDeployContract(host.contract, host.env, category, templateName, args)

	if err == errExecutionReverted {
		err = evmc.Revert
	} else if err != nil {
		fmt.Printf("execute custom instruction DeployContract error:%s\n", err.Error())
		err = evmc.Failure
	}

	return &addr, err
}

func (host *HostContext) AccountExists(addr common.Address) bool {
	fmt.Println("debug::evmc AccountExists")
	env := host.env
	if !env.StateDB.Empty(addr) {
		return true
	}
	return false
}

func (host *HostContext) GetStorage(addr common.Address, key common.Hash) common.Hash {
	env := host.env
	return env.StateDB.GetState(addr, key)
}

func (host *HostContext) SetStorage(addr common.Address, key common.Hash, value common.Hash) (status evmc.StorageStatus) {
	env := host.env

	oldValue := env.StateDB.GetState(addr, key)
	if oldValue == value {
		return evmc.StorageUnchanged
	}

	env.StateDB.SetState(addr, key, value)

	zero := common.Hash{}
	status = evmc.StorageModified
	if oldValue == zero {
		return evmc.StorageAdded
	} else if value == zero {
		env.StateDB.AddRefund(params.SstoreRefundGas)
		return evmc.StorageDeleted
	}
	return evmc.StorageModified
}

func (host *HostContext) GetBalance(addr common.Address) common.Hash {
	env := host.env
	balance := env.StateDB.GetBalance(addr)
	return common.BigToHash(balance)
}

func (host *HostContext) GetCodeSize(addr common.Address) int {
	env := host.env
	return env.StateDB.GetCodeSize(addr)
}

func (host *HostContext) GetCodeHash(addr common.Address) common.Hash {
	env := host.env
	return env.StateDB.GetCodeHash(addr)
}

func (host *HostContext) GetCode(addr common.Address) []byte {
	env := host.env
	return env.StateDB.GetCode(addr)
}

func (host *HostContext) Selfdestruct(addr common.Address, beneficiary common.Address) {
	env := host.env
	db := env.StateDB
	if !db.HasSuicided(addr) {
		db.AddRefund(params.SuicideRefundGas)
	}
	balance := db.GetBalance(addr)
	db.AddBalance(beneficiary, balance)
	db.Suicide(addr)
}

func (host *HostContext) GetTxContext() (gasPrice common.Hash, origin common.Address, coinbase common.Address,
	number int64, timestamp int64, gasLimit int64, difficulty common.Hash) {

	env := host.env
	gasPrice = common.BigToHash(env.GasPrice)
	origin = env.Origin
	coinbase = env.Coinbase
	number = env.BlockNumber.Int64()
	timestamp = env.Time.Int64()
	gasLimit = int64(env.GasLimit)

	//append wasm difficulty is not defined now
	//difficulty = common.BigToHash(env.Difficulty)
	difficulty = common.BigToHash(big.NewInt(0x1))

	return gasPrice, origin, coinbase, number, timestamp, gasLimit, difficulty
}

func (host *HostContext) GetBlockHash(number int64) common.Hash {
	env := host.env
	b := env.BlockNumber.Int64()
	if number >= (b-256) && number < b {
		return env.GetHash(uint64(number))
	}
	return common.Hash{}
}

func (host *HostContext) EmitLog(addr common.Address, topics []common.Hash, data []byte) {
	env := host.env
	env.StateDB.AddLog(&types.Log{
		Address:     addr,
		Topics:      topics,
		Data:        data,
		BlockNumber: env.BlockNumber.Uint64(),
	})
}

func (host *HostContext) Call(kind evmc.CallKind,
	destination common.Address, sender common.Address,
	value *big.Int, assets *protos.Asset, input []byte, gas int64, depth int,
	static bool, int2 *big.Int) (output []byte, gasLeft int64, createAddr common.Address, err error) {

	env := host.env

	gasU := uint64(gas)
	var gasLeftU uint64

	switch kind {
	case evmc.Call:
		if static {
			output, gasLeftU, err = env.StaticCall(host.contract, destination, input, gasU)
		} else {
			// append wasm TODO ???
			//output, gasLeftU, err = env.Call(host.contract, destination, input, gasU, value)

			fmt.Printf("	<<<opCall\n")
			fmt.Printf("			toAddr:%v\n", destination)
			fmt.Printf("			args:%v\n", input)
			fmt.Printf("			gas:%v\n", gasU)
			fmt.Printf("			value:%v\n", value)
			fmt.Printf("			asset:%v\n", assets)

			output, gasLeftU, err = env.Call(host.contract, destination, input, gasU, value, assets, true)

			fmt.Printf("			ret:%v\n", output)
			fmt.Printf("			returnGas:%v\n", gasLeftU)
			fmt.Printf("	<<<end opCall\n")
		}
	case evmc.DelegateCall:
		output, gasLeftU, err = env.DelegateCall(host.contract, destination, input, gasU)
	case evmc.CallCode:
		// append wasm
		//output, gasLeftU, err = env.CallCode(host.contract, destination, input, gasU, value)
		output, gasLeftU, err = env.CallCode(host.contract, destination, input, gasU, value, nil)
	case evmc.Create:
		var createOutput []byte
		// append wasm
		//createOutput, createAddr, gasLeftU, err = env.Create(host.contract, input, gasU, value)
		createOutput, createAddr, gasLeftU, err = env.Create(host.contract, input, gasU, value, nil, nil, nil, true)
		if err == errExecutionReverted {
			// Assign return buffer from REVERT.
			// TODO: Bad API design: return data buffer and the code is returned in the same place. In worst case
			//       the code is returned also when there is not enough funds to deploy the code.
			output = createOutput
		}
	}

	// Map errors.
	if err == errExecutionReverted {
		err = evmc.Revert
	} else if err != nil {
		err = evmc.Failure
	}

	gasLeft = int64(gasLeftU)
	return output, gasLeft, createAddr, err
}

func getRevision(env *FVM) evmc.Revision {
	return evmc.Byzantium
}

// append wasm:remove param readOnly
func (evm *EVMC) Run(contract *Contract, input []byte, readOnly bool) (ret []byte, err error) {
	evm.env.depth++
	defer func() { evm.env.depth-- }()

	// Don't bother with the execution if there's no code.
	if len(contract.Code) == 0 {
		return nil, nil
	}

	kind := evmc.Call
	if evm.env.StateDB.GetCodeSize(contract.Address()) == 0 {
		// Guess if this is a CREATE.
		//fmt.Printf( "create contract %s\n", contract.Address().Hex() )
		kind = evmc.Create
	} else {
		//fmt.Printf( "call contract %s\n", contract.Address().Hex() )
	}

	// Make sure the readOnly is only set if we aren't in readOnly yet.
	// This makes also sure that the readOnly flag isn't removed for child calls.
	/*if readOnly && !evm.readOnly {
		evm.readOnly = true
		defer func() { evm.readOnly = false }()
	}*/

	if readOnly && !evm.IsReadOnly() {
		evm.SetReadOnly(true)
		defer func() { evm.SetReadOnly(false) }()
	}

	output, gasLeft, err := evm.instance.Execute(
		&HostContext{evm.env, contract},
		getRevision(evm.env),
		kind,
		evm.IsReadOnly(),
		evm.env.depth-1,
		int64(contract.Gas),
		contract.Address(),
		contract.Caller(),
		input,
		common.BigToHash(contract.value),
		contract.Code,
		contract.asset,
		common.Hash{})

	contract.Gas = uint64(gasLeft)

	if err == evmc.Revert {
		err = errExecutionReverted
	} else if evmcError, ok := err.(evmc.Error); ok && evmcError.IsInternalError() {
		panic(fmt.Sprintf("EVMC VM internal error: %s", evmcError.Error()))
	}

	return output, err
}

func (evm *EVMC) CanRun(code []byte) bool {
	return isWasmCode(code)
	/*cap := evmc.CapabilityEVM1
	//wasmPreamble := []byte("\x00asm\x01\x00\x00\x00")
	//if bytes.HasPrefix(code, wasmPreamble) {
	if isWasmCode( code ) {
		cap = evmc.CapabilityEWASM
	}
	// FIXME: Optimize. Access capabilities once.
	return evm.instance.HasCapability(cap)*/
}

func (evm *EVMC) IsReadOnly() bool {
	return evm.readOnly
}

func (evm *EVMC) SetReadOnly(ro bool) {
	evm.readOnly = ro
}
