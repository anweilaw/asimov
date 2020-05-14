// Copyright (c) 2018-2020. The asimov developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/AsimovNetwork/asimov/asiutil"
	"github.com/AsimovNetwork/asimov/common"
	"github.com/AsimovNetwork/asimov/common/hexutil"
	"github.com/AsimovNetwork/asimov/common/serialization"
	"github.com/AsimovNetwork/asimov/crypto"
	"github.com/AsimovNetwork/asimov/protos"
	"github.com/AsimovNetwork/asimov/txscript"
	"io"
	"io/ioutil"
	"os"
	"reflect"
	"strconv"
	"unsafe"
)

func myformat(tests []interface{}) string {
	s := "["
	for i, test := range tests {
		if i > 0 {
			s = s + ", "
		}
		if inputs, ok := test.([]interface{}); ok {
			s = s + myformat(inputs)
		} else if input, ok := test.(string); ok {
			s = s + "\"" + input + "\""
		} else if input, ok := test.(int); ok {
			s = s + strconv.Itoa(input)
		}  else if input, ok := test.(float64); ok {
			s = s + strconv.FormatFloat(input, 'f', -1, 64)
		} else {
			fmt.Println("Unknown format ", reflect.TypeOf(test).String(), tests)
		}
	}

	s = s + "]"
	return s
}

// messageError creates an error for the given function and description.
func messageError(f string, desc string) *protos.MessageError {
	return &protos.MessageError{Func: f, Description: desc}
}

// readOutPoint reads the next sequence of bytes from r as an OutPoint.
func readOutPoint(r io.Reader, op *protos.OutPoint) error {
	_, err := io.ReadFull(r, op.Hash[:])
	if err != nil {
		return err
	}

	return serialization.ReadUint32(r, &op.Index)
}

func readScript(r io.Reader, pver uint32, maxAllowed uint32, fieldName string) ([]byte, error) {
	count, err := serialization.ReadVarInt(r, pver)
	if err != nil {
		return nil, err
	}

	// Prevent byte array larger than the max message size.  It would
	// be possible to cause memory exhaustion and panics without a sane
	// upper bound on this count.
	if count > uint64(maxAllowed) {
		str := fmt.Sprintf("%s is larger than the max allowed size "+
			"[count %d, max %d]", fieldName, count, maxAllowed)
		return nil, messageError("readScript", str)
	}

	b := make([]byte, count)
	_, err = io.ReadFull(r, b)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// readTxIn reads the next sequence of bytes from r as a transaction input
// (TxIn).
func readTxIn(r io.Reader, pver uint32, ti *protos.TxIn) error {
	err := readOutPoint(r, &ti.PreviousOutPoint)
	if err != nil {
		return err
	}

	ti.SignatureScript, err = readScript(r, pver, protos.MaxMessagePayload,
		"transaction input signature script")
	if err != nil {
		return err
	}

	err = serialization.ReadUint32(r, &ti.Sequence)
	if err != nil {
		return err
	}

	return nil
}

// readTxOut reads the next sequence of bytes from r as a transaction output
// (TxOut).
func readTxOut(r io.Reader, pver uint32, to *protos.TxOut) error {
	err := serialization.ReadUint64(r, (*uint64)(unsafe.Pointer(&to.Value)))
	if err != nil {
		return err
	}

	to.PkScript, err = readScript(r, pver, protos.MaxMessagePayload,
		"transaction output public key script")
	return err
}

func oldDeserialize(r io.Reader) (*protos.MsgTx, error) {
	var msg protos.MsgTx
	err := serialization.ReadUint32(r, &msg.Version)
	if err != nil {
		return nil, err
	}

	count, err := serialization.ReadVarInt(r, 0)
	if err != nil {
		return nil, err
	}

	// Ignore witness cases
	if count == 0 {
		return nil, nil
	}

	// Deserialize the inputs.
	txIns := make([]protos.TxIn, count)
	msg.TxIn = make([]*protos.TxIn, count)
	for i := uint64(0); i < count; i++ {
		// The pointer is set now in case a script buffer is borrowed
		// and needs to be returned to the pool on error.
		ti := &txIns[i]
		msg.TxIn[i] = ti
		err = readTxIn(r, 0, ti)
		if err != nil {
			return nil, err
		}
	}

	count, err = serialization.ReadVarInt(r, 0)
	if err != nil {
		return nil, err
	}

	// Deserialize the outputs.
	txOuts := make([]protos.TxOut, count)
	msg.TxOut = make([]*protos.TxOut, count)
	for i := uint64(0); i < count; i++ {
		// The pointer is set now in case a script buffer is borrowed
		// and needs to be returned to the pool on error.
		to := &txOuts[i]
		msg.TxOut[i] = to
		err = readTxOut(r, 0, to)
		if err != nil {
			return nil, err
		}
		to.Asset = asiutil.AsimovAsset
	}

	err = serialization.ReadUint32(r, &msg.LockTime)
	if err != nil {
		return nil, err
	}

	return &msg, nil
}

// createSpendTx generates a basic spending transaction given the passed
// signature, witness and public key scripts.
func createSpendingTx(sigScript, pkScript []byte,
	outputValue int64) *protos.MsgTx {

	coinbaseTx := protos.NewMsgTx(protos.TxVersion)

	outPoint := protos.NewOutPoint(&common.Hash{}, ^uint32(0))
	txIn := protos.NewTxIn(outPoint, []byte{txscript.OP_0, txscript.OP_0})
	txOut := protos.NewTxOut(outputValue, pkScript, protos.Asset{})
	coinbaseTx.AddTxIn(txIn)
	coinbaseTx.AddTxOut(txOut)

	spendingTx := protos.NewMsgTx(protos.TxVersion)
	coinbaseTxSha := coinbaseTx.TxHash()
	outPoint = protos.NewOutPoint(&coinbaseTxSha, 0)
	txIn = protos.NewTxIn(outPoint, sigScript)
	txOut = protos.NewTxOut(outputValue, nil, protos.Asset{})

	spendingTx.AddTxIn(txIn)
	spendingTx.AddTxOut(txOut)

	return spendingTx
}

// ConvertCalcSignatureHash runs the Bitcoin Core signature hash calculation tests
// in sighash.json.
// https://github.com/bitcoin/bitcoin/blob/master/src/test/data/sighash.json
func ConvertCalcSignatureHash(w *bytes.Buffer) {
	file, err := ioutil.ReadFile("txscript/data/sighash.json")
	if err != nil {
		fmt.Println("TestCalcSignatureHash", err)
	}

	var tests [][]interface{}
	err = json.Unmarshal(file, &tests)
	if err != nil {
		fmt.Println("TestCalcSignatureHash couldn't Unmarshal:", err)
	}

	for i, test := range tests {
		if i == 0 {
			w.WriteString(myformat(test))
			// Skip first line -- contains comments only.
			continue
		}
		if len(test) != 5 {
			fmt.Println("TestCalcSignatureHash: Test", i, " has wrong length.")
		}
		rawTx, _ := hex.DecodeString(test[0].(string))
		tx, err := oldDeserialize(bytes.NewReader(rawTx))
		if err != nil {
			fmt.Println("TestCalcSignatureHash failed test : ", i,
				"Failed to parse transaction:", err)
			continue
		}

		subScript, _ := hex.DecodeString(test[1].(string))
		_, err = txscript.GetParseScript(subScript)
		if err != nil {
			fmt.Println("TestCalcSignatureHash failed test: ", i,
				"Failed to parse sub-script: ", err)
			continue
		}

		hashType := txscript.SigHashType(uint32(int32(test[3].(float64))))
		hash, _ := txscript.CalcSignatureHash(subScript, hashType, tx,
			int(test[2].(float64)))

		txw := bytes.Buffer{}
		tx.Serialize(&txw)
		txbytes := txw.Bytes()
		test[0] = hexutil.Encode(txbytes)[2:]
		test[4] = hex.EncodeToString(hash)
		expectedHash := common.HexToHash(test[4].(string))
		if !bytes.Equal(hash, expectedHash[:]) {
			fmt.Println("TestCalcSignatureHash failed test : ", i,
				"Signature hash mismatch.")
		}
		w.WriteString(",\n" +myformat(test))
		continue
	}
}

// signhashmain create valid tx test cases in tx_valid.json.
func signhashmain(privatekeys []*crypto.PrivateKey, addresses []common.IAddress) {
	w := bytes.Buffer{}
	w.Write([]byte("[\n"))
	ConvertCalcSignatureHash(&w)

	w.Write([]byte("\n]\n"))

	f, err := os.Create("txscript/data/sighashnew.json")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer f.Close()
	f.Write(w.Bytes())
	f.Sync()
}
