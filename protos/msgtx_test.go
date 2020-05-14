// Copyright (c) 2018-2020 The asimov developers
// Copyright (c) 2013-2017 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package protos

import (
	"bytes"
	"fmt"
	"io"
	"reflect"
	"testing"

	"github.com/AsimovNetwork/asimov/common"
)

// TestTx tests the MsgTx API.
func TestTx(t *testing.T) {
	pver := common.ProtocolVersion

	// Block 100000 hash.
	hashStr := "3ba27aa200b1cecaad478d2b00432346c3f1f3986da1afd33e506"
	hash, err := common.NewHashFromStr(hashStr)
	if err != nil {
		t.Errorf("NewHashFromStr: %v", err)
	}

	// Ensure the command is expected value.
	wantCmd := "tx"
	msg := NewMsgTx(1)
	if cmd := msg.Command(); cmd != wantCmd {
		t.Errorf("NewMsgAddr: wrong command - got %v want %v",
			cmd, wantCmd)
	}

	// Ensure max payload is expected value for latest protocol version.
	wantPayload := uint32(2 * 1024 * 1024)
	maxPayload := msg.MaxPayloadLength(pver)
	if maxPayload != wantPayload {
		t.Errorf("MaxPayloadLength: wrong max payload length for "+
			"protocol version %d - got %v, want %v", pver,
			maxPayload, wantPayload)
	}

	// Ensure we get the same transaction output point data back out.
	// NOTE: This is a block hash and made up index, but we're only
	// testing package functionality.
	prevOutIndex := uint32(1)
	prevOut := NewOutPoint(hash, prevOutIndex)
	if !prevOut.Hash.IsEqual(hash) {
		t.Errorf("NewOutPoint: wrong hash - got %v, want %v",
			&prevOut.Hash, hash)
	}
	if prevOut.Index != prevOutIndex {
		t.Errorf("NewOutPoint: wrong index - got %v, want %v",
			prevOut.Index, prevOutIndex)
	}
	prevOutStr := fmt.Sprintf("%s:%d", hash.String(), prevOutIndex)
	if s := prevOut.String(); s != prevOutStr {
		t.Errorf("OutPoint.String: unexpected result - got %v, "+
			"want %v", s, prevOutStr)
	}

	// Ensure we get the same transaction input back out.
	sigScript := []byte{0x04, 0x31, 0xdc, 0x00, 0x1b, 0x01, 0x62}
	txIn := NewTxIn(prevOut, sigScript)
	if !reflect.DeepEqual(&txIn.PreviousOutPoint, prevOut) {
		t.Errorf("NewTxIn: wrong prev outpoint - got %v, want %v",
			&txIn.PreviousOutPoint,
			prevOut)
	}
	if !bytes.Equal(txIn.SignatureScript, sigScript) {
		t.Errorf("NewTxIn: wrong signature script - got %v, want %v",
			txIn.SignatureScript,
			sigScript)
	}

	// Ensure we get the same transaction output back out.
	txValue := int64(5000000000)
	pkScript := []byte{
		0x41, // OP_DATA_65
		0x04, 0xd6, 0x4b, 0xdf, 0xd0, 0x9e, 0xb1, 0xc5,
		0xfe, 0x29, 0x5a, 0xbd, 0xeb, 0x1d, 0xca, 0x42,
		0x81, 0xbe, 0x98, 0x8e, 0x2d, 0xa0, 0xb6, 0xc1,
		0xc6, 0xa5, 0x9d, 0xc2, 0x26, 0xc2, 0x86, 0x24,
		0xe1, 0x81, 0x75, 0xe8, 0x51, 0xc9, 0x6b, 0x97,
		0x3d, 0x81, 0xb0, 0x1c, 0xc3, 0x1f, 0x04, 0x78,
		0x34, 0xbc, 0x06, 0xd6, 0xd6, 0xed, 0xf6, 0x20,
		0xd1, 0x84, 0x24, 0x1a, 0x6a, 0xed, 0x8b, 0x63,
		0xa6, // 65-byte signature
		0xac, // OP_CHECKSIG
	}
	txOut := NewTxOut(txValue, pkScript, *NewAsset(0, 1, 2))
	if txOut.Value != txValue {
		t.Errorf("NewTxOut: wrong pk script - got %v, want %v",
			txOut.Value, txValue)

	}
	if !bytes.Equal(txOut.PkScript, pkScript) {
		t.Errorf("NewTxOut: wrong pk script - got %v, want %v",
			txOut.PkScript,
			pkScript)
	}

	// Ensure transaction inputs are added properly.
	msg.AddTxIn(txIn)
	if !reflect.DeepEqual(msg.TxIn[0], txIn) {
		t.Errorf("AddTxIn: wrong transaction input added - got %v, want %v",
			msg.TxIn[0], txIn)
	}

	// Ensure transaction outputs are added properly.
	msg.AddTxOut(txOut)
	if !reflect.DeepEqual(msg.TxOut[0], txOut) {
		t.Errorf("AddTxIn: wrong transaction output added - got %v, want %v",
			msg.TxOut[0], txOut)
	}

	// Ensure the copy produced an identical transaction message.
	newMsg := msg.Copy()
	if !reflect.DeepEqual(newMsg, msg) {
		t.Errorf("Copy: mismatched tx messages - got %v, want %v",
			newMsg, msg)
	}
}

// TestTx tests the MsgTx API.
func TestTx_VM(t *testing.T) {
	pver := common.ProtocolVersion

	// Block 100000 hash.
	hashStr := "3ba27aa200b1cecaad478d2b00432346c3f1f3986da1afd33e506"
	hash, err := common.NewHashFromStr(hashStr)
	if err != nil {
		t.Errorf("NewHashFromStr: %v", err)
	}

	// Ensure the command is expected value.
	wantCmd := "tx"
	msg := NewMsgTx(1)
	if cmd := msg.Command(); cmd != wantCmd {
		t.Errorf("NewMsgAddr: wrong command - got %v want %v",
			cmd, wantCmd)
	}

	// Ensure max payload is expected value for latest protocol version.
	wantPayload := uint32(2 * 1024 * 1024)
	maxPayload := msg.MaxPayloadLength(pver)
	if maxPayload != wantPayload {
		t.Errorf("MaxPayloadLength: wrong max payload length for "+
			"protocol version %d - got %v, want %v", pver,
			maxPayload, wantPayload)
	}

	// Ensure we get the same transaction output point data back out.
	// NOTE: This is a block hash and made up index, but we're only
	// testing package functionality.
	prevOutIndex := uint32(1)
	prevOut := NewOutPoint(hash, prevOutIndex)
	if !prevOut.Hash.IsEqual(hash) {
		t.Errorf("NewOutPoint: wrong hash - got %v, want %v",
			&prevOut.Hash, hash)
	}
	if prevOut.Index != prevOutIndex {
		t.Errorf("NewOutPoint: wrong index - got %v, want %v",
			prevOut.Index, prevOutIndex)
	}
	prevOutStr := fmt.Sprintf("%s:%d", hash.String(), prevOutIndex)
	if s := prevOut.String(); s != prevOutStr {
		t.Errorf("OutPoint.String: unexpected result - got %v, "+
			"want %v", s, prevOutStr)
	}

	// Ensure we get the same transaction input back out.
	sigScript := []byte{0x04, 0x31, 0xdc, 0x00, 0x1b, 0x01, 0x62}
	txIn := NewTxIn(prevOut, sigScript)
	if !reflect.DeepEqual(&txIn.PreviousOutPoint, prevOut) {
		t.Errorf("NewTxIn: wrong prev outpoint - got %v, want %v",
			&txIn.PreviousOutPoint,
			prevOut)
	}
	if !bytes.Equal(txIn.SignatureScript, sigScript) {
		t.Errorf("NewTxIn: wrong signature script - got %v, want %v",
			txIn.SignatureScript,
			sigScript)
	}

	// Ensure transaction inputs are added properly.
	msg.AddTxIn(txIn)
	if !reflect.DeepEqual(msg.TxIn[0], txIn) {
		t.Errorf("AddTxIn: wrong transaction input added - got %v, want %v",
			msg.TxIn[0], txIn)
	}

	// Ensure the copy produced an identical transaction message.
	newMsg := msg.Copy()
	if !reflect.DeepEqual(newMsg, msg) {
		t.Errorf("Copy: mismatched tx messages - got %v, want %v",
			newMsg, msg)
	}

	var buf bytes.Buffer
	msg.Serialize(&buf)
	newMsg = &MsgTx{}
	newMsg.Deserialize(&buf)
	if !reflect.DeepEqual(newMsg, msg) {
		t.Errorf("Serialize: mismatched tx messages - got %v, want %v",
			newMsg, msg)
	}
}

// TestTxHash tests the ability to generate the hash of a transaction accurately.
func TestTxHash(t *testing.T) {
	hashStr := "4b9b665e1c3675f620c350498cb8d2b213b1b1916836bd682d92a147feed0842"
	wantHash := common.HexToHash(hashStr)

	// First transaction from block 113875.
	msgTx := NewMsgTx(1)
	txIn := TxIn{
		PreviousOutPoint: OutPoint{
			Hash:  common.Hash{},
			Index: 0xffffffff,
		},
		SignatureScript: []byte{0x04, 0x31, 0xdc, 0x00, 0x1b, 0x01, 0x62},
		Sequence:        0xffffffff,
	}
	txOut := TxOut{
		Value: 5000000000,
		PkScript: []byte{
			0x41, // OP_DATA_65
			0x04, 0xd6, 0x4b, 0xdf, 0xd0, 0x9e, 0xb1, 0xc5,
			0xfe, 0x29, 0x5a, 0xbd, 0xeb, 0x1d, 0xca, 0x42,
			0x81, 0xbe, 0x98, 0x8e, 0x2d, 0xa0, 0xb6, 0xc1,
			0xc6, 0xa5, 0x9d, 0xc2, 0x26, 0xc2, 0x86, 0x24,
			0xe1, 0x81, 0x75, 0xe8, 0x51, 0xc9, 0x6b, 0x97,
			0x3d, 0x81, 0xb0, 0x1c, 0xc3, 0x1f, 0x04, 0x78,
			0x34, 0xbc, 0x06, 0xd6, 0xd6, 0xed, 0xf6, 0x20,
			0xd1, 0x84, 0x24, 0x1a, 0x6a, 0xed, 0x8b, 0x63,
			0xa6, // 65-byte signature
			0xac, // OP_CHECKSIG
		},
		Asset: Asset{0,0},
	}
	msgTx.AddTxIn(&txIn)
	msgTx.AddTxOut(&txOut)
	msgTx.LockTime = 0

	// Ensure the hash produced is expected.
	txHash := msgTx.TxHash()
	if !bytes.Equal(txHash[:], wantHash[:]) {
		t.Errorf("TxHash: wrong hash - got %v, want %v",
			txHash, wantHash)
	}
}

// TestTxSha tests the ability to generate the wtxid
func TestTxSha(t *testing.T) {
	hashStrTxid := "1ecb47c75f9f84199268c26999c0d79ba567d555897a68e893ea478605993181"
	wantHashTxid := common.HexToHash(hashStrTxid)

	// From block 23157 in a past version of segnet.
	msgTx := NewMsgTx(1)
	txIn := TxIn{
		PreviousOutPoint: OutPoint{
			Hash: common.Hash{
				0xa5, 0x33, 0x52, 0xd5, 0x13, 0x57, 0x66, 0xf0,
				0x30, 0x76, 0x59, 0x74, 0x18, 0x26, 0x3d, 0xa2,
				0xd9, 0xc9, 0x58, 0x31, 0x59, 0x68, 0xfe, 0xa8,
				0x23, 0x52, 0x94, 0x67, 0x48, 0x1f, 0xf9, 0xcd,
			},
			Index: 19,
		},
		Sequence: 0xffffffff,
	}
	txOut := TxOut{
		Value: 395019,
		PkScript: []byte{
			0x00,
			0x14, // OP_DATA_20
			0x9d, 0xda, 0xc6, 0xf3, 0x9d, 0x51, 0xe0, 0x39,
			0x8e, 0x53, 0x2a, 0x22, 0xc4, 0x1b, 0xa1, 0x89,
			0x40, 0x6a, 0x85, 0x23, // 20-byte pub key hash
		},
		Asset: Asset{0,0},
	}
	msgTx.AddTxIn(&txIn)
	msgTx.AddTxOut(&txOut)
	msgTx.LockTime = 0

	// Ensure the correct txid, and wtxid is produced as expected.
	txid := msgTx.TxHash()
	if !txid.IsEqual(&wantHashTxid) {
		t.Errorf("TxSha: wrong hash - got %v, want %v",
			txid, wantHashTxid)
	}
}

// TestTxWire tests the MsgTx protos encode and decode for various numbers
// of transaction inputs and outputs and protocol versions.
func TestTxWire(t *testing.T) {
	// Empty tx message.
	noTx := NewMsgTx(1)
	noTx.Version = 1
	noTxEncoded := []byte{
		0x01, 0x00, 0x00, 0x00, // Version
		0x00,                   // Varint for number of input transactions
		0x00,                   // Varint for number of output transactions
		0x00, 0x00, 0x00, 0x00, // Gaslimit
		0x00, 0x00, 0x00, 0x00, // Lock time
	}

	tests := []struct {
		in   *MsgTx          // Message to encode
		out  *MsgTx          // Expected decoded message
		buf  []byte          // Wire encoding
		pver uint32          // Protocol version for protos encoding
		enc  MessageEncoding // Message encoding format
	}{
		// Latest protocol version with no transactions.
		{
			noTx,
			noTx, noTxEncoded,
			common.ProtocolVersion,
			BaseEncoding,
		},

		// Latest protocol version with multiple transactions.
		{
			multiTx,
			multiTx,
			multiTxEncoded,
			common.ProtocolVersion,
			BaseEncoding,
		},
	}

	t.Logf("Running %d tests", len(tests))
	for i, test := range tests {
		// Encode the message to protos format.
		var buf bytes.Buffer
		err := test.in.VVSEncode(&buf, test.pver, test.enc)
		if err != nil {
			t.Errorf("VVSEncode #%d error %v", i, err)
			continue
		}
		if !bytes.Equal(buf.Bytes(), test.buf) {
			t.Errorf("VVSEncode #%d\n got: %v want: %v", i,
				buf.Bytes(), test.buf)
			continue
		}

		// Decode the message from protos format.
		var msg MsgTx
		rbuf := bytes.NewReader(test.buf)
		err = msg.VVSDecode(rbuf, test.pver, test.enc)
		if err != nil {
			t.Errorf("VVSDecode #%d error %v", i, err)
			continue
		}
		if !reflect.DeepEqual(&msg, test.out) {
			t.Errorf("VVSDecode #%d\n got: %v want: %v", i,
				&msg, test.out)
			continue
		}
	}
}

// TestTxWireErrors performs negative tests against protos encode and decode
// of MsgTx to confirm error paths work correctly.
func TestTxWireErrors(t *testing.T) {
	pver := common.ProtocolVersion
	length := len(multiTxEncoded)
	fmt.Println(length)
	tests := []struct {
		in       *MsgTx          // Value to encode
		buf      []byte          // Wire encoding
		pver     uint32          // Protocol version for protos encoding
		enc      MessageEncoding // Message encoding format
		max      int             // Max size of fixed buffer to induce errors
		writeErr error           // Expected write error
		readErr  error           // Expected read error
	}{
		// Force error in version.
		{multiTx, multiTxEncoded, pver, BaseEncoding, 0, io.ErrShortWrite, io.EOF},
		// Force error in number of transaction inputs.
		{multiTx, multiTxEncoded, pver, BaseEncoding, 4, io.ErrShortWrite, io.EOF},
		// Force error in number of transaction outputs.
		{multiTx, multiTxEncoded, pver, BaseEncoding, 54, io.ErrShortWrite, io.EOF},
		// Force error in TxContract.
		{multiTx, multiTxEncoded, pver, BaseEncoding, 298, io.ErrShortWrite, io.EOF},
		// Force error in transaction output lock time.
		{multiTx, multiTxEncoded, pver, BaseEncoding, 302, io.ErrShortWrite, io.EOF},
	}

	t.Logf("Running %d tests", len(tests))
	for i, test := range tests {
		// Encode to protos format.
		w := newFixedWriter(test.max)
		err := test.in.VVSEncode(w, test.pver, test.enc)
		if err != test.writeErr {
			t.Errorf("VVSEncode #%d wrong error got: %v, want: %v",
				i, err, test.writeErr)
			continue
		}

		// Decode from protos format.
		var msg MsgTx
		r := newFixedReader(test.max, test.buf)
		err = msg.VVSDecode(r, test.pver, test.enc)
		if err != test.readErr {
			t.Errorf("VVSDecode #%d wrong error got: %v, want: %v",
				i, err, test.readErr)
			continue
		}
	}
}

// TestTxSerialize tests MsgTx serialize and deserialize.
func TestTxSerialize(t *testing.T) {
	// Empty tx message.
	noTx := NewMsgTx(1)
	noTx.Version = 1
	noTxEncoded := []byte{
		0x01, 0x00, 0x00, 0x00, // Version
		0x00,                   // Varint for number of input transactions
		0x00,                   // Varint for number of output transactions
		0x00, 0x00, 0x00, 0x00, // Gaslimit
		0x00, 0x00, 0x00, 0x00, // Lock time
	}

	tests := []struct {
		in           *MsgTx // Message to encode
		out          *MsgTx // Expected decoded message
		buf          []byte // Serialized data
		pkScriptLocs []int  // Expected output script locations
		dataLoc      int    // Expected output data location
	}{
		// No transactions.
		{
			noTx,
			noTx,
			noTxEncoded,
			nil,
			0,
		},

		// Multiple transactions.
		{
			multiTx,
			multiTx,
			multiTxEncoded,
			multiTxPkScriptLocs,
			multiTxDataLoc,
		},
	}

	t.Logf("Running %d tests", len(tests))
	for i, test := range tests {
		// Serialize the transaction.
		var buf bytes.Buffer
		err := test.in.Serialize(&buf)
		if err != nil {
			t.Errorf("Serialize #%d error %v", i, err)
			continue
		}
		if !bytes.Equal(buf.Bytes(), test.buf) {
			t.Errorf("Serialize #%d\n got: %v want: %v", i,
				buf.Bytes(), test.buf)
			continue
		}

		// Deserialize the transaction.
		var tx MsgTx
		rbuf := bytes.NewReader(test.buf)
		err = tx.Deserialize(rbuf)
		if err != nil {
			t.Errorf("Deserialize #%d error %v", i, err)
			continue
		}
		if !reflect.DeepEqual(&tx, test.out) {
			t.Errorf("Deserialize #%d\n got: %v want: %v", i,
				&tx, test.out)
			continue
		}

		// Ensure the public key script locations are accurate.
		pkScriptLocs := test.in.PkScriptLocs()
		if !reflect.DeepEqual(pkScriptLocs, test.pkScriptLocs) {
			t.Errorf("PkScriptLocs #%d\n got: %v want: %v", i,
				pkScriptLocs,
				test.pkScriptLocs)
			continue
		}
		for j, loc := range pkScriptLocs {
			wantPkScript := test.in.TxOut[j].PkScript
			gotPkScript := test.buf[loc : loc+len(wantPkScript)]
			if !bytes.Equal(gotPkScript, wantPkScript) {
				t.Errorf("PkScriptLocs #%d:%d\n unexpected "+
					"script got: %v want: %v", i, j,
					gotPkScript,
					wantPkScript)
			}
		}

		// Ensure the data location is accurate.
		dataLoc := test.in.DataLoc()
		if dataLoc != test.in.DataLoc() {
			t.Errorf("DataLoc #%d\n got: %d want: %d", i,
				dataLoc, test.in.DataLoc())
			continue
		}
		if len(test.in.TxOut) > 0 {
			wantData := test.in.TxOut[0].Data
			gotData := test.buf[dataLoc : dataLoc+len(wantData)]
			if !bytes.Equal(gotData, wantData) {
				t.Errorf("DataLoc #%d\n unexpected "+
					"data got: %v want: %v", i,
					gotData,
					wantData)
			}
		}
	}
}

// TestTxSerializeErrors performs negative tests against protos encode and decode
// of MsgTx to confirm error paths work correctly.
func TestTxSerializeErrors(t *testing.T) {
	tests := []struct {
		in       *MsgTx // Value to encode
		buf      []byte // Serialized data
		max      int    // Max size of fixed buffer to induce errors
		writeErr error  // Expected write error
		readErr  error  // Expected read error
	}{
		// Force error in version.
		{multiTx, multiTxEncoded, 0, io.ErrShortWrite, io.EOF},
		// Force error in number of transaction inputs.
		{multiTx, multiTxEncoded, 4, io.ErrShortWrite, io.EOF},
		// Force error in number of transaction outputs.
		{multiTx, multiTxEncoded, 54, io.ErrShortWrite, io.EOF},
		// Force error in TxContract.
		{multiTx, multiTxEncoded, 298, io.ErrShortWrite, io.EOF},
		// Force error in transaction output lock time.
		{multiTx, multiTxEncoded, 302, io.ErrShortWrite, io.EOF},
	}

	t.Logf("Running %d tests", len(tests))
	for i, test := range tests {
		// Deserialize the transaction.
		var tx MsgTx
		r := newFixedReader(test.max, test.buf)
		err := tx.Deserialize(r)
		if err != test.readErr {
			t.Errorf("Deserialize #%d wrong error got: %v, want: %v",
				i, err, test.readErr)
			continue
		}
	}
}

// TestTxOverflowErrors performs tests to ensure deserializing transactions
// which are intentionally crafted to use large values for the variable number
// of inputs and outputs are handled properly.  This could otherwise potentially
// be used as an attack vector.
func TestTxOverflowErrors(t *testing.T) {
	pver := uint32(0)
	txVer := uint32(1)

	tests := []struct {
		buf     []byte          // Wire encoding
		pver    uint32          // Protocol version for protos encoding
		enc     MessageEncoding // Message encoding format
		version uint32          // Transaction version
		err     error           // Expected error
	}{
		// Transaction that claims to have ~uint64(0) inputs.
		{
			[]byte{
				0x01, 0x00, 0x00, 0x00, // Version
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, // Varint for number of input transactions
			}, pver, BaseEncoding, txVer, &MessageError{},
		},

		// Transaction that claims to have ~uint64(0) outputs.
		{
			[]byte{
				0x01, 0x00, 0x00, 0x00, // Version
				0x00, // Varint for number of input transactions
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, // Varint for number of output transactions
			}, pver, BaseEncoding, txVer, &MessageError{},
		},

		// Transaction that has an input with a signature script that
		// claims to have ~uint64(0) length.
		{
			[]byte{
				0x01, 0x00, 0x00, 0x00, // Version
				0x01, // Varint for number of input transactions
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // Previous output hash
				0xff, 0xff, 0xff, 0xff, // Prevous output index
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, // Varint for length of signature script
			}, pver, BaseEncoding, txVer, &MessageError{},
		},

		// Transaction that has an output with a public key script
		// that claims to have ~uint64(0) length.
		{
			[]byte{
				0x01, 0x00, 0x00, 0x00, // Version
				0x01, // Varint for number of input transactions
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // Previous output hash
				0xff, 0xff, 0xff, 0xff, // Prevous output index
				0x07,                                     // Varint for length of signature script
				0x04, 0x31, 0xdc, 0x00, 0x1b, 0x01, 0x62, // Signature script
				0xff, 0xff, 0xff, 0xff, // Sequence
				0x01,                                           // Varint for number of output transactions
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // Transaction amount
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, // Varint for length of public key script
			}, pver, BaseEncoding, txVer, &MessageError{},
		},
	}

	t.Logf("Running %d tests", len(tests))
	for i, test := range tests {
		// Decode from protos format.
		var msg MsgTx
		r := bytes.NewReader(test.buf)
		err := msg.VVSDecode(r, test.pver, test.enc)
		if reflect.TypeOf(err) != reflect.TypeOf(test.err) {
			t.Errorf("VVSDecode #%d wrong error got: %v, want: %v",
				i, err, reflect.TypeOf(test.err))
			continue
		}

		// Decode from protos format.
		r = bytes.NewReader(test.buf)
		err = msg.Deserialize(r)
		if reflect.TypeOf(err) != reflect.TypeOf(test.err) {
			t.Errorf("Deserialize #%d wrong error got: %v, want: %v",
				i, err, reflect.TypeOf(test.err))
			continue
		}
	}
}

// TestTxSerializeSizeStripped performs tests to ensure the serialize size for
// various transactions is accurate.
func TestTxSerializeSizeStripped(t *testing.T) {
	// Empty tx message.
	noTx := NewMsgTx(1)
	noTx.Version = 1

	tests := []struct {
		in   *MsgTx // Tx to encode
		size int    // Expected serialized size
	}{
		// No inputs or outpus.
		{noTx, 14},

		// Transcaction with an input and an output.
		{multiTx, 306},
	}

	t.Logf("Running %d tests", len(tests))
	for i, test := range tests {
		serializedSize := test.in.SerializeSize()
		if serializedSize != test.size {
			t.Errorf("MsgTx.SerializeSize: #%d got: %d, want: %d", i,
				serializedSize, test.size)
			continue
		}
	}
}

// multiTx is a MsgTx with an input and output and used in various tests.
var multiTx = &MsgTx{
	Version: 1,
	TxIn: []*TxIn{
		{
			PreviousOutPoint: OutPoint{
				Hash:  common.Hash{},
				Index: 0xffffffff,
			},
			SignatureScript: []byte{
				0x04, 0x31, 0xdc, 0x00, 0x1b, 0x01, 0x62,
			},
			Sequence: 0xffffffff,
		},
	},
	TxOut: []*TxOut{
		{
			Value: 0x00,
			PkScript: []byte{
				0xc0, // OP_TEMPLATE
			},
			Asset: Asset{0,0},
			Data: []byte{
				0x00, 0x01, // type
				0x04, 0x00, 0x00, 0x00, // name length
				0x05, 0x00, 0x00, 0x00, // byte code length
				0x06, 0x00, 0x00, 0x00, // abi length
				0x07, 0x00, 0x00, 0x00, // source length
				0x50, 0x51, 0x52, 0x53, // name
				0x63, 0x63, 0x63, 0x63, 0x63, // byte code
				0x61, 0x61, 0x61, 0x61, 0x61, 0x61, // abi
				0x73, 0x73, 0x73, 0x73, 0x73, 0x73, 0x73, // source
			},
		},
		{
			Value: 0x12a05f200,
			PkScript: []byte{
				0x41, // OP_DATA_65
				0x04, 0xd6, 0x4b, 0xdf, 0xd0, 0x9e, 0xb1, 0xc5,
				0xfe, 0x29, 0x5a, 0xbd, 0xeb, 0x1d, 0xca, 0x42,
				0x81, 0xbe, 0x98, 0x8e, 0x2d, 0xa0, 0xb6, 0xc1,
				0xc6, 0xa5, 0x9d, 0xc2, 0x26, 0xc2, 0x86, 0x24,
				0xe1, 0x81, 0x75, 0xe8, 0x51, 0xc9, 0x6b, 0x97,
				0x3d, 0x81, 0xb0, 0x1c, 0xc3, 0x1f, 0x04, 0x78,
				0x34, 0xbc, 0x06, 0xd6, 0xd6, 0xed, 0xf6, 0x20,
				0xd1, 0x84, 0x24, 0x1a, 0x6a, 0xed, 0x8b, 0x63,
				0xa6, // 65-byte signature
				0xac, // OP_CHECKSIG
			},
			Asset: Asset{0,0},
			Data:  []byte{},
		},
		{
			Value: 0x5f5e100,
			PkScript: []byte{
				0x41, // OP_DATA_65
				0x04, 0xd6, 0x4b, 0xdf, 0xd0, 0x9e, 0xb1, 0xc5,
				0xfe, 0x29, 0x5a, 0xbd, 0xeb, 0x1d, 0xca, 0x42,
				0x81, 0xbe, 0x98, 0x8e, 0x2d, 0xa0, 0xb6, 0xc1,
				0xc6, 0xa5, 0x9d, 0xc2, 0x26, 0xc2, 0x86, 0x24,
				0xe1, 0x81, 0x75, 0xe8, 0x51, 0xc9, 0x6b, 0x97,
				0x3d, 0x81, 0xb0, 0x1c, 0xc3, 0x1f, 0x04, 0x78,
				0x34, 0xbc, 0x06, 0xd6, 0xd6, 0xed, 0xf6, 0x20,
				0xd1, 0x84, 0x24, 0x1a, 0x6a, 0xed, 0x8b, 0x63,
				0xa6, // 65-byte signature
				0xac, // OP_CHECKSIG
			},
			Asset: Asset{0,0},
			Data:  []byte{},
		},
	},
	TxContract: TxContract{
		GasLimit: 0,
	},
	LockTime: 0,
}

// multiTxEncoded is the protos encoded bytes for multiTx and is used in the various tests.
var multiTxEncoded = []byte{
	0x01, 0x00, 0x00, 0x00, // Version
	0x01, // Varint for number of input transactions
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // Previous output hash
	0xff, 0xff, 0xff, 0xff, // Prevous output index
	0x07,                                     // Varint for length of signature script
	0x04, 0x31, 0xdc, 0x00, 0x1b, 0x01, 0x62, // Signature script
	0xff, 0xff, 0xff, 0xff, // Sequence
	0x03,                                           // Varint for number of output transactions
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // Transaction amount
	0x01, // Varint for length of pk script
	0xc0, // OP_TEMPLATE
	0x0c,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, // asset
	0x28,       // OP_DATA_40
	0x00, 0x01, // type
	0x04, 0x00, 0x00, 0x00, // name length
	0x05, 0x00, 0x00, 0x00, // byte code length
	0x06, 0x00, 0x00, 0x00, // abi length
	0x07, 0x00, 0x00, 0x00, // source length
	0x50, 0x51, 0x52, 0x53, // name
	0x63, 0x63, 0x63, 0x63, 0x63, // byte code
	0x61, 0x61, 0x61, 0x61, 0x61, 0x61, // abi
	0x73, 0x73, 0x73, 0x73, 0x73, 0x73, 0x73, // source
	0x00, 0xf2, 0x05, 0x2a, 0x01, 0x00, 0x00, 0x00, // Transaction amount
	0x43, // Varint for length of pk script
	0x41, // OP_DATA_65
	0x04, 0xd6, 0x4b, 0xdf, 0xd0, 0x9e, 0xb1, 0xc5,
	0xfe, 0x29, 0x5a, 0xbd, 0xeb, 0x1d, 0xca, 0x42,
	0x81, 0xbe, 0x98, 0x8e, 0x2d, 0xa0, 0xb6, 0xc1,
	0xc6, 0xa5, 0x9d, 0xc2, 0x26, 0xc2, 0x86, 0x24,
	0xe1, 0x81, 0x75, 0xe8, 0x51, 0xc9, 0x6b, 0x97,
	0x3d, 0x81, 0xb0, 0x1c, 0xc3, 0x1f, 0x04, 0x78,
	0x34, 0xbc, 0x06, 0xd6, 0xd6, 0xed, 0xf6, 0x20,
	0xd1, 0x84, 0x24, 0x1a, 0x6a, 0xed, 0x8b, 0x63,
	0xa6, // 65-byte signature
	0xac, // OP_CHECKSIG
	0x0c,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, // asset
	0x00,                                           // Data
	0x00, 0xe1, 0xf5, 0x05, 0x00, 0x00, 0x00, 0x00, // Transaction amount
	0x43, // Varint for length of pk script
	0x41, // OP_DATA_65
	0x04, 0xd6, 0x4b, 0xdf, 0xd0, 0x9e, 0xb1, 0xc5,
	0xfe, 0x29, 0x5a, 0xbd, 0xeb, 0x1d, 0xca, 0x42,
	0x81, 0xbe, 0x98, 0x8e, 0x2d, 0xa0, 0xb6, 0xc1,
	0xc6, 0xa5, 0x9d, 0xc2, 0x26, 0xc2, 0x86, 0x24,
	0xe1, 0x81, 0x75, 0xe8, 0x51, 0xc9, 0x6b, 0x97,
	0x3d, 0x81, 0xb0, 0x1c, 0xc3, 0x1f, 0x04, 0x78,
	0x34, 0xbc, 0x06, 0xd6, 0xd6, 0xed, 0xf6, 0x20,
	0xd1, 0x84, 0x24, 0x1a, 0x6a, 0xed, 0x8b, 0x63,
	0xa6, // 65-byte signature
	0xac, // OP_CHECKSIG
	0x0c,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, // asset
	0x00,                   // Data
	0x00, 0x00, 0x00, 0x00, // GasLimit
	0x00, 0x00, 0x00, 0x00, // Lock time
}

// multiTxPkScriptLocs is the location information for the public key scripts
// located in multiTx.
var multiTxPkScriptLocs = []int{63, 127, 217}
var multiTxDataLoc = 0
