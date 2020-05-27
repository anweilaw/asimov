// Copyright (c) 2018-2020. The asimov developers
// Copyright (c) 2013-2017 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.
package blockchain

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/AsimovNetwork/asimov/blockchain/txo"
	"github.com/AsimovNetwork/asimov/common"
	"github.com/AsimovNetwork/asimov/common/hexutil"
	"reflect"
	"testing"

	"github.com/AsimovNetwork/asimov/database"
	"github.com/AsimovNetwork/asimov/protos"
)

func newHashFromStr(hash string) *common.Hash {
	ret, err := common.NewHashFromStr(hash)
	if err != nil {
		fmt.Println(err.Error())
		return nil
	}

	return ret
}

// TestErrNotInMainChain ensures the functions related to errNotInMainChain work
// as expected.
func TestErrNotInMainChain(t *testing.T) {
	errStr := "no block at height 1 exists"
	err := error(errNotInMainChain(errStr))

	// Ensure the stringized output for the error is as expected.
	if err.Error() != errStr {
		t.Fatalf("errNotInMainChain retuned unexpected error string - got %q, want %q", err.Error(), errStr)
	}

	// Ensure error is detected as the correct type.
	if !isNotInMainChainErr(err) {
		t.Fatalf("isNotInMainChainErr did not detect as expected type")
	}
	err = errors.New("something else")
	if isNotInMainChainErr(err) {
		t.Fatalf("isNotInMainChainErr detected incorrect type")
	}
}


var coinbaseStxo = txo.SpentTxOut{
	Amount:     500000000,
	PkScript:   []byte{118,169,21,102,227,5,75,65,16,81,218,84,146,174,199,168,35,176,12,179,173,215,114,215,197,172},
	Height:     1,
	IsCoinBase: true,
	Asset:      &protos.Asset{0,0},
}

var coinbaseStxo2 = txo.SpentTxOut{
	Amount:     500000000,
	PkScript:   []byte{118,169,21,102,227,5,75,65,16,81,218,84,146,174,199,168,35,176,12,179,173,215,114,215,197,172},
	Height:     2,
	IsCoinBase: true,
	Asset:      &protos.Asset{0,0},
}

var coinbaseStxo10001 = txo.SpentTxOut{
	Amount:     500000000,
	PkScript:   []byte{118,169,21,102,227,5,75,65,16,81,218,84,146,174,199,168,35,176,12,179,173,215,114,215,197,172},
	Height:     10001,
	IsCoinBase: false,
	Asset:      &protos.Asset{0,1},
}

var normalStxo = txo.SpentTxOut{
	Amount:     100000000,
	PkScript:   []byte{118,169,21,102,227,5,75,65,16,81,218,84,146,174,199,168,35,176,12,179,173,215,114,215,197,172},
	Height:     1,
	IsCoinBase: false,
	Asset:      &protos.Asset{0,0},
}

var normalStxo2 = txo.SpentTxOut{
	Amount:     100000000,
	PkScript:   []byte{118,169,21,102,227,5,75,65,16,81,218,84,146,174,199,168,35,176,12,179,173,215,114,215,197,172},
	Height:     2,
	IsCoinBase: false,
	Asset:      &protos.Asset{0,0},
}

var normalStxo10001 = txo.SpentTxOut{
	Amount:     100000000,
	PkScript:   []byte{118,169,21,102,227,5,75,65,16,81,218,84,146,174,199,168,35,176,12,179,173,215,114,215,197,172},
	Height:     10001,
	IsCoinBase: false,
	Asset:      &protos.Asset{0,0},
}

var normalStxoAmount123 = txo.SpentTxOut{
	Amount:     123,
	PkScript:   []byte{118,169,21,102,227,5,75,65,16,81,218,84,146,174,199,168,35,176,12,179,173,215,114,215,197,172},
	Height:     2,
	IsCoinBase: false,
	Asset:      &protos.Asset{0,0},
}

var normalStxoAmountMax = txo.SpentTxOut{
	Amount:     210000000000,
	PkScript:   []byte{118,169,21,102,227,5,75,65,16,81,218,84,146,174,199,168,35,176,12,179,173,215,114,215,197,172},
	Height:     2,
	IsCoinBase: false,
	Asset:      &protos.Asset{0,0},
}

//txo.SpentTxOut PkScript is ScriptHash:
var normalScriptHashStxo = txo.SpentTxOut{
	Amount:     500000000,
	PkScript:   []byte{169,21,115,227,5,75,65,16,81,218,84,146,174,199,168,35,176,12,179,173,215,114,215,196},
	Height:     1,
	IsCoinBase: true,
	Asset:      &protos.Asset{0,0},
}

//txo.SpentTxOut PkScript is isContract:
var contractStxo = txo.SpentTxOut{
	Amount:     500000000,
	PkScript:   []byte{194,21,99,227,5,75,65,16,81,218,84,146,174,199,168,35,176,12,179,173,215,114,215},
	Height:     1,
	IsCoinBase: true,
	Asset:      &protos.Asset{0,0},
}

//txo.SpentTxOut PkScript is vote:
var voteStxo = txo.SpentTxOut{
	Amount:     500000000,
	PkScript:   []byte{198,21,99,227,5,75,65,16,81,218,84,146,174,199,168,35,176,12,179,173,215,114,215},
	Height:     1,
	IsCoinBase: true,
	Asset:      &protos.Asset{0,0},
}

// TestStxoSerialization ensures serializing and deserializing spent transaction
// output entries works as expected.
func TestStxoSerialization(t *testing.T) {

	tests := []struct {
		name       string
		stxo       txo.SpentTxOut
		serialized string
	}{
		{
			name: "Spends last output of coinbase, height 1",
			stxo: coinbaseStxo,
			serialized: "0x030065cd1d0000000000e3054b411051da5492aec7a823b00cb3add772d70c000000000000000000000000",
		},
		{
			name: "Spends last output of coinbase, height 2",
			stxo: coinbaseStxo2,
			serialized: "0x050065cd1d0000000000e3054b411051da5492aec7a823b00cb3add772d70c000000000000000000000000",
		},
		{
			name: "Spends last output of coinbase, height 10001",
			stxo: coinbaseStxo10001,
			serialized: "0x809b220065cd1d0000000000e3054b411051da5492aec7a823b00cb3add772d70c000000000000000000000001",
		},
		{
			name: "Spends last output of non coinbase,height 1",
			stxo: normalStxo,
			serialized: "0x0200e1f5050000000000e3054b411051da5492aec7a823b00cb3add772d70c000000000000000000000000",
		},
		{
			name: "Spends last output of non coinbase,height 2",
			stxo: normalStxo2,
			serialized: "0x0400e1f5050000000000e3054b411051da5492aec7a823b00cb3add772d70c000000000000000000000000",
		},
		{
			name: "Spends last output of non coinbase, height 10001",
			stxo: normalStxo10001,
			serialized: "0x809b2200e1f5050000000000e3054b411051da5492aec7a823b00cb3add772d70c000000000000000000000000",
		},
		{
			name: "Spends last output of non coinbase, amount 123",
			stxo: normalStxoAmount123,
			serialized: "0x047b0000000000000000e3054b411051da5492aec7a823b00cb3add772d70c000000000000000000000000",
		},
		{
			name: "Spends last output of non coinbase, amount 210000000000",
			stxo: normalStxoAmountMax,
			serialized: "0x0400b4f9e43000000000e3054b411051da5492aec7a823b00cb3add772d70c000000000000000000000000",
		},
		{
			name: "txo.SpentTxOut PkScript is ScriptHash",
			stxo: normalScriptHashStxo,
			serialized: "0x030065cd1d0000000001e3054b411051da5492aec7a823b00cb3add772d70c000000000000000000000000",
		},
		{
			name: "txo.SpentTxOut PkScript is contract",
			stxo: contractStxo,
			serialized: "0x030065cd1d0000000006e3054b411051da5492aec7a823b00cb3add772d70c000000000000000000000000",
		},
		{
			name: "txo.SpentTxOut PkScript is vote",
			stxo: voteStxo,
			serialized: "0x030065cd1d0000000007e3054b411051da5492aec7a823b00cb3add772d70c000000000000000000000000",
		},
	}

	for i, test := range tests {
		// Ensure the function to calculate the serialized size without
		// actually serializing it is calculated properly.
		gotSize := SpentTxOutSerializeSize(&test.stxo)
		// Ensure the stxo serializes to the expected value.
		gotSerialized := make([]byte, gotSize)
		gotBytesWritten := putSpentTxOut(gotSerialized, &test.stxo)
		gotBytexHex := hexutil.Encode(gotSerialized)
		if gotBytexHex != test.serialized {
			t.Errorf("case %d, puttxo.SpentTxOut (%s): did not get expected bytes - got %x, want %x",
				i, test.name, gotSerialized, test.serialized)
			continue
		}
		if gotBytesWritten * 2 + 2 != len(test.serialized) {
			t.Errorf("puttxo.SpentTxOut (%s): did not get expected number of bytes written - got %d, want %d",
				test.name, gotBytesWritten, len(test.serialized))
			continue
		}

		// Ensure the serialized bytes are decoded back to the expected
		// stxo.
		var gotStxo txo.SpentTxOut
		gotBytesRead, err := decodeSpentTxOut(gotSerialized, &gotStxo)
		if err != nil {
			t.Errorf("decodeSpentTxOut (%s): unexpected error: %v", test.name, err)
			continue
		}
		if !reflect.DeepEqual(gotStxo, test.stxo) {
			t.Errorf("decodeSpentTxOut (%s) mismatched entries - got %v, want %v",
				test.name, gotStxo, test.stxo)
			continue
		}
		if gotBytesRead * 2 + 2 != len(test.serialized) {
			t.Errorf("decodeSpentTxOut (%s): did not get expected number of bytes read - got %d, want %d",
				test.name, gotBytesRead, len(test.serialized))
			continue
		}
	}
}

// TestStxoDecodeErrors performs negative tests against decoding spent
// transaction outputs to ensure error paths work as expected.
func TestStxoDecodeErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		stxo       txo.SpentTxOut
		serialized []byte
		bytesRead  int // Expected number of bytes read.
		errType    error
	}{
		{
			name:       "nothing serialized",
			stxo:       txo.SpentTxOut{},
			serialized: hexToBytes(""),
			errType:    common.DeserializeError(""),
			bytesRead:  0,
		},
		{
			name:       "no data after header code w/o reserved",
			stxo:       txo.SpentTxOut{},
			serialized: hexToBytes("00"),
			errType:    common.DeserializeError(""),
			bytesRead:  1,
		},
		{
			name:       "no data after header code with reserved",
			stxo:       txo.SpentTxOut{},
			serialized: hexToBytes("13"),
			errType:    common.DeserializeError(""),
			bytesRead:  1,
		},
		{
			name:       "no data after reserved",
			stxo:       txo.SpentTxOut{},
			serialized: hexToBytes("1300"),
			errType:    common.DeserializeError(""),
			bytesRead:  1,
		},
		{
			name:       "incomplete compressed txout",
			stxo:       txo.SpentTxOut{},
			serialized: hexToBytes("1332"),
			errType:    common.DeserializeError(""),
			bytesRead:  1,
		},
	}

	for i, test := range tests {
		// Ensure the expected error type is returned.
		gotBytesRead, err := decodeSpentTxOut(test.serialized, &test.stxo)
		if reflect.TypeOf(err) != reflect.TypeOf(test.errType) {
			t.Errorf("case %d, decodeSpentTxOut (%s): expected error type does not match - got %T, want %T",
				i, test.name, err, test.errType)
			continue
		}

		// Ensure the expected number of bytes read is returned.
		if gotBytesRead != test.bytesRead {
			t.Errorf("case %d, decodeSpentTxOut (%s): unexpected number of bytes read - got %d, want %d",
				i, test.name, gotBytesRead, test.bytesRead)
			continue
		}
	}
}

// TestSpendJournalSerialization ensures serializing and deserializing spend
// journal entries works as expected.
func TestSpendJournalSerialization(t *testing.T) {
	tests := []struct {
		name       string
		entry      []txo.SpentTxOut
		blockTxns  []*protos.MsgTx
		serialized string
	}{
		//test0: input nil
		{
			name:       "No spends",
			entry:      nil,
			blockTxns:  nil,
			serialized: "0x",
		},

		//test1: input block height = 2
		{
			name: "One tx with one input spends last output of coinbase",
			entry: []txo.SpentTxOut{{
				Amount:     500000000,
				PkScript:   []byte{118,169,21,102,227,5,75,65,16,81,218,84,146,174,199,168,35,176,12,179,173,215,114,215,197,172},
				Height:     1,
				IsCoinBase: true,
				Asset:      &protos.Asset{0,0},
			}},
			blockTxns: []*protos.MsgTx{{ // Coinbase omitted.
				Version: 1,
				TxIn: []*protos.TxIn{{
					PreviousOutPoint: protos.OutPoint{
						Hash:  *newHashFromStr("c13bd0cf1f1209c07da2f785c2af52f10b6dacade271e375b0096967dbe33a8b"),
						Index: 0,
					},
					SignatureScript: hexToBytes("4830450221008d3ffcad657a91d7f63e3ee9d11b4658ca7abadd6cb80bfd7625bbaee3e8142a0220795468e7e601083f6de198798fc8411486d30a3e32c6cf23144048cf40cbf3e4012103d33a68fbb0070e518cd98ce391bbdc781d07360ebc6d0743424172a7542210c9"),
					Sequence:        0xffffffff,
				}},
				TxOut: []*protos.TxOut{{
					Value:    100000000,
					PkScript: []byte{118,169,21,102,227,5,75,65,16,81,218,84,146,174,199,168,35,176,12,179,173,215,114,215,197,172},
					Asset:    protos.Asset{0,0},
				}, {
					Value:    400000000,
					PkScript: []byte{118,169,21,102,227,5,75,65,16,81,218,84,146,174,199,168,35,176,12,179,173,215,114,215,197,172},
					Asset:    protos.Asset{0,0},
				}},
				LockTime: 0,
			}},
			serialized: "0x030065cd1d0000000000e3054b411051da5492aec7a823b00cb3add772d70c000000000000000000000000",
		},

		//test2: input block height = 2
		{
			name: "Two txns when one spends last output, one doesn't",
			entry: []txo.SpentTxOut{{
				Amount:     100000000,
				PkScript:   []byte{118,169,21,102,227,5,75,65,16,81,218,84,146,174,199,168,35,176,12,179,173,215,114,215,197,172},
				Height:     2,
				IsCoinBase: false,
				Asset:      &protos.Asset{0,0},
			}, {
				Amount:     400000000,
				PkScript:   []byte{118,169,21,102,227,5,75,65,16,81,218,84,146,174,199,168,35,176,12,179,173,215,114,215,197,172},
				Height:     2,
				IsCoinBase: false,
				Asset:      &protos.Asset{0,0},
			}, {
				Amount:     500000000,
				PkScript:   []byte{118,169,21,102,227,5,75,65,16,81,218,84,146,174,199,168,35,176,12,179,173,215,114,215,197,172},
				Height:     2,
				IsCoinBase: true,
				Asset:      &protos.Asset{0,0},
			}},
			blockTxns: []*protos.MsgTx{{ // Coinbase omitted.
				Version: 1,
				TxIn: []*protos.TxIn{{
					PreviousOutPoint: protos.OutPoint{
						Hash:  *newHashFromStr("d74e2f04e8a64df95d14f3ad627717bea897e47125d8708635da42d35355d1de"),
						Index: 0,
					},
					SignatureScript: hexToBytes("483045022100b73c1c552dd4cf2630352e0870e62eb678ea69634e244b7770a71be23dafb4b90220596b5446a16f1f42cec95e53396db9554488334836a35c5671948519863d97f4012103d33a68fbb0070e518cd98ce391bbdc781d07360ebc6d0743424172a7542210c9"),
					Sequence:        0xffffffff,
				}},
				TxOut: []*protos.TxOut{{
					Value:    100000000,
					PkScript: []byte{118,169,21,102,227,5,75,65,16,81,218,84,146,174,199,168,35,176,12,179,173,215,114,215,197,172},
				}, {
					Value:    0,
					PkScript: []byte{118,169,21,102,227,5,75,65,16,81,218,84,146,174,199,168,35,176,12,179,173,215,114,215,197,172},
				}},
				LockTime: 0,
			}, {
				Version: 1,
				TxIn: []*protos.TxIn{{
					PreviousOutPoint: protos.OutPoint{
						Hash:  *newHashFromStr("d74e2f04e8a64df95d14f3ad627717bea897e47125d8708635da42d35355d1de"),
						Index: 1,
					},
					SignatureScript: hexToBytes("4730440220473f28f75754cd9c772676d99213a77dfe01bbb531552ae8f2372be9d4cf4393022063f935c022cf70d4694f215d5b2da0cc6093bb83accae48ba615e606b6178d1e012103d33a68fbb0070e518cd98ce391bbdc781d07360ebc6d0743424172a7542210c9"),
					Sequence:        0xffffffff,
				}},
				TxOut: []*protos.TxOut{{
					Value:    100000000,
					PkScript: []byte{118,169,21,102,227,5,75,65,16,81,218,84,146,174,199,168,35,176,12,179,173,215,114,215,197,172},
				}, {
					Value:    300000000,
					PkScript: []byte{118,169,21,102,227,5,75,65,16,81,218,84,146,174,199,168,35,176,12,179,173,215,114,215,197,172},
				}},
				LockTime: 0,
			}, {
				Version: 1,
				TxIn: []*protos.TxIn{{
					PreviousOutPoint: protos.OutPoint{
						Hash:  *newHashFromStr("1cf1c6f5f9b0e75226bdee82fd248d63a26326a65608aa049269abdf45135e38"),
						Index: 0,
					},
					SignatureScript: hexToBytes("483045022100ca5f82ec38771cf34fd6b01f0bf7d8b9d6a6a5219fd9fb8f81d22bafe36ab5bb02203df55c61c2959aad21336f971d2ae69cd28f9a20eeb2468005ff36ca0b62af7e012103d33a68fbb0070e518cd98ce391bbdc781d07360ebc6d0743424172a7542210c9"),
					Sequence:        0xffffffff,
				}},
				TxOut: []*protos.TxOut{{
					Value:    100000000,
					PkScript: []byte{118,169,21,102,227,5,75,65,16,81,218,84,146,174,199,168,35,176,12,179,173,215,114,215,197,172},
				}, {
					Value:    400000000,
					PkScript: []byte{118,169,21,102,227,5,75,65,16,81,218,84,146,174,199,168,35,176,12,179,173,215,114,215,197,172},
				}},
				LockTime: 0,
			}},
			serialized: "0x050065cd1d0000000000e3054b411051da5492aec7a823b00cb3add772d70c000000000000000000000000040084d7170000000000e3054b411051da5492aec7a823b00cb3add772d70c0000000000000000000000000400e1f5050000000000e3054b411051da5492aec7a823b00cb3add772d70c000000000000000000000000",
		},

		//test3: txo.SpentTxOut PkScript is ScriptHash:
		{
			name: "One tx with one input spends last output of coinbase",
			entry: []txo.SpentTxOut{normalScriptHashStxo},
			blockTxns: []*protos.MsgTx{{ // Coinbase omitted.
				Version: 1,
				TxIn: []*protos.TxIn{{
					PreviousOutPoint: protos.OutPoint{
						Hash:  *newHashFromStr("c13bd0cf1f1209c07da2f785c2af52f10b6dacade271e375b0096967dbe33a8b"),
						Index: 0,
					},
					SignatureScript: hexToBytes("4830450221008d3ffcad657a91d7f63e3ee9d11b4658ca7abadd6cb80bfd7625bbaee3e8142a0220795468e7e601083f6de198798fc8411486d30a3e32c6cf23144048cf40cbf3e4012103d33a68fbb0070e518cd98ce391bbdc781d07360ebc6d0743424172a7542210c9"),
					Sequence:        0xffffffff,
				}},
				TxOut: []*protos.TxOut{{
					Value:    100000000,
					PkScript: []byte{118,169,21,102,227,5,75,65,16,81,218,84,146,174,199,168,35,176,12,179,173,215,114,215,197,172},
					Asset:    protos.Asset{0,0},
				}, {
					Value:    400000000,
					PkScript: []byte{118,169,21,102,227,5,75,65,16,81,218,84,146,174,199,168,35,176,12,179,173,215,114,215,197,172},
					Asset:    protos.Asset{0,0},
				}},
				LockTime: 0,
			}},
			serialized: "0x030065cd1d0000000001e3054b411051da5492aec7a823b00cb3add772d70c000000000000000000000000",
		},

		//test4: txo.SpentTxOut PkScript is isContract:
		{
			name: "One tx with one input spends last output of coinbase",
			entry: []txo.SpentTxOut{contractStxo},
			blockTxns: []*protos.MsgTx{{ // Coinbase omitted.
				Version: 1,
				TxIn: []*protos.TxIn{{
					PreviousOutPoint: protos.OutPoint{
						Hash:  *newHashFromStr("c13bd0cf1f1209c07da2f785c2af52f10b6dacade271e375b0096967dbe33a8b"),
						Index: 0,
					},
					SignatureScript: hexToBytes("4830450221008d3ffcad657a91d7f63e3ee9d11b4658ca7abadd6cb80bfd7625bbaee3e8142a0220795468e7e601083f6de198798fc8411486d30a3e32c6cf23144048cf40cbf3e4012103d33a68fbb0070e518cd98ce391bbdc781d07360ebc6d0743424172a7542210c9"),
					Sequence:        0xffffffff,
				}},
				TxOut: []*protos.TxOut{{
					Value:    100000000,
					PkScript: []byte{118,169,21,102,227,5,75,65,16,81,218,84,146,174,199,168,35,176,12,179,173,215,114,215,197,172},
					Asset:    protos.Asset{0,0},
				}, {
					Value:    400000000,
					PkScript: []byte{118,169,21,102,227,5,75,65,16,81,218,84,146,174,199,168,35,176,12,179,173,215,114,215,197,172},
					Asset:    protos.Asset{0,0},
				}},
				LockTime: 0,
			}},
			serialized: "0x030065cd1d0000000006e3054b411051da5492aec7a823b00cb3add772d70c000000000000000000000000",
		},

		//test5: txo.SpentTxOut PkScript is isVote:
		{
			name: "One tx with one input spends last output of coinbase",
			entry: []txo.SpentTxOut{voteStxo},
			blockTxns: []*protos.MsgTx{{ // Coinbase omitted.
				Version: 1,
				TxIn: []*protos.TxIn{{
					PreviousOutPoint: protos.OutPoint{
						Hash:  *newHashFromStr("c13bd0cf1f1209c07da2f785c2af52f10b6dacade271e375b0096967dbe33a8b"),
						Index: 0,
					},
					SignatureScript: hexToBytes("4830450221008d3ffcad657a91d7f63e3ee9d11b4658ca7abadd6cb80bfd7625bbaee3e8142a0220795468e7e601083f6de198798fc8411486d30a3e32c6cf23144048cf40cbf3e4012103d33a68fbb0070e518cd98ce391bbdc781d07360ebc6d0743424172a7542210c9"),
					Sequence:        0xffffffff,
				}},
				TxOut: []*protos.TxOut{{
					Value:    100000000,
					PkScript: []byte{118,169,21,102,227,5,75,65,16,81,218,84,146,174,199,168,35,176,12,179,173,215,114,215,197,172},
					Asset:    protos.Asset{0,0},
				}, {
					Value:    400000000,
					PkScript: []byte{118,169,21,102,227,5,75,65,16,81,218,84,146,174,199,168,35,176,12,179,173,215,114,215,197,172},
					Asset:    protos.Asset{0,0},
				}},
				LockTime: 0,
			}},
			serialized: "0x030065cd1d0000000007e3054b411051da5492aec7a823b00cb3add772d70c000000000000000000000000",
		},
	}

	for i, test := range tests {
		// Ensure the journal entry serializes to the expected value.
		gotBytes := serializeSpendJournalEntry(test.entry)
		gotBytesHex := hexutil.Encode(gotBytes)
		if gotBytesHex != test.serialized {
			t.Errorf("serializeSpendJournalEntry #%d (%s): mismatched bytes - got %x, want %x",
				i, test.name, gotBytes, test.serialized)
			continue
		}

		// Deserialize to a spend journal entry.
		gotEntry, err := deserializeSpendJournalEntry(gotBytes, test.blockTxns,nil)
		if err != nil {
			t.Errorf("deserializeSpendJournalEntry #%d (%s) unexpected error: %v", i, test.name, err)
			continue
		}

		// Ensure that the deserialized spend journal entry has the
		// correct properties.
		if !reflect.DeepEqual(gotEntry, test.entry) {
			t.Errorf("deserializeSpendJournalEntry #%d (%s) mismatched entries - got %v, want %v",
				i, test.name, gotEntry, test.entry)
			continue
		}
	}
}

// TestSpendJournalErrors performs negative tests against deserializing spend
// journal entries to ensure error paths work as expected.
func TestSpendJournalErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		blockTxns  []*protos.MsgTx
		serialized []byte
		errType    error
	}{
		// Adapted from block 170 in main blockchain.
		{
			name: "Force assertion due to missing stxos",
			blockTxns: []*protos.MsgTx{{ // Coinbase omitted.
				Version: 1,
				TxIn: []*protos.TxIn{{
					PreviousOutPoint: protos.OutPoint{
						Hash:  *newHashFromStr("0437cd7f8525ceed2324359c2d0ba26006d92d856a9c20fa0241106ee5a597c9"),
						Index: 0,
					},
					SignatureScript: hexToBytes("47304402204e45e16932b8af514961a1d3a1a25fdf3f4f7732e9d624c6c61548ab5fb8cd410220181522ec8eca07de4860a4acdd12909d831cc56cbbac4622082221a8768d1d0901"),
					Sequence:        0xffffffff,
				}},
				LockTime: 0,
			}},
			serialized: hexToBytes(""),
			errType:    common.AssertError(""),
		},
		{
			name: "Force deserialization error in stxos",
			blockTxns: []*protos.MsgTx{{ // Coinbase omitted.
				Version: 1,
				TxIn: []*protos.TxIn{{
					PreviousOutPoint: protos.OutPoint{
						Hash:  *newHashFromStr("0437cd7f8525ceed2324359c2d0ba26006d92d856a9c20fa0241106ee5a597c9"),
						Index: 0,
					},
					SignatureScript: hexToBytes("47304402204e45e16932b8af514961a1d3a1a25fdf3f4f7732e9d624c6c61548ab5fb8cd410220181522ec8eca07de4860a4acdd12909d831cc56cbbac4622082221a8768d1d0901"),
					Sequence:        0xffffffff,
				}},
				LockTime: 0,
			}},
			serialized: hexToBytes("1301320511db93e1dcdb8a016b49840f8c53bc1eb68a382e97b1482ecad7b148a6909a"),
			errType:    common.DeserializeError(""),
		},
	}

	for i, test := range tests {
		// Ensure the expected error type is returned and the returned
		// slice is nil.
		stxos, err := deserializeSpendJournalEntry(test.serialized,
			test.blockTxns,nil)
		if reflect.TypeOf(err) != reflect.TypeOf(test.errType) {
			t.Errorf("the %d test error:deserializeSpendJournalEntry (%s): expected "+
				"error type does not match - got %T, want %T",
				i,test.name, err, test.errType)
			continue
		}
		if stxos != nil {
			t.Errorf("the %d test error:deserializeSpendJournalEntry (%s): returned "+
				"slice of spent transaction outputs is not nil", i, test.name)
			continue
		}
	}
}

// TestUtxoSerialization ensures serializing and deserializing unspent
// trasaction output entries works as expected.
func TestUtxoSerialization(t *testing.T) {
	t.Parallel()

	entry0 := txo.NewUtxoEntry(
		5000000000,
		[]byte{118,169,21,102,227,5,75,65,16,81,218,84,146,174,199,168,35,176,12,179,173,215,114,215,197,172},
		1,
		true,
		&protos.Asset{0,0},nil)
	entry1 := txo.NewUtxoEntry(
		5000000000,
		[]byte{118,169,21,102,227,5,75,65,16,81,218,84,146,174,199,168,35,176,12,179,173,215,114,215,197,172},
		1,
		true,
		&protos.Asset{0,0},nil)
	entry1.Spend()
	entry2 := txo.NewUtxoEntry(
		1000000,
		[]byte{118,169,21,102,227,5,75,65,16,81,218,84,146,174,199,168,35,176,12,179,173,215,114,215,197,172},
		100001,
		false,
		&protos.Asset{0,0},nil)
	entry3 := txo.NewUtxoEntry(
		1000000,
		[]byte{118,169,21,102,227,5,75,65,16,81,218,84,146,174,199,168,35,176,12,179,173,215,114,215,197,172},
		100001,
		false,
		&protos.Asset{0,0},nil)
	entry3.Spend()
	entry4 := txo.NewUtxoEntry(
		2100000000000000,
		[]byte{169,21,115,227,5,75,65,16,81,218,84,146,174,199,168,35,176,12,179,173,215,114,215,196},
		1,
		false,
		&protos.Asset{1,1},nil)
	entry5 := txo.NewUtxoEntry(
		1000000,
		[]byte{194,21,99,227,5,75,65,16,81,218,84,146,174,199,168,35,176,12,179,173,215,114,215},
		2,
		false,
		&protos.Asset{0,0},nil)
	entry6 := txo.NewUtxoEntry(
		1000000,[]byte{198,21,99,227,5,75,65,16,81,218,84,146,174,199,168,35,176,12,179,173,215,114,215},
		3,
		false,
		&protos.Asset{0,1},nil)
	entry7 := txo.NewUtxoEntry(
		100,
		[]byte{1,2,3,4,5,6},
		3,
		false,
		&protos.Asset{0,1},nil)
	entry8 := txo.NewUtxoEntry(
		100,
		nil,
		3,
		false,
		&protos.Asset{0,1},nil)
	entry9 := txo.NewUtxoEntry(
		2,
		[]byte{169,21,115,227,5,75,65,16,81,218,84,146,174,199,168,35,176,12,179,173,215,114,215,196},
		688,
		false,
		&protos.Asset{0,4294967304},nil)
	tests := []struct {
		name       string
		entry      *txo.UtxoEntry
		serialized string
	}{
		//test0:
		{
			name: "height 1, coinbase",
			entry: entry0,
			serialized: "0x0300f2052a0100000000e3054b411051da5492aec7a823b00cb3add772d70c000000000000000000000000",
		},
		//test1:
		{
			name: "height 1, coinbase, spent",
			entry: entry1,
			serialized: "0x",
		},
		//test2:
		{
			name: "height 100001, not coinbase",
			entry: entry2,
			serialized: "0x8b994240420f000000000000e3054b411051da5492aec7a823b00cb3add772d70c000000000000000000000000",
		},
		//test3:
		{
			name: "height 100001, not coinbase, spent",
			entry: entry3,
			serialized: "0x",
		},
		//test4:
		{
			name: "height 1, not coinbase, ScriptHash",
			entry: entry4,
			serialized: "0x020040075af075070001e3054b411051da5492aec7a823b00cb3add772d70c000000010000000000000001",
		},
		//test5:
		{
			name: "height 2, not coinbase, isContract",
			entry: entry5,
			serialized: "0x0440420f000000000006e3054b411051da5492aec7a823b00cb3add772d70c000000000000000000000000",
		},
		//test6:
		{
			name: "height 3, not coinbase, isVote",
			entry: entry6,
			serialized: "0x0640420f000000000007e3054b411051da5492aec7a823b00cb3add772d70c000000000000000000000001",
		},
		//test7:
		{
			name: "height 3, not coinbase, invalid pkscript",
			entry: entry7,
			serialized: "0x066400000000000000100102030405060c000000000000000000000001",
		},
		//test8:
		{
			name: "height 3, not coinbase, no pkscript",
			entry: entry8,
			serialized: "0x0664000000000000000a0c000000000000000000000001",
		},
		//test9:
		{
			name: "height 688, not coinbase, pkscript",
			entry: entry9,
			serialized: "0x8960020000000000000001e3054b411051da5492aec7a823b00cb3add772d70c000000000000000100000008",
		},
	}

	for i, test := range tests {
		// Ensure the utxo entry serializes to the expected value.
		gotBytes, err := serializeUtxoEntry(test.entry)
		if err != nil {
			t.Errorf("serializeUtxoEntry #%d (%s) unexpected error: %v", i, test.name, err)
			continue
		}
		gotHex := hexutil.Encode(gotBytes)
		if gotHex != test.serialized {
			t.Errorf("serializeUtxoEntry #%d (%s): mismatched - got %s, want %s",
				i, test.name, gotHex, test.serialized)
			continue
		}

		// Don't try to deserialize if the test entry was spent since it
		// will have a nil serialization.
		if test.entry.IsSpent() {
			continue
		}

		serializedBytes, err := hexutil.Decode(test.serialized)
		// Deserialize to a utxo entry.
		utxoEntry, err := DeserializeUtxoEntry(serializedBytes)
		if err != nil {
			t.Errorf("DeserializeUtxoEntry #%d (%s) unexpected error: %v", i, test.name, err)
			continue
		}

		// The deserialized entry must not be marked spent since unspent
		// entries are not serialized.
		if utxoEntry.IsSpent() {
			t.Errorf("DeserializeUtxoEntry #%d (%s) output should not be marked spent", i, test.name)
			continue
		}

		// Ensure the deserialized entry has the same properties as the
		// ones in the test entry.
		if utxoEntry.Amount() != test.entry.Amount() {
			t.Errorf("DeserializeUtxoEntry #%d (%s) mismatched amounts: got %d, want %d",
				i, test.name, utxoEntry.Amount(), test.entry.Amount())
			continue
		}

		if !bytes.Equal(utxoEntry.PkScript(), test.entry.PkScript()) {
			t.Errorf("DeserializeUtxoEntry #%d (%s) mismatched scripts: got %x, want %x",
				i, test.name, utxoEntry.PkScript(), test.entry.PkScript())
			continue
		}
		if utxoEntry.BlockHeight() != test.entry.BlockHeight() {
			t.Errorf("DeserializeUtxoEntry #%d (%s) mismatched block height: got %d, want %d",
				i, test.name, utxoEntry.BlockHeight(), test.entry.BlockHeight())
			continue
		}
		if utxoEntry.IsCoinBase() != test.entry.IsCoinBase() {
			t.Errorf("DeserializeUtxoEntry #%d (%s) mismatched coinbase flag: got %v, want %v",
				i, test.name, utxoEntry.IsCoinBase(), test.entry.IsCoinBase())
			continue
		}
	}
}

// TestUtxoEntryHeaderCodeErrors performs negative tests against unspent
// transaction output header codes to ensure error paths work as expected.
func TestUtxoEntryHeaderCodeErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		entry   *txo.UtxoEntry
		code    uint64
		errType error
	}{
		{
			name:    "Force assertion due to spent output",
			entry:   &txo.UtxoEntry{},
			errType: common.AssertError(""),
		},
	}
	tests[0].entry.Spend()

	for _, test := range tests {
		// Ensure the expected error type is returned and the code is 0.
		code, err := utxoEntryHeaderCode(test.entry)
		if reflect.TypeOf(err) != reflect.TypeOf(test.errType) {
			t.Errorf("utxoEntryHeaderCode (%s): expected error type does not match - got %T, want %T",
				test.name, err, test.errType)
			continue
		}
		if code != 0 {
			t.Errorf("utxoEntryHeaderCode (%s): unexpected code on error - got %d, want 0", test.name, code)
			continue
		}
	}
}

// TestUtxoEntryDeserializeErrors performs negative tests against deserializing
// unspent transaction outputs to ensure error paths work as expected.
func TestUtxoEntryDeserializeErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		serialized []byte
		errType    error
	}{
		{
			name:       "no data after header code",
			serialized: hexToBytes("02"),
			errType:    common.DeserializeError(""),
		},
		{
			name:       "incomplete compressed txout",
			serialized: hexToBytes("0232"),
			errType:    common.DeserializeError(""),
		},
	}

	for _, test := range tests {
		// Ensure the expected error type is returned and the returned
		// entry is nil.
		entry, err := DeserializeUtxoEntry(test.serialized)
		if reflect.TypeOf(err) != reflect.TypeOf(test.errType) {
			t.Errorf("DeserializeUtxoEntry (%s): expected error type does not match - got %T, want %T",
				test.name, err, test.errType)
			continue
		}
		if entry != nil {
			t.Errorf("DeserializeUtxoEntry (%s): returned entry is not nil", test.name)
			continue
		}
	}
}

// TestBestChainStateSerialization ensures serializing and deserializing the
// best chain state works as expected.
func TestBestChainStateSerialization(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		state      bestChainState
		serialized []byte
	}{
		{
			name: "genesis",
			state: bestChainState{
				hash:      *newHashFromStr("000000000019d6689c085ae165831e934ff763ae46a2a6c172b3f1b60a8ce26f"),
				height:    0,
				totalTxns: 1,
			},
			serialized: hexToBytes("6fe28c0ab6f1b372c1a6a246ae63f74f931e8365e15a089c68d6190000000000000000000100000000000000"),
		},
		{
			name: "block 1",
			state: bestChainState{
				hash:      *newHashFromStr("00000000839a8e6886ab5951d76f411475428afc90947ee320161bbf18eb6048"),
				height:    1,
				totalTxns: 2,
			},
			serialized: hexToBytes("4860eb18bf1b1620e37e9490fc8a427514416fd75159ab86688e9a8300000000010000000200000000000000"),
		},
	}

	for i, test := range tests {
		// Ensure the state serializes to the expected value.
		gotBytes := serializeBestChainState(test.state)
		if !bytes.Equal(gotBytes, test.serialized) {
			t.Errorf("serializeBestChainState #%d (%s): mismatched bytes - got %x, want %x",
				i, test.name, gotBytes, test.serialized)
			continue
		}

		// Ensure the serialized bytes are decoded back to the expected
		// state.
		state, err := deserializeBestChainState(test.serialized)
		if err != nil {
			t.Errorf("deserializeBestChainState #%d (%s) unexpected error: %v", i, test.name, err)
			continue
		}
		if !reflect.DeepEqual(state, test.state) {
			t.Errorf("deserializeBestChainState #%d (%s) mismatched state - got %v, want %v",
				i, test.name, state, test.state)
			continue

		}
	}
}

// TestBestChainStateDeserializeErrors performs negative tests against
// deserializing the chain state to ensure error paths work as expected.
func TestBestChainStateDeserializeErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		serialized []byte
		errType    error
	}{
		{
			name:       "nothing serialized",
			serialized: hexToBytes(""),
			errType:    database.Error{ErrorCode: database.ErrCorruption},
		},
		{
			name:       "short data in hash",
			serialized: hexToBytes("0000"),
			errType:    database.Error{ErrorCode: database.ErrCorruption},
		},
	}

	for _, test := range tests {
		// Ensure the expected error type and code is returned.
		_, err := deserializeBestChainState(test.serialized)
		if reflect.TypeOf(err) != reflect.TypeOf(test.errType) {
			t.Errorf("deserializeBestChainState (%s): expected error type does not match - got %T, want %T",
				test.name, err, test.errType)
			continue
		}
		if derr, ok := err.(database.Error); ok {
			tderr := test.errType.(database.Error)
			if derr.ErrorCode != tderr.ErrorCode {
				t.Errorf("deserializeBestChainState (%s): wrong  error code got: %v, want: %v",
					test.name, derr.ErrorCode, tderr.ErrorCode)
				continue
			}
		}
	}
}

func TestDbFetchBalance(t *testing.T) {
	parivateKeyList := []string{
		"0xd0f0461b7b4d26cf370e6c73b58ef7fa26e8e30853a8cee901ed42cf0879cb6e",  //privateKey0
		"0xd07f68f78fc58e3dc8ea72ff69784aa9542c452a4ee66b2665fa3cccb48441c2",  //privateKey1
	}

	testRoundSize := uint16(10)
	accList, netParam, chain, teardownFunc, err := createFakeChainByPrivateKeys(parivateKeyList, testRoundSize)
	defer teardownFunc()

	validators, filters, _ := chain.GetValidatorsByNode(1, chain.bestChain.tip())

	addrs := make([]common.Address, 0)
	for i:=0; i<len(validators); i++ {
		find := false
		for k:=0;k<len(addrs);k++ {
			if *validators[i] == addrs[k] {
				find = true
			}
		}
		if !find {
			addrs = append(addrs, *validators[i])
		}
	}

	resultEntryPairs := make(map[common.Address][]protos.OutPoint)

	curSlot := uint16(0)
	curEpoch := uint32(1)
	testBlkCnt := int32(3)
	for addNum:=int32(0); addNum<testBlkCnt; addNum++ {
		if addNum != 0 {
			if 0 == addNum % int32(testRoundSize) {
				curSlot = 0
				curEpoch++
				validators, filters, _ = chain.GetValidatorsByNode(curEpoch, chain.bestChain.tip())
			} else {
				curSlot++
			}
		}

		//create block:
		block, _, err := createAndSignBlock(netParam, accList, validators, filters, chain, uint32(curEpoch),
			uint16(curSlot), chain.bestChain.height(), protos.Asset{0,0}, 0,
			validators[curSlot],nil,0,chain.bestChain.tip())
		if err != nil {
			t.Errorf("create block error %v", err)
		}
		blkNums := len(block.Transactions())
		coinBaseTx := block.Transactions()[blkNums - 1]

		idx := 0
		if curSlot == 0 {
			idx = 1
		}
		preOut := protos.OutPoint{
			*coinBaseTx.Hash(),
			uint32(idx),
		}
		if _,ok := resultEntryPairs[*validators[curSlot]]; ok {
			resultEntryPairs[*validators[curSlot]] = append(resultEntryPairs[*validators[curSlot]],preOut)
		} else {
			tmpPreOuts := make([]protos.OutPoint,0)
			tmpPreOuts = append(tmpPreOuts,preOut)
			resultEntryPairs[*validators[curSlot]] = tmpPreOuts
		}

		// Insert the block to bestChain:
		_, isOrphan, err := chain.ProcessBlock(block, nil, common.BFNone)
		if err != nil {
			t.Errorf("ProcessBlock err %v", err)
		}
		log.Infof("isOrphan = %v",isOrphan)
	}

	for i, addr := range addrs {
		log.Infof("=============the %d test start=============", i)

		err = chain.db.View(func(dbTx database.Tx) error {
			mp, err := dbFetchBalance(dbTx, addr[:])
			if err != nil {
				return err
			}

			if len(*mp) > 0 {
				if _,ok := resultEntryPairs[addr]; !ok {
					t.Errorf("the %d test error: can not find addr in resultEntryPairs",i)
				}
				resultPreOuts := resultEntryPairs[addr]

				if len(resultPreOuts) != len(*mp) {
					t.Errorf("the %d test error: the numbers of resultPreOut do not equal the expect ",i)
				}

				for _, data := range *mp {
					findPreOut := false
					for _, preOut := range resultPreOuts {
						if preOut == data.Key {
							findPreOut = true
						}
					}
					if !findPreOut {
						t.Errorf("the %d test error: the preOut do not equal the expect",i)
					}
				}
			}

			for _, data := range *mp {
				op := data.Key
				entity := data.Value
				log.Infof("Outpoint hash %x, its index %d, utxo amount %d, height %d, flags %d, asset %v",
					op.Hash, op.Index, entity.Amount(), entity.BlockHeight(), entity.PackedFlags(), entity.Asset())
			}
			return nil
		})

		if err != nil {
			t.Log(err)
		}
	}
}