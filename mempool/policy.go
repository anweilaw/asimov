// Copyright (c) 2018-2020 The asimov developers
// Copyright (c) 2013-2017 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package mempool

import (
	"fmt"
	"github.com/AsimovNetwork/asimov/asiutil"
	"github.com/AsimovNetwork/asimov/blockchain"
	"github.com/AsimovNetwork/asimov/protos"
	"github.com/AsimovNetwork/asimov/txscript"
)

const (
	// maxStandardP2SHSigOps is the maximum number of signature operations
	// that are considered standard in a pay-to-script-hash script.
	maxStandardP2SHSigOps = 20

	// maxStandardTxCost is the max size permitted by any transaction
	// according to the current default policy.
	maxStandardTxSize = 100000

	// maxStandardSigScriptSize is the maximum size allowed for a
	// transaction input signature script to be considered standard.  This
	// value allows for a 15-of-15 CHECKMULTISIG pay-to-script-hash with
	// compressed keys.
	//
	// The form of the overall script is: OP_0 <15 signatures> OP_PUSHDATA2
	// <2 bytes len> [OP_15 <15 pubkeys> OP_15 OP_CHECKMULTISIG]
	//
	// For the p2sh script portion, each of the 15 compressed pubkeys are
	// 33 bytes (plus one for the OP_DATA_33 opcode), and the thus it totals
	// to (15*34)+3 = 513 bytes.  Next, each of the 15 signatures is a max
	// of 73 bytes (plus one for the OP_DATA_73 opcode).  Also, there is one
	// extra byte for the initial extra OP_0 push and 3 bytes for the
	// OP_PUSHDATA2 needed to specify the 513 bytes for the script push.
	// That brings the total to 1+(15*74)+3+513 = 1627.  This value also
	// adds a few extra bytes to provide a little buffer.
	// (1 + 15*74 + 3) + (15*34 + 3) + 23 = 1650
	maxStandardSigScriptSize = 1650
)

// checkInputsStandard performs a series of checks on a transaction's inputs
// to ensure they are "standard".  A standard transaction input within the
// context of this function is one whose referenced public key script is of a
// standard form and, for pay-to-script-hash, does not have more than
// maxStandardP2SHSigOps signature operations.  However, it should also be noted
// that standard inputs also are those which have a clean stack after execution
// and only contain pushed data in their signature scripts.  This function does
// not perform those checks because the script engine already does this more
// accurately and concisely via the txscript.ScriptVerifyCleanStack and
// txscript.ScriptVerifySigPushOnly flags.
func checkInputsStandard(tx *asiutil.Tx, utxoView *blockchain.UtxoViewpoint) error {
	// NOTE: The reference implementation also does a coinbase check here,
	// but coinbases have already been rejected prior to calling this
	// function so no need to recheck.

	for i, txIn := range tx.MsgTx().TxIn {
		// It is safe to elide existence and index checks here since
		// they have already been checked prior to calling this
		// function.
		entry := utxoView.LookupEntry(txIn.PreviousOutPoint)
		originPkScript := entry.PkScript()
		switch txscript.GetScriptClass(originPkScript) {
		case txscript.ScriptHashTy:
			numSigOps := txscript.GetPreciseSigOpCount(
				txIn.SignatureScript, originPkScript)
			if numSigOps > maxStandardP2SHSigOps {
				str := fmt.Sprintf("transaction input #%d has "+
					"%d signature operations which is more "+
					"than the allowed max amount of %d",
					i, numSigOps, maxStandardP2SHSigOps)
				return txRuleError(protos.RejectNonstandard, str)
			}

		case txscript.NonStandardTy:
			str := fmt.Sprintf("transaction input #%d has a "+
				"non-standard script form", i)
			return txRuleError(protos.RejectNonstandard, str)
		}
	}

	return nil
}

// checkTransactionStandard performs a series of checks on a transaction to
// ensure it is a "standard" transaction.  A standard transaction is one that
// conforms to several additional limiting cases over what is considered a
// "sane" transaction such as having a version in the supported range, being
// finalized, conforming to more stringent size constraints, having scripts
// of recognized forms.
func checkTransactionStandard(tx *asiutil.Tx, height int32,
	medianTimePast int64, maxTxVersion uint32) error {

	// The transaction must be a currently supported version.
	msgTx := tx.MsgTx()
	if msgTx.Version > maxTxVersion || msgTx.Version < 1 {
		str := fmt.Sprintf("transaction version %d is not in the "+
			"valid range of %d-%d", msgTx.Version, 1,
			maxTxVersion)
		return txRuleError(protos.RejectNonstandard, str)
	}

	// The transaction must be finalized to be standard and therefore
	// considered for inclusion in a block.
	if !blockchain.IsFinalizedTransaction(tx, height, medianTimePast) {
		return txRuleError(protos.RejectNonstandard,
			"transaction is not finalized")
	}

	contractLength := 0
	for i, txOut := range msgTx.TxOut {
		scriptClass := txscript.GetScriptClass(txOut.PkScript)
		if scriptClass == txscript.PubKeyHashTy || scriptClass == txscript.ScriptHashTy {
			if len(txOut.Data) > 0 {
				str := fmt.Sprintf("normal pay is not allow data")
				return txRuleError(protos.RejectInvalid, str)
			}
		} else if i == 0 && (scriptClass == txscript.CreateTy || scriptClass == txscript.CallTy ||
			scriptClass == txscript.TemplateTy || scriptClass == txscript.VoteTy) {
			contractLength = len(txOut.Data)
			if contractLength == 0 && scriptClass != txscript.CallTy {
				str := fmt.Sprintf("contract invoke has no script data %d ", i)
				return txRuleError(protos.RejectInvalid, str)
			}
		} else {
			str := "non standard transaction is not acceptable"
			return txRuleError(protos.RejectNonstandard, str)
		}
	}

	for i, txIn := range msgTx.TxIn {
		// Each transaction input signature script must not exceed the
		// maximum size allowed for a standard transaction.  See
		// the comment on maxStandardSigScriptSize for more details.
		sigScriptLen := len(txIn.SignatureScript)
		//validate the size if not a contract tx
		if sigScriptLen > maxStandardSigScriptSize {
			str := fmt.Sprintf("transaction input %d: signature "+
				"script size of %d bytes is large than max "+
				"allowed size of %d bytes", i, sigScriptLen,
				maxStandardSigScriptSize)
			return txRuleError(protos.RejectNonstandard, str)
		}

		// Each transaction input signature script must only contain
		// opcodes which push data onto the stack.
		if !txscript.IsPushOnlyScript(txIn.SignatureScript) {
			str := fmt.Sprintf("transaction input %d: signature "+
				"script is not push only", i)
			return txRuleError(protos.RejectNonstandard, str)
		}
	}
	// Since extremely large transactions with a lot of inputs can cost
	// almost as much to process as the sender fees, limit the maximum
	// size of a transaction.  This also helps mitigate CPU exhaustion
	// attacks.
	txSize := tx.MsgTx().SerializeSize()
	if txSize - contractLength > maxStandardTxSize {
		str := fmt.Sprintf("size of transaction %v is larger than max "+
			"allowed size of %v", txSize, maxStandardTxSize)
		return txRuleError(protos.RejectNonstandard, str)
	}

	return nil
}
