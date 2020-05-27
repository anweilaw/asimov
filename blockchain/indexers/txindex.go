// Copyright (c) 2018-2020. The asimov developers
// Copyright (c) 2013-2017 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.
package indexers

import (
	"errors"
	"fmt"
	"github.com/AsimovNetwork/asimov/asiutil"
	"github.com/AsimovNetwork/asimov/blockchain"
	"github.com/AsimovNetwork/asimov/blockchain/txo"
	"github.com/AsimovNetwork/asimov/common"
	"github.com/AsimovNetwork/asimov/database"
	"github.com/AsimovNetwork/asimov/protos"
)

const (
	// txIndexName is the human-readable name for the index.
	txIndexName = "transaction index"
)

var (
	// txIndexKey is the key of the transaction index and the db bucket used
	// to house it.
	txIndexKey = []byte("txbyhashidx")

	// idByHashIndexBucketName is the name of the db bucket used to house
	// the block id -> block hash index.
	idByHashIndexBucketName = []byte("idbyhashidx")

	// hashByIDIndexBucketName is the name of the db bucket used to house
	// the block hash -> block id index.
	hashByIDIndexBucketName = []byte("hashbyididx")

	// errNoBlockIDEntry is an error that indicates a requested entry does
	// not exist in the block ID index.
	errNoBlockIDEntry = errors.New("no entry in the block ID index")
)

// -----------------------------------------------------------------------------
// The transaction index consists of an entry for every transaction in the main
// chain.  In order to significantly optimize the space requirements a separate
// index which provides an internal mapping between each block that has been
// indexed and a unique ID for use within the hash to location mappings.  The ID
// is simply a sequentially incremented uint32.  This is useful because it is
// only 4 bytes versus 32 bytes hashes and thus saves a ton of space in the
// index.
//
// There are three buckets used in total.  The first bucket maps the hash of
// each transaction to the specific block location.  The second bucket maps the
// hash of each block to the unique ID and the third maps that ID back to the
// block hash.
//
// NOTE: Although it is technically possible for multiple transactions to have
// the same hash as long as the previous transaction with the same hash is fully
// spent, this code only stores the most recent one because doing otherwise
// would add a non-trivial amount of space and overhead for something that will
// realistically never happen per the probability and even if it did, the old
// one must be fully spent and so the most likely transaction a caller would
// want for a given hash is the most recent one anyways.
//
// The serialized format for keys and values in the block hash to ID bucket is:
//   <hash> = <ID>
//
//   Field           Type              Size
//   hash            common.Hash    32 bytes
//   ID              uint32            4 bytes
//   -----
//   Total: 36 bytes
//
// The serialized format for keys and values in the ID to block hash bucket is:
//   <ID> = <hash>
//
//   Field           Type              Size
//   ID              uint32            4 bytes
//   hash            common.Hash    32 bytes
//   -----
//   Total: 36 bytes
//
// The serialized format for the keys and values in the tx index bucket is:
//
//   <txhash> = <block id><block type><start offset><tx length>
//
//   Field           Type              Size
//   txhash          common.Hash    32 bytes
//   block id        uint32          4 bytes
//   block type      byte            1 byte
//   start offset    uint32          4 bytes
//   tx length       uint32          4 bytes
//   -----
//   Total: 45 bytes
// -----------------------------------------------------------------------------

// dbPutBlockIDIndexEntry uses an existing database transaction to update or add
// the index entries for the hash to id and id to hash mappings for the provided
// values.
func dbPutBlockIDIndexEntry(dbTx database.Tx, hash *common.Hash, id uint32) error {
	// Serialize the height for use in the index entries.
	var serializedID [4]byte
	byteOrder.PutUint32(serializedID[:], id)

	// Add the block hash to ID mapping to the index.
	meta := dbTx.Metadata()
	hashIndex := meta.Bucket(idByHashIndexBucketName)
	if err := hashIndex.Put(hash[:], serializedID[:]); err != nil {
		return err
	}

	// Add the block ID to hash mapping to the index.
	idIndex := meta.Bucket(hashByIDIndexBucketName)
	return idIndex.Put(serializedID[:], hash[:])
}

// dbRemoveBlockIDIndexEntry uses an existing database transaction remove index
// entries from the hash to id and id to hash mappings for the provided hash.
func dbRemoveBlockIDIndexEntry(dbTx database.Tx, hash *common.Hash) error {
	// Remove the block hash to ID mapping.
	meta := dbTx.Metadata()
	hashIndex := meta.Bucket(idByHashIndexBucketName)
	serializedID := hashIndex.Get(hash[:])
	if serializedID == nil {
		return nil
	}
	if err := hashIndex.Delete(hash[:]); err != nil {
		return err
	}

	// Remove the block ID to hash mapping.
	idIndex := meta.Bucket(hashByIDIndexBucketName)
	return idIndex.Delete(serializedID)
}

// dbFetchBlockIDByHash uses an existing database transaction to retrieve the
// block id for the provided hash from the index.
func dbFetchBlockIDByHash(dbTx database.Tx, hash *common.Hash) (uint32, error) {
	hashIndex := dbTx.Metadata().Bucket(idByHashIndexBucketName)
	serializedID := hashIndex.Get(hash[:])
	if serializedID == nil {
		return 0, errNoBlockIDEntry
	}

	return byteOrder.Uint32(serializedID), nil
}

// dbFetchBlockHashBySerializedID uses an existing database transaction to
// retrieve the hash for the provided serialized block id from the index.
func dbFetchBlockHashBySerializedID(dbTx database.Tx, serializedID []byte) (*common.Hash, error) {
	idIndex := dbTx.Metadata().Bucket(hashByIDIndexBucketName)
	hashBytes := idIndex.Get(serializedID)
	if hashBytes == nil {
		return nil, errNoBlockIDEntry
	}

	var hash common.Hash
	copy(hash[:], hashBytes)
	return &hash, nil
}

// dbFetchBlockHashByID uses an existing database transaction to retrieve the
// hash for the provided block id from the index.
func dbFetchBlockHashByID(dbTx database.Tx, id uint32) (*common.Hash, error) {
	var serializedID [4]byte
	byteOrder.PutUint32(serializedID[:], id)
	return dbFetchBlockHashBySerializedID(dbTx, serializedID[:])
}

// putTxIndexEntry serializes the provided values according to the format
// described about for a transaction index entry.  The target byte slice must
// be at least large enough to handle the number of bytes defined by the
// txEntrySize constant or it will panic.
func putTxIndexEntry(target []byte, blockID uint32, blockType database.BlockType, txLoc protos.TxLoc) {
	byteOrder.PutUint32(target, blockID)
	target[4] = byte(blockType)
	byteOrder.PutUint32(target[5:], uint32(txLoc.TxStart))
	byteOrder.PutUint32(target[9:], uint32(txLoc.TxLen))
}

// dbPutTxIndexEntry uses an existing database transaction to update the
// transaction index given the provided serialized data that is expected to have
// been serialized putTxIndexEntry.
func dbPutTxIndexEntry(dbTx database.Tx, txHash *common.Hash, serializedData []byte) error {
	txIndex := dbTx.Metadata().Bucket(txIndexKey)
	return txIndex.Put(txHash[:], serializedData)
}

// dbFetchTxIndexEntry uses an existing database transaction to fetch the block
// region for the provided transaction hash from the transaction index.  When
// there is no entry for the provided hash, nil will be returned for the both
// the region and the error.
func dbFetchTxIndexEntry(dbTx database.Tx, txHash []byte) (*database.BlockRegion, error) {
	// Load the record from the database and return now if it doesn't exist.
	txIndex := dbTx.Metadata().Bucket(txIndexKey)
	serializedData := txIndex.Get(txHash)
	if len(serializedData) == 0 {
		return nil, nil
	}

	// Ensure the serialized data has enough bytes to properly deserialize.
	// 4 bytes block id + 1 byte block type + 4 bytes offset + 4 bytes length.
	if len(serializedData) < txEntrySize {
		return nil, database.Error{
			ErrorCode: database.ErrCorruption,
			Description: fmt.Sprintf("corrupt transaction index "+
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
	key := database.NewBlockKey(hash, database.BlockType(serializedData[4]))
	region := database.BlockRegion{Key: key}
	region.Offset = byteOrder.Uint32(serializedData[5:9])
	region.Len = byteOrder.Uint32(serializedData[9:13])

	return &region, nil
}

// dbAddTxIndexEntries uses an existing database transaction to add a
// transaction index entry for every transaction in the passed block.
func dbAddTxIndexEntries(dbTx database.Tx, txLocs []protos.TxLoc, txs []*asiutil.Tx, blockID uint32, blockType database.BlockType) error {
	// As an optimization, allocate a single slice big enough to hold all
	// of the serialized transaction index entries for the block and
	// serialize them directly into the slice.  Then, pass the appropriate
	// subslice to the database to be written.  This approach significantly
	// cuts down on the number of required allocations.
	offset := 0
	serializedValues := make([]byte, len(txs)*txEntrySize)
	for i, tx := range txs {
		putTxIndexEntry(serializedValues[offset:], blockID, blockType, txLocs[i])
		endOffset := offset + txEntrySize
		err := dbPutTxIndexEntry(dbTx, tx.Hash(),
			serializedValues[offset:endOffset:endOffset])
		if err != nil {
			return err
		}
		offset += txEntrySize
	}

	return nil
}

// dbRemoveTxIndexEntry uses an existing database transaction to remove the most
// recent transaction index entry for the given hash.
func dbRemoveTxIndexEntry(dbTx database.Tx, txHash *common.Hash, key []byte) error {
	txIndex := dbTx.Metadata().Bucket(key)
	serializedData := txIndex.Get(txHash[:])
	if len(serializedData) == 0 {
		return common.AssertError(fmt.Sprintf("dbRemoveTxIndexEntry: can't remove non-existent transaction %s "+
			"from the transaction index", txHash))
	}

	return txIndex.Delete(txHash[:])
}

// dbRemoveTxIndexEntries uses an existing database transaction to remove the
// latest transaction entry for every transaction in the passed block.
func dbRemoveTxIndexEntries(dbTx database.Tx, txs []*asiutil.Tx) error {
	for _, tx := range txs {
		err := dbRemoveTxIndexEntry(dbTx, tx.Hash(), txIndexKey)
		if err != nil {
			return err
		}
	}

	return nil
}

// TxIndex implements a transaction by hash index.  That is to say, it supports
// querying all transactions by their hash.
type TxIndex struct {
	db         database.Transactor
	curBlockID uint32
}

// Ensure the TxIndex type implements the Indexer interface.
var _ blockchain.Indexer = (*TxIndex)(nil)

// Init initializes the hash-based transaction index.  In particular, it finds
// the highest used block ID and stores it for later use when connecting or
// disconnecting blocks.
//
// This is part of the Indexer interface.
func (idx *TxIndex) Init() error {
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
func (idx *TxIndex) Key() []byte {
	return txIndexKey
}

// Name returns the human-readable name of the index.
//
// This is part of the Indexer interface.
func (idx *TxIndex) Name() string {
	return txIndexName
}

// Create is invoked when the indexer manager determines the index needs
// to be created for the first time.  It creates the buckets for the hash-based
// transaction index and the internal block ID indexes.
//
// This is part of the Indexer interface.
func (idx *TxIndex) Create(dbTx database.Tx) error {
	meta := dbTx.Metadata()
	if _, err := meta.CreateBucket(idByHashIndexBucketName); err != nil {
		return err
	}
	if _, err := meta.CreateBucket(hashByIDIndexBucketName); err != nil {
		return err
	}
	if _, err := meta.CreateBucket(txIndexKey); err != nil {
		return err
	}

	return nil
}

// Checking if there is a new bucket added to this indexer
// This method is invoked each time when node started.
func (idx *TxIndex) Check(dbTx database.Tx) error {
	meta := dbTx.Metadata()
	if _, err := meta.CreateBucketIfNotExists(idByHashIndexBucketName); err != nil {
		return err
	}
	if _, err := meta.CreateBucketIfNotExists(hashByIDIndexBucketName); err != nil {
		return err
	}
	if _, err := meta.CreateBucketIfNotExists(txIndexKey); err != nil {
		return err
	}

	return nil
}

// ConnectBlock is invoked by the index manager when a new block has been
// connected to the main chain.  This indexer adds a hash-to-transaction mapping
// for every transaction in the passed block.
//
// This is part of the Indexer interface.
func (idx *TxIndex) ConnectBlock(dbTx database.Tx, block *asiutil.Block,
	stxos []txo.SpentTxOut, vblock *asiutil.VBlock) error {
	// The offset and length of the transactions within the serialized
	// block.
	txLocs, err := block.TxLoc()
	if err != nil {
		return err
	}
	// The offset and length of the transactions within the serialized
	// vblock.
	vtxLocs, err := vblock.TxLoc()
	if err != nil {
		return err
	}
	// Increment the internal block ID to use for the block being connected
	// and add all of the transactions in the block to the index.
	newBlockID := idx.curBlockID + 1
	if err := dbAddTxIndexEntries(dbTx, txLocs, block.Transactions(), newBlockID, database.BlockNormal); err != nil {
		return err
	}
	if err := dbAddTxIndexEntries(dbTx, vtxLocs, vblock.Transactions(), newBlockID, database.BlockVirtual); err != nil {
		return err
	}

	// Add the new block ID index entry for the block being connected and
	// update the current internal block ID accordingly.
	if err := dbPutBlockIDIndexEntry(dbTx, block.Hash(), newBlockID); err != nil {
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
func (idx *TxIndex) DisconnectBlock(dbTx database.Tx, block *asiutil.Block,
	stxos []txo.SpentTxOut, vblock *asiutil.VBlock) error {

	// Remove all of the transactions in the block from the index.
	if err := dbRemoveTxIndexEntries(dbTx, block.Transactions()); err != nil {
		return err
	}
	if err := dbRemoveTxIndexEntries(dbTx, vblock.Transactions()); err != nil {
		return err
	}

	// Remove the block ID index entry for the block being disconnected and
	// decrement the current internal block ID to account for it.
	if err := dbRemoveBlockIDIndexEntry(dbTx, block.Hash()); err != nil {
		return err
	}

	idx.curBlockID--
	return nil
}

// FetchBlockRegion returns the block region for the provided transaction hash
// from the transaction index.  The block region can in turn be used to load the
// raw transaction bytes.  When there is no entry for the provided hash, nil
// will be returned for the both the entry and the error.
//
// This function is safe for concurrent access.
func (idx *TxIndex) FetchBlockRegion(hash []byte) (*database.BlockRegion, error) {
	var region *database.BlockRegion
	err := idx.db.View(func(dbTx database.Tx) error {
		var err error
		region, err = dbFetchTxIndexEntry(dbTx, hash)
		return err
	})
	return region, err
}

// NewTxIndex returns a new instance of an indexer that is used to create a
// mapping of the hashes of all transactions in the blockchain to the respective
// block, location within the block, and size of the transaction.
//
// It implements the Indexer interface which plugs into the IndexManager that in
// turn is used by the blockchain package.  This allows the index to be
// seamlessly maintained along with the chain.
func NewTxIndex(db database.Transactor) *TxIndex {
	log.Info("Transaction index is enabled")
	return &TxIndex{db: db}
}
