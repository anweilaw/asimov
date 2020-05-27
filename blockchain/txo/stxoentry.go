// Copyright (c) 2018-2020. The asimov developers
// Copyright (c) 2013-2017 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.
package txo

import "github.com/AsimovNetwork/asimov/protos"

// -----------------------------------------------------------------------------
// The transaction spend journal consists of an entry for each block connected
// to the main chain which contains the transaction outputs the block spends
// serialized such that the order is the reverse of the order they were spent.
//
// This is required because reorganizing the chain necessarily entails
// disconnecting blocks to get back to the point of the fork which implies
// unspending all of the transaction outputs that each block previously spent.
// Since the utxo set, by definition, only contains unspent transaction outputs,
// the spent transaction outputs must be resurrected from somewhere.  There is
// more than one way this could be done, however this is the most straight
// forward method that does not require having a transaction index and unpruned
// blockchain.
//
// NOTE: This format is NOT self describing.  The additional details such as
// the number of entries (transaction inputs) are expected to come from the
// block itself and the utxo set (for legacy entries).  The rationale in doing
// this is to save space.  This is also the reason the spent outputs are
// serialized in the reverse order they are spent because later transactions are
// allowed to spend outputs from earlier ones in the same block.
//
// The reserved field below used to keep track of the version of the containing
// transaction when the height in the header code was non-zero, however the
// height is always non-zero now, but keeping the extra reserved field allows
// backwards compatibility.
//
// The serialized format is:
//
//   [<header code><reserved><compressed txout>],...
//
//   Field                Type     Size
//   header code          VLQ      variable
//   reserved             byte     1
//   compressed txout
//     compressed amount  VLQ      variable
//     compressed script  []byte   variable
//
// The serialized header code format is:
//   bit 0 - containing transaction is a coinbase
//   bits 1-x - height of the block that contains the spent txout
//
// Example 1:
// From block 170 in main blockchain.
//
//    1300320511db93e1dcdb8a016b49840f8c53bc1eb68a382e97b1482ecad7b148a6909a5c
//    <><><------------------------------------------------------------------>
//     | |                                  |
//     | reserved                  compressed txout
//    header code
//
//  - header code: 0x13 (coinbase, height 9)
//  - reserved: 0x00
//  - compressed txout 0:
//    - 0x32: VLQ-encoded compressed amount for 5000000000 (50 BTC)
//    - 0x05: special script type pay-to-pubkey
//    - 0x11...5c: x-coordinate of the pubkey
//
// Example 2:
// Adapted from block 100025 in main blockchain.
//
//    8b99700091f20f006edbc6c4d31bae9f1ccc38538a114bf42de65e868b99700086c64700b2fb57eadf61e106a100a7445a8c3f67898841ec
//    <----><><----------------------------------------------><----><><---------------------------------------------->
//     |    |                         |                        |    |                         |
//     |    reserved         compressed txout                  |    reserved         compressed txout
//    header code                                          header code
//
//  - Last spent output:
//    - header code: 0x8b9970 (not coinbase, height 100024)
//    - reserved: 0x00
//    - compressed txout:
//      - 0x91f20f: VLQ-encoded compressed amount for 34405000000 (344.05 BTC)
//      - 0x00: special script type pay-to-pubkey-hash
//      - 0x6e...86: pubkey hash
//  - Second to last spent output:
//    - header code: 0x8b9970 (not coinbase, height 100024)
//    - reserved: 0x00
//    - compressed txout:
//      - 0x86c647: VLQ-encoded compressed amount for 13761000000 (137.61 BTC)
//      - 0x00: special script type pay-to-pubkey-hash
//      - 0xb2...ec: pubkey hash
// -----------------------------------------------------------------------------

// SpentTxOut contains a spent transaction output and potentially additional
// contextual information such as whether or not it was contained in a coinbase
// transaction, the version of the transaction it was contained in, and which
// block height the containing transaction was included in.  As described in
// the comments above, the additional contextual information will only be valid
// when this spent txout is spending the last unspent output of the containing
// transaction.
type SpentTxOut struct {
	// Amount is the amount of the output.
	Amount int64

	// PkScipt is the the public key script for the output.
	PkScript []byte

	// Round is the height of the the block containing the creating tx.
	Height int32

	// Denotes if the creating tx is a coinbase.
	IsCoinBase bool

	Asset *protos.Asset
}
