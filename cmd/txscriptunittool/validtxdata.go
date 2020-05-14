// Copyright (c) 2018-2020. The asimov developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"fmt"
	"github.com/AsimovNetwork/asimov/common"
	"github.com/AsimovNetwork/asimov/common/hexutil"
	"github.com/AsimovNetwork/asimov/crypto"
	"github.com/AsimovNetwork/asimov/protos"
	"github.com/AsimovNetwork/asimov/txscript"
	"os"
	"strconv"
	"strings"
)

func parseShortFormStartFromCODESEPARATOR(script string) ([]byte, error) {
	// Split only does one separator so convert all \n and tab into  space.
	script = strings.Replace(script, "\n", " ", -1)
	script = strings.Replace(script, "\t", " ", -1)
	tokens := strings.Split(script, " ")
	builder := txscript.NewScriptBuilder()

	for _, tok := range tokens {
		if len(tok) == 0 {
			continue
		}
		// if parses as a plain number
		if num, err := strconv.ParseInt(tok, 10, 64); err == nil {
			builder.AddInt64(num)
			continue
		} else if bts, err := parseHex(tok); err == nil {
			// Concatenate the bytes manually since the test code
			// intentionally creates scripts that are too large and
			// would cause the builder to error otherwise.
			builder.AddOps(bts)
		} else if len(tok) >= 2 &&
			tok[0] == '\'' && tok[len(tok)-1] == '\'' {
			builder.AddFullData([]byte(tok[1 : len(tok)-1]))
		} else if opcode, ok := shortFormOps[tok]; ok {
			builder.AddOp(opcode)
		} else {
			return nil, fmt.Errorf("bad token %q", tok)
		}
	}
	return builder.Script()
}

func writeCase(w *bytes.Buffer, mutxos []*MockUTXO, tx *protos.MsgTx, verifyFlags string) {
	w.WriteByte('[')

	w.WriteByte('[')
    for i, mutxo := range mutxos {
    	if i > 0 {
    		w.WriteByte(',')
		}
		w.WriteByte('[')
		w.WriteByte('"')
    	w.Write([]byte(mutxo.hash))
		w.WriteByte('"')
		w.WriteByte(',')
    	w.Write([]byte(strconv.Itoa(int(int32(mutxo.idx)))))
		w.WriteByte(',')
		w.WriteByte('"')
		w.Write([]byte(mutxo.pkscript))
		w.WriteByte('"')
    	if mutxo.amount > 0 {
			w.WriteByte(',')
			w.Write([]byte(strconv.Itoa(int(mutxo.idx))))
		}
		w.WriteByte(']')
	}
	w.WriteByte(']')
	w.WriteByte(',')

	w.WriteByte('"')
	txw := bytes.Buffer{}
	tx.Serialize(&txw)
	txbytes := txw.Bytes()
	w.WriteString(hexutil.Encode(txbytes)[2:])
	w.WriteByte('"')
	w.WriteByte(',')

	w.WriteByte('"')
	w.Write([]byte(verifyFlags))
	w.WriteByte('"')

	w.WriteByte(']')
}

func varifyCase(tx *protos.MsgTx, prevOuts map[protos.OutPoint]scriptWithInputVal) {
	for k, txin := range tx.TxIn {
		prevOut, ok := prevOuts[txin.PreviousOutPoint]
		if !ok {
			fmt.Printf("bad test (missing %dth input) :%v\n",
				k, tx)
			return
		}
		vm, err := txscript.NewEngine(prevOut.pkScript, tx, k,
			txscript.ScriptBip16 | txscript.ScriptVerifyStrictEncoding, prevOut.inputVal,
			&tx.TxOut[0].Asset, 100)
		if err != nil {
			fmt.Printf("test (%v:%d) failed to create "+
				"script: %v\n", tx, k, err)
			return
		}

		err = vm.Execute()
		if err != nil {
			fmt.Printf("test (%v:%d) failed to execute: "+
				"%v\n", tx, k, err)
			return
		}
	}
	fmt.Println("Success!")
}

func joinAddress(addresses []common.IAddress) string {
	var script string
	for _, address := range addresses {
		pubk := address.String()
		script = script + " 0x" + strconv.FormatInt(int64(len(pubk)+1)/2-1, 16) + " " + pubk
	}
	return script
}

func createMockUTXO(addresses []common.IAddress, prehash string, idx uint32, amount int64,
	prepk string, postpk string) *MockUTXO {
	mutxo := MockUTXO{
		hash:prehash,
		idx:idx,
		amount:amount,
	}
	mutxo.pkscript = prepk + joinAddress(addresses) + postpk
	return &mutxo
}

func createStandardMultisign(w *bytes.Buffer, addresses []common.IAddress, 
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
	txhash := calcSignatureHash(prepk, txscript.SigHashAll, tx, 0)
	sig, _ := privatekeys[signIdx].Sign(txhash[:])
	sigbytes := sig.Serialize()
	sigs := make([]byte, len(sigbytes) + 1)
	copy(sigs[:], sigbytes[:])
	sigs[len(sigbytes)] = byte(txscript.SigHashAll)
	sigscript, _ := txscript.NewScriptBuilder().AddData(sigs).Script()
	tx.TxIn[0].SignatureScript = sigscript

	writeCase(w, mutxos, tx, "P2SH")

	v := scriptWithInputVal{
		inputVal: inputValue,
		pkScript: prepk,
	}
	prevOuts[outpoint] = v
	varifyCase(tx, prevOuts)
}

func createCHECKSIGVERIFY_1(w *bytes.Buffer, addresses []common.IAddress, 
	privatekeys []*crypto.PrivateKey) {
	prevOuts := make(map[protos.OutPoint]scriptWithInputVal)
	mutxos := make([]*MockUTXO, 0)
	inputValue := int64(0)
	inputIdx := 0
	signIdx := 0
	prehash := "0001000000000000000000000000000000000000000000000000000000000000"
	mutxo := createMockUTXO(addresses[signIdx:signIdx+1], prehash, uint32(inputIdx), inputValue,
		"DUP HASH160", " OP_IFLAG_EQUALVERIFY OP_CHECKSIGVERIFY 1")
	mutxos = append(mutxos, mutxo)
	tx := protos.NewMsgTx(1)
	outpoint := protos.OutPoint{Hash:common.HexToHash(prehash), Index:uint32(inputIdx)}
	prepk, _ := parseShortForm(mutxo.pkscript)
	in := protos.NewTxIn(&outpoint, prepk)
	tx.AddTxIn(in)
	pkscript, _ := txscript.NewScriptBuilder().AddData([]byte{0x51}).Script()
	out := protos.NewTxOut(inputValue, pkscript, assetId)
	tx.AddTxOut(out)

	txhash := calcSignatureHash(prepk, txscript.SigHashAll, tx, 0)
	sig, _ := privatekeys[signIdx].Sign(txhash[:])
	sigbytes := sig.Serialize()
	sigs := make([]byte, len(sigbytes) + 1)
	copy(sigs[:], sigbytes[:])
	sigs[len(sigbytes)] = byte(txscript.SigHashAll)
	sigscript, _ := txscript.NewScriptBuilder().AddData(sigs).
		AddData(privatekeys[signIdx].PubKey().SerializeCompressed()).Script()
	tx.TxIn[0].SignatureScript = sigscript

	writeCase(w, mutxos, tx, "P2SH")

	v := scriptWithInputVal{
		inputVal: inputValue,
		pkScript: prepk,
	}
	prevOuts[outpoint] = v
	varifyCase(tx, prevOuts)
}

func create2InputCHECKSIG(w *bytes.Buffer, addresses []common.IAddress, 
	privatekeys []*crypto.PrivateKey) {
	prevOuts := make(map[protos.OutPoint]scriptWithInputVal)
	mutxos := make([]*MockUTXO, 0)
	inputValue := int64(0)
	idxs := []uint32{0,1,}
	signIdx := 1
	prehashes := []string{"0001000000000000000000000000000000000000000000000000000000000000",
		"0002000000000000000000000000000000000000000000000000000000000000",}

	tx := protos.NewMsgTx(1)

	for i := 0; i < 2; i++ {
		mutxo := createMockUTXO(addresses[signIdx:signIdx+1], prehashes[i], idxs[i], inputValue,
			"DUP HASH160", " OP_IFLAG_EQUALVERIFY OP_CHECKSIG")
		mutxos = append(mutxos, mutxo)

		outpoint := protos.OutPoint{Hash: common.HexToHash(prehashes[i]), Index:idxs[i]}
		in := protos.NewTxIn(&outpoint, nil)
		tx.AddTxIn(in)
	}
	pkscript, _ := txscript.NewScriptBuilder().AddOp(txscript.OP_1).Script()
	out := protos.NewTxOut(inputValue, pkscript, assetId)
	tx.AddTxOut(out)

	signs := make([][]byte, 0, 2)
	for i := 0; i < 2; i++ {
		prepk, _ := parseShortForm(mutxos[i].pkscript)
		txhash := calcSignatureHash(prepk, txscript.SigHashAll, tx, i)
		sig, _ := privatekeys[signIdx].Sign(txhash[:])
		sigbytes := sig.Serialize()
		sigs := make([]byte, len(sigbytes)+1)
		copy(sigs[:], sigbytes[:])
		sigs[len(sigbytes)] = byte(txscript.SigHashAll)
		sigscript, _ := txscript.NewScriptBuilder().AddData(sigs).
			AddData(privatekeys[signIdx].PubKey().SerializeUncompressed()).Script()
		signs = append(signs, sigscript)
		tx.TxIn[i].SignatureScript = nil

		v := scriptWithInputVal{
			inputVal: inputValue,
			pkScript: prepk,
		}
		prevOuts[tx.TxIn[i].PreviousOutPoint] = v
	}
	for i := 0; i < 2; i++ {
		tx.TxIn[i].SignatureScript = signs[i]
	}

	writeCase(w, mutxos, tx, "P2SH")
	varifyCase(tx, prevOuts)
}

func createSigHashSingle(w *bytes.Buffer, addresses []common.IAddress, 
	privatekeys []*crypto.PrivateKey) {
	prevOuts := make(map[protos.OutPoint]scriptWithInputVal)
	mutxos := make([]*MockUTXO, 0)
	inputValue := int64(0)
	idxs := []uint32{0,1,}
	signIdx := 0
	prehashes := []string{"0001000000000000000000000000000000000000000000000000000000000000",
		"0002000000000000000000000000000000000000000000000000000000000000",}
	tx := protos.NewMsgTx(1)

	// first inpout
	for i := 0; i < 2; i++ {
		mutxo := createMockUTXO(addresses[signIdx:signIdx+1], prehashes[i], idxs[i], inputValue,
				"DUP HASH160", " IFLAG_EQUALVERIFY CHECKSIG")
		mutxos = append(mutxos, mutxo)

		outpoint := protos.OutPoint{Hash: common.HexToHash(prehashes[i]), Index:idxs[i]}
		in := protos.NewTxIn(&outpoint, nil)
		tx.AddTxIn(in)
		if i > 0 {
			pkscript, _ := txscript.NewScriptBuilder().AddData([]byte{txscript.OP_1}).Script()
			out := protos.NewTxOut(inputValue, pkscript, assetId)
			tx.AddTxOut(out)
		}
	}

	sigscript, _ := txscript.NewScriptBuilder().AddData([]byte{txscript.OP_1}).Script()
	tx.TxIn[0].SignatureScript = sigscript
	for i := 0; i < 2; i++ {
		prepk, _ := parseShortForm(mutxos[i].pkscript)
		txhash := calcSignatureHash(prepk, txscript.SigHashSingle, tx, i)
		sig, _ := privatekeys[signIdx].Sign(txhash[:])
		sigbytes := sig.Serialize()
		sigs := make([]byte, len(sigbytes)+1)
		copy(sigs[:], sigbytes[:])
		sigs[len(sigbytes)] = byte(txscript.SigHashSingle)
		sigscript, _ := txscript.NewScriptBuilder().AddData(sigs).
			AddData(privatekeys[signIdx].PubKey().SerializeCompressed()).Script()
		tx.TxIn[i].SignatureScript = sigscript

		v := scriptWithInputVal{
			inputVal: inputValue,
			pkScript: prepk,
		}
		prevOuts[tx.TxIn[i].PreviousOutPoint] = v
	}

	writeCase(w, mutxos, tx, "P2SH")
	varifyCase(tx, prevOuts)
}

func createvalidMaxOutput(w *bytes.Buffer, addresses []common.IAddress, privatekeys []*crypto.PrivateKey,
	outvalues []int64) {
	tx, prevOuts := createbyOutValue(w, addresses, privatekeys, outvalues)
	varifyCase(tx, prevOuts)
}

func createbyOutValue(w *bytes.Buffer, addresses []common.IAddress, privatekeys []*crypto.PrivateKey,
		outvalues []int64) (*protos.MsgTx, map[protos.OutPoint]scriptWithInputVal) {
	prevOuts := make(map[protos.OutPoint]scriptWithInputVal)
	mutxos := make([]*MockUTXO, 0)
	inputValue := int64(0)
	idxs := []uint32{0,1,}
	signIdx := 0
	prehashes := []string{"0001000000000000000000000000000000000000000000000000000000000000",
		"0002000000000000000000000000000000000000000000000000000000000000",}

	buff := make([]byte, 0x23)
	buff[0] = 0x21
	copy(buff[1:], privatekeys[signIdx].PubKey().SerializeCompressed())
	buff[0x22] = txscript.OP_CHECKSIG

	tx := protos.NewMsgTx(1)

	for i := 0; i < 1; i++ {
		addr, _ := common.NewAddressWithId(common.ScriptHashAddrID, common.Hash160(buff[:]))
		mutxo := &MockUTXO{
			hash:prehashes[i],
			idx:idxs[i],
			pkscript:"HASH160 0x15 " + addr.String() + " IFLAG_EQUAL",
			amount:inputValue,
		}
		mutxos = append(mutxos, mutxo)

		outpoint := protos.OutPoint{Hash: common.HexToHash(prehashes[i]), Index:idxs[i]}
		in := protos.NewTxIn(&outpoint, nil)
		tx.AddTxIn(in)
	}

	// Output
	for _, outvalue := range outvalues {
		pkscript, _ := txscript.NewScriptBuilder().AddData([]byte{0x51}).Script()
		out := protos.NewTxOut(outvalue, pkscript, assetId)
		tx.AddTxOut(out)
	}

	for i := 0; i < 1; i++ {
		prepk, _ := parseShortForm(mutxos[i].pkscript)
		txhash,_ := txscript.CalcSignatureHash(buff[:], txscript.SigHashAll, tx, i)
		sig, _ := privatekeys[signIdx].Sign(txhash[:])
		sigbytes := sig.Serialize()
		sigs := make([]byte, len(sigbytes)+1)
		copy(sigs[:], sigbytes[:])
		sigs[len(sigbytes)] = byte(txscript.SigHashAll)
		sigscript, _ := txscript.NewScriptBuilder().AddData(sigs).AddData(buff[:]).Script()
		tx.TxIn[i].SignatureScript = sigscript

		v := scriptWithInputVal{
			inputVal: inputValue,
			pkScript: prepk,
		}
		prevOuts[tx.TxIn[i].PreviousOutPoint] = v
	}

	writeCase(w, mutxos, tx, "P2SH")
	return tx, prevOuts
}

func createCoinbase(w *bytes.Buffer,  coinbaseSize int) {
	prevOuts := make(map[protos.OutPoint]scriptWithInputVal)
	mutxos := make([]*MockUTXO, 0)
	inputValue := int64(0)
	prehash := "0000000000000000000000000000000000000000000000000000000000000000"
	mutxo := &MockUTXO{
		hash:prehash,
		idx:0xffffffff,
		pkscript:"1",
		amount:inputValue,
	}
	mutxos = append(mutxos, mutxo)

	tx := protos.NewMsgTx(1)

	builder := txscript.NewScriptBuilder()
	for i := 0; i < coinbaseSize; i++ {
		builder.AddOp(0x51)
	}
	sigscript, _ := builder.Script()

	outpoint := protos.OutPoint{
		Hash: common.HexToHash("0000000000000000000000000000000000000000000000000000000000000000"),
		Index: 0xffffffff,
	}
	in := protos.NewTxIn(&outpoint, sigscript)
	tx.AddTxIn(in)

	// Output
	for i := 0; i < 1; i++ {
		pkscript, _ := txscript.NewScriptBuilder().AddData([]byte{0x51}).Script()
		out := protos.NewTxOut(0, pkscript, assetId)
		tx.AddTxOut(out)
	}

	for i := 0; i < 1; i++ {
		prepk, _ := parseShortForm(mutxos[0].pkscript)

		v := scriptWithInputVal{
			inputVal: inputValue,
			pkScript: prepk,
		}
		prevOuts[tx.TxIn[i].PreviousOutPoint] = v
	}

	writeCase(w, mutxos, tx, "P2SH")
	varifyCase(tx, prevOuts)
}

func createSigAnyOneCanPay(w *bytes.Buffer, addresses []common.IAddress, 
	privatekeys []*crypto.PrivateKey) {
	prevOuts := make(map[protos.OutPoint]scriptWithInputVal)
	mutxos := make([]*MockUTXO, 0)
	inputValue := int64(0)
	idxs := []uint32{0,1,}
	signIdx := 2
	prehashes := []string{"0001000000000000000000000000000000000000000000000000000000000000",
		"0002000000000000000000000000000000000000000000000000000000000000",}

	buff := make([]byte, 0x23)
	buff[0] = 0x21
	copy(buff[1:], privatekeys[signIdx].PubKey().SerializeCompressed())
	buff[0x22] = txscript.OP_CHECKSIG

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
	varifyCase(tx, prevOuts)
}

func createNegativePay(w *bytes.Buffer, addresses []common.IAddress, 
	privatekeys []*crypto.PrivateKey) {

	prevOuts := make(map[protos.OutPoint]scriptWithInputVal)
	mutxos := make([]*MockUTXO, 0)
	inputValue := int64(0)
	idxs := []uint32{0,1,}
	signIdx := 1
	prehashes := []string{"482f7a028730a233ac9b48411a8edfb107b749e61faf7531f4257ad95d0a51c5",}

	tx := protos.NewMsgTx(1)

	for i := 0; i < 1; i++ {
		mutxo := createMockUTXO(addresses[signIdx:signIdx+1], prehashes[i], idxs[i], inputValue,
			"0x4c 0xae 0x606563686f2022553246736447566b58312b5a536e587574356542793066794778625456415675534a6c376a6a334878416945325364667657734f53474f36633338584d7439435c6e543249584967306a486956304f376e775236644546673d3d22203e20743b206f70656e73736c20656e63202d7061737320706173733a5b314a564d7751432d707269766b65792d6865785d202d64202d6165732d3235362d636263202d61202d696e207460 DROP DUP HASH160",
			" IFLAG_EQUALVERIFY CHECKSIG")
		mutxos = append(mutxos, mutxo)

		outpoint := protos.OutPoint{Hash: common.HexToHash(prehashes[i]), Index:idxs[i]}
		in := protos.NewTxIn(&outpoint, nil)
		tx.AddTxIn(in)
	}

	// Output
	for i := 0; i < 1; i++ {
		pkscript, _ := txscript.NewScriptBuilder().AddData([]byte{0x51}).Script()
		var ovalue uint64
		ovalue = 0x8096980000000000
		out := protos.NewTxOut(int64(ovalue), pkscript, assetId)
		tx.AddTxOut(out)
	}

	for i := 0; i < 1; i++ {
		hashType := txscript.SigHashAll
		prepk, _ := parseShortForm(mutxos[i].pkscript)
		txhash,_ := txscript.CalcSignatureHash(prepk, hashType, tx, i)
		sig, _ := privatekeys[signIdx].Sign(txhash[:])
		sigbytes := sig.Serialize()
		sigs := make([]byte, len(sigbytes)+1)
		copy(sigs[:], sigbytes[:])
		sigs[len(sigbytes)] = byte(hashType)
		sigscript, _ := txscript.NewScriptBuilder().AddData(sigs).
			AddData(privatekeys[signIdx].PubKey().SerializeUncompressed()).Script()
		tx.TxIn[i].SignatureScript = sigscript

		v := scriptWithInputVal{
			inputVal: inputValue,
			pkScript: prepk,
		}
		prevOuts[tx.TxIn[i].PreviousOutPoint] = v
	}

	writeCase(w, mutxos, tx, "P2SH")
	varifyCase(tx, prevOuts)
}

func createCODESEPARATOR(w *bytes.Buffer, addresses []common.IAddress, 
	privatekeys []*crypto.PrivateKey, prepk string, postpk string) {
	prevOuts := make(map[protos.OutPoint]scriptWithInputVal)
	mutxos := make([]*MockUTXO, 0)
	inputValue := int64(0)
	idxs := []uint32{0,1,}
	signIdx := 2
	prehashes := []string{"2432b60dc72cebc1a27ce0969c0989c895bdd9e62e8234839117f8fc32d17fbc",}

	tx := protos.NewMsgTx(1)

	for i := 0; i < 1; i++ {
		mutxo := createMockUTXO(addresses[signIdx:signIdx+1], prehashes[i], idxs[i],
			0, prepk, postpk)
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

	for i := 0; i < 1; i++ {
		hashType := txscript.SigHashAll
		prepk, _ := parseShortFormStartFromCODESEPARATOR(mutxos[i].pkscript)
		txhash,_ := txscript.CalcSignatureHash(prepk, hashType, tx, i)
		sig, _ := privatekeys[signIdx].Sign(txhash[:])
		sigbytes := sig.Serialize()
		sigs := make([]byte, len(sigbytes)+1)
		copy(sigs[:], sigbytes[:])
		sigs[len(sigbytes)] = byte(hashType)
		sigscript, _ := txscript.NewScriptBuilder().AddData(sigs).Script()
		tx.TxIn[i].SignatureScript = sigscript

		pubkeyScript, _ := parseShortForm(mutxos[i].pkscript)
		v := scriptWithInputVal{
			inputVal: inputValue,
			pkScript: pubkeyScript,
		}
		prevOuts[tx.TxIn[i].PreviousOutPoint] = v
	}

	writeCase(w, mutxos, tx, "P2SH")
	varifyCase(tx, prevOuts)
}

func createCODESEPARATORExcution(w *bytes.Buffer, addresses []common.IAddress, 
	privatekeys []*crypto.PrivateKey) {
	prevOuts := make(map[protos.OutPoint]scriptWithInputVal)
	mutxos := make([]*MockUTXO, 0)
	inputValue := int64(0)
	idxs := []uint32{0,1,}
	signIdx := 2
	prehashes := []string{"44490eda355be7480f2ec828dcc1b9903793a8008fad8cfe9b0c6b4d2f0355a9",}
	tx := protos.NewMsgTx(1)

	var pksript2nd string
	for i := 0; i < 1; i++ {
		mutxo := createMockUTXO(addresses[signIdx:signIdx+1], prehashes[i], idxs[i],
			0, "", " CHECKSIGVERIFY CODESEPARATOR")
		pksript2nd = mutxo.pkscript + " 1"
		mutxo.pkscript = mutxo.pkscript + pksript2nd
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
	for i := 0; i < 2; i++ {
		hashType := txscript.SigHashAll
		var txhash []byte
		if i == 0 {
			prepk, _ := parseShortForm(pksript2nd)
			txhash, _ = txscript.CalcSignatureHash(prepk, hashType, tx, 0)
		} else {
			prepk, _ := parseShortForm(mutxos[0].pkscript)
			txhash, _ = txscript.CalcSignatureHash(prepk, hashType, tx, 0)
		}
		sig, _ := privatekeys[signIdx].Sign(txhash[:])
		sigbytes := sig.Serialize()
		sigs := make([]byte, len(sigbytes)+1)
		copy(sigs[:], sigbytes[:])
		sigs[len(sigbytes)] = byte(hashType)
		builder.AddData(sigs)
	}
	sigscript, _ := builder.Script()
	tx.TxIn[0].SignatureScript = sigscript

	prepk, _ := parseShortForm(mutxos[0].pkscript)
	v := scriptWithInputVal{
		inputVal: inputValue,
		pkScript: prepk,
	}
	prevOuts[tx.TxIn[0].PreviousOutPoint] = v

	writeCase(w, mutxos, tx, "P2SH")
	varifyCase(tx, prevOuts)
}

func createCODESEPARATORUnderIF(w *bytes.Buffer, addresses []common.IAddress, 
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
			builder.AddOp(txscript.OP_1)
		} else {
			builder.AddOp(txscript.OP_0)
		}
		sigscript, _ := builder.Script()
		tx.TxIn[i].SignatureScript = sigscript

		prevOuts[tx.TxIn[i].PreviousOutPoint] = v
	}

	writeCase(w, mutxos, tx, "P2SH")
	varifyCase(tx, prevOuts)
}

func mssignprepare(signIdx int, addresses []common.IAddress, masteronly, rollback bool) (*protos.MsgTx, []byte, []byte,
	[]*MockUTXO, map[protos.OutPoint]scriptWithInputVal) {
	script := "1 " + joinAddress(addresses[signIdx:signIdx+1]) + " 1 1 " +
		joinAddress(addresses[signIdx+1:signIdx+4]) + " 3 "
	if rollback {
		script = script + "OP_CHECKMSROLLBACKSIG"
	} else {
		script = script + "CHECKMSSIG"
	}
	shortScript, _ := parseShortForm(script)
	scriptAddress, _ := common.NewAddressWithId(common.ScriptHashAddrID, common.Hash160(shortScript))
	hexAddress := scriptAddress.String()
	mutxos := make([]*MockUTXO, 0)
	inputValue := int64(0)
	inputIdx := uint32(0)
	prehash := "b14bdcbc3e01bdaad36cc08e81e69c82e1060bc14e518db2b49aa43ad90ba260"
	mutxo := &MockUTXO{
		hash:prehash,
		idx:inputIdx,
		amount:inputValue,
		pkscript:"HASH160 0x" + strconv.FormatInt(int64(len(hexAddress)+1)/2-1, 16) + " " + hexAddress + " IFLAG_EQUAL",
	}
	mutxos = append(mutxos, mutxo)

	tx := protos.NewMsgTx(1)
	outpoint := protos.OutPoint{Hash:common.HexToHash(prehash), Index:uint32(inputIdx)}
	prepk, _ := parseShortForm(mutxo.pkscript)
	in := protos.NewTxIn(&outpoint, prepk)
	tx.AddTxIn(in)
	tx.LockTime = txscript.MinMasterSlaveLockTime + 1000
	builder := txscript.NewScriptBuilder()
	if masteronly {
		scriptr := "1 " + joinAddress(addresses[signIdx:signIdx+1]) + " 1 1 " +
			joinAddress(addresses[signIdx+1:signIdx+4]) + " 3 "
		if rollback {
			scriptr = scriptr + "CHECKMSSIG"
		} else {
			scriptr = scriptr + "OP_CHECKMSROLLBACKSIG"
		}
		shortScriptr, _ := parseShortForm(scriptr)
		fmt.Println("rollback address", hexutil.Encode(shortScriptr))
		scriptAddressr, _ := common.NewAddressWithId(common.ScriptHashAddrID, common.Hash160(shortScriptr))
		fmt.Println("rollback address", scriptAddressr.String())
		builder.AddOp(txscript.OP_HASH160).AddData(scriptAddressr.ScriptAddress()).AddOp(txscript.OP_IFLAG_EQUAL)
	} else {
		builder.AddData([]byte{0x51})
	}
	pkscript, _ := builder.Script()
	out := protos.NewTxOut(inputValue, pkscript, assetId)
	tx.AddTxOut(out)
	txhash := calcSignatureHash(shortScript, txscript.SigHashAll, tx, 0)
	fmt.Println("createMasterSlaveSign txhash", hexutil.Encode(txhash[:]))

	v := scriptWithInputVal{
		inputVal: inputValue,
		pkScript: prepk,
	}
	prevOuts := make(map[protos.OutPoint]scriptWithInputVal)
	prevOuts[outpoint] = v
	return tx, txhash, shortScript, mutxos, prevOuts
}

func createMasterSlaveSign(w *bytes.Buffer, addresses []common.IAddress,
	privatekeys []*crypto.PrivateKey) {
	signIdx := 2
	tx, txhash, shortScript, mutxos, prevOuts := mssignprepare(signIdx, addresses, false, false)
	builder := txscript.NewScriptBuilder()
	for i:=signIdx; i < signIdx + 2; i++ {
		sig, _ := privatekeys[i].Sign(txhash[:])
		sigbytes := sig.Serialize()
		sigs := make([]byte, len(sigbytes)+1)
		copy(sigs[:], sigbytes[:])
		sigs[len(sigbytes)] = byte(txscript.SigHashAll)
		builder.AddData(sigs)
		fmt.Println("pubk-sig", addresses[i].String(), hexutil.Encode(sigs))
	}
	builder.AddData(shortScript)
	sigscript, _ := builder.Script()
	tx.TxIn[0].SignatureScript = sigscript

	writeCase(w, mutxos, tx, "P2SH")

	varifyCase(tx, prevOuts)
}

func createMasterSlaveSignMasterOnly(w *bytes.Buffer, addresses []common.IAddress,
	privatekeys []*crypto.PrivateKey) {
	signIdx := 2
	tx, txhash, shortScript, mutxos, prevOuts := mssignprepare(signIdx, addresses, true, false)
	builder := txscript.NewScriptBuilder()
	for i:=signIdx; i < signIdx + 1; i++ {
		sig, _ := privatekeys[i].Sign(txhash[:])
		sigbytes := sig.Serialize()
		sigs := make([]byte, len(sigbytes)+1)
		copy(sigs[:], sigbytes[:])
		sigs[len(sigbytes)] = byte(txscript.SigHashAll)
		builder.AddData(sigs)
		fmt.Println("pubk-sig", addresses[i].String(), hexutil.Encode(sigs))
	}
	builder.AddData([]byte{})
	builder.AddData(shortScript)
	sigscript, _ := builder.Script()
	tx.TxIn[0].SignatureScript = sigscript

	writeCase(w, mutxos, tx, "P2SH")

	varifyCase(tx, prevOuts)
}

func createMasterSlaveRollbackSign(w *bytes.Buffer, addresses []common.IAddress,
	privatekeys []*crypto.PrivateKey) {
	signIdx := 2
	tx, txhash, shortScript, mutxos, prevOuts := mssignprepare(signIdx, addresses, true, true)
	builder := txscript.NewScriptBuilder()
	builder.AddData([]byte{})
	for i:=signIdx+1; i < signIdx + 2; i++ {
		sig, _ := privatekeys[i].Sign(txhash[:])
		sigbytes := sig.Serialize()
		sigs := make([]byte, len(sigbytes)+1)
		copy(sigs[:], sigbytes[:])
		sigs[len(sigbytes)] = byte(txscript.SigHashAll)
		builder.AddData(sigs)
		fmt.Println("pubk-sig", addresses[i].String(), hexutil.Encode(sigs))
	}
	builder.AddData(shortScript)
	sigscript, _ := builder.Script()
	tx.TxIn[0].SignatureScript = sigscript

	writeCase(w, mutxos, tx, "P2SH")

	varifyCase(tx, prevOuts)
}

// validmain create valid tx test cases in tx_valid.json.
func validmain(privatekeys []*crypto.PrivateKey, addresses []common.IAddress) {
	w := bytes.Buffer{}
	w.Write([]byte("[\n"))
    w.Write([]byte("[\"The following are deserialized transactions which are valid.\"],\n"))
	w.Write([]byte("[\"They are in the form\"],\n"))
	w.Write([]byte("[\"[[[prevout hash, prevout index, prevout scriptPubKey, amount?], [input 2], ...],\"],\n"))
	w.Write([]byte("[\"serializedTransaction, verifyFlags]\"],\n"))
	w.Write([]byte("[\"Objects that are only a single string (like this one) are ignored\"],\n"))


	w.Write([]byte("[\"It is the first OP_CHECKMULTISIG transaction in standard form\"],\n"))
	createStandardMultisign(&w, addresses, privatekeys)
	w.Write([]byte(",\n"))

	w.Write([]byte("[\"Two input with uncompressed pubkey hash in standard pay-to-pubkey-hash format\"],\n"))
	create2InputCHECKSIG(&w, addresses, privatekeys)
	w.Write([]byte(",\n"))

	w.Write([]byte("[\"A nearly-standard transaction with CHECKSIGVERIFY 1 instead of CHECKSIG\"],\n"))
	createCHECKSIGVERIFY_1(&w, addresses, privatekeys)
	w.Write([]byte(",\n"))

	w.Write([]byte("[\"The following tests for the presence of a bug in the handling of SIGHASH_SINGLE\"],\n"))
	w.Write([]byte("[\"Use SigHashSingle to signature\"],\n"))
	createSigHashSingle(&w, addresses, privatekeys)
	w.Write([]byte(",\n"))

	w.Write([]byte("[\"Tests for CheckTransaction()\"],\n"))
	w.Write([]byte("[\"MAX_MONEY output\"],\n"))
	createvalidMaxOutput(&w, addresses, privatekeys, []int64{0x000775f05a074000})
	w.Write([]byte(",\n"))
	w.Write([]byte("[\"MAX_MONEY output + 0 output\"],\n"))
	createvalidMaxOutput(&w, addresses, privatekeys, []int64{0x000775f05a074000, 0})
	w.Write([]byte(",\n"))

	w.Write([]byte("[\"Coinbase of size 2, Note the input is just required to make the tester happy\"],\n"))
	createCoinbase(&w, 2)
	w.Write([]byte(",\n"))
	w.Write([]byte("[\"Coinbase of size 100, Note the input is just required to make the tester happy\"],\n"))
	createCoinbase(&w, 100)
	w.Write([]byte(",\n"))

	w.Write([]byte("[\"Simple transaction with first input is signed with SIGHASH_ALL, second with SIGHASH_ANYONECANPAY\"],\n"))
	createSigAnyOneCanPay(&w, addresses, privatekeys)
	w.Write([]byte(",\n"))

	/*
	disable non standard opcode.
	// w.Write([]byte("[\"Spends an input that pushes using a PUSHDATA1 that is negative when read as signed\"],\n"))
	// createNegativePay(&w, addresses, privatekeys)
	// w.Write([]byte(",\n"))
	//
	// w.Write([]byte("[\"OP_CODESEPARATOR tests\"],\n"))
	// w.Write([]byte("[\"Test that SignatureHash() removes OP_CODESEPARATOR with FindAndDelete()\"],\n"))
	// createCODESEPARATOR(&w, addresses, privatekeys, "CODESEPARATOR", " CHECKSIG")
	// w.Write([]byte(",\n"))
	// createCODESEPARATOR(&w, addresses, privatekeys, "CODESEPARATOR CODESEPARATOR", " CHECKSIG")
	// w.Write([]byte(",\n"))
	//
	// w.Write([]byte("[\"Hashed data starts at the CODESEPARATOR\"],\n"))
	// createCODESEPARATOR(&w, addresses, privatekeys, "", "  CODESEPARATOR CHECKSIG")
	// w.Write([]byte(",\n"))
	//
	// w.Write([]byte("[\"But only if execution has reached it\"],\n"))
	// createCODESEPARATORExcution(&w, addresses, privatekeys)
	// w.Write([]byte(",\n"))
	//
	// w.Write([]byte("[\"CODESEPARATOR in an unexecuted IF block does not change what is hashed\"],\n"))
	// createCODESEPARATORUnderIF(&w, addresses, privatekeys, false)
	// w.Write([]byte(",\n"))
	//
	// w.Write([]byte("[\"As above, with the IF block executed\"],\n"))
	// createCODESEPARATORUnderIF(&w, addresses, privatekeys, true)
	*/

	w.Write([]byte("[\"Master slave sign\"],\n"))
	createMasterSlaveSign(&w, addresses, privatekeys)
	createMasterSlaveSignMasterOnly(&w, addresses, privatekeys)
	createMasterSlaveRollbackSign(&w, addresses, privatekeys)

	w.Write([]byte("\n]\n"))

	f, err := os.Create("txscript/data/tx_valid.json")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer f.Close()
	f.Write(w.Bytes())
	f.Sync()
}