// Copyright (c) 2018-2020 The asimov developers
// Copyright (c) 2013-2017 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package mining

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/AsimovNetwork/asimov/ainterface"
	"github.com/AsimovNetwork/asimov/asiutil"
	"github.com/AsimovNetwork/asimov/blockchain"
	"github.com/AsimovNetwork/asimov/blockchain/txo"
	"github.com/AsimovNetwork/asimov/chaincfg"
	"github.com/AsimovNetwork/asimov/common"
	"github.com/AsimovNetwork/asimov/common/address"
	"github.com/AsimovNetwork/asimov/common/hexutil"
	"github.com/AsimovNetwork/asimov/consensus/satoshiplus/minersync"
	"github.com/AsimovNetwork/asimov/crypto"
	"github.com/AsimovNetwork/asimov/database"
	"github.com/AsimovNetwork/asimov/database/dbdriver"
	"github.com/AsimovNetwork/asimov/database/dbimpl/ethdb"
	"github.com/AsimovNetwork/asimov/protos"
	"github.com/AsimovNetwork/asimov/rpcs/rpc"
	"github.com/AsimovNetwork/asimov/txscript"
	"github.com/AsimovNetwork/asimov/vm/fvm"
	"github.com/AsimovNetwork/asimov/vm/fvm/core/vm"
	"github.com/AsimovNetwork/asimov/vm/fvm/params"
	"math/big"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

const (
	testDbType          = "ffldb"
)

var (
	defaultHomeDir = asiutil.AppDataDir("asimovd", false)
)

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// Manager defines an contract manager that manages multiple system contracts and
// implements the blockchain.ContractManager interface so it can be seamlessly
// plugged into normal chain processing.
type ManagerTmp struct {
	chain fvm.ChainContext
	//  genesis transaction data cache
	genesisDataCache map[common.ContractCode][]chaincfg.ContractInfo
}

// Init manager by genesis data.
func (m *ManagerTmp) Init(chain fvm.ChainContext, dataBytes [] byte) error {
	var cMap map[common.ContractCode][]chaincfg.ContractInfo
	err := json.Unmarshal(dataBytes, &cMap)
	if err != nil {
		return err
	}
	m.chain = chain
	m.genesisDataCache = cMap
	return nil
}

// Get latest contract by height.
func (m *ManagerTmp) GetActiveContractByHeight(height int32, delegateAddr common.ContractCode) *chaincfg.ContractInfo {
	contracts, ok := m.genesisDataCache[delegateAddr]
	if !ok {
		return nil
	}
	for i := len(contracts) - 1; i >= 0; i-- {
		if height >= contracts[i].BlockHeight {
			return &contracts[i]
		}
	}
	return nil
}

func NewContractManagerTmp() ainterface.ContractManager {
	return &ManagerTmp {}
}

// Ensure the Manager type implements the blockchain.ContractManager interface.
var _ ainterface.ContractManager = (*ManagerTmp)(nil)

func (m *ManagerTmp) GetSignedUpValidators(
	consensus common.ContractCode,
	block *asiutil.Block,
	stateDB vm.StateDB,
	chainConfig *params.ChainConfig,
	miners []string) ([]common.Address, []uint32, error){

	gas := uint64(common.SystemContractReadOnlyGas)
	officialAddr := chaincfg.OfficialAddress
	contract := m.GetActiveContractByHeight(block.Height(), common.ConsensusSatoshiPlus)
	if contract == nil {
		errStr := fmt.Sprintf("Failed to get active contract %s, %d", common.ConsensusSatoshiPlus, block.Height())
		log.Error(errStr)
		panic(errStr)
	}
	proxyAddr, abi := vm.ConvertSystemContractAddress(common.ConsensusSatoshiPlus), contract.AbiInfo

	funcName := common.ContractConsensusSatoshiPlus_GetSignupValidatorsFunction()
	runCode, err := fvm.PackFunctionArgs(abi, funcName, miners)

	result, _, err := fvm.CallReadOnlyFunction(officialAddr, block, m.chain, stateDB, chainConfig,
		gas, proxyAddr, runCode)
	if err != nil {
		log.Errorf("Get signed up validators failed, error: %s", err)
		return nil, nil, err
	}

	validators := make([]common.Address,0)
	heights := make([]*big.Int,0)
	outData := []interface {}{
		&validators,
		&heights,
	}

	err = fvm.UnPackFunctionResult(abi, &outData, funcName, result)
	if err != nil {
		log.Errorf("Get signed up validators failed, error: %s", err)
		return nil, nil, err
	}

	if len(validators) != len(heights) || len(miners) != len(heights) {
		errStr := "get signed up validators failed, length of validators does not match length of height"
		log.Errorf(errStr)
		return nil, nil, errors.New(errStr)
	}

	height32 := make([]uint32, len(heights))
	for i, h := range heights {
		height := uint32(h.Int64())
		height32[i] = height
	}

	return validators, height32, nil
}


func (m *ManagerTmp) GetContractAddressByAsset(
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

	//获取发币合约地址
	funcName := common.ContractRegistryCenter_GetOrganizationAddressesByAssetsFunction()
	assetId := make([]uint64, 0, 1)
	for _, asset := range assets {
		assetBytes := common.Hex2Bytes(asset)
		assetId = append(assetId, binary.BigEndian.Uint64(assetBytes[4:]))

	}
	runCode, err := fvm.PackFunctionArgs(abi, funcName, assetId)

	result, leftOvergas, err := fvm.CallReadOnlyFunction(officialAddr, block, m.chain, stateDB, chainConfig,
		gas, proxyAddr, runCode)
	if err != nil {
		log.Errorf("Get  contract address of asset failed, error: %s", err)
		return nil, false, leftOvergas
	}

	var outType []common.Address

	err = fvm.UnPackFunctionResult(abi, &outType, funcName, result)
	if err != nil {
		log.Errorf("Get contract address of asset failed, error: %s", err)
		return nil, false, leftOvergas
	}

	return outType, true, leftOvergas
}

func (m *ManagerTmp) GetAssetInfoByAssetId(
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

	//获取发币合约地址
	funcName := common.ContractRegistryCenter_GetAssetInfoByAssetIdFunction()
	//
	results := make([]ainterface.AssetInfo, 0)
	var err error
	for _, asset := range assets {
		assetBytes := common.Hex2Bytes(asset)
		orgIdBytes := assetBytes[4:8]
		assetIndexBytes := assetBytes[8:]

		var orgId = binary.BigEndian.Uint32(orgIdBytes)
		var assetIndex = binary.BigEndian.Uint32(assetIndexBytes)

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

func (m *ManagerTmp) GetTemplates(
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
	proxyAddr, abi := vm.ConvertSystemContractAddress(common.TemplateWarehouse), contract.AbiInfo

	// 1、获取模板数量
	getTemplatesCount := getCountFunc
	runCode, err := fvm.PackFunctionArgs(abi, getTemplatesCount, category)
	result, leftOvergas, err := fvm.CallReadOnlyFunction(officialAddr, block, m.chain, stateDB, chainConfig, gas,
		proxyAddr, runCode)
	if err != nil {
		log.Errorf("Get contract templates failed, error: %s", err)
		return 0, nil, err, leftOvergas
	}

	var outInt *big.Int
	err = fvm.UnPackFunctionResult(abi, &outInt, getTemplatesCount, result)
	if err != nil {
		log.Errorf("Get contract templates failed, error: %s", err)
		return 0, nil, err, leftOvergas
	}
	if outInt.Int64() == 0 {
		return 0, nil, nil, leftOvergas
	}

	// 2、获取模板信息
	template := make([]ainterface.TemplateWarehouseContent, 0)
	getTemplate := getTemplatesFunc

	// settings of Pagination
	fromIndex := int(outInt.Int64())-pageNo*pageSize-1
	if fromIndex < 0 {
		return 0, nil, nil, leftOvergas
	}
	endIndex := fromIndex+1-pageSize
	if endIndex < 0 {
		endIndex = 0
	}

	for i := fromIndex; i >= endIndex; i-- {
		runCode, err := fvm.PackFunctionArgs(abi, getTemplate, category, big.NewInt(int64(i)))
		if err != nil {
			log.Errorf("Get contract templates failed, error: %s", err)
			return 0, nil, err, leftOvergas
		}
		ret, leftOvergas, err := fvm.CallReadOnlyFunction(officialAddr, block, m.chain, stateDB, chainConfig,
			leftOvergas, proxyAddr, runCode)
		if err != nil {
			log.Errorf("Get contract templates failed, error: %s", err)
			return 0, nil, err, leftOvergas
		}

		cTime := new(big.Int)
		var keyType [32]byte
		outType := &[]interface{}{new(string), &keyType, &cTime, new(uint8), new(uint8), new(uint8), new(uint8)}
		err = fvm.UnPackFunctionResult(abi, outType, getTemplate, ret)
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
			template = append(template, ainterface.TemplateWarehouseContent{name, key, createTime, approveCount, rejectCount, reviewers, status})
		}
	}

	return int(outInt.Int64()), template, nil, leftOvergas
}

func (m *ManagerTmp) GetTemplate(
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
	proxyAddr, abi := vm.ConvertSystemContractAddress(common.TemplateWarehouse), contract.AbiInfo
	// 保存模板的合约的地址和ABI
	// createTemplateAddr, _, createTemplateABI := b.GetSystemContractInfo(chaincfg.TemplateWarehouse)
	// 通过名称获取合约的方法
	getTemplate := common.ContractTemplateWarehouse_GetTemplateFunction()
	runCode, err := fvm.PackFunctionArgs(abi, getTemplate, category, name)
	if err != nil {
		log.Errorf("Get contract template failed, error: %s", err)
		return ainterface.TemplateWarehouseContent{}, false, gas
	}
	ret, leftOvergas, err := fvm.CallReadOnlyFunction(officialAddr, block, m.chain, stateDB, chainConfig,
		gas, proxyAddr, runCode)
	if err != nil {
		log.Errorf("Get contract template failed, error: %s", err)
		return ainterface.TemplateWarehouseContent{}, false, leftOvergas
	}

	cTime := new(big.Int)
	var keyType [32]byte
	outType := &[]interface{}{new(string), &keyType, &cTime, new(uint8), new(uint8), new(uint8), new(uint8)}

	err = fvm.UnPackFunctionResult(abi, outType, getTemplate, ret)
	if err != nil {
		log.Errorf("Get contract template failed, error: %s", err)
		return ainterface.TemplateWarehouseContent{}, false, leftOvergas
	}

	key := common.Bytes2Hex(keyType[:])
	createTime := cTime.Int64()
	approveCount := *((*outType)[3]).(*uint8)
	rejectCount := *((*outType)[4]).(*uint8)
	reviewers := *((*outType)[5]).(*uint8)
	status := *((*outType)[6]).(*uint8)
	if status == blockchain.TEMPLATE_STATUS_NOTEXIST {
		return ainterface.TemplateWarehouseContent{}, false, leftOvergas
	}
	return ainterface.TemplateWarehouseContent{name, key,
		createTime, approveCount,
		rejectCount, reviewers,
		status},
		true, leftOvergas
}


type FeeItem struct {
	Assets   *protos.Assets
	Height   int32
}

func (m *ManagerTmp) GetFees(
	block *asiutil.Block,
	stateDB vm.StateDB,
	chainConfig *params.ChainConfig) (map[protos.Assets]int32, error, uint64) {

	officialAddr := chaincfg.OfficialAddress
	contract := m.GetActiveContractByHeight(block.Height(), common.ValidatorCommittee)
	if contract == nil {
		errStr := fmt.Sprintf("Failed to get active contract %s, %d", common.ValidatorCommittee, block.Height())
		log.Error(errStr)
		panic(errStr)
	}
	proxyAddr, abi := vm.ConvertSystemContractAddress(common.ValidatorCommittee), contract.AbiInfo

	// feelist func
	feelistFunc := common.ContractValidatorCommittee_GetAssetFeeListFunction()

	runCode, err := fvm.PackFunctionArgs(abi, feelistFunc)
	result, leftOvergas, err := fvm.CallReadOnlyFunction(officialAddr, block, m.chain, stateDB, chainConfig,
		common.SystemContractReadOnlyGas,
		proxyAddr, runCode)
	if err != nil {
		log.Errorf("Get contract templates failed, error: %s", err)
		return nil, err, leftOvergas
	}
	assets := make([]*big.Int,0)
	height := make([]*big.Int,0)
	outData := []interface {}{
		&assets,
		&height,
	}
	err = fvm.UnPackFunctionResult(abi, &outData, feelistFunc, result)
	if err != nil {
		log.Errorf("Get fee list failed, error: %s", err)
		return nil, err, leftOvergas
	}
	if len(assets) != len(height) {
		errStr := "Get fee list failed, length of assets does not match length of height"
		log.Errorf(errStr)
		return nil, errors.New(errStr), leftOvergas
	}

	fees := make(map[protos.Assets]int32)
	for i := 0; i < len(assets); i++ {
		pAssets := protos.AssetFromBytes(assets[i].Bytes())
		fees[*pAssets] = int32(height[i].Int64())
	}

	return fees, nil, leftOvergas
}

func (m *ManagerTmp) IsLimit(block *asiutil.Block,
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
		common.SystemContractReadOnlyGas, proxyAddr, input)
	if err != nil {
		log.Error(err)
		return -1
	}

	var outType = &[]interface{}{new(bool), new(bool)}
	err = fvm.UnPackFunctionResult(abi, &outType, funcName, result)
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

func (m *ManagerTmp) IsSupport(block *asiutil.Block,
	stateDB vm.StateDB, gasLimit uint64, assets *protos.Assets, address []byte) (bool, uint64) {
	if gasLimit < common.SupportCheckGas {
		return false, 0
	}
	_, organizationId, assetIndex := assets.AssetsFields()
	transferAddress := common.BytesToAddress(address)

	caller := chaincfg.OfficialAddress

	contract := m.GetActiveContractByHeight(block.Height(), common.RegistryCenter)
	if contract == nil {
		errStr := fmt.Sprintf("Failed to get active contract %s, %d", common.RegistryCenter, block.Height())
		log.Error(errStr)
		panic(errStr)
	}
	proxyAddr, abi := vm.ConvertSystemContractAddress(common.RegistryCenter), contract.AbiInfo
	funcName := common.ContractRegistryCenter_GetOrganizationAddressByIdFunction()
	input, err := fvm.PackFunctionArgs(abi, funcName, organizationId, assetIndex, transferAddress)
	if err != nil {
		return false, gasLimit
	}

	result, leftOverGas, err := fvm.CallReadOnlyFunction(caller, block, m.chain,
		stateDB, chaincfg.ActiveNetParams.FvmParam,
		common.SupportCheckGas, proxyAddr, input)
	if err != nil {
		log.Error(err)
		return false, leftOverGas
	}

	var outType common.Address
	err = fvm.UnPackFunctionResult(abi, &outType, funcName, result)
	if err != nil {
		log.Error(err)
	}

	if common.EmptyAddressValue == outType.String() {
		return false, gasLimit - common.SupportCheckGas + leftOverGas
	}

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
/////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

/////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
type HistoryFlag byte

var (
	roundBucketName = []byte("roundIdx")
)

type RoundValidators struct {
	blockHash  common.Hash
	validators []*common.Address
	weightmap  map[common.Address]uint16
}

type RoundMinerInfo struct {
	round      uint32
	middleTime uint32
	miners     map[string]float64
	mapping    map[string]*ainterface.ValidatorInfo
}

type RoundManager struct {
	minerCollector *minersync.BtcPowerCollector
	db             database.Transactor

	// round cache
	vLock                sync.RWMutex
	roundValidatorsCache []*RoundValidators
	historyValidators map[common.Address]HistoryFlag

	rmLock  sync.RWMutex
	rmCache []*RoundMinerInfo
}

func (m *RoundManager) HasValidator(validator common.Address) bool {
	return false
}

func (m *RoundManager) GetValidators(blockHash common.Hash, round uint32, fn ainterface.GetValidatorsCallBack) (
	[]*common.Address, map[common.Address]uint16, error) {
	privateKey := "0xd0f0461b7b4d26cf370e6c73b58ef7fa26e8e30853a8cee901ed42cf0879cb6e"
	privKeyBytes, _ := hexutil.Decode(privateKey)
	_, publicKey := crypto.PrivKeyFromBytes(crypto.S256(), privKeyBytes)
	pkaddr, _ := address.NewAddressPubKey(publicKey.SerializeCompressed())
	addr := pkaddr.AddressPubKeyHash()
	validators := make([]*common.Address, 0, chaincfg.ActiveNetParams.RoundSize)
	for i := uint16(0); i < chaincfg.ActiveNetParams.RoundSize; i++ {
		validators = append(validators, addr)
	}
	weightmap := make(map[common.Address]uint16)
	weightmap[*addr] = chaincfg.ActiveNetParams.RoundSize
	return validators, weightmap, nil
}

func (m *RoundManager) setValidators(blockHash common.Hash, validators []*common.Address) map[common.Address]uint16 {
	return nil
}

func (m *RoundManager) getRoundMinerFromCache(round uint32) *RoundMinerInfo {
	return nil
}

func (m *RoundManager) setRoundMiner(roundMiner *RoundMinerInfo) {
}

func (m *RoundManager) getMinersInfo(round uint32) (*RoundMinerInfo, error) {
	return nil, nil
}

func (m *RoundManager) GetMinersByRound(round uint32) (map[string]float64, error) {
	return nil, nil
}

func (m *RoundManager) GetHsMappingByRound(round uint32) (map[string]*ainterface.ValidatorInfo, error) {
	return nil, nil
}

func (m *RoundManager) Init(round uint32, db database.Transactor, c ainterface.IBtcClient) error {
	return nil
}

func (m *RoundManager) Start() {
}

func (m *RoundManager) Halt() {
}

func (m *RoundManager) GetContract() common.ContractCode {
	return common.ConsensusSatoshiPlus
}

func NewRoundManager() *RoundManager {
	return &RoundManager{}
}

func (m *RoundManager) GetRoundInterval(round int64) int64 {
	return 5 * 120
}

func (m *RoundManager) GetNextRound(round *ainterface.Round) (*ainterface.Round, error) {
	return nil, nil
}

var _ ainterface.IRoundManager = (*RoundManager)(nil)
/////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

/////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
type FakeBtcClient struct {
	minerInfo []minersync.GetBitcoinBlockMinerInfoResult
	chainInfo *minersync.GetBlockChainInfoResult
}

func NewFakeBtcClient(height, count int32) *FakeBtcClient {
	minerInfo := make([]minersync.GetBitcoinBlockMinerInfoResult, 0, count)
	c := &FakeBtcClient{
		minerInfo: minerInfo,
	}
	c.push(height, count)
	return c
}

func (c *FakeBtcClient) Push(count int32) {
	height := c.minerInfo[len(c.minerInfo)-1].Height + 1
	c.push(height, count)
}

func (c *FakeBtcClient) push(height, count int32) {

	for i := int32(0); i < count; i++ {
		c.minerInfo = append(c.minerInfo, minersync.GetBitcoinBlockMinerInfoResult{
			Height:  height + i,
			Pool:    "",
			Hash:    strconv.Itoa(int(height + i)),
			Address: "11",
			Time:    600 * uint32(i),
		})
	}

	c.chainInfo = &minersync.GetBlockChainInfoResult{
		Blocks: height + count - 1,
	}
}

func (c *FakeBtcClient) GetBitcoinMinerInfo(result interface{}, height, count int32) error {
	begin := c.minerInfo[0].Height
	last := c.minerInfo[len(c.minerInfo)-1].Height
	if height < begin || height+count-1 > last {
		return errors.New("miners not find")
	}
	miners := result.(*[]minersync.GetBitcoinBlockMinerInfoResult)
	*miners = c.minerInfo[height-begin : height-begin+count]
	return nil
}

func (c *FakeBtcClient) GetBitcoinBlockChainInfo(result interface{}) error {
	chaininfo := result.(*minersync.GetBlockChainInfoResult)
	*chaininfo = *c.chainInfo
	return nil
}

/////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

type fakeIn struct {
	account     *crypto.Account
	amount      int64
	assets      *protos.Assets
	index       uint32
	coinbase    bool
	blockHeight int32
	blockHash   common.Hash
}

type fakeOut struct {
	addr   *common.Address
	amount int64
	assets *protos.Assets
}

func createFakeTx(vin []*fakeIn, vout []*fakeOut, global_view *blockchain.UtxoViewpoint) *asiutil.Tx {
	txMsg := protos.NewMsgTx(protos.TxVersion)

	if global_view == nil {
		global_view = blockchain.NewUtxoViewpoint()
	}
	for _, v := range vin {
		inaddr := v.account.Address
		prePkScript, _ := txscript.PayToAddrScript(inaddr)
		outpoint := protos.OutPoint{v.blockHash, v.index}
		txin := protos.NewTxIn(&outpoint, nil)
		txMsg.AddTxIn(txin)

		entry := txo.NewUtxoEntry(v.amount, prePkScript, v.blockHeight, v.coinbase, v.assets, nil)
		global_view.AddEntry(outpoint, entry)
	}

	for _, v := range vout {
		pkScript, _ := txscript.PayToAddrScript(v.addr)
		txout := protos.NewTxOut(v.amount, pkScript, *v.assets)
		txMsg.AddTxOut(txout)
	}

	for i, v := range vin {
		entry := global_view.LookupEntry(protos.OutPoint{v.blockHash, v.index})
		// Sign the txMsg transaction.
		lookupKey := func(a common.IAddress) (*crypto.PrivateKey, bool, error) {
			return &v.account.PrivateKey, true, nil
		}
		sigScript, _ := txscript.SignTxOutput(txMsg, i, entry.PkScript(), txscript.SigHashAll, txscript.KeyClosure(lookupKey), nil, nil)
		txMsg.TxIn[i].SignatureScript = sigScript
	}

	tx := asiutil.NewTx(txMsg)
	return tx
}

func cleanAndExpandPath(path string) string {
	// Expand initial ~ to OS specific home directory.
	if strings.HasPrefix(path, "~") {
		homeDir := filepath.Dir(defaultHomeDir)
		path = strings.Replace(path, "~", homeDir, 1)
	}

	// NOTE: The os.ExpandEnv doesn't work with Windows-style %VARIABLE%,
	// but they variables can still be expanded via POSIX-style $VARIABLE.
	return filepath.Clean(os.ExpandEnv(path))
}

func newFakeChain(paramstmp *chaincfg.Params) (*blockchain.BlockChain, func(), error) {
	cfg := &chaincfg.FConfig{
		ConfigFile:           chaincfg.DefaultConfigFile,
		DebugLevel:           chaincfg.DefaultLogLevel,
		MaxPeers:             chaincfg.DefaultMaxPeers,
		BanDuration:          chaincfg.DefaultBanDuration,
		BanThreshold:         chaincfg.DefaultBanThreshold,
		RPCMaxClients:        chaincfg.DefaultMaxRPCClients,
		RPCMaxWebsockets:     chaincfg.DefaultMaxRPCWebsockets,
		RPCMaxConcurrentReqs: chaincfg.DefaultMaxRPCConcurrentReqs,
		DataDir:              chaincfg.DefaultDataDir,
		LogDir:               chaincfg.DefaultLogDir,
		StateDir:             chaincfg.DefaultStateDir,
		RPCKey:               chaincfg.DefaultRPCKeyFile,
		RPCCert:              chaincfg.DefaultRPCCertFile,
		MaxOrphanTxs:         chaincfg.DefaultMaxOrphanTransactions,
		MaxOrphanTxSize:      chaincfg.DefaultMaxOrphanTxSize,
		HTTPEndpoint:         chaincfg.DefaultHTTPEndPoint,
		HTTPModules:          chaincfg.DefaultHttpModules,
		HTTPVirtualHosts:     chaincfg.DefaultHTTPVirtualHosts,
		HTTPTimeouts:         rpc.DefaultHTTPTimeouts,
		WSEndpoint:           chaincfg.DefaultWSEndPoint,
		WSModules:            chaincfg.DefaultWSModules,
		DevelopNet:           true,
		Consensustype:        "poa",
	}

	consensus := common.GetConsensus(cfg.Consensustype)
	if consensus < 0 {
		log.Errorf("The consensus type is %v, which is not support", consensus)
		return nil, nil, nil
	}
	chaincfg.ActiveNetParams.Params = paramstmp

	var teardown func()
	cfg.DataDir = cleanAndExpandPath(cfg.DataDir)
	cfg.DataDir = filepath.Join(cfg.DataDir, "devnetUnitTest")

	// Append the network type to the logger directory so it is "namespaced"
	// per network in the same fashion as the data directory.
	cfg.LogDir = cleanAndExpandPath(cfg.LogDir)
	cfg.LogDir = filepath.Join(cfg.LogDir, "devnetUnitTest")

	cfg.StateDir = cleanAndExpandPath(cfg.StateDir)
	cfg.StateDir = filepath.Join(cfg.StateDir, "devnetUnitTest")

	// Load the block database.
	db, err := loadBlockDB(cfg)
	if err != nil {
		return nil, nil, err
	}

	// Load StateDB
	stateDB, stateDbErr := ethdb.NewLDBDatabase(cfg.StateDir, 768, 1024)
	if stateDbErr != nil {
		teardown()
		log.Errorf("%v", stateDbErr)
		return nil, nil, stateDbErr
	}

	// Create a btc rpc client
	btcClient := NewFakeBtcClient(chaincfg.ActiveNetParams.CollectHeight,
		chaincfg.ActiveNetParams.CollectInterval+int32(chaincfg.ActiveNetParams.BtcBlocksPerRound)*24)

	// Create a round manager.
	roundManger := NewRoundManager()

	// Create a contract manager.
	contractManager := NewContractManagerTmp()

	dir, err := os.Getwd()
	genesisBlock, err := asiutil.LoadBlockFromFile("../genesisbin/devnet.block")
	if err != nil {
		strErr := "Load genesis block error, " + err.Error() + dir
		return nil, nil, errors.New(strErr)
	}
	genesisHash := asiutil.NewBlock(genesisBlock).Hash()
	if *chaincfg.ActiveNetParams.GenesisHash != *genesisHash {
		strErr := fmt.Sprintf("Load genesis block genesis hash mismatch expected %s, but %s",
			chaincfg.ActiveNetParams.GenesisHash.String(), genesisHash.String())
		return nil, nil, errors.New(strErr)
	}
	chaincfg.ActiveNetParams.GenesisBlock = genesisBlock

	chain, err := blockchain.New(&blockchain.Config{
		DB: db,
		//Interrupt:     interrupt,
		ChainParams: paramstmp,
		//Checkpoints:   checkpoints,
		TimeSource: blockchain.NewMedianTime(),
		//IndexManager:  indexManager,
		StateDB: stateDB,
		//Account:       &acc,
		//TemplateIndex: s.templateIndex,
		BtcClient:       btcClient,
		RoundManager:    roundManger,
		ContractManager: contractManager,
	}, nil)
	if err != nil {
		teardown = func() {
			db.Close()
			stateDB.Close()
			os.RemoveAll(cfg.DataDir)
			os.RemoveAll(cfg.LogDir)
			os.RemoveAll(cfg.StateDir)
		}
		return nil, nil, err
	}

	teardown = func() {
		db.Close()
		stateDB.Close()
		os.RemoveAll(cfg.DataDir)
		os.RemoveAll(cfg.LogDir)
		os.RemoveAll(cfg.StateDir)
	}

	chaincfg.Cfg = cfg
	return chain, teardown, nil
}

const (
	// blockDbNamePrefix is the prefix for the block database name.  The
	// database type is appended to this value to form the full block
	// database name.
	blockDbNamePrefix = "blocks"
)

func blockDbPath(dataDir, dbType string) string {
	// The database name is based on the database type.
	dbName := blockDbNamePrefix + "_" + dbType
	if dbType == "sqlite" {
		dbName = dbName + ".db"
	}
	dbPath := filepath.Join(dataDir, dbName)
	return dbPath
}

func loadBlockDB(cfg *chaincfg.FConfig) (database.Database, error) {
	// The memdb backend does not have a file path associated with it, so
	// handle it uniquely.  We also don't want to worry about the multiple
	// database type warnings when running with the memory database.

	// The database name is based on the database type.
	dbPath := blockDbPath(cfg.DataDir, testDbType)

	//在测试代码中要连接测试网络
	//db, err := database.Open(cfg.DbType, dbPath, configuration.ActiveNetParams.Net)
	db, err := dbdriver.Open(testDbType, dbPath, chaincfg.ActiveNetParams.Params.Net)
	if err != nil {
		// Return the error if it's not because the database doesn't
		// exist.
		if dbErr, ok := err.(database.Error); !ok || dbErr.ErrorCode !=
			database.ErrDbDoesNotExist {

			return nil, err
		}

		// Create the db if it does not exist.
		err = os.MkdirAll(cfg.DataDir, 0700)
		if err != nil {
			return nil, err
		}
		//在测试代码中要连接测试网络
		//db, err = database.Create(cfg.DbType, dbPath, configuration.ActiveNetParams.Net)
		db, err = dbdriver.Create(testDbType, dbPath, chaincfg.ActiveNetParams.Params.Net)
		if err != nil {
			return nil, err
		}
	}

	return db, nil
}

type fakeTxSource struct {
	pool map[common.Hash]*TxDesc
}

func (fts *fakeTxSource) clear() {
	fts.pool = make(map[common.Hash]*TxDesc)
}

func (fts *fakeTxSource) push(tx *TxDesc) {
	fts.pool[*tx.Tx.Hash()] = tx
}

// MiningDescs returns a slice of mining descriptors for all the
// transactions in the source pool.
func (fts *fakeTxSource) TxDescs() TxDescList {
	descs := make(TxDescList, len(fts.pool))
	i := 0
	for _, desc := range fts.pool {
		descs[i] = desc
		i++
	}
	return descs
}

func (fts *fakeTxSource) UpdateForbiddenTxs(txHashes []*common.Hash, height int64) {
}

type fakeSigSource struct {
	pool []*asiutil.BlockSign
}

func (fss *fakeSigSource) push(sign *asiutil.BlockSign) {
	fss.pool = append(fss.pool, sign)
}

func (fss *fakeSigSource) MiningDescs(height int32) []*asiutil.BlockSign {
	descs := make([]*asiutil.BlockSign, 0, common.BlockSignDepth)
	for _, v := range fss.pool {
		if v.MsgSign.BlockHeight >= height-common.BlockSignDepth {
			descs = append(descs, v)
		}
	}
	return descs
}
