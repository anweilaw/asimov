// Copyright (c) 2018-2020. The asimov developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.
package indexers

import (
	"fmt"
	"github.com/AsimovNetwork/asimov/asiutil"
	"github.com/AsimovNetwork/asimov/blockchain"
	"github.com/AsimovNetwork/asimov/blockchain/txo"
	"github.com/AsimovNetwork/asimov/common"
	"github.com/AsimovNetwork/asimov/common/hexutil"
	"github.com/AsimovNetwork/asimov/database"
	"github.com/AsimovNetwork/asimov/protos"
	"github.com/AsimovNetwork/asimov/txscript"
)

const (
	// templateIndexName is the human-readable name for the index.
	templateIndexName = "template index"
)

var (
	// templateIndexKey is the key of the template index and the db bucket used
	// to house it.
	templateIndexKey = []byte("templatebyhashidx")
)

// TemplateIndex implements a transaction by hash index.  That is to say, it supports
// querying all templates by their hash.
type TemplateIndex struct {
	db         database.Transactor
	curBlockID uint32
}

// putTemplateIndexEntry serializes the provided values according to the format
// described about for a template index entry.  The target byte slice must
// be at least large enough to handle the number of bytes defined by the
// txTemplateSize constant or it will panic.
func putTemplateIndexEntry(target []byte, blockID uint32, tx *asiutil.Tx, txLoc protos.TxLoc) {
	byteOrder.PutUint32(target, blockID)
	dataLoc := tx.MsgTx().DataLoc()
	offset := 4 // blockID takes 4 bytes
	byteOrder.PutUint32(target[offset:], uint32(txLoc.TxStart+dataLoc))
	offset += 4 // template data location takes 4 bytes
	byteOrder.PutUint32(target[offset:], uint32(len(tx.MsgTx().TxOut[0].Data)))
}

// dbPutTemplateIndexEntry uses an existing database template to update the
// template index given the provided serialized data that is expected to have
// been serialized putTemplateIndexEntry.
func dbPutTemplateIndexEntry(dbTx database.Tx, templateHash *common.Hash, serializedData []byte) error {
	index := dbTx.Metadata().Bucket(templateIndexKey)
	return index.Put(templateHash[:], serializedData)
}

// Ensure the TxIndex type implements the Indexer interface.
var _ blockchain.Indexer = (*TemplateIndex)(nil)

func (idx *TemplateIndex) Init() error {
	// Find the latest known block id field for the internal block id
	// index and initialize it.  This is done because it's a lot more
	// efficient to do a single search at initialize time than it is to
	// write another value to the database on every update.
	err := idx.db.View(func(dbTx database.Tx) error {
		// Scan forward in large gaps to find a block id that doesn't
		// exist yet to serve as an upper bound for the binary search
		// below.
		var highestKnown, nextUnknown uint32
		testBlockID := uint32(1)
		increment := uint32(100000)
		for {
			_, err := dbFetchBlockHashByID(dbTx, testBlockID)
			if err != nil {
				nextUnknown = testBlockID
				break
			}

			highestKnown = testBlockID
			testBlockID += increment
		}
		log.Tracef("Forward scan (highest known %d, next unknown %d)",
			highestKnown, nextUnknown)

		// No used block IDs due to new database.
		if nextUnknown == 1 {
			return nil
		}

		// Use a binary search to find the final highest used block id.
		// This will take at most ceil(log_2(increment)) attempts.
		for {
			testBlockID = (highestKnown + nextUnknown) / 2
			_, err := dbFetchBlockHashByID(dbTx, testBlockID)
			if err != nil {
				nextUnknown = testBlockID
			} else {
				highestKnown = testBlockID
			}
			log.Tracef("Binary scan (highest known %d, next "+
				"unknown %d)", highestKnown, nextUnknown)
			if highestKnown+1 == nextUnknown {
				break
			}
		}

		idx.curBlockID = highestKnown
		return nil
	})
	if err != nil {
		return err
	}

	log.Debugf("Current internal block ID: %d", idx.curBlockID)
	return nil
}

// Key returns the database key to use for the index as a byte slice.
//
// This is part of the Indexer interface.
func (idx *TemplateIndex) Key() []byte {
	return templateIndexKey
}

// Name returns the human-readable name of the index.
//
// This is part of the Indexer interface.
func (idx *TemplateIndex) Name() string {
	return templateIndexName
}

// Create is invoked when the indexer manager determines the index needs
// to be created for the first time.  It creates the buckets for the hash-based
// transaction index and the internal block ID indexes.
//
// This is part of the Indexer interface.
func (idx *TemplateIndex) Create(dbTx database.Tx) error {
	meta := dbTx.Metadata()
	if _, err := meta.CreateBucket(templateIndexKey); err != nil {
		return err
	}

	return nil
}

// Checking if there is a new bucket added to this indexer
// This method is invoked each time when node started.
func (idx *TemplateIndex) Check(dbTx database.Tx) error {
	meta := dbTx.Metadata()
	if _, err := meta.CreateBucketIfNotExists(templateIndexKey); err != nil {
		return err
	}

	return nil
}

// ConnectBlock is invoked by the index manager when a new block has been
// connected to the main chain.  This indexer adds a hash-to-transaction mapping
// for every transaction in the passed block.
//
// This is part of the Indexer interface.
func (idx *TemplateIndex) ConnectBlock(dbTx database.Tx, block *asiutil.Block,
	stxos []txo.SpentTxOut, vblock *asiutil.VBlock) error {
	// The offset and length of the transactions within the serialized
	// block.
	txLocs, err := block.TxLoc()
	if err != nil {
		return err
	}
	// Increment the internal block ID to use for the block being connected
	// and add all of the templates in the block to the index.
	newBlockID := idx.curBlockID + 1
	if err := dbAddTemplateIndexEntries(dbTx, txLocs, block.Transactions(), newBlockID); err != nil {
		return err
	}
	idx.curBlockID = newBlockID
	return nil
}

// DisconnectBlock is invoked by the index manager when a block has been
// disconnected from the main chain.  This indexer removes the
// hash-to-transaction mapping for every transaction in the block.
//
// This is part of the Indexer interface.
func (idx *TemplateIndex) DisconnectBlock(dbTx database.Tx, block *asiutil.Block,
	stxos []txo.SpentTxOut, vblock *asiutil.VBlock) error {
	// Remove all of the templates in the block from the index.
	if err := dbRemoveTemplateIndexEntries(dbTx, block); err != nil {
		return err
	}
	idx.curBlockID--
	return nil
}

// FetchBlockRegion returns the block region for the provided key
// from the index.  The block region can in turn be used to load the
// raw object bytes.  When there is no entry for the provided hash, nil
// will be returned for the both the entry and the error.
//
// This function is safe for concurrent access.
func (idx *TemplateIndex) FetchBlockRegion(txhash []byte) (*database.BlockRegion, error) {
	log.Debug("Template Index Fetch ", hexutil.Encode(txhash))
	var region *database.BlockRegion
	err := idx.db.View(func(dbTx database.Tx) error {
		var err error
		region, err = dbFetchTemplateIndexEntry(dbTx, txhash)
		return err
	})
	return region, err
}

// NewTemplateIndex returns a new instance of an indexer that is used to create a
// mapping of the hashes of all templates in the blockchain to the respective
// block, location within the block, and size of the template.
//
// It implements the Indexer interface which plugs into the IndexManager that in
// turn is used by the blockchain package.  This allows the index to be
// seamlessly maintained along with the chain.
func NewTemplateIndex(db database.Transactor) *TemplateIndex {
	log.Info("Template index is enabled")
	return &TemplateIndex{db: db}
}

// dbAddTemplateIndexEntries uses an existing database transaction to add a
// transaction's template index entry for necessary transaction in the passed block.
func dbAddTemplateIndexEntries(dbTx database.Tx, txLocs []protos.TxLoc, txs []*asiutil.Tx, blockID uint32) error {
	offset := 0
	serializedValues := make([]byte, len(txs)*templateEntrySize)
	for i, tx := range txs {
		if len(tx.MsgTx().TxOut) == 0 {
			continue
		}

		txout := tx.MsgTx().TxOut[0]
		if len(txout.Data) > 0 {
			preOps, _ := txscript.GetParseScript(txout.PkScript)
			if txscript.ContractOpCode(preOps) == txscript.OP_TEMPLATE {
				_, _, _, _, _, err := blockchain.DecodeTemplateContractData(txout.Data)
				if err != nil {
					continue // Unexpected
				}
				putTemplateIndexEntry(serializedValues[offset:], blockID, tx, txLocs[i])
				endOffset := offset + templateEntrySize
				err = dbPutTemplateIndexEntry(dbTx, tx.Hash(),
					serializedValues[offset:endOffset:endOffset])
				if err != nil {
					return err
				}
				offset += templateEntrySize
			}
		}
	}

	return nil
}

// dbFetchTemplateIndexEntry uses an existing database transaction to fetch the block
// region for the provided transaction hash from the transaction index.  When
// there is no entry for the provided hash, nil will be returned for the both
// the region and the error.
func dbFetchTemplateIndexEntry(dbTx database.Tx, txHash []byte) (*database.BlockRegion, error) {
	// Load the record from the database and return now if it doesn't exist.
	templateIndex := dbTx.Metadata().Bucket(templateIndexKey)
	serializedData := templateIndex.Get(txHash[:])
	if len(serializedData) == 0 {
		return nil, blockchain.RuleError{
			ErrorCode: blockchain.ErrNoTemplate,
			Description: fmt.Sprintf("fetch %s returns empty "+
				"entry for %s", templateIndexKey, txHash),
		}
	}

	// Ensure the serialized data has enough bytes to properly deserialize.
	if len(serializedData) < templateEntrySize {
		return nil, database.Error{
			ErrorCode: database.ErrCorruption,
			Description: fmt.Sprintf("corrupt template index "+
				"entry for %s", txHash),
		}
	}

	// Load the block hash associated with the block ID.
	hash, err := dbFetchBlockHashBySerializedID(dbTx, serializedData[0:4])
	if err != nil {
		return nil, database.Error{
			ErrorCode: database.ErrCorruption,
			Description: fmt.Sprintf("corrupt transaction index "+
				"entry for %s: %v", txHash, err),
		}
	}

	// Deserialize the final entry.
	key := database.NewNormalBlockKey(hash)
	region := database.BlockRegion{Key: key}
	region.Offset = byteOrder.Uint32(serializedData[4:8])
	region.Len = byteOrder.Uint32(serializedData[8:12])

	return &region, nil
}

// dbRemoveTemplateIndexEntries uses an existing database transaction to remove
// the latest transaction entry for every transaction in the passed block.
func dbRemoveTemplateIndexEntries(dbTx database.Tx, block *asiutil.Block) error {
	templateIndex := dbTx.Metadata().Bucket(templateIndexKey)
	for _, tx := range block.Transactions() {
		txout := tx.MsgTx().TxOut[0]
		if len(txout.Data) > 0 {
			preOps, _ := txscript.GetParseScript(txout.PkScript)
			if txscript.ContractOpCode(preOps) == txscript.OP_TEMPLATE {
				_, _, _, _, _, err := blockchain.DecodeTemplateContractData(txout.Data)
				if err != nil {
					continue // Unexpected
				}
				err = templateIndex.Delete(tx.Hash()[:])
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}
