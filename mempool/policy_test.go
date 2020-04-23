// Copyright (c) 2018-2020 The asimov developers
// Copyright (c) 2013-2017 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package mempool

import (
	"bytes"
	"github.com/AsimovNetwork/asimov/common"
	"github.com/AsimovNetwork/asimov/protos"
	"github.com/AsimovNetwork/asimov/txscript"
	"github.com/AsimovNetwork/asimov/asiutil"
	"testing"
	"time"
)

// TestCheckTransactionStandard tests the checkTransactionStandard API.
func TestCheckTransactionStandard(t *testing.T) {
	// Create some dummy, but otherwise standard, data for transactions.
	prevOutHash, err :=  common.NewHashFromStr("01")
	if err != nil {
		t.Fatalf("NewShaHashFromStr: unexpected error: %v", err)
	}
	dummyPrevOut := protos.OutPoint{Hash: *prevOutHash, Index: 1}
	dummySigScript := bytes.Repeat([]byte{0x00}, 65)
	dummyTxIn := protos.TxIn{
		PreviousOutPoint: dummyPrevOut,
		SignatureScript:  dummySigScript,
		Sequence:         protos.MaxTxInSequenceNum,
	}
	addrHash := [20]byte{0x01}
	addr, err := common.NewAddressWithId(common.PubKeyHashAddrID,addrHash[:])
	if err != nil {
		t.Fatalf("NewAddressPubKeyHash: unexpected error: %v", err)
	}
	dummyPkScript, err := txscript.PayToAddrScript(addr)
	if err != nil {
		t.Fatalf("PayToAddrScript: unexpected error: %v", err)
	}
	dummyTxOut := protos.TxOut{
		Value:    100000000,
		PkScript: dummyPkScript,
	}

	tests := []struct {
		name       string
		tx         protos.MsgTx
		height     int32
		isStandard bool
		code       protos.RejectCode
	}{
		{
			name: "Typical pay-to-pubkey-hash transaction",
			tx: protos.MsgTx{
				Version:  1,
				TxIn:     []*protos.TxIn{&dummyTxIn},
				TxOut:    []*protos.TxOut{&dummyTxOut},
				LockTime: 0,
			},
			height:     300000,
			isStandard: true,
		},
		{
			name: "Transaction version too high",
			tx: protos.MsgTx{
				Version:  protos.TxVersion + 1,
				TxIn:     []*protos.TxIn{&dummyTxIn},
				TxOut:    []*protos.TxOut{&dummyTxOut},
				LockTime: 0,
			},
			height:     300000,
			isStandard: false,
			code:       protos.RejectNonstandard,
		},
		{
			name: "Transaction is not finalized",
			tx: protos.MsgTx{
				Version: 1,
				TxIn: []*protos.TxIn{{
					PreviousOutPoint: dummyPrevOut,
					SignatureScript:  dummySigScript,
					Sequence:         0,
				}},
				TxOut:    []*protos.TxOut{&dummyTxOut},
				LockTime: 300001,
			},
			height:     300000,
			isStandard: false,
			code:       protos.RejectNonstandard,
		},
		{
			name: "Transaction size is too large",
			tx: protos.MsgTx{
				Version: 1,
				TxIn:    []*protos.TxIn{&dummyTxIn},
				TxOut: []*protos.TxOut{{
					Value: 0,
					PkScript: bytes.Repeat([]byte{0x00},
						(maxStandardTxSize/4)+1),
				}},
				LockTime: 0,
			},
			height:     300000,
			isStandard: false,
			code:       protos.RejectNonstandard,
		},
		{
			name: "Signature script size is too large",
			tx: protos.MsgTx{
				Version: 1,
				TxIn: []*protos.TxIn{{
					PreviousOutPoint: dummyPrevOut,
					SignatureScript: bytes.Repeat([]byte{0x00},
						maxStandardSigScriptSize+1),
					Sequence: protos.MaxTxInSequenceNum,
				}},
				TxOut:    []*protos.TxOut{&dummyTxOut},
				LockTime: 0,
			},
			height:     300000,
			isStandard: false,
			code:       protos.RejectNonstandard,
		},
		{
			name: "Signature script that does more than push data",
			tx: protos.MsgTx{
				Version: 1,
				TxIn: []*protos.TxIn{{
					PreviousOutPoint: dummyPrevOut,
					SignatureScript: []byte{
						txscript.OP_CHECKSIGVERIFY},
					Sequence: protos.MaxTxInSequenceNum,
				}},
				TxOut:    []*protos.TxOut{&dummyTxOut},
				LockTime: 0,
			},
			height:     300000,
			isStandard: false,
			code:       protos.RejectNonstandard,
		},
		{
			name: "Valid but non standard public key script",
			tx: protos.MsgTx{
				Version: 1,
				TxIn:    []*protos.TxIn{&dummyTxIn},
				TxOut: []*protos.TxOut{{
					Value:    100000000,
					PkScript: []byte{txscript.OP_TRUE},
				}},
				LockTime: 0,
			},
			height:     300000,
			isStandard: false,
			code:       protos.RejectNonstandard,
		},
	}

	pastMedianTime := time.Now().Unix()
	for _, test := range tests {
		// Ensure standardness is as expected.
		err := checkTransactionStandard(asiutil.NewTx(&test.tx),
			test.height, pastMedianTime, 1)
		if err == nil && test.isStandard {
			// Test passes since function returned standard for a
			// transaction which is intended to be standard.
			continue
		}
		if err == nil && !test.isStandard {
			t.Errorf("checkTransactionStandard (%s): standard when "+
				"it should not be", test.name)
			continue
		}
		if err != nil && test.isStandard {
			t.Errorf("checkTransactionStandard (%s): nonstandard "+
				"when it should not be: %v", test.name, err)
			continue
		}

		// Ensure error type is a TxRuleError inside of a RuleError.
		rerr, ok := err.(RuleError)
		if !ok {
			t.Errorf("checkTransactionStandard (%s): unexpected "+
				"error type - got %T", test.name, err)
			continue
		}
		txrerr, ok := rerr.Err.(TxRuleError)
		if !ok {
			t.Errorf("checkTransactionStandard (%s): unexpected "+
				"error type - got %T", test.name, rerr.Err)
			continue
		}

		// Ensure the reject code is the expected one.
		if txrerr.RejectCode != test.code {
			t.Errorf("checkTransactionStandard (%s): unexpected "+
				"error code - got %v, want %v", test.name,
				txrerr.RejectCode, test.code)
			continue
		}
	}
}
