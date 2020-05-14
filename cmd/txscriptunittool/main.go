// Copyright (c) 2018-2020. The asimov developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/AsimovNetwork/asimov/common"
	"github.com/AsimovNetwork/asimov/common/address"
	"github.com/AsimovNetwork/asimov/crypto"
	"github.com/AsimovNetwork/asimov/protos"
	"github.com/AsimovNetwork/asimov/txscript"
	"strconv"
	"strings"
)

const(
    sigHashMask = 0x1f
	CPKHASH = 0
	CPK = 1
)

var assetId = protos.Asset{
    0,0x01010101,
}

// parse hex string into a []byte.
func parseHex(tok string) ([]byte, error) {
	if !strings.HasPrefix(tok, "0x") {
		return nil, errors.New("not a hex number")
	}
	return hex.DecodeString(tok[2:])
}

func initShortFormOps() map[string]byte {
	// Only create the short form opcode map once.
	ops := make(map[string]byte)
	for opcodeName, opcodeValue := range txscript.OpcodeByName {
		if strings.Contains(opcodeName, "OP_UNKNOWN") {
			continue
		}
		ops[opcodeName] = opcodeValue

		// The opcodes named OP_# can't have the OP_ prefix
		// stripped or they would conflict with the plain
		// numbers.  Also, since OP_FALSE and OP_TRUE are
		// aliases for the OP_0, and OP_1, respectively, they
		// have the same value, so detect those by name and
		// allow them.
		if (opcodeName == "OP_FALSE" || opcodeName == "OP_TRUE") ||
			(opcodeValue != txscript.OP_0 && (opcodeValue < txscript.OP_1 ||
				opcodeValue > txscript.OP_16)) {

			ops[strings.TrimPrefix(opcodeName, "OP_")] = opcodeValue
		}
	}
	return ops
}
// shortFormOps holds a map of opcode names to values for use in short form
// parsing.  It is declared here so it only needs to be created once.
var shortFormOps map[string]byte = initShortFormOps()


type MockUTXO struct {
	hash string
	idx uint32
	pkscript string
	amount int64
}
type scriptWithInputVal struct {
	inputVal int64
	pkScript []byte
}

// parseShortForm parses a string as as used in the Bitcoin Core reference tests
// into the script it came from.
//
// The format used for these tests is pretty simple if ad-hoc:
//   - Opcodes other than the push opcodes and unknown are present as
//     either OP_NAME or just NAME
//   - Plain numbers are made into push operations
//   - Numbers beginning with 0x are inserted into the []byte as-is (so
//     0x14 is OP_DATA_20)
//   - Single quoted strings are pushed as data
//   - Anything else is an error
func parseShortForm(script string) ([]byte, error) {

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


func shallowCopyTx(tx *protos.MsgTx) protos.MsgTx {
	// As an additional memory optimization, use contiguous backing arrays
	// for the copied inputs and outputs and point the final slice of
	// pointers into the contiguous arrays.  This avoids a lot of small
	// allocations.
	txCopy := protos.MsgTx{
		Version:  tx.Version,
		TxIn:     make([]*protos.TxIn, len(tx.TxIn)),
		TxOut:    make([]*protos.TxOut, len(tx.TxOut)),
		LockTime: tx.LockTime,
	}
	txIns := make([]protos.TxIn, len(tx.TxIn))
	for i, oldTxIn := range tx.TxIn {
		txIns[i] = *oldTxIn
		txCopy.TxIn[i] = &txIns[i]
	}
	txOuts := make([]protos.TxOut, len(tx.TxOut))
	for i, oldTxOut := range tx.TxOut {
		txOuts[i] = *oldTxOut
		txCopy.TxOut[i] = &txOuts[i]
	}
	return txCopy
}

func calcSignatureHash(script []byte, hashType txscript.SigHashType, tx *protos.MsgTx, idx int) []byte {
	if hashType&sigHashMask == txscript.SigHashSingle && idx >= len(tx.TxOut) {
		var hash common.Hash
		hash[0] = 0x01
		return hash[:]
	}

	// Make a shallow copy of the transaction, zeroing out the script for
	// all inputs that are not currently being processed.
	txCopy := shallowCopyTx(tx)
	for i := range txCopy.TxIn {
		if i == idx {
			txCopy.TxIn[idx].SignatureScript = script
		} else {
			txCopy.TxIn[i].SignatureScript = nil
		}
	}

	switch hashType & sigHashMask {
	case txscript.SigHashNone:
		txCopy.TxOut = txCopy.TxOut[0:0] // Empty slice.
		for i := range txCopy.TxIn {
			if i != idx {
				txCopy.TxIn[i].Sequence = 0
			}
		}

	case txscript.SigHashSingle:
		// Resize output array to up to and including requested index.
		txCopy.TxOut = txCopy.TxOut[:idx+1]

		// All but current output get zeroed out.
		for i := 0; i < idx; i++ {
			txCopy.TxOut[i].Value = -1
			txCopy.TxOut[i].PkScript = nil
		}

		// Sequence on all other inputs is 0, too.
		for i := range txCopy.TxIn {
			if i != idx {
				txCopy.TxIn[i].Sequence = 0
			}
		}

	default:
		// Consensus treats undefined hashtypes like normal SigHashAll
		// for purposes of hash generation.
		fallthrough
	case txscript.SigHashAll:
		// Nothing special here.
	}
	if hashType&txscript.SigHashAnyOneCanPay != 0 {
		txCopy.TxIn = txCopy.TxIn[idx : idx+1]
	}

	// The final hash is the double sha256 of both the serialized modified
	// transaction and the hash type (encoded as a 4-byte little-endian
	// value) appended.
	wbuf := bytes.NewBuffer(make([]byte, 0, txCopy.SerializeSize()+4))
	txCopy.Serialize(wbuf)
	binary.Write(wbuf, binary.LittleEndian, hashType)
	return common.DoubleHashB(wbuf.Bytes())
}

func main() {
	privatekeys := make([]*crypto.PrivateKey, 0, 10)
	addresses := make([]common.IAddress, 0, 10)

	for ti:= 0; ti < 10; ti++ {
		privKey, err := crypto.NewPrivateKey(crypto.S256())
		if err != nil {
			fmt.Println("new priv err ", err)
			continue
		}
		privatekeys = append(privatekeys, privKey)
		var addr common.IAddress
		if ti == CPKHASH {
			addr, _ = common.NewAddressWithId(common.PubKeyHashAddrID,
				common.Hash160(privKey.PubKey().SerializeCompressed()))
		} else if ti == CPK {
			addr, _ = common.NewAddressWithId(common.PubKeyHashAddrID,
				common.Hash160(privKey.PubKey().SerializeUncompressed()))
		} else {
			pkaddr, _ := address.NewAddressPubKey(privKey.PubKey().SerializeCompressed())
			pkaddr.SetFormat(address.PKFUncompressed)
			addr = pkaddr
		}
		addresses = append(addresses, addr)
	}
	//validmain(privatekeys, addresses)
	//invalidmain(privatekeys, addresses)
	//scriptmain(privatekeys, addresses)
	//signhashmain(privatekeys, addresses)
}