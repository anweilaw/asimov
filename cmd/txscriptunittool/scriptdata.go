// Copyright (c) 2018-2020. The asimov developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"github.com/AsimovNetwork/asimov/asiutil"
	"github.com/AsimovNetwork/asimov/common"
	"github.com/AsimovNetwork/asimov/common/address"
	"github.com/AsimovNetwork/asimov/crypto"
	"github.com/AsimovNetwork/asimov/protos"
	"github.com/AsimovNetwork/asimov/txscript"
	"os"
	"strconv"
)


func writeScriptCase(w *bytes.Buffer, mutxo *MockUTXO, sigScripts [][]byte, op string, result string, msg string) {
	w.Write([]byte("[\n"))

	w.WriteByte('"')
	for i,sigScript := range sigScripts{
		if i > 0 {
			w.WriteByte(' ')
		}
		length := int64(len(sigScript))
		hexlen := strconv.FormatInt(length, 16)
		sigScriptStr := "0x"
		if length > 75 {
			sigScriptStr = sigScriptStr + "4c"
		}
		if len(hexlen) % 2 == 1 {
			sigScriptStr = sigScriptStr + "0"
		}
		sigScriptStr = sigScriptStr + hexlen
		if length > 0 {
			sigScriptStr = sigScriptStr + " 0x" + hex.EncodeToString(sigScript)
		}
		w.Write([]byte(sigScriptStr))
	}
	w.Write([]byte("\",\n"))

	w.WriteByte('"')
	w.Write([]byte(mutxo.pkscript))
	w.Write([]byte("\",\n"))

	w.WriteByte('"')
	w.Write([]byte(op))
	w.Write([]byte("\","))

	w.WriteByte('"')
	w.Write([]byte(result))
	w.Write([]byte("\","))

	w.WriteByte('"')
	w.Write([]byte(msg))
	w.Write([]byte("\"\n"))

	w.Write([]byte("],\n"))
}

func createP2PK(privatekeys []*crypto.PrivateKey, addresses []common.IAddress,
	sigtype txscript.SigHashType, postpk string) (*MockUTXO, [][]byte) {
	signIdx := 2
	prehash := "0001000000000000000000000000000000000000000000000000000000000000"
	tx := protos.NewMsgTx(protos.TxVersion)
	mutxo := createMockUTXO(addresses[signIdx:signIdx+1], prehash, 0,
		0, "", postpk)

	outpoint := protos.OutPoint{Hash: common.HexToHash(prehash)}
	in := protos.NewTxIn(&outpoint, nil)
	tx.AddTxIn(in)

	// Output
	out := protos.NewTxOut(0, nil, asiutil.AsimovAsset)
	tx.AddTxOut(out)

	prepk, _ := parseShortForm(mutxo.pkscript)
	txhash, _ := txscript.CalcSignatureHash(prepk, sigtype, tx, 0)
	sig, _ := privatekeys[signIdx].Sign(txhash[:])
	sigbytes := sig.Serialize()
	sigs := make([]byte, len(sigbytes)+1)
	copy(sigs[:], sigbytes[:])
	sigs[len(sigbytes)] = byte(sigtype)

	return mutxo, [][]byte{sigs}
}

func createP2PKH(privatekeys []*crypto.PrivateKey, addresses []common.IAddress) (*MockUTXO, []byte) {
	signIdx := 0
	prehash := "0001000000000000000000000000000000000000000000000000000000000000"
	tx := protos.NewMsgTx(protos.TxVersion)

	mutxo := createMockUTXO(addresses[signIdx:signIdx+1], prehash, 0,
		0, "DUP HASH160", " IFLAG_EQUALVERIFY CHECKSIG")

	outpoint := protos.OutPoint{Hash: common.HexToHash(prehash)}
	in := protos.NewTxIn(&outpoint, nil)
	tx.AddTxIn(in)

	// Output
	out := protos.NewTxOut(0, nil, asiutil.AsimovAsset)
	tx.AddTxOut(out)

	hashType := txscript.SigHashAll
	prepk, _ := parseShortForm(mutxo.pkscript)
	txhash, _ := txscript.CalcSignatureHash(prepk, hashType, tx, 0)
	sig, _ := privatekeys[signIdx].Sign(txhash[:])
	sigbytes := sig.Serialize()
	sigs := make([]byte, len(sigbytes)+1)
	copy(sigs[:], sigbytes[:])
	sigs[len(sigbytes)] = byte(hashType)
	return mutxo, sigs
}

func createP2SHP2PK(privatekeys []*crypto.PrivateKey, addresses []common.IAddress) (*MockUTXO, [][]byte) {
	signIdx := 2
	prehash := "0001000000000000000000000000000000000000000000000000000000000000"
	tx := protos.NewMsgTx(protos.TxVersion)

	outpoint := protos.OutPoint{Hash: common.HexToHash(prehash)}
	in := protos.NewTxIn(&outpoint, nil)
	tx.AddTxIn(in)

	// Output
	out := protos.NewTxOut(0, nil, asiutil.AsimovAsset)
	tx.AddTxOut(out)

	hashType := txscript.SigHashAll

	buff := make([]byte, 0x23)
	buff[0] = 0x21
	copy(buff[1:], privatekeys[signIdx].PubKey().SerializeCompressed())
	buff[0x22] = txscript.OP_CHECKSIG
	txhash, _ := txscript.CalcSignatureHash(buff, hashType, tx, 0)
	sig, _ := privatekeys[signIdx].Sign(txhash[:])
	sigbytes := sig.Serialize()
	sigs := make([]byte, len(sigbytes)+1)
	copy(sigs[:], sigbytes[:])
	sigs[len(sigbytes)] = byte(hashType)


	addr, _ := common.NewAddressWithId(common.ScriptHashAddrID, common.Hash160(buff[:]))
	mutxo := &MockUTXO{
		hash:prehash,
		idx:0,
		pkscript:"HASH160 0x15 " + addr.String() + " IFLAG_EQUAL",
		amount:0,
	}
	return mutxo, [][]byte{sigs, buff}
}

func createP2SHP2PKH(privatekeys []*crypto.PrivateKey, addresses []common.IAddress) (*MockUTXO, [][]byte) {
	signIdx := 1
	prehash := "0001000000000000000000000000000000000000000000000000000000000000"
	tx := protos.NewMsgTx(protos.TxVersion)

	outpoint := protos.OutPoint{Hash: common.HexToHash(prehash)}
	in := protos.NewTxIn(&outpoint, nil)
	tx.AddTxIn(in)

	// Output
	out := protos.NewTxOut(0, nil, asiutil.AsimovAsset)
	tx.AddTxOut(out)

	hashType := txscript.SigHashAll

	buff := make([]byte, 0x1a)
	buff[0] = txscript.OP_DUP
	buff[1] = txscript.OP_HASH160
	buff[2] = 0x15 // standard address length
	copy(buff[3:], addresses[signIdx].ScriptAddress())
	buff[0x18] = txscript.OP_IFLAG_EQUALVERIFY
	buff[0x19] = txscript.OP_CHECKSIG
	txhash, _ := txscript.CalcSignatureHash(buff, hashType, tx, 0)
	sig, _ := privatekeys[signIdx].Sign(txhash[:])
	sigbytes := sig.Serialize()
	sigs := make([]byte, len(sigbytes)+1)
	copy(sigs[:], sigbytes[:])
	sigs[len(sigbytes)] = byte(hashType)

	addr, _ := common.NewAddressWithId(common.ScriptHashAddrID, common.Hash160(buff[:]))
	mutxo := &MockUTXO{
		hash:prehash,
		idx:0,
		pkscript:"HASH160 0x15 " + addr.String() + " IFLAG_EQUAL",
		amount:0,
	}
	return mutxo, [][]byte{sigs, privatekeys[signIdx].PubKey().SerializeUncompressed(), buff}
}


func createMultisign(privatekeys []*crypto.PrivateKey, addresses []common.IAddress,
	numsig int, numpbk int) (*MockUTXO, [][]byte) {
	inputValue := int64(0)
	inputIdx := 0
	signIdx := 2
	prehash := "0001000000000000000000000000000000000000000000000000000000000000"
	mutxo := createMockUTXO(addresses[signIdx:signIdx+numpbk], prehash, uint32(inputIdx), inputValue,
		strconv.FormatInt(int64(numsig), 10), " "+strconv.FormatInt(int64(numpbk), 10)+" CHECKMULTISIG")
	tx := protos.NewMsgTx(1)
	outpoint := protos.OutPoint{Hash:common.HexToHash(prehash), Index:uint32(inputIdx)}
	prepk, _ := parseShortForm(mutxo.pkscript)
	in := protos.NewTxIn(&outpoint, prepk)
	tx.AddTxIn(in)
	out := protos.NewTxOut(0, nil, asiutil.AsimovAsset)
	tx.AddTxOut(out)
	hashType := txscript.SigHashAll
	txhash, _ := txscript.CalcSignatureHash(prepk, hashType, tx, 0)

	var sigs [][]byte
	for idx := signIdx; idx < signIdx + numsig; idx++ {
		sig, _ := privatekeys[idx].Sign(txhash[:])
		sigbytes := sig.Serialize()
		sigb := make([]byte, len(sigbytes)+1)
		copy(sigb[:], sigbytes[:])
		sigb[len(sigbytes)] = byte(txscript.SigHashAll)
		sigs = append(sigs, sigb)
	}

	return mutxo, sigs
}

func createP2SH(privatekeys []*crypto.PrivateKey, addresses []common.IAddress) (*MockUTXO, [][]byte) {
	signIdx := 2
	prehash := "0001000000000000000000000000000000000000000000000000000000000000"
	tx := protos.NewMsgTx(protos.TxVersion)

	outpoint := protos.OutPoint{Hash: common.HexToHash(prehash)}
	in := protos.NewTxIn(&outpoint, nil)
	tx.AddTxIn(in)

	// Output
	out := protos.NewTxOut(0, nil, asiutil.AsimovAsset)
	tx.AddTxOut(out)

	hashType := txscript.SigHashAll

	builder := txscript.NewScriptBuilder()
	builder.AddOp(txscript.OP_2)
	for i := 0; i < 3; i++ {
		builder.AddData(privatekeys[signIdx + i].PubKey().SerializeCompressed())
	}
	builder.AddOp(txscript.OP_3)
	builder.AddOp(txscript.OP_CHECKMULTISIG)
	buff, _ := builder.Script()

	txhash, _ := txscript.CalcSignatureHash(buff, hashType, tx, 0)

	var sigs [][]byte
	for idx := signIdx; idx < signIdx + 2; idx++  {
		sig, _ := privatekeys[idx].Sign(txhash[:])
		sigbytes := sig.Serialize()
		sigb := make([]byte, len(sigbytes)+1)
		copy(sigb[:], sigbytes[:])
		sigb[len(sigbytes)] = byte(hashType)
		sigs = append(sigs, sigb)
	}

	sigs = append(sigs, buff)

	addr, _ := common.NewAddressWithId(common.ScriptHashAddrID, common.Hash160(buff[:]))
	mutxo := &MockUTXO{
		hash:prehash,
		idx:0,
		pkscript:"HASH160 0x15 " + addr.String() + " IFLAG_EQUAL",
		amount:0,
	}
	return mutxo, sigs
}

// scriptmain create script test cases in script_tests.json.
func scriptmain(privatekeys []*crypto.PrivateKey, addresses []common.IAddress) {
	w := &bytes.Buffer{}
	var (
		mutxo *MockUTXO
		sigs [][]byte
	)
	// w.Write([]byte("[\"Automatically generated test cases\"],\n"))
	// mutxo, sigs = createP2PK(privatekeys, addresses, txscript.SigHashAll, " CHECKSIG")
	// writeScriptCase(w, mutxo, [][]byte{sigs}, "", "OK","P2PK")

	// sigs[1] = sigs[1] + 1
	// writeScriptCase(w, mutxo, [][]byte{sigs}, "", "EVAL_FALSE","P2PK, bad sig")

	//mutxo, sigs = createP2PKH(privatekeys, addresses)
	//writeScriptCase(w, mutxo, [][]byte{sigs, privatekeys[0].PubKey().SerializeCompressed()}, "", "OK","P2PKH")

	// mutxo = createMockUTXO(addresses[1:2], "0001000000000000000000000000000000000000000000000000000000000000", 0,
	// 	0, "DUP HASH160", " IFLAG_EQUALVERIFY CHECKSIG")
	// writeScriptCase(w, mutxo, [][]byte{sigs, privatekeys[0].PubKey().SerializeCompressed()}, "", "EQUALVERIFY","P2PKH, bad pubkey")

	// mutxo, sigs = createP2PK(privatekeys, addresses, txscript.SigHashAnyOneCanPay | txscript.SigHashAll,
	//     " CHECKSIG")
	// writeScriptCase(w, mutxo, [][]byte{sigs}, "", "OK","anyonecanpay")

	// sigs[len(sigs) - 1] = byte(txscript.SigHashAll)
	// writeScriptCase(w, mutxo, [][]byte{sigs}, "", "EVAL_FALSE","P2PK anyonecanpay marked with normal hashtype")

	// mutxo, sigs = createP2SHP2PK(privatekeys, addresses)
	// writeScriptCase(w, mutxo, sigs, "P2SH", "OK","P2SH(P2PK)")
	// sigs[1][10] = sigs[1][10] - 1
	// writeScriptCase(w, mutxo, sigs, "P2SH", "EVAL_FALSE","P2SH(P2PK), bad redeemscript")
	//
	// mutxo, sigs = createP2SHP2PKH(privatekeys, addresses)
	// writeScriptCase(w, mutxo, sigs, "P2SH", "OK","P2SH(P2PKH)")
	// writeScriptCase(w, mutxo, [][]byte{sigs[0],sigs[2]}, "P2SH", "EQUALVERIFY","P2SH(P2PKH), bad sig")

	// mutxo, sigs = createMultisign(privatekeys, addresses, 3, 3)
	// writeScriptCase(w, mutxo, sigs, "", "OK","3-of-3")
	// sigs[2] = []byte{}
	// writeScriptCase(w, mutxo, sigs, "", "EVAL_FALSE","3-of-3, 2 sigs")

	// mutxo, sigs = createP2SH(privatekeys, addresses)
	// writeScriptCase(w, mutxo, sigs, "P2SH", "OK","P2SH(2-of-3)")
	// writeScriptCase(w, mutxo, [][]byte{sigs[0], []byte{}, sigs[2]}, "P2SH", "EVAL_FALSE","P2SH(2-of-3), 1 sig")

	// mutxo, sigs = createP2PK(privatekeys, addresses, txscript.SigHashAll, " CHECKSIG")
	// writeScriptCase(w, mutxo, sigs, "", "OK","P2PK")
	// w.Write([]byte("\n"))

	// write until r negative
	// mutxo, sigs = createMultisign(privatekeys, addresses, 2, 2)
	// writeScriptCase(w, mutxo, sigs, "STRICTENC", "SIG_DER","BIP66 example 7, with DERSIG")

	// mutxo, sigs = createP2PK(privatekeys, addresses, txscript.SigHashType(5), " CHECKSIG")
	// writeScriptCase(w, mutxo, sigs, "", "OK","P2PK NOT with hybrid pubkey but no STRICTENC")
	// w.Write([]byte("\n"))

	format := addresses[2].(*address.AddressPubKey).Format()
	addresses[2].(*address.AddressPubKey).SetFormat(address.PKFHybrid)
	// mutxo, sigs = createP2PK(privatekeys, addresses, txscript.SigHashAll, " CHECKSIG")
	// writeScriptCase(w, mutxo, sigs, "", "OK","P2PK with hybrid pubkey")
	// w.Write([]byte("\n"))

	// mutxo, sigs = createP2PK(privatekeys, addresses, txscript.SigHashAll, " CHECKSIG NOT")
	// writeScriptCase(w, mutxo, sigs, "", "EVAL_FALSE","P2PK NOT with hybrid pubkey but no STRICTENC")
	// w.Write([]byte("\n"))

	mutxo, sigs = createMultisign(privatekeys, addresses, 1, 2)
	writeScriptCase(w, mutxo, sigs, "", "OK","1-of-2 with the second 1 hybrid pubkey and no STRICTENC")
	w.Write([]byte("\n"))
	addresses[2].(*address.AddressPubKey).SetFormat(format)

	f, err := os.Create("txscript/data/temp.json")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer f.Close()
	f.Write(w.Bytes())
	f.Sync()
}
