// Copyright (c) 2018-2020. The asimov developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.
package blockchain

import (
	"bytes"
	"fmt"
	"github.com/AsimovNetwork/asimov/asiutil"
	"github.com/AsimovNetwork/asimov/blockchain/txo"
	"github.com/AsimovNetwork/asimov/common"
	"github.com/AsimovNetwork/asimov/database"
	"github.com/AsimovNetwork/asimov/protos"
	"github.com/AsimovNetwork/asimov/txscript"
	"sort"
)

// fetch the balance of one asset for address,
// if it's erc721, return voucher id if there's voucherId in the balance and voucherId > 0.
// otherwise return the count of asset.
func (b *BlockChain) CalculateBalance(view *txo.UtxoViewpoint, block *asiutil.Block, address common.Address, asset *protos.Asset, voucherId int64) (int64, error) {
	balanceMap, err := b.fetchAssetByAddress(block, view, address)
	if err != nil {
		return 0, err
	}
	if asset == nil {
		return 0, nil
	}

	if !asset.IsIndivisible() {
		var balance int64
		for _, entry := range *balanceMap {
			if !entry.IsSpent() {
				balance += entry.Amount()
			}
		}
		fmt.Println("DEBUG LOG NIL CRASH CalculateBalance", address.String(), asset.String(), voucherId, balance)

		return balance, nil

	} else {
		if voucherId > 0 {
			for _, entry := range *balanceMap {
				if entry.Amount() == voucherId && !entry.IsSpent() {
					return voucherId, nil
				}
			}
			return 0, nil

		} else {
			count := int64(0)
			for _, entry := range *balanceMap {
				if entry.Amount() > 0 && !entry.IsSpent() {
					count++
				}
			}

			return count, nil
		}
	}
}


// FetchUtxoView loads unspent transaction outputs for the inputs referenced by
// the passed transaction from the point of view of the end of the main chain.
// It also attempts to fetch the utxos for the outputs of the transaction itself
// so the returned view can be examined for duplicate transactions.
//
// This function is safe for concurrent access however the returned view is NOT.
func (b *BlockChain) FetchUtxoView(tx *asiutil.Tx, dolock bool) (*txo.UtxoViewpoint, error) {
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
	view := txo.NewUtxoViewpoint()
	if dolock {
		b.chainLock.RLock()
	}
	err := FetchUtxosMain(view, b.db, neededSet)
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

func (b *BlockChain) FetchUtxoViewByAddressAndAsset(view *txo.UtxoViewpoint, address []byte, asset *protos.Asset) (*[]protos.OutPoint, error) {
	b.chainLock.RLock()
	defer b.chainLock.RUnlock()
	return b.fetchUtxoViewByAddressAndAsset(view, address, asset)
}

func (b *BlockChain) fetchUtxoViewByAddressAndAsset(view *txo.UtxoViewpoint, address []byte, asset *protos.Asset) (*[]protos.OutPoint, error) {

	err := b.db.View(func(dbTx database.Tx) error {
		entryPairs, err := dbFetchBalance(dbTx, address)
		if err != nil {
			return err
		}
		for _, entryPair := range *entryPairs {
			view.AddEntry(entryPair.Key, entryPair.Value)
		}
		return err
	})
	if err != nil {
		return nil, err
	}

	//check the items already in the global view.
	var assetEntries EntryPairList
	for out, entry := range view.Entries() {
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

func (b *BlockChain) FetchUtxoViewByAddress(view *txo.UtxoViewpoint, address []byte) (*[]protos.OutPoint, error) {
	b.chainLock.RLock()
	defer b.chainLock.RUnlock()

	return b.fetchUtxoViewByAddress(view, address)
}

func (b *BlockChain) fetchUtxoViewByAddress(view *txo.UtxoViewpoint, address []byte) (*[]protos.OutPoint, error) {
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
			view.AddEntry(entryPair.Key, entryPair.Value)

			outpoints = append(outpoints, entryPair.Key)
		}

		return err
	})
	if err != nil {
		return nil, err
	}

	return &outpoints, nil
}

//cache the balance for contract address which will be used to validate the "transfer" inside contract execution.
func (b *BlockChain) updateBalanceCache(block *asiutil.Block, view *txo.UtxoViewpoint, tx *asiutil.Tx) {

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
			if balance != nil {
				(*balance)[in.PreviousOutPoint] = usedEntry
			}
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
		if balance != nil {
			entry := view.LookupEntry(outpoint)
			if entry != nil {
				(*balance)[outpoint] = entry
			}
		}
	}
}

//fetch, sort the asset of certain address.
func (b *BlockChain) fetchAssetBalance(block *asiutil.Block, view *txo.UtxoViewpoint, address common.Address, asset *protos.Asset) ([]protos.OutPoint, error) {
	balance, err := b.fetchAssetByAddress(block, view, address)
	if err != nil {
		return nil, err
	}
	var assetEntries EntryPairList

	//check the items already in the global view.
	for out, entry := range view.Entries() {
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
			viewEntry := view.LookupEntry(outpoint)
			if viewEntry == nil {
				view.AddEntry(outpoint, entry)
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

func (b *BlockChain) fetchAssetByAddress(block *asiutil.Block, view *txo.UtxoViewpoint,
	address common.Address) (*txo.UtxoMap, error) {
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
			if view != nil && view.Exist(entryPair.Key) {
				entry := view.LookupEntry(entryPair.Key)
				if !entry.IsSpent() {
					(*balance)[entryPair.Key] = entry
				}
				continue
			} else {
				entry := entryPair.Value
				(*balance)[entryPair.Key] = entry
				if view != nil {
					view.AddEntry(entryPair.Key, entry)
				}
			}
		}
		block.PutBalance(address, balance)
	}

	if view != nil {
		for outpoint, entry := range view.Entries() {
			if entry.IsSpent() {
				continue
			}
			_, addrs, _, _ := txscript.ExtractPkScriptAddrs(entry.PkScript())
			if len(addrs) > 0 && addrs[0].StandardAddress() == address {
				(*balance)[outpoint] = entry
			}
		}
	}

	return balance, nil
}

