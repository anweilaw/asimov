// Copyright (c) 2018-2020. The asimov developers
// Copyright (c) 2013-2017 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.
package txo

import (
	"github.com/AsimovNetwork/asimov/common"
	"github.com/AsimovNetwork/asimov/protos"
	"github.com/AsimovNetwork/asimov/txscript"
	"math/big"
)

type ViewAction int

const (
	// solidity compiler pick four bytes for method mark.
	ContractMethodLength = 4

	ViewUnknown ViewAction = 0
	ViewAdd     ViewAction = 1
	ViewRm      ViewAction = 2
)

type TxMark struct {
	MsgTx  *protos.MsgTx
	Action ViewAction
}

// UtxoViewpoint represents a view into the set of unspent transaction outputs
// from a specific point of view in the chain.  For example, it could be for
// the end of the main chain, some point in the history of the main chain, or
// down a side chain.
//
// The unspent outputs are needed by other transactions for things such as
// script validation and double spend prevention.
type UtxoViewpoint struct {
	entries  UtxoMap
	bestHash common.Hash
	txs      map[common.Hash]TxMark
	signs    map[protos.MsgBlockSign]ViewAction
}

// BestHash returns the hash of the best block in the chain the view currently
// respresents.
func (view *UtxoViewpoint) BestHash() *common.Hash {
	return &view.bestHash
}

// SetBestHash sets the hash of the best block in the chain the view currently
// respresents.
func (view *UtxoViewpoint) SetBestHash(hash *common.Hash) {
	view.bestHash = *hash
}

// Exist returns true if the passed output exists in the view otherwise false.
func (view *UtxoViewpoint) Exist(outpoint protos.OutPoint) bool {
	_, ok := view.entries[outpoint]
	return ok
}

// LookupEntry returns information about a given transaction output according to
// the current state of the view.  It will return nil if the passed output does
// not exist in the view or is otherwise not available such as when it has been
// disconnected during a reorg.
func (view *UtxoViewpoint) LookupEntry(outpoint protos.OutPoint) *UtxoEntry {
	return view.entries[outpoint]
}

// AddEntry update entries via outpoint and entry.
func (view *UtxoViewpoint) AddEntry(outpoint protos.OutPoint, entry *UtxoEntry) {
	view.entries[outpoint] = entry
}

// RemoveEntry removes the given transaction output from the current state of
// the view.  It will have no effect if the passed output does not exist in the
// view.
func (view *UtxoViewpoint) RemoveEntry(outpoint protos.OutPoint) {
	delete(view.entries, outpoint)
}

// Entries returns the underlying map that stores of all the utxo entries.
func (view *UtxoViewpoint) Entries() map[protos.OutPoint]*UtxoEntry {
	return view.entries
}

// Commit prunes all entries marked modified that are now fully spent and marks
// all entries as unmodified.
func (view *UtxoViewpoint) Commit() {
	for outpoint, entry := range view.entries {
		if entry == nil || (entry.IsModified() && entry.IsSpent()) {
			delete(view.entries, outpoint)
			continue
		}

		entry.ResetModified()
	}
}

// Txs returns the underlying map that stores of all the txs.
func (view *UtxoViewpoint) Txs() map[common.Hash]TxMark {
	return view.txs
}

// TryGetSignAction returns action about a given block sign according to the
// current state of the view.  It will return ViewUnknown if the passed block
// sign does not exist in the view.
func (view *UtxoViewpoint) TryGetSignAction(sign *protos.MsgBlockSign) ViewAction {
	action, ok := view.signs[*sign]
	if ok {
		return action
	}
	return ViewUnknown
}


// addTxOut adds the specified output to the view if it is not provably
// unspendable.  When the view already has an entry for the output, it will be
// marked unspent.  All fields will be updated for existing entries since it's
// possible it has changed during a reorg.
func (view *UtxoViewpoint) AddTxOut(outpoint protos.OutPoint, txOut *protos.TxOut,
	isCoinBase bool, blockHeight int32, lockitem *LockItem) {
	// Don't add provably unspendable outputs.
	if txscript.IsUnspendable(txOut.PkScript) {
		return
	}

	if txOut.Value <= 0 {
		return
	}

	// Update existing entries.  All fields are updated because it's
	// possible (although extremely unlikely) that the existing entry is
	// being replaced by a different transaction with the same hash.  This
	// is allowed so long as the previous transaction is fully spent.
	entry := view.LookupEntry(outpoint)
	if entry == nil {
		entry = new(UtxoEntry)
		view.entries[outpoint] = entry
	}
	entry.Update(txOut.Value, txOut.PkScript, blockHeight, isCoinBase, &txOut.Asset, lockitem)
}

//// AddTxOut adds the specified output of the passed transaction to the view if
//// it exists and is not provably unspendable.  When the view already has an
//// entry for the output, it will be marked unspent.  All fields will be updated
//// for existing entries since it's possible it has changed during a reorg.
//func (view *UtxoViewpoint) AddTxOut(hash *common.Hash, tx *protos.MsgTx, isCoinBase bool,
//	txOutIdx uint32, blockHeight int32) {
//	// Can't add an output for an out of bounds index.
//	if txOutIdx >= uint32(len(tx.TxOut)) {
//		return
//	}
//
//	// Update existing entries.  All fields are updated because it's
//	// possible (although extremely unlikely) that the existing entry is
//	// being replaced by a different transaction with the same hash.  This
//	// is allowed so long as the previous transaction is fully spent.
//	prevOut := protos.OutPoint{Hash: *hash, Index: txOutIdx}
//	txOut := tx.TxOut[txOutIdx]
//	view.addTxOut(prevOut, txOut, isCoinBase, blockHeight, nil)
//}

// AddTxOuts adds all outputs in the passed transaction which are not provably
// unspendable to the view.  When the view already has entries for any of the
// outputs, they are simply marked unspent.  All fields will be updated for
// existing entries since it's possible it has changed during a reorg.
func (view *UtxoViewpoint) AddTxOuts(hash *common.Hash, tx *protos.MsgTx,
	isCoinBase bool, blockHeight int32) map[protos.Asset]*LockItem {
	// Loop all of the transaction outputs and add those which are not
	// provably unspendable.
	prevOut := protos.OutPoint{Hash: *hash}

	lockItems, feeLockItems := view.ConstructLockItems(tx, blockHeight)

	for txOutIdx, txOut := range tx.TxOut {
		// Update existing entries.  All fields are updated because it's
		// possible (although extremely unlikely) that the existing
		// entry is being replaced by a different transaction with the
		// same hash.  This is allowed so long as the previous
		// transaction is fully spent.
		prevOut.Index = uint32(txOutIdx)
		view.AddTxOut(prevOut, txOut, isCoinBase, blockHeight, lockItems[txOutIdx])
	}
	return feeLockItems
}

// AddViewTx updates the view by adding txs by the
// passed transaction.
func (view *UtxoViewpoint) AddViewTx(hash *common.Hash, tx *protos.MsgTx) {
	view.txs[*hash] = TxMark{
		MsgTx:  tx,
		Action: ViewAdd,
	}
}

// RemoveViewTx mark the passed transaction as ViewRm.
func (view *UtxoViewpoint) RemoveViewTx(hash *common.Hash) {
	view.txs[*hash] = TxMark{Action: ViewRm}
}

// AddViewSign updates the view by adding block sign by the
// passed transaction.
func (view *UtxoViewpoint) AddViewSign(sig *protos.MsgBlockSign) {
	view.signs[*sig] = ViewAdd
}

// mark sign as ViewRm.
func (view *UtxoViewpoint) RemoveViewSign(sig *protos.MsgBlockSign) {
	view.signs[*sig] = ViewRm
}

// NewUtxoViewpoint returns a new empty unspent transaction output view.
func NewUtxoViewpoint() *UtxoViewpoint {
	return  &UtxoViewpoint{
		entries: make(map[protos.OutPoint]*UtxoEntry),
		txs:     make(map[common.Hash]TxMark),
		signs:   make(map[protos.MsgBlockSign]ViewAction),
	}
}

// Outputs' lock items are inherited from inputs lock items.
// The rule is very simple: Order First. In the case that some asset's
// outputs amount is less than lock item amount, fee inherit the remain
// lock item.
// Note:
// 1. Each lock item for asset is independent.
// 2. Any lock item is not allowed exceed than input or output's amount.
//
//  Idx |       Input        |         Output     |   fee
//      | Amount | lock item | Amount | lock item | Amount | lock item |
// eg 1.
//   0  | 10000  |  10000    | 10000  |    10000  |
// eg 2.
//   0  | 10000  |  10000    | 30000  |    15000  |
//   1  | 20000  |  5000     |
// eg 3.
//   0  | 10000  |  10000    | 12000  |    12000  |
//   1  | 20000  |  5000     | 18000  |    3000   |
// eg 4.
//   0  | 10000  |  10000    | 2000   |    2000   | 1000   |    1000   |
//   1  |                    | 7000   |    7000   |
func (view *UtxoViewpoint) ConstructLockItems(tx *protos.MsgTx, nextHeight int32) ([]*LockItem, map[protos.Asset]*LockItem) {
	if len(tx.TxOut) == 0 {
		return nil, nil
	}
	temp := make(map[protos.Asset]*LockItem)
	for _, txin := range tx.TxIn {
		entry := view.LookupEntry(txin.PreviousOutPoint)
		if entry == nil {
			continue
		}
		if entry.LockItem() == nil || len(entry.LockItem().Entries) == 0 {
			continue
		}
		tempitem, ok := temp[*entry.Asset()]
		if !ok {
			tempitem = NewLockItem()
			temp[*entry.Asset()] = tempitem
		}
		for _, lockentry := range entry.LockItem().Entries {

			if tempentry, ok := tempitem.Entries[lockentry.Id]; ok {
				tempentry.Amount += lockentry.Amount
			} else {
				tempitem.Entries[lockentry.Id] = lockentry.Clone()
			}
		}
	}

	result := make([]*LockItem, len(tx.TxOut))
	for i, txout := range tx.TxOut {
		if txout.Value == 0 {
			continue
		}
		tempitem, ok := temp[txout.Asset]
		if !ok || len(tempitem.Entries) == 0 {
			continue
		}
		cloneitem := tempitem.Clone()
		for k, v := range cloneitem.Entries {
			if v.Amount > txout.Value {
				tempitem.Entries[k].Amount = v.Amount - txout.Value
				v.Amount = txout.Value
			} else {
				delete(tempitem.Entries, k)
			}
		}
		result[i] = cloneitem
	}

	for k, item := range temp {
		if len(item.Entries) == 0 {
			delete(temp, k)
		}
	}

	// Add self lock if it is votety
	firstOut := tx.TxOut[0]
	scriptClass, addrs, _, _ := txscript.ExtractPkScriptAddrs(firstOut.PkScript)
	if scriptClass == txscript.VoteTy && len(addrs) > 0 && len(firstOut.Data) > common.HashLength && !firstOut.Asset.IsIndivisible() {
		id := PickVoteArgument(firstOut.Data)
		voteId := NewVoteId(addrs[0].StandardAddress(), id)
		for i, txout := range tx.TxOut {
			if txout.Value > 0 && txout.Asset.Equal(&firstOut.Asset) {
				if result[i] == nil {
					result[i] = NewLockItem()
				}
				if lockentry, ok := result[i].Entries[*voteId]; ok {
					lockentry.Amount = txout.Value
				} else {
					result[i].Entries[*voteId] = &LockEntry{
						Id:     *voteId,
						Amount: txout.Value,
					}
				}
			}
		}
	}
	return result, temp
}

// PickVoteArgument return the vote id from contract data
// A standard vote tx request data contains at least one param which
// takes 32 bytes.
func PickVoteArgument(data []byte) uint32 {
	length := len(data)
	if length >= common.HashLength+ContractMethodLength {
		id := uint32(new(big.Int).SetBytes(data[ContractMethodLength : ContractMethodLength+common.HashLength]).Int64())
		return id
	}
	return 0
}
