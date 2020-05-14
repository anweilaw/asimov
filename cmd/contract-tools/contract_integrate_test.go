package contract_tools

import (
	"errors"
	"fmt"
	"github.com/AsimovNetwork/asimov/asiutil"
	"github.com/AsimovNetwork/asimov/chaincfg"
	"github.com/AsimovNetwork/asimov/common"
	"github.com/AsimovNetwork/asimov/database/dbimpl/ethdb"
	"github.com/AsimovNetwork/asimov/protos"
	"github.com/AsimovNetwork/asimov/vm/fvm"
	"github.com/AsimovNetwork/asimov/vm/fvm/core/state"
	"github.com/AsimovNetwork/asimov/vm/fvm/core/vm"
	"io/ioutil"
	"math/big"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	GAS_LIMIT = uint64(50000000)
	IS_SOLCJS = false
	NODE_CMD  = "node"
)

////////////////// begin ContractContext /////////////////
type ContractRes struct {
	projectName string
	mainClass   string
	srcFile     string
	abiFile     string
	codeFile    string
	owner       *common.Address
	address     *common.Address
	abiInfo     string
	code        []byte
	isWasm      bool
}

type ContractContext struct {
	gasLimit  uint64
	value     *big.Int
	asset     *protos.Asset
	fvmInst   *vm.FVM
	sender    *common.Address
	contracts map[string]*ContractRes
	chain     *BlockChain
}

func (ctx *ContractContext) addContractRes(res *ContractRes) error {
	if ctx == nil || res == nil || len(res.mainClass) == 0 {
		return errors.New("ContractContext addContractRes param error")
	}

	if ctx.contracts == nil {
		ctx.contracts = make(map[string]*ContractRes, 0)
	}

	ctx.contracts[res.mainClass] = res
	return nil
}

func (ctx *ContractContext) getContractRes(mainClass string) *ContractRes {
	if ctx == nil || ctx.contracts == nil || len(mainClass) == 0 {
		return nil
	}

	return ctx.contracts[mainClass]
}

func (ctx *ContractContext) getContractAbi(mainClass string) (string, error) {
	if ctx == nil || ctx.contracts == nil ||
		len(mainClass) == 0 {
		return "", errors.New("ContractContext getContractAbi param error")
	}

	res := ctx.contracts[mainClass]
	if res == nil {
		return "", errors.New(fmt.Sprintf("ContractContext getContractAbi contract %s is not exist", mainClass))
	}

	if len(res.abiInfo) > 0 {
		return res.abiInfo, nil
	}

	if len(res.abiFile) == 0 {
		return "", errors.New(fmt.Sprintf("ContractContext getContractAbi contract %s abiFile is not exist", mainClass))
	}

	binCode, err := ioutil.ReadFile(res.abiFile)
	if err != nil {
		return "", err
	}

	res.abiInfo = fmt.Sprintf("%s", binCode)
	return res.abiInfo, nil
}
func (ctx *ContractContext) isWasmCode(mainClass string) bool {
	return ctx != nil && ctx.contracts != nil &&
		ctx.contracts[mainClass] != nil && ctx.contracts[mainClass].isWasm
}

func (ctx *ContractContext) getContractCodeByte(mainClass string) ([]byte, error) {
	if ctx == nil || ctx.contracts == nil ||
		len(mainClass) == 0 {
		return nil, errors.New("ContractContext getContractCodeByte param error")
	}

	res := ctx.contracts[mainClass]
	if res == nil {
		return nil, errors.New(fmt.Sprintf("ContractContext getContractCodeByte contract %s is not exist", mainClass))
	}

	if res.code != nil || len(res.code) > 0 {
		return res.code, nil
	}

	if len(res.codeFile) == 0 {
		return nil, errors.New(fmt.Sprintf("ContractContext getContractCodeByte contract %s codeFile is not exist", mainClass))
	}

	binCode, err := ioutil.ReadFile(res.codeFile)
	if err != nil {
		return nil, err
	}

	codeHex := fmt.Sprintf("%s", binCode)
	res.code = common.Hex2Bytes(codeHex)
	return res.code, nil
}

func (ctx *ContractContext) setContractCodeByte(mainClass string, code []byte) error {
	if ctx == nil || ctx.contracts == nil ||
		len(mainClass) == 0 || code == nil {
		return errors.New("ContractContext setContractCodeByte param error")
	}

	res := ctx.contracts[mainClass]
	if res == nil {
		return errors.New(fmt.Sprintf("ContractContext setContractCodeByte contract %s is not exist", mainClass))
	}

	res.code = code
	return nil
}

func (ctx *ContractContext) setContractAddress(mainClass string, address *common.Address) error {
	contractRes := ctx.getContractRes(mainClass)

	if contractRes == nil {
		return errors.New(fmt.Sprintf("ContractContext setContractAddress contract %s is not exist", mainClass))
	}

	contractRes.address = address
	return nil
}

func (ctx *ContractContext) setContractOwner(mainClass string, owner *common.Address) error {
	contractRes := ctx.getContractRes(mainClass)

	if contractRes == nil {
		return errors.New(fmt.Sprintf("ContractContext setContractAddress contract %s is not exist", mainClass))
	}

	contractRes.owner = owner
	return nil
}

type ArgMgr struct {
	paramTypes []string
	paramVals  []string
}

func (argMgr *ArgMgr) addParam(paramType, paramVal string) {
	if argMgr.paramTypes == nil {
		argMgr.paramTypes = make([]string, 0, 0)
	}

	if argMgr.paramVals == nil {
		argMgr.paramVals = make([]string, 0, 0)
	}

	argMgr.paramTypes = append(argMgr.paramTypes, paramType)

	argMgr.paramVals = append(argMgr.paramVals, paramVal)
}

func (argMgr *ArgMgr) buildParams() (argTypes, argVals string, err error) {
	if argMgr.paramTypes == nil &&
		argMgr.paramVals == nil {
		return "", "", nil
	}

	if argMgr.paramTypes == nil ||
		argMgr.paramVals == nil ||
		len(argMgr.paramTypes) != len(argMgr.paramVals) {
		return "", "", errors.New("type and val is not match")
	}

	argTypes = strings.Join(argMgr.paramTypes, ",")
	argVals = strings.Join(argMgr.paramVals, ",,")

	return argTypes, argVals, nil
}

////////////////// begin ContractAction /////////////////
type ContractAction interface {
	execute(ctx *ContractContext) ([]byte, error)
	getName() string
}

/// Deploy evm system contract
type DeployEvmSystemContractAction struct {
}

func (action *DeployEvmSystemContractAction) execute(ctx *ContractContext) ([]byte, error) {
	chainParams := chaincfg.ActiveNetParams.Params
	if chainParams == nil {
		return nil, errors.New("DeployEvmSystemContractAction execute param chainParams error")
	}

	beneficiary := chainParams.GenesisBlock.Transactions[0].TxOut[0]
	if beneficiary == nil {
		return nil, errors.New("DeployEvmSystemContractAction execute param TxOut[0] error")
	}

	cMap := chaincfg.TransferGenesisData(beneficiary.Data)
	if cMap == nil {
		return nil, errors.New("DeployEvmSystemContractAction execute param Genesis Data error")
	}

	sender := vm.AccountRef(chaincfg.OfficialAddress)

	var err error

	//ctx.chain.genesisDataCache = cMap
	for k, contractInfos := range cMap {
		for _, v := range contractInfos {
			_, addr, _, _, err := ctx.fvmInst.Create(sender,
				common.Hex2Bytes(v.Code+v.InitCode),
				ctx.gasLimit,
				common.Big0,
				&beneficiary.Asset,
				nil,
				nil,
				false)

			if err != nil {
				break
			}

			ctx.addContractRes(&ContractRes{
				"system_contracts",
				k.String(),
				"",
				"",
				"",
				&chaincfg.OfficialAddress,
				&addr,
				v.AbiInfo,
				nil,
				false,
			})

			fmt.Printf("deploy system contract %s success, address is %s\n", k, common.Bytes2Hex(addr[:]))
		}
	}

	return nil, err
}

func (action *DeployEvmSystemContractAction) getName() string {
	return "DeployEvmSystemContractAction"
}

func newDeployEvmSystemContractAction() ContractAction {
	return &DeployEvmSystemContractAction{}
}

/// Evm Contract compile action
type CompileEvmAction struct {
	projectName string // contract base dir
	srcFile     string // contract file
	mainClass   string // contract main class name
	abiFile     string // target abi file
	codeFile    string // target code bytes file
}

func (action *CompileEvmAction) execute(ctx *ContractContext) ([]byte, error) {
	fmt.Printf("<<<<<<start execute CompileEvmAction %s\n", action.projectName)

	if action == nil {
		return nil, errors.New("CompileEvmAction execute param error")
	}

	fileName := filepath.Join("testdata", action.projectName, action.srcFile)
	outputDir := filepath.Join("testdata", action.projectName, "target")

	var cmd *exec.Cmd
	if IS_SOLCJS {
		cmd = exec.Command("solcjs", "--overwrite", "--optimize", "--abi", "--bin", "-o", outputDir, fileName)
	} else {
		cmd = exec.Command("contract-tools/solc", "--overwrite", "--optimize", "--abi", "--bin", "-o", outputDir, fileName)
	}

	fmt.Printf("%v", cmd)

	ret, err := Output()

	if err != nil {
		return nil, errors.New(fmt.Sprintf("exec command %v, %s", cmd, err))
	}

	if IS_SOLCJS {
		cmd = exec.Command("find", outputDir, "-depth", "1", "-name", "*.abi")
	} else {
		cmd = exec.Command("find", outputDir, "-depth", "1", "-name", fmt.Sprintf("%s.abi", action.mainClass))
	}
	ret, err = Output()
	if err != nil {
		return nil, errors.New(fmt.Sprintf("find abi file error"))
	}

	action.abiFile = fmt.Sprintf("%s", ret)
	action.abiFile = strings.Replace(action.abiFile, "\n", "", -1)

	if len(action.abiFile) == 0 {
		return nil, errors.New(fmt.Sprintf("compile %s error, abiFile is not generated", action.projectName))
	}

	if IS_SOLCJS {
		cmd = exec.Command("find", outputDir, "-depth", "1", "-name", "*.bin")
	} else {
		cmd = exec.Command("find", outputDir, "-depth", "1", "-name", fmt.Sprintf("%s.bin", action.mainClass))
	}
	ret, err = Output()
	if err != nil {
		return nil, errors.New(fmt.Sprintf("find bin file error"))
	}

	action.codeFile = fmt.Sprintf("%s", ret)
	action.codeFile = strings.Replace(action.codeFile, "\n", "", -1)

	if len(action.abiFile) == 0 {
		return nil, errors.New(fmt.Sprintf("compile %s error, codeFile is not generated", action.projectName))
	}

	ctx.addContractRes(&ContractRes{
		action.projectName,
		action.mainClass,
		action.srcFile,
		action.abiFile,
		action.codeFile,
		nil,
		nil,
		"",
		nil,
		false,
	})
	fmt.Printf("<<<<<<finish execute CompileEvmAction %s\n", action.projectName)

	ret, err = ctx.getContractCodeByte(action.mainClass)
	return ret, err
}

func (action *CompileEvmAction) getName() string {
	return "CompileEvmAction"
}

// @func create CompileEvmAction
// @param projectName: contract project name that must be created in dir `testdata`
// @param srcFile: contract main file name that located in dir `projectName`
// @param mainClass: contract main class name
func newCompileEvmAction(projectName, srcFile, mainClass string) ContractAction {
	return &CompileEvmAction{
		projectName: projectName,
		srcFile:     srcFile,
		mainClass:   mainClass,
	}
}

/// Wasm Contract compile action
type CompileWasmAction struct {
	projectName string // contract base dir
	targetName  string // compile target name
	buildScript string // compile script
	abiFile     string // target abi file
	codeFile    string // target code bytes file
}

func (action *CompileWasmAction) execute(ctx *ContractContext) ([]byte, error) {
	fmt.Printf("<<<<<<start execute CompileWasmAction %s\n", action.projectName)

	if action == nil {
		return nil, errors.New("CompileWasmAction execute param error")
	}

	shellCmd := fmt.Sprintf("cd testdata/%s && sh %s", action.projectName, action.buildScript)
	cmd := exec.Command("sh", "-c", shellCmd)
	fmt.Printf("%v\n", cmd)

	ret, err := Output()
	if err != nil {
		fmt.Printf("%s\n", ret)
		return nil, err
	}

	if err != nil {
		return nil, errors.New(fmt.Sprintf("exec command %v, %s", cmd, err))
	}

	outputDir := fmt.Sprintf("testdata/%s/target", action.projectName)
	wasmFile := fmt.Sprintf("%s.wasm", action.targetName)
	cmd = exec.Command("find", outputDir, "-depth", "1", "-name", wasmFile)

	ret, err = Output()
	if err != nil {
		return nil, errors.New(fmt.Sprintf("find wasm file error"))
	}

	action.codeFile = fmt.Sprintf("%s", ret)
	action.codeFile = strings.Replace(action.codeFile, "\n", "", -1)

	if len(action.codeFile) == 0 {
		return nil, errors.New(fmt.Sprintf("compile %s error, codeFile is not generated", action.projectName))
	}

	cmd = exec.Command("find", outputDir+"/json", "-depth", "1", "-name", "*.json")
	ret, err = Output()
	if err != nil {
		return nil, errors.New(fmt.Sprintf("find abi file error"))
	}

	action.abiFile = fmt.Sprintf("%s", ret)
	action.abiFile = strings.Replace(action.abiFile, "\n", "", -1)

	if len(action.abiFile) == 0 {
		return nil, errors.New(fmt.Sprintf("compile %s error, abiFile is not generated", action.projectName))
	}

	code, err := ioutil.ReadFile(action.codeFile)
	if err != nil {
		return nil, err
	}

	ctx.addContractRes(&ContractRes{
		action.projectName,
		action.targetName,
		"",
		action.abiFile,
		action.codeFile,
		nil,
		nil,
		"",
		code,
		true,
	})
	fmt.Printf("<<<<<<finish execute CompileEvmAction %s\n", action.projectName)

	return code, err
}

func (action *CompileWasmAction) getName() string {
	return "CompileWasmAction"
}

func newCompileWasmAction(projectName, targetName, buildScript string) ContractAction {
	return &CompileWasmAction{
		projectName: projectName,
		targetName:  targetName,
		buildScript: buildScript,
	}
}

/// Contract deploy action
type DeployAction struct {
	mainClass           string
	constructorArgTypes string
	constructorArgVals  string

	ArgMgr
}

func (action *DeployAction) execute(ctx *ContractContext) ([]byte, error) {
	fmt.Printf("<<<<<<start execute DeployAction %s\n", action.mainClass)
	if action == nil || len(action.mainClass) == 0 ||
		ctx == nil || ctx.sender == nil || ctx.fvmInst == nil {
		return nil, errors.New("DeployAction execute param error")
	}

	byteCode, err := ctx.getContractCodeByte(action.mainClass)
	if err != nil {
		return nil, err
	}

	argTypes, argVals, err := action.buildParams()
	if err != nil {
		return nil, err
	}

	action.constructorArgTypes = argTypes
	action.constructorArgVals = argVals

	var paramHex string

	if len(action.constructorArgTypes) > 0 &&
		len(action.constructorArgVals) > 0 {
		cmd := exec.Command(NODE_CMD, "contract-tools/encode_parameter.js",
			"-types", action.constructorArgTypes, "-vals", action.constructorArgVals)
		fmt.Printf("command %v\n", cmd)

		ret, err := Output()

		if err != nil {
			return nil, errors.New("encode constructor param error")
		}

		paramHex = fmt.Sprintf("%s", ret[:len(ret)-1])
		fmt.Printf("param hex %s\n", paramHex)
	}

	var paramBytes []byte
	if len(paramHex) > 0 {
		paramBytes = common.Hex2Bytes(paramHex)
		// wasm code and param passed seprated
		// evm code = code + param
		/*if !ctx.isWasmCode( action.mainClass ) {
			byteCode = append( byteCode, paramBytes... )
			paramBytes = nil
		}*/
	}

	caller := vm.AccountRef(*ctx.sender)
	ret, addr, _, _, err := ctx.fvmInst.Create(
		caller,
		byteCode,
		ctx.gasLimit,
		common.Big0,
		ctx.asset,
		nil,
		paramBytes,
		false)

	if err != nil {
		return nil, err
	}

	ctx.setContractAddress(action.mainClass, &addr)
	ctx.setContractOwner(action.mainClass, ctx.sender)

	fmt.Printf("<<<<<<finish execute DeployAction %s address is %v\n", action.mainClass, addr)

	return ret, nil
}

func (action *DeployAction) getName() string {
	return "DeployAction"
}

// @func create DeployAction instance
// @param constructorArgTypes:  constructor arg types that passed as format: "string,uint256"
// @param constructorArgVals:  constructor arg vals that passed as format: "abc,123"
func newDeployAction(contractMainClass string) ContractAction {
	return &DeployAction{
		mainClass: contractMainClass,
	}
}

/// Contract call action
type CallAction struct {
	mainClass string
	funcName  string
	argTypes  string
	argVals   string
	caller    *common.Address

	ArgMgr
}

func (action *CallAction) execute(ctx *ContractContext) ([]byte, error) {
	fmt.Printf("<<<<<<start execute CallAction %s : %s\n", action.mainClass, action.funcName)

	if action == nil || ctx == nil {
		return nil, errors.New("CallAction execute param error")
	}

	contractRes := ctx.getContractRes(action.mainClass)
	if contractRes == nil {
		return nil, errors.New(fmt.Sprintf("CallAction execute contract %s is not exist", action.mainClass))
	}

	argTypes, argVals, err := action.buildParams()
	if err != nil {
		return nil, err
	}

	action.argTypes = argTypes
	action.argVals = argVals

	var cmd *exec.Cmd

	if len(action.argTypes) > 0 && len(action.argVals) > 0 {
		cmd = exec.Command(NODE_CMD, "contract-tools/encode_function_call.js",
			"-func", action.funcName, "-types", action.argTypes, "-vals", action.argVals)
	} else {
		cmd = exec.Command(NODE_CMD, "contract-tools/encode_function_call.js",
			"-func", action.funcName)
	}

	fmt.Printf("%v\n", cmd)
	ret, err := Output()
	if err != nil {
		return nil, errors.New(fmt.Sprintf("%v %s", cmd, err.Error()))
	}

	inputHex := fmt.Sprintf("%s", ret[:len(ret)-1])
	fmt.Printf("inputHex %s\n", inputHex)

	caller := vm.AccountRef(*ctx.sender)
	if action.caller != nil {
		caller = vm.AccountRef(*action.caller)
	}

	res, _, _, err := ctx.fvmInst.Call(
		caller,
		*contractRes.address,
		common.Hex2Bytes(inputHex),
		ctx.gasLimit,
		ctx.value,
		ctx.asset,
		false)

	if err != nil {
		return nil, err
	}

	fmt.Printf("<<<<<<finish execute CallAction %s result is %v\n", action.funcName, res)
	return res, nil
}

func (action *CallAction) getName() string {
	return "CallAction"
}

// @func create CallAction instance
// @param argTypes: function arg types that passed as format: "string,uint256", keep the declare order
// @param argVals	function arg vals that passed as format: "abc,123"
// @param caller 	some methods must be called by official adderss
func newCallAction(mainClass, funcName string, caller *common.Address) ContractAction {
	return &CallAction{
		mainClass: mainClass,
		funcName:  funcName,
		caller:    caller,
	}
}

//////////////// begin ContractWorkFlow /////////////////
/// keep several caontract action
type ContractWorkFlow struct {
	actions []ContractAction
}

/// add an contract base action
func (cw *ContractWorkFlow) addAction(action ContractAction) error {
	if cw == nil || action == nil {
		return errors.New("ContractWorkFlow addAction exception: param error")
	}

	cw.actions = append(cw.actions, action)
	return nil
}

func (cw *ContractWorkFlow) removeAllAction() {
	cw.actions = make([]ContractAction, 0)
}

func (cw *ContractWorkFlow) execute(ctx *ContractContext) ([]byte, error) {
	if cw == nil || cw.actions == nil {
		return nil, errors.New("ContractWorkFlow execute execption: param error")
	}

	var ret []byte
	var err error
	for _, action := range cw.actions {
		if action != nil {
			ret, err = action.execute(ctx)
			if err != nil {
				break
			}

			fmt.Println("")
		}
	}

	return ret, err
}

//////////////// begin ContractContainer ////////////////
/// contract user interface
type ContractContainer struct {
	workflow      *ContractWorkFlow
	chain         *BlockChain
	stateDB       *state.StateDB
	fvmInst       *vm.FVM
	isSupportWasm bool
	sender        *common.Address
	ctx           *ContractContext
}

func NewContractContainer(
	callerAddrHex string,
	asset *protos.Asset,
	isSupportWasm bool) (*ContractContainer, error) {
	cc := &ContractContainer{
		nil,
		nil,
		nil,
		nil,
		isSupportWasm,
		nil,
		&ContractContext{
			value: common.Big0,
			asset: asset,
		},
	}

	// init test chain
	_, err := cc.initChain()
	if err != nil {
		return nil, err
	}

	// init stateDB
	_, err = cc.initStateDB()
	if err != nil {
		return nil, err
	}

	// init fvm instance
	//addr := "mieQGjCWCk9h13FSXfqQ5tN3zGCbBXDXEN"
	_, err = cc.initFvmInst(callerAddrHex)
	if err != nil {
		return nil, err
	}

	cc.ctx.fvmInst = cc.fvmInst
	cc.ctx.sender = cc.sender
	cc.ctx.gasLimit = GAS_LIMIT

	err = cc.initWorkflow()
	if err != nil {
		return nil, err
	}

	err = cc.CheckGlobalEnv()
	if err != nil {
		return cc, err
	}

	err = cc.CheckEvmEnv()
	if err != nil {
		return cc, err
	}

	if isSupportWasm {
		err = cc.CheckWasmEnv()
	}

	cc.ctx.chain = cc.chain

	return cc, err
}

func (cc *ContractContainer) initWorkflow() error {
	if cc == nil {
		return errors.New("init work flow error")
	}

	cc.workflow = &ContractWorkFlow{
		make([]ContractAction, 0),
	}

	return nil
}

func (cc *ContractContainer) initChain() (*BlockChain, error) {
	//privateKey := "0xd0f0461b7b4d26cf370e6c73b58ef7fa26e8e30853a8cee901ed42cf0879cb6e"
	chain, teardownFunc, err := newFakeChain(&chaincfg.DevelopNetParams)
	if err != nil {
		log.Errorf("newFakeChain error %v", err)
	}
	if chain == nil {
		return nil, errors.New("init chain error")
	}
	defer teardownFunc()

	cc.chain = chain
	return chain, nil
}

func (cc *ContractContainer) initStateDB() (*state.StateDB, error) {
	db := ethdb.NewMemDatabase()

	stateDB, err := state.New(common.Hash{}, state.NewDatabase(db))

	if err != nil {
		return nil, err
	}

	cc.stateDB = stateDB

	return stateDB, nil
}

func (cc *ContractContainer) initFvmInst(callerAddrHex string) (*vm.FVM, error) {
	caller := common.HexToAddress(callerAddrHex)

	header := protos.BlockHeader{
		Height:    cc.chain.bestChain.Height(),
		Timestamp: cc.chain.bestChain.tip().GetTime(),
	}

	gasPrice := new(big.Int).SetInt64(1)

	block := asiutil.NewBlock(&protos.MsgBlock{
		Header:header,
	})
	context := fvm.NewFVMContext(caller, gasPrice, block, cc.chain, nil,nil)

	/*if cc.isSupportWasm {
		params.TestChainConfig.EWASMBlock = common.Big0
	} else {
		params.TestChainConfig.EWASMBlock = big.NewInt( 100 )
	}*/

	vmConfig := &vm.Config{
		FVMInterpreter: "",
	}

	if cc.isSupportWasm {
		// path of libhera.dylib
		vmConfig.EWASMInterpreter = "/Users/zhangjing/coding/flow_rust/hera/build/src/libhera.dylib,engine=binaryen,metering=false,evm1mode=evm2wasm.js"
	}

	inst := vm.NewFVM(context, cc.stateDB, chaincfg.ActiveNetParams.FvmParam, *vmConfig)
	cc.fvmInst = inst
	cc.sender = &caller

	return inst, nil
}

func (cc *ContractContainer) CheckGlobalEnv() error {
	cmdTest := exec.Command(NODE_CMD, "contract-tools/test_env.js")
	ret, err := cmdTest.Output()

	if err != nil {
		fmt.Printf("%s\n", ret)
		fmt.Println("web3 js lib is not available, you can run script `init_web3_env.sh` firstly which located in dir `contract-tools`")
	}

	return err
}

func (cc *ContractContainer) CheckWasmEnv() error {
	cmdTest := exec.Command("which", "wasm-build")
	ret, err := cmdTest.Output()

	if err != nil {
		fmt.Printf("%s\n", ret)
		fmt.Println("rust compile tool-chain is not available, you can run script `init_wasm_env.sh` firstly which located in dir `contract-tools`")
	}

	return err
}

func (cc *ContractContainer) CheckEvmEnv() error {
	// test env first
	cmdTest := exec.Command("find", "/usr/local/opt/z3/lib/", "-name", "libz3.dylib")
	ret, err := cmdTest.Output()

	if err != nil || ret == nil || len(ret) == 0 {
		fmt.Printf("%s\n", ret)
		fmt.Println("libz3.dylib is not exist which used by compiler solc, you can run script `init_evm_env.sh` firstly which located in dir `contract-tools`")
		return errors.New("libz3.dylib is not exist")
	}

	return err
	/*
		// install filesync
		fmt.Println( "start yarn add filesync" )
		cmd := exec.Command( "yarn", "add", "filesync" )
		ret, err := Output()
		if err !=  nil {
			fmt.Printf( "%s\n", ret )
			return err
		}
		fmt.Println( "yarn add filesync success" )

		// install web3
		fmt.Println( "start yarn add web3" )
		cmd = exec.Command( "yarn", "add", "web3@1.0.0-beta.34" )
		ret, err = Output()
		if err != nil {
			fmt.Printf( "%s\n", ret )
			return err
		}
		fmt.Println( "yarn add web3 success" )

		// move to specified dir
		cmd = exec.Command( "mv", "-f", "node_modules", "contract-tools/" )
		ret, err = Output()
		if err != nil {
			fmt.Printf( "%s\n", ret )
			return err
		}
		fmt.Println( "mv node_modules success" )

		// create lib dir
		libDir := "/usr/local/opt/z3/lib/"
		cmd = exec.Command( "mkdir", "-p", libDir )
		ret, err = Output()
		if err != nil {
			fmt.Printf( "%s\n", ret )
			return err
		}

		// download lib
		libUrl := "https://git.cs.colorado.edu/csci7135-analysis-s16/project-anna/raw/master/project/symbc_test/lib/libz3.dylib"
		fmt.Println( "start download libz3.dylib" )
		cmd = exec.Command( "wget", "-p", libDir, libUrl )
		ret, err = Output()

		if err != nil {
			fmt.Printf( "%s\n", ret )
			fmt.Printf( "you can execute command in shell directly: wget -p %s %s\n", libDir, libUrl )
			return err
		}
		fmt.Println( "download libz3.dylib success" )

		ret, err = cmdTest.Output()
		if err != nil {
			fmt.Printf( "%s\n", ret )
		}

		return err*/
}

func (cc *ContractContainer) AddAction(action ContractAction) error {
	if cc == nil || cc.workflow == nil || action == nil {
		return errors.New("ContractContainer AddAction exception: param error")
	}

	return cc.workflow.addAction(action)
}

// @function execute action flow
// @return []byte is result of last action, it is valid when error is nil
// @return error
func (cc *ContractContainer) Execute() ([]byte, error) {
	if cc == nil || cc.workflow == nil || cc.ctx == nil {
		return nil, errors.New("ContractContainer Execute exception: param error")
	}

	ret, err := cc.workflow.execute(cc.ctx)

	cc.workflow.removeAllAction()

	return ret, err
}
