// Copyright (c) 2018-2020. The asimov developers
// Copyright (c) 2013-2017 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.
package blockchain

import (
    "github.com/AsimovNetwork/asimov/asiutil"
    "github.com/AsimovNetwork/asimov/blockchain/txo"
    "github.com/AsimovNetwork/asimov/protos"
    "github.com/AsimovNetwork/asimov/txscript"
    "runtime"
    "testing"
)

// TestCheckBlockScripts ensures that validating the all of the scripts in a
// known-good block doesn't return an error.
func TestCheckBlockScripts(t *testing.T) {
    runtime.GOMAXPROCS(runtime.NumCPU())

    parivateKeyList := []string{
        "0xd0f0461b7b4d26cf370e6c73b58ef7fa26e8e30853a8cee901ed42cf0879cb6e", //privateKey0
    }
    roundSize := uint16(3)
    accList, netParam, chain, teardownFunc, err := createFakeChainByPrivateKeys(parivateKeyList, roundSize)
    if err != nil {
        t.Errorf("createFakeChainByPrivateKeys error %v", err)
    }
    defer teardownFunc()

    //genesisNode, _ := chain.GetNodeByHeight(0)
    validators, filters, _ := chain.GetValidators(1)
    blocks := make([]*asiutil.Block, 0)
    epoch := uint32(1)
    slot := uint16(0)
    //create 2 block:
    for i := int32(0); i < 2; i++ {
        if i != 0 {
            if i == int32(roundSize) {
                slot = 0
                epoch++
                validators, filters, _ = chain.GetValidators(epoch)
            } else {
                slot++
            }
        }
        normalTxList := make([]*asiutil.Tx, 0)
        if i == 1 {
            //create a tx using validators[0]:
            index := 0
            node1, _ := chain.GetNodeByHeight(1)
            coinbaseAddr := node1.coinbase
            for k := 0; k < len(netParam.GenesisCandidates); k++ {
                if netParam.GenesisCandidates[k] == coinbaseAddr {
                    index = k
                    break
                }
            }
            normalTx, _ := chain.createNormalTx(parivateKeyList[index], protos.Asset{0, 0}, *validators[0],
                2000000000, 5000000, 100000, nil)
            normalTxList = append(normalTxList, normalTx)
        }

        block, _, err := createAndSignBlock(netParam, accList, validators, filters, chain, epoch, slot, int32(i),
            protos.Asset{0, 0}, 0, validators[slot], normalTxList,
            0, chain.bestChain.tip())
        if err != nil {
            t.Errorf("create block error %v", err)
        }

        if i < 1 {
            isMain, isOrphan, checkErr := chain.ProcessBlock(block, nil, 0)
            if checkErr != nil {
                t.Errorf("ProcessBlock error %v", checkErr)
            }
            log.Infof("isMain = %v, isOrphan = %v", isMain, isOrphan)
        }
        blocks = append(blocks, block)
    }

    //test block with tx:
    block := blocks[1]
    view := txo.NewUtxoViewpoint()
    view.SetBestHash(&chain.bestChain.tip().hash)
    err = fetchInputUtxos(view, chain.db, block)
    if err != nil {
        t.Errorf("test TestCheckBlockScripts error %v", err)
    }

    scriptFlags := txscript.ScriptBip16
    err = checkBlockScripts(block, view, scriptFlags)
    if err != nil {
      t.Errorf("Transaction script validation failed: %v\n", err)
      return
    }
}
