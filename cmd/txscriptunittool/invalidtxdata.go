// Copyright (c) 2018-2020. The asimov developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"github.com/AsimovNetwork/asimov/common"
	"github.com/AsimovNetwork/asimov/crypto"
	"github.com/AsimovNetwork/asimov/protos"
	"github.com/AsimovNetwork/asimov/txscript"
	"os"
	"strconv"
)

func varifyInvalidCase(tx *protos.MsgTx, prevOuts map[protos.OutPoint]scriptWithInputVal) {
	for k, txin := range tx.TxIn {
		prevOut, ok := prevOuts[txin.PreviousOutPoint]
		if !ok {
			fmt.Printf("bad test (missing %dth input) :%v\n",
				k, tx)
			return
		}
		vm, err := txscript.NewEngine(prevOut.pkScript, tx, k,
			txscript.ScriptBip16 | txscript.ScriptVerifyStrictEncoding | txscript.ScriptVerifyCleanStack,
			prevOut.inputVal, &tx.TxOut[0].Asset, 100)
		if err != nil {
			fmt.Println("OK" + err.Error())
			return
		}

		err = vm.Execute()
		if err != nil {
			fmt.Println("OK" + err.Error())
			return
		}
	}
	fmt.Println("Expect fail!")
}

func createExtraJunkEnd(w *bytes.Buffer, addresses []common.IAddress, 
	privatekeys []*crypto.PrivateKey) {
	prevOuts := make(map[protos.OutPoint]scriptWithInputVal)
	mutxos := make([]*MockUTXO, 0)
	inputValue := int64(0)
	idxs := []uint32{0,1,}
	signIdx := 2
	prehashes := []string{"27587a10248001f424ad94bb55cd6cd6086a0e05767173bdbdf647187beca76c",}

	tx := protos.NewMsgTx(1)

	for i := 0; i < 1; i++ {
		mutxo := createMockUTXO(addresses[signIdx:signIdx+1], prehashes[i], idxs[i], inputValue,
			"", " CHECKSIG VERIFY 1")
		mutxos = append(mutxos, mutxo)

		outpoint := protos.OutPoint{Hash: common.HexToHash(prehashes[i]), Index:idxs[i]}
		in := protos.NewTxIn(&outpoint, nil)
		tx.AddTxIn(in)
	}

	// Output
	for i := 0; i < 1; i++ {
		pkscript, _ := txscript.NewScriptBuilder().AddData([]byte{0x51}).Script()
		out := protos.NewTxOut(1, pkscript, assetId)
		tx.AddTxOut(out)
	}

	for i := 0; i < 1; i++ {
		hashType := txscript.SigHashAll
		prepk, _ := parseShortForm(mutxos[i].pkscript)
		v := scriptWithInputVal{
			inputVal: inputValue,
			pkScript: prepk,
		}
		prevOuts[tx.TxIn[i].PreviousOutPoint] = v

		txhash,_ := txscript.CalcSignatureHash(prepk[:len(prepk)-2], hashType, tx, i)
		sig, _ := privatekeys[signIdx].Sign(txhash[:])
		sigbytes := sig.Serialize()
		sigs := make([]byte, len(sigbytes)+1)
		copy(sigs[:], sigbytes[:])
		sigs[len(sigbytes)] = byte(hashType)
		sigscript, _ := txscript.NewScriptBuilder().AddData(sigs).Script()
		tx.TxIn[i].SignatureScript = sigscript
	}

	writeCase(w, mutxos, tx, "P2SH")
	varifyInvalidCase(tx, prevOuts)
}

func createDupScriptPubkey(w *bytes.Buffer, addresses []common.IAddress, 
	privatekeys []*crypto.PrivateKey, sigPUSHDATA bool, hashType txscript.SigHashType) {
	prevOuts := make(map[protos.OutPoint]scriptWithInputVal)
	mutxos := make([]*MockUTXO, 0)
	inputValue := int64(0)
	idxs := []uint32{0,1,}
	signIdx := CPKHASH
	prehashes := []string{"0001000000000000000000000000000000000000000000000000000000000000",}

	tx := protos.NewMsgTx(1)
	for i := 0; i < 1; i++ {
		mutxo := createMockUTXO(addresses[signIdx:signIdx+1], prehashes[i], idxs[i], inputValue,
			"DUP HASH160", " IFLAG_EQUALVERIFY CHECKSIGVERIFY 1 ")
		mutxos = append(mutxos, mutxo)

		outpoint := protos.OutPoint{Hash: common.HexToHash(prehashes[i]), Index:idxs[i]}
		in := protos.NewTxIn(&outpoint, nil)
		tx.AddTxIn(in)
	}

	// Output
	for i := 0; i < 1; i++ {
		pkscript, _ := txscript.NewScriptBuilder().AddData([]byte{0x51}).Script()
		out := protos.NewTxOut(1, pkscript, assetId)
		tx.AddTxOut(out)
	}

	for i := 0; i < 1; i++ {
		prepk, _ := parseShortForm(mutxos[i].pkscript)

		txhash,_ := txscript.CalcSignatureHash(prepk[:len(prepk)-2], hashType, tx, i)
		sig, _ := privatekeys[signIdx].Sign(txhash[:])
		sigbytes := sig.Serialize()
		sigbuflength := len(sigbytes)+1
		if sigPUSHDATA {
			sigbuflength ++
		}
		sigs := make([]byte, sigbuflength)
		if sigPUSHDATA {
			sigs[0] = byte(sigbuflength)
			copy(sigs[1:], sigbytes[:])
		} else {
			copy(sigs[:], sigbytes[:])
		}
		sigs[sigbuflength - 1] = byte(hashType)
		builder := txscript.NewScriptBuilder()
		if sigPUSHDATA {
			builder.AddOp(txscript.OP_PUSHDATA1)
		}
		sigscript, _ := builder.AddData(sigs).
			AddData(privatekeys[signIdx].PubKey().SerializeCompressed()).Script()
		tx.TxIn[i].SignatureScript = sigscript
	}

	if !sigPUSHDATA {
		pubkey := hex.EncodeToString(privatekeys[signIdx].PubKey().SerializeUncompressed())
		mutxos[0].pkscript = mutxos[0].pkscript + " 0x4c 0x" + strconv.FormatInt(int64(len(pubkey)+1)/2, 16) +
			" 0x" + pubkey
	}
	prepk, _ := parseShortForm(mutxos[0].pkscript)
	v := scriptWithInputVal{
		inputVal: inputValue,
		pkScript: prepk,
	}
	prevOuts[tx.TxIn[0].PreviousOutPoint] = v

	writeCase(w, mutxos, tx, "P2SH")
	varifyInvalidCase(tx, prevOuts)
}


func createInvalidTx(w *bytes.Buffer, addresses []common.IAddress,
	privatekeys []*crypto.PrivateKey) {
	prevOuts := make(map[protos.OutPoint]scriptWithInputVal)
	mutxos := make([]*MockUTXO, 0)
	inputValue := int64(0)
	idxs := []uint32{0,1,}
	signIdx := 0
	prehashes := []string{"0001000000000000000000000000000000000000000000000000000000000000",}
/**

["An invalid P2SH Transaction"],
[[["0000000000000000000000000000000000000000000000000000000000000100", 0,
"HASH160 0x14 0x7a052c840ba73af26755de42cf01cc9e0a49fef0 EQUAL"]],
"
01000000
01
0001000000000000000000000000000000000000000000000000000000000000
00000000
09
085768617420697320
 */
	tx := protos.NewMsgTx(1)
	for i := 0; i < 1; i++ {
		mutxo := createMockUTXO(addresses[signIdx:signIdx+1], prehashes[i], idxs[i], inputValue,
			"HASH160 ", " IFLAG_EQUAL")
		mutxos = append(mutxos, mutxo)

		outpoint := protos.OutPoint{Hash: common.HexToHash(prehashes[i]), Index:idxs[i]}
		in := protos.NewTxIn(&outpoint, nil)
		tx.AddTxIn(in)
	}

	// Output
	for i := 0; i < 1; i++ {
		pkscript, _ := txscript.NewScriptBuilder().AddData([]byte{0x51}).Script()
		out := protos.NewTxOut(1, pkscript, assetId)
		tx.AddTxOut(out)
	}

	for i := 0; i < 1; i++ {
		prepk, _ := parseShortForm(mutxos[i].pkscript)
		v := scriptWithInputVal{
			inputVal: inputValue,
			pkScript: prepk,
		}
		prevOuts[tx.TxIn[i].PreviousOutPoint] = v

		sigscript, _ := txscript.NewScriptBuilder().AddData([]byte{0x08, 0x57, 0x68, 0x61, 0x74, 0x20, 0x69, 0x73, 0x20}).Script()
		tx.TxIn[i].SignatureScript = sigscript
	}

	writeCase(w, mutxos, tx, "P2SH")
	varifyInvalidCase(tx, prevOuts)
}

func createinvalidMaxOutput(w *bytes.Buffer, addresses []common.IAddress, privatekeys []*crypto.PrivateKey,
	outvalues []int64) {
	tx, prevOuts := createbyOutValue(w, addresses, privatekeys, outvalues)
	varifyInvalidCase(tx, prevOuts)
}


func createSigAll_SigAnyOneCanPay(w *bytes.Buffer, addresses []common.IAddress,
	privatekeys []*crypto.PrivateKey) {
	prevOuts := make(map[protos.OutPoint]scriptWithInputVal)
	mutxos := make([]*MockUTXO, 0)
	inputValue := int64(0)
	idxs := []uint32{0,1,}
	signIdx := 2
	prehashes := []string{"0001000000000000000000000000000000000000000000000000000000000000",
		"0002000000000000000000000000000000000000000000000000000000000000",}

	tx := protos.NewMsgTx(1)
	for i := 0; i < 2; i++ {
		mutxo := createMockUTXO(addresses[signIdx:signIdx+1], prehashes[i], idxs[i], inputValue,
			"", " CHECKSIG")
		mutxos = append(mutxos, mutxo)

		outpoint := protos.OutPoint{Hash: common.HexToHash(prehashes[i]), Index:idxs[i]}
		in := protos.NewTxIn(&outpoint, nil)
		tx.AddTxIn(in)
	}

	// Output
	for i := 0; i < 1; i++ {
		pkscript, _ := txscript.NewScriptBuilder().AddData([]byte{0x51}).Script()
		out := protos.NewTxOut(1, pkscript, assetId)
		tx.AddTxOut(out)
	}

	// first sig - SigHashAll
	// sencond sig - SigHashAll | SIGHASH_ANYONECANPAY
	for i := 0; i < 2; i++ {
		hashType := txscript.SigHashAll
		if i > 0 {
			hashType = hashType | txscript.SigHashAnyOneCanPay
			tx.TxIn[i].Sequence = 1
		}
		prepk, _ := parseShortForm(mutxos[i].pkscript)
		txhash,_ := txscript.CalcSignatureHash(prepk, hashType, tx, i)
		sig, _ := privatekeys[signIdx].Sign(txhash[:])
		sigbytes := sig.Serialize()
		sigs := make([]byte, len(sigbytes)+1)
		copy(sigs[:], sigbytes[:])
		sigs[len(sigbytes)] = byte(hashType)
		sigscript, _ := txscript.NewScriptBuilder().AddData(sigs).Script()
		tx.TxIn[i].SignatureScript = sigscript

		v := scriptWithInputVal{
			inputVal: inputValue,
			pkScript: prepk,
		}
		prevOuts[tx.TxIn[i].PreviousOutPoint] = v
	}

	writeCase(w, mutxos, tx, "P2SH")
	varifyInvalidCase(tx, prevOuts)
}

func createMultiSigWithDummyValue(w *bytes.Buffer, addresses []common.IAddress,
	privatekeys []*crypto.PrivateKey) {
	prevOuts := make(map[protos.OutPoint]scriptWithInputVal)
	mutxos := make([]*MockUTXO, 0)
	inputValue := int64(0)
	inputIdx := 0
	signIdx := 2
	prehash := "b14bdcbc3e01bdaad36cc08e81e69c82e1060bc14e518db2b49aa43ad90ba260"
	mutxo := createMockUTXO(addresses[signIdx:signIdx+2], prehash, uint32(inputIdx), inputValue,
		"1", " 2 OP_CHECKMULTISIG")

	tx := protos.NewMsgTx(1)
	outpoint := protos.OutPoint{Hash:common.HexToHash(prehash), Index:uint32(inputIdx)}
	prepk, _ := parseShortForm(mutxo.pkscript)
	in := protos.NewTxIn(&outpoint, prepk)
	tx.AddTxIn(in)
	pkscript, _ := txscript.NewScriptBuilder().AddData([]byte{0x51}).Script()
	out := protos.NewTxOut(inputValue, pkscript, assetId)
	tx.AddTxOut(out)
	mutxos = append(mutxos, mutxo)
	wbuf := bytes.NewBuffer(make([]byte, 0, tx.SerializeSize()+4))
	tx.Serialize(wbuf)
	binary.Write(wbuf, binary.LittleEndian, txscript.SigHashAll)
	txhash := common.DoubleHashH(wbuf.Bytes())
	sig, _ := privatekeys[signIdx].Sign(txhash[:])
	sigbytes := sig.Serialize()
	sigs := make([]byte, len(sigbytes) + 1)
	copy(sigs[:], sigbytes[:])
	sigs[len(sigbytes)] = byte(txscript.SigHashAll)
	sigscript, _ := txscript.NewScriptBuilder().AddOp(txscript.OP_0).AddData(sigs).Script()
	tx.TxIn[0].SignatureScript = sigscript

	writeCase(w, mutxos, tx, "P2SH,CLEANSTACK")

	v := scriptWithInputVal{
		inputVal: inputValue,
		pkScript: prepk,
	}
	prevOuts[outpoint] = v
	varifyInvalidCase(tx, prevOuts)
}


func createUnexpectedCODESEPARATOR(w *bytes.Buffer, addresses []common.IAddress,
	privatekeys []*crypto.PrivateKey, ifExcute bool) {
	prevOuts := make(map[protos.OutPoint]scriptWithInputVal)
	mutxos := make([]*MockUTXO, 0)
	inputValue := int64(0)
	idxs := []uint32{0,1,}
	signIdx := 2
	prehashes := []string{"44490eda355be7480f2ec828dcc1b9903793a8008fad8cfe9b0c6b4d2f0355a9",}

	tx := protos.NewMsgTx(1)

	for i := 0; i < 1; i++ {
		mutxo := createMockUTXO(addresses[signIdx:signIdx+1], prehashes[i], idxs[i],
			0, "IF CODESEPARATOR ENDIF", " CHECKSIGVERIFY CODESEPARATOR 1")
		mutxos = append(mutxos, mutxo)

		outpoint := protos.OutPoint{Hash: common.HexToHash(prehashes[i]), Index:idxs[i]}
		in := protos.NewTxIn(&outpoint, nil)
		tx.AddTxIn(in)
	}

	// Output
	for i := 0; i < 1; i++ {
		pkscript, _ := txscript.NewScriptBuilder().AddData([]byte{0x51}).Script()
		out := protos.NewTxOut(0, pkscript, assetId)
		tx.AddTxOut(out)
	}

	builder := txscript.NewScriptBuilder()
	for i := 0; i < 1; i++ {
		hashType := txscript.SigHashAll
		prepk, _ := parseShortForm(mutxos[i].pkscript)

		v := scriptWithInputVal{
			inputVal: inputValue,
			pkScript: prepk,
		}
		if ifExcute {
			prepk = prepk[2:]
		}
		txhash,_ := txscript.CalcSignatureHash(prepk, hashType, tx, i)
		sig, _ := privatekeys[signIdx].Sign(txhash[:])
		sigbytes := sig.Serialize()
		sigs := make([]byte, len(sigbytes)+1)
		copy(sigs[:], sigbytes[:])
		sigs[len(sigbytes)] = byte(hashType)
		builder.AddData(sigs)
		if ifExcute {
			builder.AddOp(txscript.OP_0)
		} else {
			builder.AddOp(txscript.OP_1)
		}
		sigscript, _ := builder.Script()
		tx.TxIn[i].SignatureScript = sigscript

		prevOuts[tx.TxIn[i].PreviousOutPoint] = v
	}

	writeCase(w, mutxos, tx, "P2SH")
	varifyInvalidCase(tx, prevOuts)
}

func createMultySig(w *bytes.Buffer, addresses []common.IAddress,
	privatekeys []*crypto.PrivateKey) {
	prevOuts := make(map[protos.OutPoint]scriptWithInputVal)
	mutxos := make([]*MockUTXO, 0)
	prehash := "b14bdcbc3e01bdaad36cc08e81e69c82e1060bc14e518db2b49aa43ad90ba260"
	mutxo := MockUTXO{
		hash:prehash,
		idx:0,
		pkscript:"HASH160 0x15 0x73b1ce99298d5f07364b57b1e5c9cc00be0b04a954 IFLAG_EQUAL",
		amount:0,
	}
	tx := protos.NewMsgTx(1)
	outpoint := protos.OutPoint{Hash:common.HexToHash(prehash), Index:0}
	prepk, _ := parseShortForm(mutxo.pkscript)
	in := protos.NewTxIn(&outpoint, prepk)
	tx.AddTxIn(in)
	pkscript, _ := txscript.NewScriptBuilder().AddData([]byte{0x51}).Script()
	out := protos.NewTxOut(0, pkscript, assetId)
	tx.AddTxOut(out)
	mutxos = append(mutxos, &mutxo)
	b, _ := hex.DecodeString("000048304502207aacee820e08b0b174e248abd8d7a34ed63b5da3abedb99934df9fddd65c05c4022100dfe87896ab5ee3df476c2655f9fbe5bd089dccbef3" +
	"e4ea05b5d121169fe7f5f401483045022100f6649b0eddfdfd4ad55426663385090d51ee86c3481bdc6b0c18ea6c0ece2c0b0220561c315b07cffa6f7dd9df96" +
	"dbae9200c2dee09bf93cc35ca05e6cdf613340aa014c695221031d11db38972b712a9fe1fc023577c7ae3ddb4a3004187d41c45121eecfdbb5b7210207ec3691" +
	"1b6ad2382860d32989c7b8728e9489d7bbc94a6b5509ef0029be128821024ea9fac06f666a4adc3fc1357b7bec1fd0bdece2b9d08579226a8ebde53058e453ae")
	sigscript, _ := txscript.NewScriptBuilder().AddData(b).Script()
	tx.TxIn[0].SignatureScript = sigscript

	writeCase(w, mutxos, tx, "P2SH")

	v := scriptWithInputVal{
		inputVal: 0,
		pkScript: prepk,
	}
	prevOuts[outpoint] = v
	varifyInvalidCase(tx, prevOuts)
}

// invalidmain create valid tx test cases in tx_valid.json.
func invalidmain(privatekeys []*crypto.PrivateKey, addresses []common.IAddress) {
	w := bytes.Buffer{}
	w.Write([]byte("[\n"))
	w.Write([]byte("[\"The following are deserialized transactions which are invalid.\"],\n"))
	w.Write([]byte("[\"They are in the form\"],\n"))
	w.Write([]byte("[\"[[[prevout hash, prevout index, prevout scriptPubKey, amount?], [input 2], ...],\"],\n"))
	w.Write([]byte("[\"serializedTransaction, verifyFlags]\"],\n"))
	w.Write([]byte("[\"Objects that are only a single string (like this one) are ignored\"],\n"))

	w.Write([]byte("[\"With extra junk appended to the end of the scriptPubKey\"],\n"))
	createExtraJunkEnd(&w, addresses, privatekeys)
	w.Write([]byte(",\n"))

	w.Write([]byte("[\"This is a nearly-standard transaction with CHECKSIGVERIFY 1 instead of CHECKSIG\"],\n"))
	w.Write([]byte("[\"but with the signature duplicated in the scriptPubKey with a non-standard pushdata prefix\"],\n"))
	w.Write([]byte("[\"See FindAndDelete, which will only remove if it uses the same pushdata prefix as is standard\"],\n"))
	createDupScriptPubkey(&w, addresses, privatekeys, false, txscript.SigHashAll)
	w.Write([]byte(",\n"))

	w.Write([]byte("[\"Same as above, but with the sig in the scriptSig also pushed with the same non-standard OP_PUSHDATA\"],\n"))
	createDupScriptPubkey(&w, addresses, privatekeys, true, txscript.SigHashAll)
	w.Write([]byte(",\n"))

	w.Write([]byte("[\"This is the nearly-standard transaction with CHECKSIGVERIFY 1 instead of CHECKSIG\"],\n"))
	w.Write([]byte("[\"but with the signature duplicated in the scriptPubKey with a different hashtype suffix\"],\n"))
	w.Write([]byte("[\"See FindAndDelete, which will only remove if the signature, including the hash type, matches\"],\n"))
	createDupScriptPubkey(&w, addresses, privatekeys, false, txscript.SigHashSingle)
	w.Write([]byte(",\n"))

	w.Write([]byte("[\"An invalid P2SH Transaction\"],\n"))
	createInvalidTx(&w, addresses, privatekeys)
	w.Write([]byte(",\n"))

	// Not varify in vm engine
	// w.Write([]byte("[\"Tests for CheckTransaction()\"],\n"))
	// w.Write([]byte("[\"No outputs\"],\n"))
	// createinvalidMaxOutput(&w, addresses, privatekeys, []int64{})
	// w.Write([]byte(",\n"))
	// w.Write([]byte("[\"MAX_MONEY + 1output\"],\n"))
	// createinvalidMaxOutput(&w, addresses, privatekeys, []int64{0x000775f05a074001})
	// w.Write([]byte(",\n"))
	// w.Write([]byte("[\"MAX_MONEY output + 1 output\"],\n"))
	// createinvalidMaxOutput(&w, addresses, privatekeys, []int64{0x000775f05a074000, 1})
	// w.Write([]byte(",\n"))

	w.Write([]byte("[\"Simple transaction with first input is signed with SIGHASH_ALL, second with SIGHASH_ANYONECANPAY\"],\n"))
    w.Write([]byte("[\", but we set the _ANYONECANPAY sequence number, invalidating the SIGHASH_ALL signature\"],\n"))
	createSigAll_SigAnyOneCanPay(&w, addresses, privatekeys)
	w.Write([]byte(",\n"))

	w.Write([]byte("[\"It is an OP_CHECKMULTISIG with a dummy value\"],\n"))
	createMultiSigWithDummyValue(&w, addresses, privatekeys)
	w.Write([]byte(",\n"))

	/*
	disable non standard opcode.
	// w.Write([]byte("[\"Inverted versions of tx_valid CODESEPARATOR IF block tests\"],\n"))
	// w.Write([]byte("[\"CODESEPARATOR in an unexecuted IF block does not change what is hashed\"],\n"))
	// createUnexpectedCODESEPARATOR(&w, addresses, privatekeys, false)
	// w.Write([]byte(",\n"))
	// w.Write([]byte("[\"As above, with the IF block executed\"],\n"))
	// createUnexpectedCODESEPARATOR(&w, addresses, privatekeys, true)
	// w.Write([]byte(",\n"))
	*/

	w.Write([]byte("[\"CHECKMULTISIG with incorrect signature order\"],\n"))
	w.Write([]byte("[\"Note the input is just required to make the tester happy\"],\n"))
	createMultySig(&w, addresses, privatekeys)
	w.Write([]byte(",\n"))

	w.Write([]byte("[\"Make diffs cleaner by leaving a comment here without comma at the end\"]\n]\n"))

	f, err := os.Create("txscript/data/tx_invalid.json")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer f.Close()
	f.Write(w.Bytes())
	f.Sync()
}