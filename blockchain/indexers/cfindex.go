// Copyright (c) 2018-2020. The asimov developers
// Copyright (c) 2013-2017 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.
package indexers

import (
	"fmt"
	"github.com/AsimovNetwork/asimov/blockchain/txo"
	"github.com/AsimovNetwork/asimov/common"

	"github.com/AsimovNetwork/asimov/asiutil"
	"github.com/AsimovNetwork/asimov/asiutil/gcs"
	"github.com/AsimovNetwork/asimov/asiutil/gcs/builder"
	"github.com/AsimovNetwork/asimov/blockchain"
	"github.com/AsimovNetwork/asimov/database"
	"github.com/AsimovNetwork/asimov/protos"
)

const (
	// cfIndexName is the human-readable name for the index.
	cfIndexName = "committed filter index"
)

// Committed filters come in one flavor currently: basic. They are generated
// and dropped in pairs, and both are indexed by a block's hash.  Besides
// holding different content, they also live in different buckets.
var (
	// cfIndexParentBucketKey is the name of the parent bucket used to
	// house the index. The rest of the buckets live below this bucket.
	cfIndexParentBucketKey = []byte("cfindexparentbucket")

	// cfIndexKeys is an array of db bucket names used to house indexes of
	// block hashes to cfilters.
	cfIndexKeys = [][]byte{
		[]byte("cf0byhashidx"),
	}

	// cfHeaderKeys is an array of db bucket names used to house indexes of
	// block hashes to cf headers.
	cfHeaderKeys = [][]byte{
		[]byte("cf0headerbyhashidx"),
	}

	// cfHashKeys is an array of db bucket names used to house indexes of
	// block hashes to cf hashes.
	cfHashKeys = [][]byte{
		[]byte("cf0hashbyhashidx"),
	}

	maxFilterType = uint8(len(cfHeaderKeys) - 1)

	// zeroHash is the common.Hash value of all zero bytes, defined here
	// for convenience.
	zeroHash common.Hash
)

// dbFetchFilterIdxEntry retrieves a data blob from the filter index database.
// An entry's absence is not considered an error.
func dbFetchFilterIdxEntry(dbTx database.Tx, key []byte, h *common.Hash) ([]byte, error) {
	idx := dbTx.Metadata().Bucket(cfIndexParentBucketKey).Bucket(key)
	return idx.Get(h[:]), nil
}

// dbStoreFilterIdxEntry stores a data blob in the filter index database.
func dbStoreFilterIdxEntry(dbTx database.Tx, key []byte, h *common.Hash, f []byte) error {
	idx := dbTx.Metadata().Bucket(cfIndexParentBucketKey).Bucket(key)
	return idx.Put(h[:], f)
}

// dbDeleteFilterIdxEntry deletes a data blob from the filter index database.
func dbDeleteFilterIdxEntry(dbTx database.Tx, key []byte, h *common.Hash) error {
	idx := dbTx.Metadata().Bucket(cfIndexParentBucketKey).Bucket(key)
	return idx.Delete(h[:])
}

// CfIndex implements a committed filter (cf) by hash index.
type CfIndex struct {
	db database.Transactor
}

// Ensure the CfIndex type implements the Indexer interface.
var _ blockchain.Indexer = (*CfIndex)(nil)

// Ensure the CfIndex type implements the NeedsInputser interface.
var _ NeedsInputser = (*CfIndex)(nil)

// NeedsInputs signals that the index requires the referenced inputs in order
// to properly create the index.
//
// This implements the NeedsInputser interface.
func (idx *CfIndex) NeedsInputs() bool {
	return true
}

// Init initializes the hash-based cf index. This is part of the Indexer
// interface.
func (idx *CfIndex) Init() error {
	return nil // Nothing to do.
}

// Key returns the database key to use for the index as a byte slice. This is
// part of the Indexer interface.
func (idx *CfIndex) Key() []byte {
	return cfIndexParentBucketKey
}

// Name returns the human-readable name of the index. This is part of the
// Indexer interface.
func (idx *CfIndex) Name() string {
	return cfIndexName
}

// Create is invoked when the indexer manager determines the index needs to
// be created for the first time. It creates buckets for the two hash-based cf
// indexes (regular only currently).
func (idx *CfIndex) Create(dbTx database.Tx) error {
	meta := dbTx.Metadata()

	cfIndexParentBucket, err := meta.CreateBucket(cfIndexParentBucketKey)
	if err != nil {
		return err
	}

	for _, bucketName := range cfIndexKeys {
		_, err = cfIndexParentBucket.CreateBucket(bucketName)
		if err != nil {
			return err
		}
	}

	for _, bucketName := range cfHeaderKeys {
		_, err = cfIndexParentBucket.CreateBucket(bucketName)
		if err != nil {
			return err
		}
	}

	for _, bucketName := range cfHashKeys {
		_, err = cfIndexParentBucket.CreateBucket(bucketName)
		if err != nil {
			return err
		}
	}

	return nil
}

// Checking if there is a new bucket added to this indexer
// This method is invoked each time when node started.
func (idx *CfIndex) Check(dbTx database.Tx) error {
	meta := dbTx.Metadata()

	cfIndexParentBucket, err := meta.CreateBucketIfNotExists(cfIndexParentBucketKey)
	if err != nil {
		return err
	}

	for _, bucketName := range cfIndexKeys {
		_, err = cfIndexParentBucket.CreateBucketIfNotExists(bucketName)
		if err != nil {
			return err
		}
	}

	for _, bucketName := range cfHeaderKeys {
		_, err = cfIndexParentBucket.CreateBucketIfNotExists(bucketName)
		if err != nil {
			return err
		}
	}

	for _, bucketName := range cfHashKeys {
		_, err = cfIndexParentBucket.CreateBucketIfNotExists(bucketName)
		if err != nil {
			return err
		}
	}

	return nil
}

// storeFilter stores a given filter, and performs the steps needed to
// generate the filter's header.
func storeFilter(dbTx database.Tx, block *asiutil.Block, f *gcs.Filter,
	filterType protos.FilterType) error {
	if uint8(filterType) > maxFilterType {
		return common.AssertError("storeFilter: unsupported filter type")
	}

	// Figure out which buckets to use.
	fkey := cfIndexKeys[filterType]
	hkey := cfHeaderKeys[filterType]
	hashkey := cfHashKeys[filterType]

	// Start by storing the filter.
	h := block.Hash()
	filterBytes, err := f.NBytes()
	if err != nil {
		return err
	}
	err = dbStoreFilterIdxEntry(dbTx, fkey, h, filterBytes)
	if err != nil {
		return err
	}

	// Next store the filter hash.
	filterHash, err := builder.GetFilterHash(f)
	if err != nil {
		return err
	}
	err = dbStoreFilterIdxEntry(dbTx, hashkey, h, filterHash[:])
	if err != nil {
		return err
	}

	// Then fetch the previous block's filter header.
	var prevHeader common.Hash
	ph := &block.MsgBlock().Header.PrevBlock
	if ph.IsEqual(&zeroHash) {
		prevHeader = zeroHash
	} else {
		pfh, err := dbFetchFilterIdxEntry(dbTx, hkey, ph)
		if err != nil {
			return err
		}

		if len(pfh) != common.HashLength {
			return common.AssertError(fmt.Sprintf("invalid hash length of %v, want %v", len(pfh),
				common.HashLength))
		}

		// Construct the new block's filter header, and store it.
		prevHeader = common.BytesToHash(pfh)
	}

	fh, err := builder.MakeHeaderForFilter(f, prevHeader)
	if err != nil {
		return err
	}
	return dbStoreFilterIdxEntry(dbTx, hkey, h, fh[:])
}

// ConnectBlock is invoked by the index manager when a new block has been
// connected to the main chain. This indexer adds a hash-to-cf mapping for
// every passed block. This is part of the Indexer interface.
func (idx *CfIndex) ConnectBlock(dbTx database.Tx, block *asiutil.Block,
	stxos []txo.SpentTxOut, vblock *asiutil.VBlock) error {

	prevScripts := make([][]byte, len(stxos))
	for i, stxo := range stxos {
		prevScripts[i] = stxo.PkScript
	}

	f, err := builder.BuildBasicFilter(block.MsgBlock(), prevScripts)
	if err != nil {
		return err
	}

	return storeFilter(dbTx, block, f, protos.GCSFilterRegular)
}

// DisconnectBlock is invoked by the index manager when a block has been
// disconnected from the main chain.  This indexer removes the hash-to-cf
// mapping for every passed block. This is part of the Indexer interface.
func (idx *CfIndex) DisconnectBlock(dbTx database.Tx, block *asiutil.Block,
	_ []txo.SpentTxOut, vblock *asiutil.VBlock) error {

	for _, key := range cfIndexKeys {
		err := dbDeleteFilterIdxEntry(dbTx, key, block.Hash())
		if err != nil {
			return err
		}
	}

	for _, key := range cfHeaderKeys {
		err := dbDeleteFilterIdxEntry(dbTx, key, block.Hash())
		if err != nil {
			return err
		}
	}

	for _, key := range cfHashKeys {
		err := dbDeleteFilterIdxEntry(dbTx, key, block.Hash())
		if err != nil {
			return err
		}
	}

	return nil
}

// entryByBlockHash fetches a filter index entry of a particular type
// (eg. filter, filter header, etc) for a filter type and block hash.
func (idx *CfIndex) entryByBlockHash(filterTypeKeys [][]byte,
	filterType protos.FilterType, h *common.Hash) ([]byte, error) {

	if uint8(filterType) > maxFilterType {
		return nil, common.AssertError("entryByBlockHash:unsupported filter type")
	}
	key := filterTypeKeys[filterType]

	var entry []byte
	err := idx.db.View(func(dbTx database.Tx) error {
		var err error
		entry, err = dbFetchFilterIdxEntry(dbTx, key, h)
		return err
	})
	return entry, err
}

// entriesByBlockHashes batch fetches a filter index entry of a particular type
// (eg. filter, filter header, etc) for a filter type and slice of block hashes.
func (idx *CfIndex) entriesByBlockHashes(filterTypeKeys [][]byte,
	filterType protos.FilterType, blockHashes []*common.Hash) ([][]byte, error) {

	if uint8(filterType) > maxFilterType {
		return nil, common.AssertError("entriesByBlockHashes:unsupported filter type")
	}
	key := filterTypeKeys[filterType]

	entries := make([][]byte, 0, len(blockHashes))
	err := idx.db.View(func(dbTx database.Tx) error {
		for _, blockHash := range blockHashes {
			entry, err := dbFetchFilterIdxEntry(dbTx, key, blockHash)
			if err != nil {
				return err
			}
			entries = append(entries, entry)
		}
		return nil
	})
	return entries, err
}

// FilterByBlockHash returns the serialized contents of a block's basic or
// committed filter.
func (idx *CfIndex) FilterByBlockHash(h *common.Hash,
	filterType protos.FilterType) ([]byte, error) {
	return idx.entryByBlockHash(cfIndexKeys, filterType, h)
}

// FiltersByBlockHashes returns the serialized contents of a block's basic or
// committed filter for a set of blocks by hash.
func (idx *CfIndex) FiltersByBlockHashes(blockHashes []*common.Hash,
	filterType protos.FilterType) ([][]byte, error) {
	return idx.entriesByBlockHashes(cfIndexKeys, filterType, blockHashes)
}

// FilterHeaderByBlockHash returns the serialized contents of a block's basic
// committed filter header.
func (idx *CfIndex) FilterHeaderByBlockHash(h *common.Hash,
	filterType protos.FilterType) ([]byte, error) {
	return idx.entryByBlockHash(cfHeaderKeys, filterType, h)
}

// FilterHeadersByBlockHashes returns the serialized contents of a block's
// basic committed filter header for a set of blocks by hash.
func (idx *CfIndex) FilterHeadersByBlockHashes(blockHashes []*common.Hash,
	filterType protos.FilterType) ([][]byte, error) {
	return idx.entriesByBlockHashes(cfHeaderKeys, filterType, blockHashes)
}

// FilterHashByBlockHash returns the serialized contents of a block's basic
// committed filter hash.
func (idx *CfIndex) FilterHashByBlockHash(h *common.Hash,
	filterType protos.FilterType) ([]byte, error) {
	return idx.entryByBlockHash(cfHashKeys, filterType, h)
}

// FilterHashesByBlockHashes returns the serialized contents of a block's basic
// committed filter hash for a set of blocks by hash.
func (idx *CfIndex) FilterHashesByBlockHashes(blockHashes []*common.Hash,
	filterType protos.FilterType) ([][]byte, error) {
	return idx.entriesByBlockHashes(cfHashKeys, filterType, blockHashes)
}

func (idx *CfIndex) FetchBlockRegion(thash []byte) (*database.BlockRegion, error) {
	return nil, nil
}

// NewCfIndex returns a new instance of an indexer that is used to create a
// mapping of the hashes of all blocks in the blockchain to their respective
// committed filters.
//
// It implements the Indexer interface which plugs into the IndexManager that
// in turn is used by the blockchain package. This allows the index to be
// seamlessly maintained along with the chain.
func NewCfIndex(db database.Transactor) *CfIndex {
	log.Info("Committed filter index is enabled")
	return &CfIndex{db: db}
}
