// Copyright (c) 2018-2020. The asimov developers
// Copyright (c) 2013-2017 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.
package blockchain

import (
	"fmt"
)

// DeploymentError identifies an error that indicates a deployment ID was
// specified that does not exist.
type DeploymentError uint32

// Error returns the assertion error as a human-readable string and satisfies
// the error interface.
func (e DeploymentError) Error() string {
	return fmt.Sprintf("deployment ID %d does not exist", uint32(e))
}

// ErrorCode identifies a kind of error.
type ErrorCode int

// These constants are used to identify a specific RuleError.
const (
	// ErrDuplicateBlock indicates a block with the same hash already
	// exists.
	ErrDuplicateBlock ErrorCode = iota

	// ErrBlockTooBig indicates the serialized block size exceeds the
	// maximum allowed size.
	ErrBlockTooBig

	// ErrBlockVersionTooOld indicates the block version is too old and is
	// no longer accepted since the majority of the network has upgraded
	// to a newer version.
	ErrBlockVersionTooOld

	// ErrHeightMismatch indicates the height is not continuous after the
	// prenode.
	ErrHeightMismatch

	// ErrTimeTooNew indicates the time is too far in the future as compared
	// the current time.
	ErrTimeTooNew

	// ErrTimeOutOfRange indicates the timestamp of the block is out of the range of
	// the expect timestamps that it should be.
	ErrTimeStampOutOfRange

	// ErrBadMerkleRoot indicates the calculated merkle root does not match
	// the expected value.
	ErrBadMerkleRoot

	// ErrBadPoaHash indicates the calculated poa hash does not match
	// the expected value.
	ErrBadPoaHash

	// ErrBadCheckpoint indicates a block that is expected to be at a
	// checkpoint height does not match the expected one.
	ErrBadCheckpoint

	// ErrForkTooOld indicates a block is attempting to fork the block chain
	// before the most recent checkpoint.
	ErrForkTooOld

	// ErrCheckpointTimeTooOld indicates a block has a timestamp before the
	// most recent checkpoint.
	ErrCheckpointTimeTooOld

	// ErrNoTransactions indicates the block does not have a least one
	// transaction.  A valid block must have at least the coinbase
	// transaction.
	ErrNoTransactions

	// ErrNoTxInputs indicates a transaction does not have any inputs.  A
	// valid transaction must have at least one input.
	ErrNoTxInputs

	// ErrTxTooBig indicates a transaction exceeds the maximum allowed size
	// when serialized.
	ErrTxTooBig

	// ErrBadTxOutValue indicates an output value for a transaction is
	// invalid in some way such as being out of range.
	ErrBadTxOutValue

	// ErrInvalidOutput indicates an output with contract call in not first output.
	ErrInvalidOutput

	// ErrDuplicateTxInputs indicates a transaction references the same
	// input more than once.
	ErrDuplicateTxInputs

	// ErrBadTxInput indicates a transaction input is invalid in some way
	// such as referencing a previous transaction outpoint which is out of
	// range or not referencing one at all.
	ErrBadTxInput

	// ErrMissingTxOut indicates a transaction output referenced by an input
	// either does not exist or has already been spent.
	ErrMissingTxOut

	// ErrUnfinalizedTx indicates a transaction has not been finalized.
	// A valid block may only contain finalized transactions.
	ErrUnfinalizedTx

	// ErrDuplicateTx indicates a block contains an identical transaction
	// (or at least two transactions which hash to the same value).  A
	// valid block may only contain unique transactions.
	ErrDuplicateTx

	// ErrImmatureSpend indicates a transaction is attempting to spend a
	// coinbase that has not yet reached the required maturity.
	ErrImmatureSpend

	// ErrSpendTooHigh indicates a transaction is attempting to spend more
	// value than the sum of all of its inputs.
	ErrSpendTooHigh

	// ErrAssetsNotEqual indicates the numbers of input asset and output asset are not equal
	ErrAssetsNotEqual

	// ErrBadFees indicates the total fees for a block are invalid due to
	// exceeding the maximum possible value.
	ErrBadFees

	// ErrTooManySigOps indicates the total number of signature operations
	// for a transaction or block exceed the maximum allowed limits.
	ErrTooManySigOps

	// ErrLastTxNotCoinbase indicates the first transaction in a block
	// is not a coinbase transaction.
	ErrLastTxNotCoinbase

	// ErrMultipleCoinbases indicates a block contains more than one
	// coinbase transaction.
	ErrMultipleCoinbases

	// ErrBadCoinbaseScriptLen indicates the length of the signature script
	// for a coinbase transaction is not within the valid range.
	ErrBadCoinbaseScriptLen

	// ErrBadCoinbaseValue indicates the amount of a coinbase value does
	// not match the expected value of the subsidy plus the sum of all fees.
	ErrBadCoinbaseValue

	// ErrBadCoinbaseData indicates the data of a coinbase txout does
	// not match the expected value.
	ErrBadCoinbaseData

	// ErrMissingCoinbaseHeight indicates the coinbase transaction for a
	// block does not start with the serialized block block height as
	// required for version 2 and higher blocks.
	ErrMissingCoinbaseHeight

	// ErrBadCoinbaseHeight indicates the serialized block height in the
	// coinbase transaction for version 2 and higher blocks does not match
	// the expected value.
	ErrBadCoinbaseHeight

	// ErrScriptMalformed indicates a transaction script is malformed in
	// some way.  For example, it might be longer than the maximum allowed
	// length or fail to parse.
	ErrScriptMalformed

	// ErrScriptValidation indicates the result of executing transaction
	// script failed.  The error covers any failure when executing scripts
	// such signature verification failures and execution past the end of
	// the stack.
	ErrScriptValidation

	// ErrPreviousBlockUnknown indicates that the previous block is not known.
	ErrPreviousBlockUnknown

	// ErrInvalidAncestorBlock indicates that an ancestor of this block has
	// already failed validation.
	ErrInvalidAncestorBlock

	// ErrPrevBlockNotBest indicates that the block's previous block is not the
	// current chain tip. This is not a block validation rule, but is required
	// for block proposals submitted via getblocktemplate RPC.
	ErrPrevBlockNotBest

	//ErrForbiddenAsset indicates the asset is forbidden to transfer as input or output
	ErrForbiddenAsset

	//ErrStateRootNotMatch indicates that the state root of block do not match we expect
	ErrStateRootNotMatch

	//ErrSignDuplicate indicates the sign hash already exist
	ErrSignDuplicate

	// ErrSignMiss indicates can not find block through sign blockhash or blockheight
	ErrSignMiss

	// ErrBadSigLength indicates the length of signature do not equal with HashSignLen
	ErrBadSigLength

	// ErrBadPreSigHeight indicates the blockHeight of preSig is out of validate range
	ErrBadPreSigHeight

	// ErrPreSigOrder indicates the order of preSig is not collect.
	ErrPreSigOrder

	// ErrInvalidSigData indicates the length of hash do not equal with HashLength
	ErrInvalidSigData

	// ErrSigAndKeyMismatch indicates sig do not match the key
	ErrSigAndKeyMismatch

	//ErrValidatorMismatch indicates the validator do not match validator that we want
	ErrValidatorMismatch

	// ErrNodeUnknown indicates the the node we want is nil or unknown
	ErrNodeUnknown

	// ErrSlotIndexOutOfRange indicates the SlotIndex of the block is bigger than RoundSize of ActiveNetParams
	ErrInvalidSlotIndex

	// ErrBadHashLength indicates the length of hash do not equal with HashLength
	ErrBadHashLength

	// ErrBadSlotOrRound indicates block has the less slot/round than parent
	ErrBadSlotOrRound

	// ErrInvalidBlockHash indicates the hash of block is invalid
	ErrInvalidBlockHash

	// ErrGasLimitOverFlow indicates txs total gaslimit is overflow by block gaslimit
	ErrGasLimitOverFlow

	// ErrHashMismatch indicates the block hash do not match the hash we want
	ErrHashMismatch

	// ErrBadBlockWeight indicates the block hash do not match the hash we want
	ErrBadBlockWeight

	//ErrGasMismatch indicates the block GasUsed do not match that we expect
	ErrGasMismatch

	// ErrBadGasLimit indicates the calculated gas limit does not match
	// the expected value.
	ErrBadGasLimit

	// ErrInvalidTemplate indicates Template is invalid
	ErrInvalidTemplate

	//ErrNoTemplate indicates fetch templateIndexKey returns empty entry
	ErrNoTemplate

	// ErrBadContractData indicates incorrect data protocol
	ErrBadContractData

	// ErrBadContractByteCode indicates get byte code from template error
	ErrBadContractByteCode

	// ErrFailedCreateContract indicates create contract failed
	ErrFailedCreateContract

	// ErrFailedInitTemplate indicates init template failed
	ErrFailedInitTemplate

	// ErrFailedCreateTemplate indicates create template error
	ErrFailedCreateTemplate

	// ErrFailedInitContractManager indicates init ContractManager failed
	ErrFailedInitContractManager

	// ErrInvalidPkScript indicates failed when get address from pkscript
	ErrInvalidPkScript

	//ErrFailedHeightToHashRange indicates failed when using height to Hash Range
	ErrBadHeightToHashRange

	// ErrInvalidBlock indicates the block is not yet validated
	ErrInvalidBlock

	//ErrStxoMismatch indicates that the numbers of stxos do not equal with we expect
	ErrStxoMismatch

	// ErrNotInMainChain indicates we can not find block in main chain by provided param
	ErrNotInMainChain

	// ErrFailedSerializedBlock indicates failed to get serialized bytes for block
	ErrFailedSerializedBlock
)

// Map of ErrorCode values back to their constant names for pretty printing.
var errorCodeStrings = map[ErrorCode]string{
	ErrDuplicateBlock:       "ErrDuplicateBlock",
	ErrBlockTooBig:          "ErrBlockTooBig",
	ErrBlockVersionTooOld:   "ErrBlockVersionTooOld",
	ErrHeightMismatch:       "ErrHeightMismatch",
	ErrTimeTooNew:            "ErrTimeTooNew",
	ErrTimeStampOutOfRange:   "ErrTimeStampOutOfRange",
	ErrBadMerkleRoot:         "ErrBadMerkleRoot",
	ErrBadPoaHash:            "ErrBadPoaHash",
	ErrBadCheckpoint:         "ErrBadCheckpoint",
	ErrForkTooOld:            "ErrForkTooOld",
	ErrCheckpointTimeTooOld:  "ErrCheckpointTimeTooOld",
	ErrNoTransactions:        "ErrNoTransactions",
	ErrNoTxInputs:            "ErrNoTxInputs",
	ErrTxTooBig:              "ErrTxTooBig",
	ErrBadTxOutValue:         "ErrBadTxOutValue",
	ErrInvalidOutput:         "ErrInvalidOutput",
	ErrDuplicateTxInputs:     "ErrDuplicateTxInputs",
	ErrBadTxInput:            "ErrBadTxInput",
	ErrMissingTxOut:          "ErrMissingTxOut",
	ErrUnfinalizedTx:         "ErrUnfinalizedTx",
	ErrDuplicateTx:           "ErrDuplicateTx",
	ErrImmatureSpend:         "ErrImmatureSpend",
	ErrSpendTooHigh:          "ErrSpendTooHigh",
	ErrAssetsNotEqual:        "ErrAssetsNotEqual",
	ErrBadFees:               "ErrBadFees",
	ErrTooManySigOps:         "ErrTooManySigOps",
	ErrLastTxNotCoinbase:     "ErrLastTxNotCoinbase",
	ErrMultipleCoinbases:     "ErrMultipleCoinbases",
	ErrBadCoinbaseScriptLen:  "ErrBadCoinbaseScriptLen",
	ErrBadCoinbaseValue:      "ErrBadCoinbaseValue",
	ErrBadCoinbaseData:       "ErrBadCoinbaseData",
	ErrMissingCoinbaseHeight: "ErrMissingCoinbaseHeight",
	ErrBadCoinbaseHeight:     "ErrBadCoinbaseHeight",
	ErrScriptMalformed:       "ErrScriptMalformed",
	ErrScriptValidation:      "ErrScriptValidation",
	ErrPreviousBlockUnknown:  "ErrPreviousBlockUnknown",
	ErrInvalidAncestorBlock:  "ErrInvalidAncestorBlock",
	ErrPrevBlockNotBest:      "ErrPrevBlockNotBest",
	ErrForbiddenAsset:        "ErrForbiddenAsset",
	ErrStateRootNotMatch:     "ErrStateRootNotMatch",
	ErrSignDuplicate:         "ErrSignDuplicate",
	ErrSignMiss:              "ErrSignMiss",
	ErrBadSigLength:          "ErrBadSigLength",
	ErrBadPreSigHeight:       "ErrBadPreSigHeight",
	ErrPreSigOrder:           "ErrPreSigOrder",
	ErrInvalidSigData:        "ErrInvalidSigData",
	ErrSigAndKeyMismatch:     "ErrSigAndKeyMismatch",
	ErrValidatorMismatch:     "ErrValidatorMismatch",
	ErrNodeUnknown:           "ErrNodeUnknown",
	ErrInvalidSlotIndex:      "ErrInvalidSlotIndex",
	ErrBadHashLength:        "ErrBadHashLength",
	ErrBadSlotOrRound:       "ErrBadSlotOrRound",
	ErrInvalidBlockHash:     "ErrInvalidBlockHash",
	ErrGasLimitOverFlow:     "ErrGasLimitOverFlow",
	ErrHashMismatch:         "ErrHashMismatch",
	ErrBadBlockWeight:       "ErrBadBlockWeight",
	ErrGasMismatch:          "ErrGasMismatch",
	ErrBadGasLimit:          "ErrBadGasLimit",
	ErrInvalidTemplate:      "ErrInvalidTemplate",
	ErrNoTemplate:           "ErrNoTemplate",
	ErrBadContractData:      "ErrBadContractData",
	ErrBadContractByteCode:  "ErrBadContractByteCode",
	ErrFailedCreateContract: "ErrFailedCreateContract",
	ErrFailedInitTemplate:   "ErrFailedInitTemplate",
	ErrFailedCreateTemplate: "ErrFailedCreateTemplate",
	ErrFailedInitContractManager: "ErrFailedInitContractManager",
	ErrInvalidPkScript:      "ErrInvalidPkScript",
	ErrBadHeightToHashRange: "ErrBadHeightToHashRange",
	ErrInvalidBlock:         "ErrInvalidBlock",
	ErrStxoMismatch:         "ErrStxoMismatch",
	ErrNotInMainChain:       "ErrNotInMainChain",
	ErrFailedSerializedBlock: "ErrFailedSerializedBlock",
}

// String returns the ErrorCode as a human-readable name.
func (e ErrorCode) String() string {
	if s := errorCodeStrings[e]; s != "" {
		return s
	}
	return fmt.Sprintf("Unknown ErrorCode (%d)", int(e))
}

// RuleError identifies a rule violation.  It is used to indicate that
// processing of a block or transaction failed due to one of the many validation
// rules.  The caller can use type assertions to determine if a failure was
// specifically due to a rule violation and access the ErrorCode field to
// ascertain the specific reason for the rule violation.
type RuleError struct {
	ErrorCode   ErrorCode // Describes the kind of error
	Description string    // Human readable description of the issue
}

// Error satisfies the error interface and prints human-readable errors.
func (e RuleError) Error() string {
	return e.Description
}

// ruleError creates an RuleError given a set of arguments.
func ruleError(c ErrorCode, desc string) RuleError {
	return RuleError{ErrorCode: c, Description: desc}
}
