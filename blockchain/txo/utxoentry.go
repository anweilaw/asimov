// Copyright (c) 2018-2020. The asimov developers
// Copyright (c) 2013-2017 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.
package txo

import "github.com/AsimovNetwork/asimov/protos"

// txoFlags is a bitmask defining additional information and state for a
// transaction output in a utxo view.
type txoFlags uint8

const (
	// tfCoinBase indicates that a txout was contained in a coinbase tx.
	tfCoinBase txoFlags = 1 << iota

	// tfSpent indicates that a txout is spent.
	tfSpent

	// tfModified indicates that a txout has been modified since it was
	// loaded.
	tfModified
)

// UtxoEntry houses details about a transaction output in a utxo
// view such as whether or not it was contained in a coinbase tx, the height of
// the block that contains the tx, whether or not it is spent, its public key
// script, and how much it pays.
type UtxoEntry struct {
	// NOTE: Additions, deletions, or modifications to the order of the
	// definitions in this struct should not be changed without considering
	// how it affects alignment on 64-bit platforms.  The current order is
	// specifically crafted to result in minimal padding.  There will be a
	// lot of these in memory, so a few extra bytes of padding adds up.

	amount      int64
	pkScript    []byte // The public key script for the output.
	blockHeight int32  // Round of block containing tx.

	// packedFlags contains additional info about output such as whether it
	// is a coinbase, whether it is spent, and whether it has been modified
	// since it was loaded.  This approach is used in order to reduce memory
	// usage since there will be a lot of these in memory.
	packedFlags txoFlags
	assets      *protos.Assets
	lockItem    *LockItem
}

// IsModified returns whether or not the output has been modified since it was
// loaded.
func (entry *UtxoEntry) IsModified() bool {
	return entry.packedFlags&tfModified == tfModified
}

func (entry *UtxoEntry) ResetModified() {
	entry.packedFlags ^= tfModified
}

// IsCoinBase returns whether or not the output was contained in a coinbase
// transaction.
func (entry *UtxoEntry) IsCoinBase() bool {
	return entry.packedFlags&tfCoinBase == tfCoinBase
}

// BlockHeight returns the height of the block containing the output.
func (entry *UtxoEntry) BlockHeight() int32 {
	return entry.blockHeight
}

// IsSpent returns whether or not the output has been spent based upon the
// current state of the unspent transaction output view it was obtained from.
func (entry *UtxoEntry) IsSpent() bool {
	return entry.packedFlags&tfSpent == tfSpent
}

// UnSpent marks the output as unspent.
func (entry *UtxoEntry) UnSpent() {
	entry.packedFlags &= (^tfSpent) | tfModified
}

func (entry *UtxoEntry) PackedFlags() uint8 {
	return uint8(entry.packedFlags)
}

// Spend marks the output as spent.  Spending an output that is already spent
// has no effect.
func (entry *UtxoEntry) Spend() {
	// Nothing to do if the output is already spent.
	if entry.IsSpent() {
		return
	}

	// Mark the output as spent and modified.
	entry.packedFlags |= tfSpent | tfModified
}

// Amount returns the amount of the output.
func (entry *UtxoEntry) Amount() int64 {
	return entry.amount
}

// PkScript returns the public key script for the output.
func (entry *UtxoEntry) PkScript() []byte {
	return entry.pkScript
}

func (entry *UtxoEntry) SetPkScript(pkscript []byte) {
	entry.pkScript = pkscript
}

// Assets returns the assets for the output.
func (entry *UtxoEntry) Assets() *protos.Assets {
	return entry.assets
}

func (entry *UtxoEntry) LockItem() *LockItem {
	return entry.lockItem
}

func (entry *UtxoEntry) SetLockItem(lockItem *LockItem)  {
	entry.lockItem = lockItem
}

func (entry *UtxoEntry) Update(amount int64, pkScript []byte, blockHeight int32, coinbase bool,
	assets *protos.Assets, lockItem *LockItem) {
	entry.amount = amount
	entry.pkScript = pkScript
	entry.blockHeight = blockHeight
	entry.packedFlags = tfModified
	if coinbase {
		entry.packedFlags |= tfCoinBase
	}
	entry.assets = assets
	entry.lockItem = lockItem
}

// Clone returns a shallow copy of the utxo entry.
func (entry *UtxoEntry) Clone() *UtxoEntry {
	if entry == nil {
		return nil
	}

	return &UtxoEntry{
		amount:      entry.amount,
		pkScript:    entry.pkScript,
		blockHeight: entry.blockHeight,
		packedFlags: entry.packedFlags,
		assets:      entry.assets,
		lockItem:    entry.lockItem,
	}
}

func NewUtxoEntry(amount int64, pkScript []byte, blockHeight int32, coinbase bool,
	assets *protos.Assets, lockItem *LockItem) *UtxoEntry {

	packedFlags := txoFlags(0)
	if coinbase {
		packedFlags |= tfCoinBase
	}
	return &UtxoEntry{
		amount:      amount,
		pkScript:    pkScript,
		blockHeight: blockHeight,
		packedFlags: packedFlags,
		assets:      assets,
		lockItem:    lockItem,
	}
}

type UtxoMap map[protos.OutPoint]*UtxoEntry
