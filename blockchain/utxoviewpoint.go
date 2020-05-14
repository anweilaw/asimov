// Copyright (c) 2018-2020. The asimov developers
// Copyright (c) 2013-2017 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.
package blockchain

import (
	"bytes"
	"github.com/AsimovNetwork/asimov/asiutil"
	"github.com/AsimovNetwork/asimov/blockchain/txo"
	"github.com/AsimovNetwork/asimov/common"
	"github.com/AsimovNetwork/asimov/database"
	"github.com/AsimovNetwork/asimov/protos"
	"github.com/AsimovNetwork/asimov/txscript"
	"math/big"
	"sort"
)

const (
	// solidity compiler pick four bytes for method mark.
	ContractMethodLength = 4
)

// UtxoViewpoint represents a view into the set of unspent transaction outputs
// from a specific point of view in the chain.  For example, it could be for
// the end of the main chain, some point in the history of the main chain, or
// down a side chain.
//
// The unspent outputs are needed by other transactions for things such as
// script validation and double spend prevention.
type UtxoViewpoint struct {
	entries  txo.UtxoMap
	bestHash common.Hash
	txs      map[common.Hash]asiutil.TxMark
	signs    map[protos.MsgBlockSign]asiutil.ViewAction
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

// LookupEntry returns information about a given transaction output according to
// the current state of the view.  It will return nil if the passed output does
// not exist in the view or is otherwise not available such as when it has been
// disconnected during a reorg.
func (view *UtxoViewpoint) LookupEntry(outpoint protos.OutPoint) *txo.UtxoEntry {
	return view.entries[outpoint]
}

// addTxOut adds the specified output to the view if it is not provably
// unspendable.  When the view already has an entry for the output, it will be
// marked unspent.  All fields will be updated for existing entries since it's
// possible it has changed during a reorg.
func (view *UtxoViewpoint) addTxOut(outpoint protos.OutPoint, txOut *protos.TxOut,
	isCoinBase bool, blockHeight int32, lockitem *txo.LockItem) {
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
		entry = new(txo.UtxoEntry)
		view.entries[outpoint] = entry
	}
	entry.Update(txOut.Value, txOut.PkScript, blockHeight, isCoinBase, &txOut.Asset, lockitem)
}

// AddTxOut adds the specified output of the passed transaction to the view if
// it exists and is not provably unspendable.  When the view already has an
// entry for the output, it will be marked unspent.  All fields will be updated
// for existing entries since it's possible it has changed during a reorg.
func (view *UtxoViewpoint) AddTxOut(tx *asiutil.Tx, txOutIdx uint32, blockHeight int32) {
	// Can't add an output for an out of bounds index.
	if txOutIdx >= uint32(len(tx.MsgTx().TxOut)) {
		return
	}

	// Update existing entries.  All fields are updated because it's
	// possible (although extremely unlikely) that the existing entry is
	// being replaced by a different transaction with the same hash.  This
	// is allowed so long as the previous transaction is fully spent.
	prevOut := protos.OutPoint{Hash: *tx.Hash(), Index: txOutIdx}
	txOut := tx.MsgTx().TxOut[txOutIdx]
	view.addTxOut(prevOut, txOut, IsCoinBase(tx), blockHeight, nil)
}

// AddTxOuts adds all outputs in the passed transaction which are not provably
// unspendable to the view.  When the view already has entries for any of the
// outputs, they are simply marked unspent.  All fields will be updated for
// existing entries since it's possible it has changed during a reorg.
func (view *UtxoViewpoint) AddTxOuts(tx *asiutil.Tx, blockHeight int32) map[protos.Asset]*txo.LockItem {
	// Loop all of the transaction outputs and add those which are not
	// provably unspendable.
	isCoinBase := IsCoinBase(tx)
	prevOut := protos.OutPoint{Hash: *tx.Hash()}

	lockItems, feeLockItems := view.ConstructLockItems(tx, blockHeight)

	for txOutIdx, txOut := range tx.MsgTx().TxOut {
		// Update existing entries.  All fields are updated because it's
		// possible (although extremely unlikely) that the existing
		// entry is being replaced by a different transaction with the
		// same hash.  This is allowed so long as the previous
		// transaction is fully spent.
		prevOut.Index = uint32(txOutIdx)
		view.addTxOut(prevOut, txOut, isCoinBase, blockHeight, lockItems[txOutIdx])
	}
	return feeLockItems
}

func (view *UtxoViewpoint) AddVtxOuts(tx *asiutil.Tx, blockHeight int32) {
	lockItems, _ := view.ConstructLockItems(tx, blockHeight)
	prevOut := protos.OutPoint{Hash: *tx.Hash()}
	for txOutIdx, txOut := range tx.MsgTx().TxOut {
		prevOut.Index = uint32(txOutIdx)
		view.addTxOut(prevOut, txOut, false, blockHeight, lockItems[txOutIdx])
	}
}

// ConnectTransaction updates the view by adding txs by the
// passed transaction.
func (view *UtxoViewpoint) connectTransaction(tx *asiutil.Tx) {
	view.txs[*tx.Hash()] = asiutil.TxMark{
		Tx:     tx,
		Action: asiutil.ViewAdd,
	}
}

// DisconnectTransactions updates the view by removing all of the transactions
// created by the passed block, restoring all utxos the transactions spent by
// using the provided spent txo information, and setting the best hash for the
// view to the block before the passed block.
func (view *UtxoViewpoint) disconnectTransactions(db database.Transactor, block *asiutil.Block, stxos []SpentTxOut,
	vblock *asiutil.VBlock) error {
	// Sanity check the correct number of stxos are provided.
	if block == nil {
		return common.AssertError("DisconnectTransactions called with bad block")
	}

	vtxsNum := countVtxSpentOutpus(vblock)
	if len(stxos) != countSpentOutputs(block)+vtxsNum {
		return common.AssertError("DisconnectTransactions called with bad " +
			"spent transaction out information")
	}

	if len(block.Signs()) > 0 {
		for _, sig := range block.Signs() {
			view.signs[*sig.MsgSign] = asiutil.ViewRm
		}
	}

	// Loop backwards through all transactions so everything is unspent in
	// reverse order.  This is necessary since transactions later in a block
	// can spend from previous ones.
	stxoIdx := len(stxos) - 1
	txs := block.Transactions()
	coinbaseIdx := len(txs) - 1
	coinbaseTx := txs[coinbaseIdx]
	for _, tx := range txs {
		view.txs[*tx.Hash()] = asiutil.TxMark{Action: asiutil.ViewRm}
	}
	vtxs := vblock.Transactions()
	transactions := asiutil.MergeTxVtx(txs, vtxs)
	for txIdx := len(transactions) - 1; txIdx > -1; txIdx-- {
		tx := transactions[txIdx]

		// All entries will need to potentially be marked as a coinbase.
		isCoinBase := tx == coinbaseTx

		// Mark all of the spendable outputs originally created by the
		// transaction as spent.  It is instructive to note that while
		// the outputs aren't actually being spent here, rather they no
		// longer exist, since a pruned utxo set is used, there is no
		// practical difference between a utxo that does not exist and
		// one that has been spent.
		//
		// When the utxo does not already exist in the view, add an
		// entry for it and then mark it spent.  This is done because
		// the code relies on its existence in the view in order to
		// signal modifications have happened.
		txHash := tx.Hash()
		prevOut := protos.OutPoint{Hash: *txHash}
		for txOutIdx, txOut := range tx.MsgTx().TxOut {
			if txscript.IsUnspendable(txOut.PkScript) {
				continue
			}

			prevOut.Index = uint32(txOutIdx)
			entry := view.entries[prevOut]
			if entry == nil {
				var amount int64
				amount = txOut.Value
				if amount == 0 {
					continue
				}

				entry = txo.NewUtxoEntry(amount, txOut.PkScript,
					block.Height(), isCoinBase, &txOut.Asset, nil)

				view.entries[prevOut] = entry
			}

			entry.Spend()
		}

		// Loop backwards through all of the transaction inputs (except
		// for the coinbase which has no inputs) and unspend the
		// referenced txos.  This is necessary to match the order of the
		// spent txout entries.
		if isCoinBase {
			continue
		}

		for txInIdx := len(tx.MsgTx().TxIn) - 1; txInIdx > -1; txInIdx-- {
			in := tx.MsgTx().TxIn[txInIdx]
			if asiutil.IsMintOrCreateInput(in) {
				continue
			}

			// Ensure the spent txout index is decremented to stay
			// in sync with the transaction input.
			stxo := &stxos[stxoIdx]
			stxoIdx--

			// When there is not already an entry for the referenced
			// output in the view, it means it was previously spent,
			// so create a new utxo entry in order to resurrect it.
			originOut := &tx.MsgTx().TxIn[txInIdx].PreviousOutPoint
			entry := view.entries[*originOut]
			if entry == nil {
				entry = new(txo.UtxoEntry)
				view.entries[*originOut] = entry
			}

			// Restore the utxo using the stxo data from the spend
			// journal and mark it as modified.
			entry.Update(stxo.Amount, stxo.PkScript, stxo.Height, stxo.IsCoinBase, stxo.Asset, nil)

			var lockItem *txo.LockItem
			err := db.View(func(dbTx database.Tx) error {
				var err error
				lockItem, err = dbFetchLockItem(dbTx, *originOut)
				if lockItem != nil {
					log.Info("DEBUG LOG NIL CRASH old lockitem active", originOut.String())
				}
				return err
			})
			if err != nil {
				return err
			}
			entry.SetLockItem(lockItem)
		}
	}

	// Update the best hash for view to the previous block since all of the
	// transactions for the current block have been disconnected.
	view.SetBestHash(&block.MsgBlock().Header.PrevBlock)
	return nil
}

// RemoveEntry removes the given transaction output from the current state of
// the view.  It will have no effect if the passed output does not exist in the
// view.
func (view *UtxoViewpoint) RemoveEntry(outpoint protos.OutPoint) {
	delete(view.entries, outpoint)
}

func (view *UtxoViewpoint) AddEntry(outpoint protos.OutPoint, entry *txo.UtxoEntry) {
	view.entries[outpoint] = entry
}

// Entries returns the underlying map that stores of all the utxo entries.
func (view *UtxoViewpoint) Entries() map[protos.OutPoint]*txo.UtxoEntry {
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

// FetchUtxosMain fetches unspent transaction output data about the provided
// set of outpoints from the point of view of the end of the main chain at the
// time of the call.
//
// Upon completion of this function, the view will contain an entry for each
// requested outpoint.  Spent outputs, or those which otherwise don't exist,
// will result in a nil entry in the view.
func (view *UtxoViewpoint) FetchUtxosMain(db database.Transactor, outpoints map[protos.OutPoint]struct{}) error {
	// Nothing to do if there are no requested outputs.
	if len(outpoints) == 0 {
		return nil
	}

	// Load the requested set of unspent transaction outputs from the point
	// of view of the end of the main chain.
	//
	// NOTE: Missing entries are not considered an error here and instead
	// will result in nil entries in the view.  This is intentionally done
	// so other code can use the presence of an entry in the store as a way
	// to unnecessarily avoid attempting to reload it from the database.
	return db.View(func(dbTx database.Tx) error {
		for outpoint := range outpoints {
			entry, err := dbFetchUtxoEntry(dbTx, outpoint)
			if err != nil {
				return err
			}

			view.entries[outpoint] = entry
		}

		return nil
	})
}

// fetchUtxos loads the unspent transaction outputs for the provided set of
// outputs into the view from the database as needed unless they already exist
// in the view in which case they are ignored.
func (view *UtxoViewpoint) fetchUtxos(db database.Transactor, outpoints map[protos.OutPoint]struct{}) error {
	// Nothing to do if there are no requested outputs.
	if len(outpoints) == 0 {
		return nil
	}

	// Filter entries that are already in the view.
	neededSet := make(map[protos.OutPoint]struct{})
	for outpoint := range outpoints {
		// Already loaded into the current view.
		if _, ok := view.entries[outpoint]; ok {
			continue
		}

		neededSet[outpoint] = struct{}{}
	}

	// Request the input utxos from the database.
	return view.FetchUtxosMain(db, neededSet)
}

// FetchInputUtxos loads the unspent transaction outputs for the inputs
// referenced by the transactions in the given block into the view from the
// database as needed.  In particular, referenced entries that are earlier in
// the block are added to the view and entries that are already in the view are
// not modified.
func (view *UtxoViewpoint) fetchInputUtxos(db database.Transactor, block *asiutil.Block) error {
	// Build a map of in-flight transactions because some of the inputs in
	// this block could be referencing other transactions earlier in this
	// block which are not yet in the chain.
	txInFlight := map[common.Hash]int{}
	transactions := block.Transactions()
	for i, tx := range transactions {
		txInFlight[*tx.Hash()] = i
	}

	// Loop through all of the transaction inputs (except for the coinbase
	// which has no inputs) collecting them into sets of what is needed and
	// what is already known (in-flight).
	neededSet := make(map[protos.OutPoint]struct{})
	coinbaseIdx := len(transactions) - 1
	for i, tx := range transactions[:coinbaseIdx] {
		for _, txIn := range tx.MsgTx().TxIn {
			// It is acceptable for a transaction input to reference
			// the output of another transaction in this block only
			// if the referenced transaction comes before the
			// current one in this block.  Add the outputs of the
			// referenced transaction as available utxos when this
			// is the case.  Otherwise, the utxo details are still
			// needed.
			//
			// NOTE: The >= is correct here because i is one less
			// than the actual position of the transaction within
			// the block due to skipping the coinbase.
			originHash := &txIn.PreviousOutPoint.Hash
			if inFlightIndex, ok := txInFlight[*originHash]; ok &&
				i >= inFlightIndex {

				originTx := transactions[inFlightIndex]
				view.AddTxOuts(originTx, block.Height())
				continue
			}

			// Don't request entries that are already in the view
			// from the database.
			if _, ok := view.entries[txIn.PreviousOutPoint]; ok {
				continue
			}

			neededSet[txIn.PreviousOutPoint] = struct{}{}
		}
	}

	// Request the input utxos from the database.
	return view.FetchUtxosMain(db, neededSet)
}

// NewUtxoViewpoint returns a new empty unspent transaction output view.
func NewUtxoViewpoint() *UtxoViewpoint {
	return  &UtxoViewpoint{
		entries: make(map[protos.OutPoint]*txo.UtxoEntry),
		txs:     make(map[common.Hash]asiutil.TxMark),
		signs:   make(map[protos.MsgBlockSign]asiutil.ViewAction),
	}
}

// FetchUtxoView loads unspent transaction outputs for the inputs referenced by
// the passed transaction from the point of view of the end of the main chain.
// It also attempts to fetch the utxos for the outputs of the transaction itself
// so the returned view can be examined for duplicate transactions.
//
// This function is safe for concurrent access however the returned view is NOT.
func (b *BlockChain) FetchUtxoView(tx *asiutil.Tx, dolock bool) (*UtxoViewpoint, error) {
	// Create a set of needed outputs based on those referenced by the
	// inputs of the passed transaction and the outputs of the transaction
	// itself.
	neededSet := make(map[protos.OutPoint]struct{})
	prevOut := protos.OutPoint{Hash: *tx.Hash()}
	for txOutIdx := range tx.MsgTx().TxOut {
		prevOut.Index = uint32(txOutIdx)
		neededSet[prevOut] = struct{}{}
	}
	if !IsCoinBase(tx) {
		for _, txIn := range tx.MsgTx().TxIn {
			neededSet[txIn.PreviousOutPoint] = struct{}{}
		}
	}

	// Request the utxos from the point of view of the end of the main
	// chain.
	view := NewUtxoViewpoint()
	if dolock {
		b.chainLock.RLock()
	}
	err := view.FetchUtxosMain(b.db, neededSet)
	if dolock {
		b.chainLock.RUnlock()
	}
	return view, err
}

// FetchUtxoEntry loads and returns the requested unspent transaction output
// from the point of view of the end of the main chain.
//
// NOTE: Requesting an output for which there is no data will NOT return an
// error.  Instead both the entry and the error will be nil.  This is done to
// allow pruning of spent transaction outputs.  In practice this means the
// caller must check if the returned entry is nil before invoking methods on it.
//
// This function is safe for concurrent access however the returned entry (if
// any) is NOT.
func (b *BlockChain) FetchUtxoEntry(outpoint protos.OutPoint) (*txo.UtxoEntry, error) {
	b.chainLock.RLock()
	defer b.chainLock.RUnlock()

	return b.fetchUtxoEntry(outpoint)
}

func (b *BlockChain) fetchUtxoEntry(outpoint protos.OutPoint) (*txo.UtxoEntry, error) {

	var entry *txo.UtxoEntry
	err := b.db.View(func(dbTx database.Tx) error {
		var err error
		entry, err = dbFetchUtxoEntry(dbTx, outpoint)
		return err
	})
	if err != nil {
		return nil, err
	}

	return entry, nil
}

func (b *BlockChain) FetchUtxoViewByAddressAndAsset(view *UtxoViewpoint, address []byte, asset *protos.Asset) (*[]protos.OutPoint, error) {
	b.chainLock.RLock()
	defer b.chainLock.RUnlock()
	return b.fetchUtxoViewByAddressAndAsset(view, address, asset)
}

func (b *BlockChain) fetchUtxoViewByAddressAndAsset(view *UtxoViewpoint, address []byte, asset *protos.Asset) (*[]protos.OutPoint, error) {

	err := b.db.View(func(dbTx database.Tx) error {
		entryPairs, err := dbFetchBalance(dbTx, address)
		if err != nil {
			return err
		}
		for _, entryPair := range *entryPairs {
			outpoint := entryPair.Key
			view.entries[outpoint] = entryPair.Value
		}
		return err
	})
	if err != nil {
		return nil, err
	}

	//check the items already in the global view.
	var assetEntries EntryPairList
	for out, entry := range view.entries {
		if entry == nil || entry.IsSpent() {
			continue
		}
		if entry.Asset().Equal(asset) {
			assetEntries = append(assetEntries, EntryPair{out, entry})
		}
	}

	sort.Sort(assetEntries)
	outpoints := make([]protos.OutPoint, 0, len(assetEntries))
	for _, data := range assetEntries {
		outpoints = append(outpoints, data.Key)
	}

	return &outpoints, nil
}

func (b *BlockChain) FetchUtxoViewByAddress(view *UtxoViewpoint, address []byte) (*[]protos.OutPoint, error) {
	b.chainLock.RLock()
	defer b.chainLock.RUnlock()

	return b.fetchUtxoViewByAddress(view, address)
}

func (b *BlockChain) fetchUtxoViewByAddress(view *UtxoViewpoint, address []byte) (*[]protos.OutPoint, error) {
	if view == nil {
		return nil, common.AssertError("param is invalid")
	}

	var outpoints []protos.OutPoint
	err := b.db.View(func(dbTx database.Tx) error {

		entryPairs, err := dbFetchBalance(dbTx, address)
		if err != nil {
			return err
		}

		for _, entryPair := range *entryPairs {
			outpoint := entryPair.Key
			view.entries[outpoint] = entryPair.Value

			outpoints = append(outpoints, outpoint)
		}

		return err
	})
	if err != nil {
		return nil, err
	}

	return &outpoints, nil
}

//cache the balance for contract address which will be used to validate the "transfer" inside contract execution.
func (b *BlockChain) updateBalanceCache(block *asiutil.Block, view *UtxoViewpoint, tx *asiutil.Tx) {

	if IsCoinBase(tx) {
		return
	}

	for _, in := range tx.MsgTx().TxIn {
		usedEntry := view.LookupEntry(in.PreviousOutPoint)
		if usedEntry != nil {
			_, addrs, _, err := txscript.ExtractPkScriptAddrs(usedEntry.PkScript())
			if err != nil || len(addrs) <= 0 {
				continue
			}

			standardAddress := addrs[0].StandardAddress()

			balance := block.GetBalance(standardAddress)
			if balance == nil {
				balance = block.GetBalanceView(standardAddress)
				if balance == nil {
					utxomap := make(txo.UtxoMap)
					balance = &utxomap
				}
				block.PutBalanceView(standardAddress, balance)
			}

			(*balance)[in.PreviousOutPoint] = usedEntry
		}
	}

	outpoint := protos.OutPoint{Hash: *tx.Hash()}
	for txOutIdx, out := range tx.MsgTx().TxOut {
		if txscript.IsUnspendable(out.PkScript) {
			continue
		}

		if out.Value <= 0 {
			continue
		}

		outpoint.Index = uint32(txOutIdx)

		_, addrs, _, err := txscript.ExtractPkScriptAddrs(out.PkScript)
		if err != nil || len(addrs) <= 0 {
			continue
		}

		standardAddress := addrs[0].StandardAddress()

		balance := block.GetBalance(standardAddress)
		if balance == nil {
			balance = block.GetBalanceView(standardAddress)
			if balance == nil {
				utxomap := make(txo.UtxoMap)
				balance = &utxomap
			}
			block.PutBalanceView(standardAddress, balance)
		}

		entry := (*balance)[outpoint]
		if entry == nil {
			entry = new(txo.UtxoEntry)
			(*balance)[outpoint] = entry
		}
		entry.Update(out.Value, out.PkScript, block.Height(), IsCoinBase(tx), &out.Asset, nil)
	}
}

//fetch, sort the asset of certain address.
func (b *BlockChain) fetchAssetBalance(block *asiutil.Block, view *UtxoViewpoint, address common.Address, asset *protos.Asset) ([]protos.OutPoint, error) {
	balance, err := b.fetchAssetByAddress(block, nil, address, asset)
	if err != nil {
		return nil, err
	}
	var assetEntries EntryPairList

	//check the items already in the global view.
	for out, entry := range view.entries {
		if entry == nil || entry.IsSpent() {
			continue
		}
		if !entry.Asset().Equal(asset) {
			continue
		}
		_, addrs, _, err := txscript.ExtractPkScriptAddrs(entry.PkScript())
		if err != nil || len(addrs) <= 0 {
			continue
		}
		if bytes.Equal(address[:], addrs[0].ScriptAddress()) {
			assetEntries = append(assetEntries, EntryPair{out, entry})
		}
	}

	for outpoint, entry := range *balance {
		if entry.IsSpent() {
			continue
		}

		if entry.Asset().Equal(asset) {
			viewEntry := view.entries[outpoint]
			if viewEntry == nil {
				view.entries[outpoint] = entry
				assetEntries = append(assetEntries, EntryPair{outpoint, entry})
			}
		}
	}

	sort.Sort(assetEntries)
	outpoints := make([]protos.OutPoint, 0, len(assetEntries))
	for _, data := range assetEntries {
		outpoints = append(outpoints, data.Key)
	}

	return outpoints, nil
}

func (b *BlockChain) fetchAssetByAddress(block *asiutil.Block, view *UtxoViewpoint, address common.Address, asset *protos.Asset) (*txo.UtxoMap, error) {
	balance := block.GetBalance(address)
	if balance == nil {
		var entryPairs *EntryPairList
		var err error
		err = b.db.View(func(dbTx database.Tx) error {
			entryPairs, err = dbFetchBalance(dbTx, address[:])
			return err
		})

		if err != nil {
			return nil, err
		}

		balance = &txo.UtxoMap{}
		for _, entryPair := range *entryPairs {
			outpoint := entryPair.Key
			entry := entryPair.Value

			if (*balance)[outpoint] != nil {
				continue
			}

			(*balance)[outpoint] = entry
		}
		bv := block.GetBalanceView(address)
		if bv != nil {
			for outpoint, entry := range *bv {
				(*balance)[outpoint] = entry
			}
		}
		block.PutBalance(address, balance)
	}

	if view != nil {
		for outpoint, entry := range *balance {
			if entry.IsSpent() {
				continue
			}

			if asset == nil || entry.Asset().Equal(asset) {
				if _, ok := view.entries[outpoint]; !ok {
					view.entries[outpoint] = entry
				}
			}
		}

		for outpoint, entry := range view.entries {
			if entry.IsSpent() {
				if _, ok := (*balance)[outpoint]; ok {
					(*balance)[outpoint] = entry
				}
			}
		}
	}

	return balance, nil
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
func (view *UtxoViewpoint) ConstructLockItems(tx *asiutil.Tx, nextHeight int32) ([]*txo.LockItem, map[protos.Asset]*txo.LockItem) {
	if len(tx.MsgTx().TxOut) == 0 {
		return nil, nil
	}
	temp := make(map[protos.Asset]*txo.LockItem)
	for _, txin := range tx.MsgTx().TxIn {
		entry := view.LookupEntry(txin.PreviousOutPoint)
		if entry == nil {
			continue
		}
		if entry.LockItem() == nil || len(entry.LockItem().Entries) == 0 {
			continue
		}
		tempitem, ok := temp[*entry.Asset()]
		if !ok {
			tempitem = txo.NewLockItem()
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

	result := make([]*txo.LockItem, len(tx.MsgTx().TxOut))
	for i, txout := range tx.MsgTx().TxOut {
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
	firstOut := tx.MsgTx().TxOut[0]
	scriptClass, addrs, _, _ := txscript.ExtractPkScriptAddrs(firstOut.PkScript)
	if scriptClass == txscript.VoteTy && len(addrs) > 0 && len(firstOut.Data) > common.HashLength && !firstOut.Asset.IsIndivisible() {
		id := pickVoteArgument(firstOut.Data)
		voteId := txo.NewVoteId(addrs[0].StandardAddress(), id)
		for i, txout := range tx.MsgTx().TxOut {
			if txout.Value > 0 && txout.Asset.Equal(&firstOut.Asset) {
				if result[i] == nil {
					result[i] = txo.NewLockItem()
				}
				if lockentry, ok := result[i].Entries[*voteId]; ok {
					lockentry.Amount = txout.Value
				} else {
					result[i].Entries[*voteId] = &txo.LockEntry{
						Id:     *voteId,
						Amount: txout.Value,
					}
				}
			}
		}
	}
	return result, temp
}

// pickVoteArgument return the vote id from contract data
// A standard vote tx request data contains at least one param which
// takes 32 bytes.
func pickVoteArgument(data []byte) uint32 {
	length := len(data)
	if length >= common.HashLength+ContractMethodLength {
		id := uint32(new(big.Int).SetBytes(data[ContractMethodLength : ContractMethodLength+common.HashLength]).Int64())
		return id
	}
	return 0
}
