// Copyright (c) 2018-2020. The asimov developers
// Copyright (c) 2013-2017 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.
package blockchain

import (
	"github.com/AsimovNetwork/asimov/asiutil"
	"github.com/AsimovNetwork/asimov/common"
	"github.com/AsimovNetwork/asimov/protos"
	"testing"
)

// TestNotifications ensures that notification callbacks are fired on events.
func TestNotifications(t *testing.T) {
	parivateKeyList := []string{
		"0xd0f0461b7b4d26cf370e6c73b58ef7fa26e8e30853a8cee901ed42cf0879cb6e",  //privateKey0
		"0xd07f68f78fc58e3dc8ea72ff69784aa9542c452a4ee66b2665fa3cccb48441c2",  //privateKey1
		"0x77366e621236e71a77236e0858cd652e92c3af0908527b5bd1542992c4d7cace",  //privateKey2
	}
	accList, netParam, chain, teardownFunc, err := createFakeChainByPrivateKeys(parivateKeyList, 10)
	defer teardownFunc()

	validators, filters, _ := chain.GetValidatorsByNode(1,chain.bestChain.tip())

	var block *asiutil.Block
	//create block:
	block, _, err = createAndSignBlock(netParam, accList, validators, filters, chain, uint32(1), uint16(0),
		chain.bestChain.Tip().height, protos.Asset{0,0}, 0,
		validators[0],nil,0,chain.bestChain.tip())
	if err != nil {
		t.Errorf("create block error %v", err)
	}

	notificationCount := 0
	callback := func(notification *Notification) {
		if notification.Type == NTBlockAccepted {
			notificationCount++
		}
	}

	// Register callback multiple times then assert it is called that many
	// times.
	const numSubscribers = 3
	for i := 0; i < numSubscribers; i++ {
		chain.Subscribe(callback)
	}

	_, _, err = chain.ProcessBlock(block, nil, nil, nil, common.BFNone)
	if err != nil {
		t.Fatalf("ProcessBlock fail on block: %v\n", err)
	}

	if notificationCount != numSubscribers {
		t.Fatalf("Expected notification callback to be executed %d times, found %d",
			numSubscribers, notificationCount)
	}
}
