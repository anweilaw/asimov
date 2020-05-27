// Copyright (c) 2018-2020. The asimov developers
// Copyright (c) 2013-2017 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.
package blockchain

import (
	"fmt"
	"github.com/AsimovNetwork/asimov/asiutil"
	"github.com/AsimovNetwork/asimov/blockchain/txo"
	"github.com/AsimovNetwork/asimov/common"
	"github.com/AsimovNetwork/asimov/crypto"
	"github.com/AsimovNetwork/asimov/database"
	"github.com/AsimovNetwork/asimov/protos"
	"github.com/AsimovNetwork/asimov/txscript"
)


// connectTransaction updates the view by adding all new utxos created by the
// passed transaction and marking all utxos that the transactions spend as
// spent.  In addition, when the 'stxos' argument is not nil, it will be updated
// to append an entry for each spent txout.  An error will be returned if the
// view does not contain the required utxos.
func connectTransaction(view *txo.UtxoViewpoint, tx *asiutil.Tx, blockHeight int32, stxos *[]txo.SpentTxOut) (
	map[protos.Asset]*txo.LockItem, error) {
	// Coinbase transactions don't have any inputs to spend.
	if IsCoinBase(tx) {
		// Add the transaction's outputs as available utxos.
		feeLockItems := view.AddTxOuts(tx.Hash(), tx.MsgTx(), true, blockHeight)
		return feeLockItems, nil
	}

	// Spend the referenced utxos by marking them spent in the view and,
	// if a slice was provided for the spent txout details, append an entry
	// to it.
	for _, txIn := range tx.MsgTx().TxIn {
		// Ensure the referenced utxo exists in the view.  This should
		// never happen unless there is a bug is introduced in the code.
		entry := view.LookupEntry(txIn.PreviousOutPoint)
		if entry == nil {
			return nil, common.AssertError(fmt.Sprintf("view missing input %v",
				txIn.PreviousOutPoint))
		}

		// Only create the stxo details if requested.
		if stxos != nil {
			// Populate the stxo details using the utxo entry.
			var stxo = txo.SpentTxOut{
				Amount:     entry.Amount(),
				PkScript:   entry.PkScript(),
				Height:     entry.BlockHeight(),
				IsCoinBase: entry.IsCoinBase(),
				Asset:      entry.Asset(),
			}
			*stxos = append(*stxos, stxo)
		}

		// Mark the entry as spent.  This is not done until after the
		// relevant details have been accessed since spending it might
		// clear the fields from memory in the future.
		entry.Spend()
	}

	// Add the transaction's outputs as available utxos.
	feeLockItems := view.AddTxOuts(tx.Hash(), tx.MsgTx(), false, blockHeight)

	txOut := tx.MsgTx().TxOut[0]
	if txOut.Value > 0 {
		scriptClass, addrs, _, _ := txscript.ExtractPkScriptAddrs(txOut.PkScript)
		if scriptClass == txscript.CreateTy || scriptClass == txscript.TemplateTy {
			var contractAddr common.IAddress
			if scriptClass == txscript.TemplateTy {
				contractAddr = addrs[0]
			} else {
				inputHash := tx.GetInputHash()
				txIn := tx.MsgTx().TxIn[0]
				entry := view.LookupEntry(txIn.PreviousOutPoint)
				_, addrs, _, _ := txscript.ExtractPkScriptAddrs(entry.PkScript())
				caller := addrs[0].StandardAddress()
				newAddr, _ := crypto.CreateContractAddress(caller[:], []byte{}, inputHash)
				contractAddr = &newAddr
			}
			// modify the utxo owner to the new contract address.
			prevout := protos.NewOutPoint(tx.Hash(), 0)
			utxo := view.LookupEntry(*prevout)
			if utxo != nil {
				pkScript, _ := txscript.PayToAddrScript(contractAddr)
				utxo.SetPkScript(pkScript)
			}
			log.Info("CREATE TX REPLACE ADDRESS IN VIEW ", contractAddr.String())
		}
	}
	return feeLockItems, nil
}

// connectTransactions updates the view by adding all new utxos created by all
// of the transactions in the passed block, marking all utxos the transactions
// spend as spent, and setting the best hash for the view to the passed block.
// In addition, when the 'stxos' argument is not nil, it will be updated to
// append an entry for each spent txout.
func connectTransactions(view *txo.UtxoViewpoint, db database.Transactor,
	block *asiutil.Block, vblock *asiutil.VBlock,
	stxos *[]txo.SpentTxOut) error {
    vIdx := 0
    vTxs := vblock.Transactions()
    vLen := len(vTxs)

    var totalFeeLockItems map[protos.Asset]*txo.LockItem
	for i, tx := range block.Transactions() {
		feeLockItems, err := connectTransaction(view, tx, block.Height(), stxos)
		if err != nil {
			return err
		}
		if totalFeeLockItems == nil {
			totalFeeLockItems = feeLockItems
		} else if feeLockItems != nil {
			for k, item := range feeLockItems {
				if titem, ok := totalFeeLockItems[k]; ok {
					titem.Merge(item)
				} else {
					totalFeeLockItems[k] = item
				}
			}
		}
		var vtx *asiutil.Tx
		for {
			if vIdx >= vLen || vTxs[vIdx].MsgTx().Version > uint32(i) {
				break
			}
			if vTxs[vIdx].MsgTx().Version == uint32(i) {
				vtx = vTxs[vIdx]
				break
			}
			vIdx++
		}
		if vtx != nil {
			err = fetchVtxInputUtxos(view, db, vtx)
			if err != nil {
				return err
			}
			for _, txIn := range vtx.MsgTx().TxIn {
				// Ensure the referenced utxo exists in the view.  This should
				// never happen unless there is a bug is introduced in the code.
				isVtxMint := asiutil.IsMintOrCreateInput(txIn)
				if isVtxMint {
					continue
				}

				entry := view.LookupEntry(txIn.PreviousOutPoint)
				if entry == nil || entry.IsSpent() {
					str := fmt.Sprintf("the entry of input PreviousOutPoint is nil or entry has been spent %v", txIn.PreviousOutPoint)
					return ruleError(ErrMissingTxOut, str)
				}

				// Only create the stxo details if requested.
				if stxos != nil {
					// Populate the stxo details using the utxo entry.
					var stxo = txo.SpentTxOut{
						Amount:     entry.Amount(),
						PkScript:   entry.PkScript(),
						Height:     entry.BlockHeight(),
						IsCoinBase: entry.IsCoinBase(),
						Asset:      entry.Asset(),
					}
					*stxos = append(*stxos, stxo)
				}

				// Mark the entry as spent.  This is not done until after the
				// relevant details have been accessed since spending it might
				// clear the fields from memory in the future.
				entry.Spend()
			}
			view.AddTxOuts(vtx.Hash(), vtx.MsgTx(), false, block.Height())
		}
	}

	// Update the best hash for view to include this block since all of its
	// transactions have been connected.
	view.SetBestHash(block.Hash())
	updateFeeLockItems(block, view, totalFeeLockItems)

	return nil
}

// DisconnectTransactions updates the view by removing all of the transactions
// created by the passed block, restoring all utxos the transactions spent by
// using the provided spent txo information, and setting the best hash for the
// view to the block before the passed block.
func disconnectTransactions(view *txo.UtxoViewpoint, db database.Transactor,
	block *asiutil.Block, stxos []txo.SpentTxOut,
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
			view.RemoveViewSign(sig.MsgSign)
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
		view.RemoveViewTx(tx.Hash())
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
			entry := view.LookupEntry(prevOut)
			if entry == nil {
				var amount int64
				amount = txOut.Value
				if amount == 0 {
					continue
				}

				entry = txo.NewUtxoEntry(amount, txOut.PkScript,
					block.Height(), isCoinBase, &txOut.Asset, nil)

				view.AddEntry(prevOut, entry)
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
			entry := view.LookupEntry(*originOut)
			if entry == nil {
				entry = new(txo.UtxoEntry)
				view.AddEntry(*originOut, entry)
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

// FetchUtxosMain fetches unspent transaction output data about the provided
// set of outpoints from the point of view of the end of the main chain at the
// time of the call.
//
// Upon completion of this function, the view will contain an entry for each
// requested outpoint.  Spent outputs, or those which otherwise don't exist,
// will result in a nil entry in the view.
func FetchUtxosMain(view *txo.UtxoViewpoint, db database.Transactor, outpoints map[protos.OutPoint]struct{}) error {
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

			view.AddEntry(outpoint, entry)
		}

		return nil
	})
}

// fetchUtxos loads the unspent transaction outputs for the provided set of
// outputs into the view from the database as needed unless they already exist
// in the view in which case they are ignored.
func fetchUtxos(view *txo.UtxoViewpoint, db database.Transactor, outpoints map[protos.OutPoint]struct{}) error {
	// Nothing to do if there are no requested outputs.
	if len(outpoints) == 0 {
		return nil
	}

	// Filter entries that are already in the view.
	neededSet := make(map[protos.OutPoint]struct{})
	for outpoint := range outpoints {
		// Already loaded into the current view.
		if view.Exist(outpoint) {
			continue
		}

		neededSet[outpoint] = struct{}{}
	}

	// Request the input utxos from the database.
	return FetchUtxosMain(view, db, neededSet)
}

// FetchInputUtxos loads the unspent transaction outputs for the inputs
// referenced by the transactions in the given block into the view from the
// database as needed.  In particular, referenced entries that are earlier in
// the block are added to the view and entries that are already in the view are
// not modified.
func fetchInputUtxos(view *txo.UtxoViewpoint, db database.Transactor, block *asiutil.Block) error {
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
				view.AddTxOuts(originTx.Hash(), originTx.MsgTx(), false, block.Height())
				continue
			}

			// Don't request entries that are already in the view
			// from the database.
			if view.Exist(txIn.PreviousOutPoint) {
				continue
			}

			neededSet[txIn.PreviousOutPoint] = struct{}{}
		}
	}

	// Request the input utxos from the database.
	return FetchUtxosMain(view, db, neededSet)
}

// fetchVtxInputUtxos loads the unspent transaction outputs for the inputs
// referenced by the transactions in the given tx into the view from the
// database as needed.  In particular, referenced entries that are already
// in the view are not modified.
func fetchVtxInputUtxos(view *txo.UtxoViewpoint, db database.Transactor, vtx *asiutil.Tx) error {
	neededSet := make(map[protos.OutPoint]struct{})
	for _, txIn := range vtx.MsgTx().TxIn {
		isVtxMint := asiutil.IsMintOrCreateInput(txIn)
		if isVtxMint {
			continue
		}
		// Don't request entries that are already in the view
		// from the database.
		if view.Exist(txIn.PreviousOutPoint) {
			continue
		}
		neededSet[txIn.PreviousOutPoint] = struct{}{}
	}

	// Request the input utxos from the database.
	return FetchUtxosMain(view, db, neededSet)
}
