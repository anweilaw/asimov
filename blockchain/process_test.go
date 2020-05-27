// Copyright (c) 2018-2020. The asimov developers
// Copyright (c) 2013-2017 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.
package blockchain

import (
	"crypto/ecdsa"
	"github.com/AsimovNetwork/asimov/asiutil"
	"github.com/AsimovNetwork/asimov/common"
	"github.com/AsimovNetwork/asimov/crypto"
	"github.com/AsimovNetwork/asimov/protos"
	"testing"
)

func TestAddressVerifySignature(t *testing.T) {
	var sigAddr = []*common.Address {
		{102,  60, 248, 184, 101, 242, 247, 229,  34, 255,  99, 144,  89, 224, 164,  55, 200,  73, 238,  90, 111},
		{},
	}

	var hash = [][]byte {
		{123, 123, 123, 123, 123, 123, 123, 123, 123, 123, 123, 123, 123, 123},
		{},
	}
	var sig = [][]byte {
		{123, 123, 123, 123, 123, 123, 123, 123, 123, 123, 123, 123, 123, 123, 123, 123, 123, 123, 123, 123, 123, 123, 123, 123, 123, 123, 123, 123, 123, 123, 123, 123},
		{},
	}

	//sigAddr[0]对应的私钥:
	privateKey := "0xd07f68f78fc58e3dc8ea72ff69784aa9542c452a4ee66b2665fa3cccb48441c2"
	// init account
	acc, _ := crypto.NewAccount(privateKey)

	tests := []struct {
		addr		*common.Address
		hash  		[]byte
		signature	[]byte
		errStr		string
	} {
		//test common sig::
		{
			nil,
			nil,
			nil,
			"",
		},
		//test sigAddr and signature mismatch:
		{
			sigAddr[0],
			nil,
			nil,
			"ErrSigAndKeyMismatch",
		},
		//test sigAddr error:
		{
			sigAddr[1],
			nil,
			nil,
			"ErrSigAndKeyMismatch",
		},
		//test hash:
		{
			nil,
			hash[0],
			nil,
			"ErrBadHashLength",
		},
		{
			nil,
			hash[1],
			nil,
			"ErrBadHashLength",
		},
		//test sigData:
		{
			nil,
			nil,
			sig[0],
			"ErrBadSigLength",
		},
		{
			nil,
			nil,
			sig[1],
			"ErrBadSigLength",
		},
	}

	t.Logf("Running %d tests", len(tests))
	for i, test := range tests {
		t.Logf("start test %d", i)
		tmpHash := common.Hash{0x01,0x02,0x03,0x04,0x05,0x06,0x07,0x08,0x01,0x02,0x03,0x04,0x05,0x06,0x07,0x08,0x01,0x02,0x03,0x04,0x05,0x06,0x07,0x08,0x01,0x02,0x03,0x04,0x05,0x06,0x07,0x08,}
		tmpHash[31] = byte(i)
		blockHash := tmpHash
		signature, sigErr := crypto.Sign(blockHash[:], (*ecdsa.PrivateKey)(&acc.PrivateKey))
		if sigErr != nil {
			t.Errorf("tests #%d error %v", i, sigErr)
		}

		var testAddr common.Address
		if test.addr == nil {
			testAddr = acc.Address.StandardAddress()
		} else {
			testAddr = *test.addr
		}

		testHash := make([]byte,0,1)
		if test.hash == nil {
			testHash = append(testHash, blockHash[:]...)
		} else {
			testHash = append(testHash, test.hash[:]...)
		}

		var sig []byte
		if test.signature == nil {
			sig = signature
		} else {
			sig = test.signature
		}

		checkErr := AddressVerifySignature(testHash[:], &testAddr, sig)
		if checkErr != nil {
			if dbErr, ok := checkErr.(RuleError); !ok || dbErr.ErrorCode.String() != test.errStr {
				t.Errorf("tests the %d error: %v", i, checkErr)
			}
		}
	}
}


func TestProcessBlock(t *testing.T) {
	parivateKeyList := []string{
		"0xd0f0461b7b4d26cf370e6c73b58ef7fa26e8e30853a8cee901ed42cf0879cb6e",  //privateKey0
		"0xd07f68f78fc58e3dc8ea72ff69784aa9542c452a4ee66b2665fa3cccb48441c2",  //privateKey1
		"0x77366e621236e71a77236e0858cd652e92c3af0908527b5bd1542992c4d7cace",  //privateKey2
	}

	accList, netParam, chain, teardownFunc, err := createFakeChainByPrivateKeys(parivateKeyList, 3)
	if err != nil || chain == nil {
		t.Errorf("create block error %v", err)
	}
	defer teardownFunc()

	genesisNode := chain.bestChain.tip()
	validators, filters, _ := chain.GetValidatorsByNode(1,genesisNode)

	//create 4 mainChainblocks:-----------------------------------------------
	mainChainBlockList := make([]asiutil.Block,0)
	tmpValidator := validators
	tmpFilters := filters
	tmpEpoch := uint32(1)
	tmpSlotIndex := uint16(0)
	tmpPreHash := genesisNode.hash
	mainChainBestNode := genesisNode
	for i:=0; i<4; i++ {
		if i != 0 {
			if i == int(netParam.RoundSize) {
				tmpSlotIndex = 0
				tmpEpoch += 1
				tmpValidator, tmpFilters, _ = chain.GetValidatorsByNode(tmpEpoch,mainChainBestNode)
				log.Infof("validators2 = %v",tmpValidator)
			} else {
				tmpSlotIndex ++
			}
		}

		mainChainBlock, mainChainNode, err := createAndSignBlock(
			netParam, accList, tmpValidator, tmpFilters, chain, tmpEpoch, tmpSlotIndex, int32(i),
			protos.Asset{0,0}, 0, tmpValidator[tmpSlotIndex],
			nil,0,mainChainBestNode)
		if err != nil {
			t.Errorf("create block error %v", err)
		}
		mainChainBlock.MsgBlock().Header.PrevBlock = tmpPreHash
		mainChainBlockList = append(mainChainBlockList, *mainChainBlock)
		mainChainBestNode = mainChainNode
		tmpPreHash = mainChainBestNode.hash
	}

	//create block with err signature:
	errSignBlock := mainChainBlockList[2]
	testSignature := [protos.HashSignLen]byte{0}
	copy(errSignBlock.MsgBlock().Header.SigData[:], testSignature[:])

	//create orphanBlock:
	orphanBlock := mainChainBlockList[3]
	orphanBlock.MsgBlock().Header.PrevBlock = common.Hash{0x01,}

	//create 4 sideChainblocks:-----------------------------------------------
	sideChainBlockList := make([]asiutil.Block,0)
	tmpValidator = validators
	tmpFilters = filters
	tmpEpoch = uint32(1)
	tmpSlotIndex = uint16(0)
	tmpPreHash = genesisNode.hash
	sideChainBestNode := genesisNode
	for i:=0; i<4; i++ {
		if i != 0 {
			if i == int(netParam.RoundSize) {
				tmpSlotIndex = 0
				tmpEpoch += 1
				tmpValidator, tmpFilters, _ = chain.GetValidatorsByNode(tmpEpoch,sideChainBestNode)
				log.Infof("validators2 = %v",tmpValidator)
			} else {
				tmpSlotIndex ++
			}
		}

		sideChainBlock, sideChainNode, err := createAndSignBlock(netParam, accList, tmpValidator, tmpFilters, chain,
			tmpEpoch, tmpSlotIndex, int32(i), protos.Asset{0,0}, 0,
			tmpValidator[tmpSlotIndex],nil,int32(i+1),sideChainBestNode)
		if err != nil {
			t.Errorf("create block error %v", err)
		}

		sideChainBlock.MsgBlock().Header.PrevBlock = tmpPreHash
		sideChainBlockList = append(sideChainBlockList, *sideChainBlock)
		sideChainBestNode = sideChainNode
		tmpPreHash = sideChainBestNode.hash
	}

	tests := []struct {
		flags        common.BehaviorFlags
		block        *asiutil.Block
		isMainFlag   bool
		isOrphanFlag bool
		errStr       string
	} {
		//test0: normal block: BehaviorFlags = 0:
		{
			1,
			&mainChainBlockList[0],
			true,
			false,
			"",
		},
		//test1: normal block: BehaviorFlags = 0:
		{
			0,
			&mainChainBlockList[1],
			true,
			false,
			"",
		},
		//test2: orphan block:
		{
			0,
			&orphanBlock,
			false,
			true,
			"",
		},
		//test3: block that already exist in bestchain:
		{
			1,
			&mainChainBlockList[1],
			false,
			false,
			"ErrDuplicateBlock",
		},

		//test4: block that already exist in orphan block pool:
		{
			0,
			&orphanBlock,
			false,
			false,
			"ErrDuplicateBlock",
		},
		//test5: block with err signature:
		{
			0,
			&errSignBlock,
			false,
			false,
			"ErrInvalidSigData",
		},
		//test6: block with height 1 in sideChain:
		{
			0,
			&sideChainBlockList[0],
			false,
			false,
			"",
		},
		//test7: add sideChainblock3 that base on sideChainblock2:
		{
			0,
			&sideChainBlockList[2],
			false,
			true,
			"",
		},
		//test8: add sideChainblock2 which is parent of sideChainblock3: test processOrphans and reorganizeChain:
		{
			0,
			&sideChainBlockList[1],
			false,
			false,
			"",
		},
		//test9: add block4:test round change:
		{
			0,
			&sideChainBlockList[3],
			true,
			false,
			"",
		},
	}

	t.Logf("Running %d TestProcessBlock tests", len(tests))
	for i, test := range tests {
		isMain,isOrphan,checkErr := chain.ProcessBlock(test.block, nil, test.flags)
		if checkErr != nil {
			if dbErr, ok := checkErr.(RuleError); !ok || dbErr.ErrorCode.String() != test.errStr {
				t.Errorf("tests the %d error: %v", i, checkErr)
			}
		} else {
			if isMain != test.isMainFlag || isOrphan != test.isOrphanFlag {
				t.Errorf("did not received expected error in the %d test", i)
			}
		}
	}
}
