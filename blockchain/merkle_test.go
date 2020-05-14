// Copyright (c) 2018-2020. The asimov developers
// Copyright (c) 2013-2017 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.
package blockchain

import (
	"fmt"
	"github.com/AsimovNetwork/asimov/protos"
	"testing"
	"github.com/AsimovNetwork/asimov/chaincfg"
)

// TestMerkle tests the BuildMerkleTreeStore API.
func TestMerkle(t *testing.T) {
	parivateKeyList := []string{
		"0xd0f0461b7b4d26cf370e6c73b58ef7fa26e8e30853a8cee901ed42cf0879cb6e", //privateKey0
	}
	accList, _, chain, teardownFunc, err := createFakeChainByPrivateKeys(parivateKeyList, chaincfg.ActiveNetParams.RoundSize)
	defer teardownFunc()

	//create block:
	block, err := createTestBlock(chain, uint32(1), 0, 0, protos.Asset{0,0},
	0, accList[0].Address, nil, chain.bestChain.Tip())
	if err != nil {
		t.Errorf("createTestBlock error %v", err)
	}

	wantMerkle := block.Transactions()[0].Hash().String()
	merkles := BuildMerkleTreeStore(block.Transactions())
	calculatedMerkleRoot := merkles[len(merkles)-1]
	str := fmt.Sprintf("%v", calculatedMerkleRoot)
	if wantMerkle != str {
		t.Errorf("BuildMerkleTreeStore: merkle root mismatch - got %v, want %v",
			calculatedMerkleRoot, wantMerkle)
	}
}
