// Copyright (c) 2018-2020. The asimov developers
// Copyright (c) 2013-2017 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.
package indexers

import (
	"fmt"
	"github.com/AsimovNetwork/asimov/blockchain/txo"
	"github.com/AsimovNetwork/asimov/protos"

	"github.com/AsimovNetwork/asimov/blockchain"
	"github.com/AsimovNetwork/asimov/common"
	"github.com/AsimovNetwork/asimov/database"
	"github.com/AsimovNetwork/asimov/asiutil"
)

var (
	// indexTipsBucketName is the name of the db bucket used to house the
	// current tip of each index.
	indexTipsBucketName = []byte("idxtips")
)

// -----------------------------------------------------------------------------
// The index manager tracks the current tip of each index by using a parent
// bucket that contains an entry for index.
//
// The serialized format for an index tip is:
//
//   [<block hash><block height>],...
//
//   Field           Type             Size
//   block hash      common.Hash   common.HashLength
//   block height    uint32           4 bytes
// -----------------------------------------------------------------------------

// dbPutIndexerTip uses an existing database transaction to update or add the
// current tip for the given index to the provided values.
func dbPutIndexerTip(dbTx database.Tx, idxKey []byte, hash *common.Hash, height int32) error {
	serialized := make([]byte, common.HashLength+4)
	copy(serialized, hash[:])
	byteOrder.PutUint32(serialized[common.HashLength:], uint32(height))

	indexesBucket := dbTx.Metadata().Bucket(indexTipsBucketName)
	return indexesBucket.Put(idxKey, serialized)
}

// dbFetchIndexerTip uses an existing database transaction to retrieve the
// hash and height of the current tip for the provided index.
func dbFetchIndexerTip(dbTx database.Tx, idxKey []byte) (*common.Hash, int32, error) {
	indexesBucket := dbTx.Metadata().Bucket(indexTipsBucketName)
	serialized := indexesBucket.Get(idxKey)
	if len(serialized) < common.HashLength+4 {
		return nil, 0, database.Error{
			ErrorCode: database.ErrCorruption,
			Description: fmt.Sprintf("unexpected end of data for "+
				"index %q tip", string(idxKey)),
		}
	}

	var hash common.Hash
	copy(hash[:], serialized[:common.HashLength])
	height := int32(byteOrder.Uint32(serialized[common.HashLength:]))
	return &hash, height, nil
}

// dbIndexConnectBlock adds all of the index entries associated with the
// given block using the provided indexer and updates the tip of the indexer
// accordingly.  An error will be returned if the current tip for the indexer is
// not the previous block for the passed block.
func dbIndexConnectBlock(dbTx database.Tx, indexer blockchain.Indexer, block *asiutil.Block,
	stxo []txo.SpentTxOut, vblock *asiutil.VBlock) error {

	// Assert that the block being connected properly connects to the
	// current tip of the index.
	idxKey := indexer.Key()
	curTipHash, _, err := dbFetchIndexerTip(dbTx, idxKey)
	if err != nil {
		return err
	}
	if !curTipHash.IsEqual(&block.MsgBlock().Header.PrevBlock) {
		return common.AssertError(fmt.Sprintf("dbIndexConnectBlock must be "+
			"called with a block that extends the current index "+
			"tip (%s, tip %s, block %s)", indexer.Name(),
			curTipHash, block.Hash()))
	}

	// Notify the indexer with the connected block so it can index it.
	if err := indexer.ConnectBlock(dbTx, block, stxo, vblock); err != nil {
		return err
	}

	// Update the current index tip.
	return dbPutIndexerTip(dbTx, idxKey, block.Hash(), block.Height())
}

// dbIndexDisconnectBlock removes all of the index entries associated with the
// given block using the provided indexer and updates the tip of the indexer
// accordingly.  An error will be returned if the current tip for the indexer is
// not the passed block.
func dbIndexDisconnectBlock(dbTx database.Tx, indexer blockchain.Indexer, block *asiutil.Block,
	stxo []txo.SpentTxOut, vblock *asiutil.VBlock) error {

	// Assert that the block being disconnected is the current tip of the
	// index.
	idxKey := indexer.Key()
	curTipHash, _, err := dbFetchIndexerTip(dbTx, idxKey)
	if err != nil {
		return err
	}
	if !curTipHash.IsEqual(block.Hash()) {
		return common.AssertError(fmt.Sprintf("dbIndexDisconnectBlock must "+
			"be called with the block at the current index tip "+
			"(%s, tip %s, block %s)", indexer.Name(),
			curTipHash, block.Hash()))
	}

	// Notify the indexer with the disconnected block so it can remove all
	// of the appropriate entries.
	if err := indexer.DisconnectBlock(dbTx, block, stxo, vblock); err != nil {
		return err
	}

	// Update the current index tip.
	prevHash := &block.MsgBlock().Header.PrevBlock
	return dbPutIndexerTip(dbTx, idxKey, prevHash, block.Height()-1)
}

// Manager defines an index manager that manages multiple optional indexes and
// implements the blockchain.IndexManager interface so it can be seamlessly
// plugged into normal chain processing.
type Manager struct {
	db             database.Transactor
	enabledIndexes []blockchain.Indexer
}

// Ensure the Manager type implements the blockchain.IndexManager interface.
var _ blockchain.IndexManager = (*Manager)(nil)

// indexDropKey returns the key for an index which indicates it is in the
// process of being dropped.
func indexDropKey(idxKey []byte) []byte {
	dropKey := make([]byte, len(idxKey)+1)
	dropKey[0] = 'd'
	copy(dropKey[1:], idxKey)
	return dropKey
}

// maybeFinishDrops determines if each of the enabled indexes are in the middle
// of being dropped and finishes dropping them when the are.  This is necessary
// because dropping and index has to be done in several atomic steps rather than
// one big atomic step due to the massive number of entries.
func (m *Manager) maybeFinishDrops(interrupt <-chan struct{}) error {
	indexNeedsDrop := make([]bool, len(m.enabledIndexes))
	err := m.db.View(func(dbTx database.Tx) error {
		// None of the indexes needs to be dropped if the index tips
		// bucket hasn't been created yet.
		indexesBucket := dbTx.Metadata().Bucket(indexTipsBucketName)
		if indexesBucket == nil {
			return nil
		}

		// Mark the indexer as requiring a drop if one is already in
		// progress.
		for i, indexer := range m.enabledIndexes {
			dropKey := indexDropKey(indexer.Key())
			if indexesBucket.Get(dropKey) != nil {
				indexNeedsDrop[i] = true
			}
		}

		return nil
	})
	if err != nil {
		return err
	}

	if interruptRequested(interrupt) {
		return errInterruptRequested
	}

	// Finish dropping any of the enabled indexes that are already in the
	// middle of being dropped.
	for i, indexer := range m.enabledIndexes {
		if !indexNeedsDrop[i] {
			continue
		}

		log.Infof("Resuming %s drop", indexer.Name())

	}

	return nil
}

// maybeCreateIndexes determines if each of the enabled indexes have already
// been created and creates them if not.
func (m *Manager) maybeCreateIndexes(dbTx database.Tx) error {
	indexesBucket := dbTx.Metadata().Bucket(indexTipsBucketName)
	for _, indexer := range m.enabledIndexes {
		// Nothing to do if the index tip already exists.
		idxKey := indexer.Key()
		if indexesBucket.Get(idxKey) != nil {
			if err := indexer.Check(dbTx); err != nil {
				return err
			}
			continue
		}

		// The tip for the index does not exist, so create it and
		// invoke the create callback for the index so it can perform
		// any one-time initialization it requires.
		if err := indexer.Create(dbTx); err != nil {
			return err
		}

		// Set the tip for the index to values which represent an
		// uninitialized index.
		err := dbPutIndexerTip(dbTx, idxKey, &common.Hash{}, -1)
		if err != nil {
			return err
		}
	}

	return nil
}

// Init initializes the enabled indexes.  This is called during chain
// initialization and primarily consists of catching up all indexes to the
// current best chain tip.  This is necessary since each index can be disabled
// and re-enabled at any time and attempting to catch-up indexes at the same
// time new blocks are being downloaded would lead to an overall longer time to
// catch up due to the I/O contention.
//
// This is part of the blockchain.IndexManager interface.
func (m *Manager) Init(chain *blockchain.BlockChain, interrupt <-chan struct{}) error {
	// Nothing to do when no indexes are enabled.
	if len(m.enabledIndexes) == 0 {
		return nil
	}

	if interruptRequested(interrupt) {
		return errInterruptRequested
	}

	// Finish and drops that were previously interrupted.
	if err := m.maybeFinishDrops(interrupt); err != nil {
		return err
	}

	// Create the initial state for the indexes as needed.
	err := m.db.Update(func(dbTx database.Tx) error {
		// Create the bucket for the current tips as needed.
		meta := dbTx.Metadata()
		_, err := meta.CreateBucketIfNotExists(indexTipsBucketName)
		if err != nil {
			return err
		}

		return m.maybeCreateIndexes(dbTx)
	})
	if err != nil {
		return err
	}

	// Initialize each of the enabled indexes.
	for _, indexer := range m.enabledIndexes {
		if err := indexer.Init(); err != nil {
			return err
		}
	}

	// Rollback indexes to the main chain if their tip is an orphaned fork.
	// This is fairly unlikely, but it can happen if the chain is
	// reorganized while the index is disabled.  This has to be done in
	// reverse order because later indexes can depend on earlier ones.
	for i := len(m.enabledIndexes); i > 0; i-- {
		indexer := m.enabledIndexes[i-1]

		// Fetch the current tip for the index.
		var height int32
		var hash *common.Hash
		err := m.db.View(func(dbTx database.Tx) error {
			idxKey := indexer.Key()
			hash, height, err = dbFetchIndexerTip(dbTx, idxKey)
			return err
		})
		if err != nil {
			return err
		}

		// Nothing to do if the index does not have any entries yet.
		if height == -1 {
			continue
		}

		// Loop until the tip is a block that exists in the main chain.
		initialHeight := height
		for !chain.MainChainHasBlock(hash) {
			// At this point the index tip is orphaned, so load the
			// orphaned block from the database directly and
			// disconnect it from the index.  The block has to be
			// loaded directly since it is no longer in the main
			// chain and thus the chain.BlockByHash function would
			// error.
			block, vblock, err := asiutil.GetBlockPair(m.db, hash)
			if err != nil {
				return err
			}

			// We'll also grab the set of outputs spent by this
			// block so we can remove them from the index.
			spentTxos, err := chain.FetchSpendJournal(block, vblock)
			if err != nil {
				return err
			}

			// With the block and stxo set for that block retrieved,
			// we can now update the index itself.
			err = m.db.Update(func(dbTx database.Tx) error {
				// Remove all of the index entries associated
				// with the block and update the indexer tip.
				err = dbIndexDisconnectBlock(
					dbTx, indexer, block, spentTxos, vblock)
				if err != nil {
					return err
				}

				// Update the tip to the previous block.
				hash = &block.MsgBlock().Header.PrevBlock
				height--

				return nil
			})
			if err != nil {
				return err
			}

			if interruptRequested(interrupt) {
				return errInterruptRequested
			}
		}

		if initialHeight != height {
			log.Infof("Removed %d orphaned blocks from %s "+
				"(heights %d to %d)", initialHeight-height,
				indexer.Name(), height+1, initialHeight)
		}
	}

	// Fetch the current tip heights for each index along with tracking the
	// lowest one so the catchup code only needs to start at the earliest
	// block and is able to skip connecting the block for the indexes that
	// don't need it.
	bestHeight := chain.BestSnapshot().Height
	lowestHeight := bestHeight
	indexerHeights := make([]int32, len(m.enabledIndexes))
	err = m.db.View(func(dbTx database.Tx) error {
		for i, indexer := range m.enabledIndexes {
			idxKey := indexer.Key()
			hash, height, err := dbFetchIndexerTip(dbTx, idxKey)
			if err != nil {
				return err
			}

			log.Debugf("Current %s tip (height %d, hash %v)",
				indexer.Name(), height, hash)
			indexerHeights[i] = height
			if height < lowestHeight {
				lowestHeight = height
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	// Nothing to index if all of the indexes are caught up.
	if lowestHeight == bestHeight {
		return nil
	}

	// Create a progress logger for the indexing process below.
	progressLogger := newBlockProgressLogger("Indexed", log)

	// At this point, one or more indexes are behind the current best chain
	// tip and need to be caught up, so logger the details and loop through
	// each block that needs to be indexed.
	log.Infof("Catching up indexes from height %d to %d", lowestHeight,
		bestHeight)
	for height := lowestHeight + 1; height <= bestHeight; height++ {
		// Load the block and vblock for the height since it is required to index
		// it.
		hash, err := chain.BlockHashByHeight(height)
		if err != nil {
			return err
		}

		block, vblock, err := asiutil.GetBlockPair(m.db, hash)
		if err != nil {
			if _, ok := err.(asiutil.MissVBlockError); ok && height == 0 {
				vblock = asiutil.NewVBlock(&protos.MsgVBlock{}, hash)
			} else {
				return err
			}
		}

		if interruptRequested(interrupt) {
			return errInterruptRequested
		}

		// Connect the block for all indexes that need it.
		var spentTxos []txo.SpentTxOut
		for i, indexer := range m.enabledIndexes {
			// Skip indexes that don't need to be updated with this
			// block.
			if indexerHeights[i] >= height {
				continue
			}

			// When the index requires all of the referenced txouts
			// and they haven't been loaded yet, they need to be
			// retrieved from the spend journal.
			if spentTxos == nil && indexNeedsInputs(indexer) {
				spentTxos, err = chain.FetchSpendJournal(block, vblock)
				if err != nil {
					return err
				}
			}

			err := m.db.Update(func(dbTx database.Tx) error {
				return dbIndexConnectBlock(
					dbTx, indexer, block, spentTxos, vblock)
			})
			if err != nil {
				return err
			}
			indexerHeights[i] = height
		}

		// Log indexing progress.
		progressLogger.LogBlockHeight(block)

		if interruptRequested(interrupt) {
			return errInterruptRequested
		}
	}

	log.Infof("Indexes caught up to height %d", bestHeight)
	return nil
}

// indexNeedsInputs returns whether or not the index needs access to the txouts
// referenced by the transaction inputs being indexed.
func indexNeedsInputs(index blockchain.Indexer) bool {
	if idx, ok := index.(NeedsInputser); ok {
		return idx.NeedsInputs()
	}

	return false
}

// ConnectBlock must be invoked when a block is extending the main chain.  It
// keeps track of the state of each index it is managing, performs some sanity
// checks, and invokes each indexer.
//
// This is part of the blockchain.IndexManager interface.
func (m *Manager) ConnectBlock(dbTx database.Tx, block *asiutil.Block,
	stxos []txo.SpentTxOut, vblock *asiutil.VBlock) error {

	// Call each of the currently active optional indexes with the block
	// being connected so they can update accordingly.
	for _, index := range m.enabledIndexes {
		err := dbIndexConnectBlock(dbTx, index, block, stxos, vblock)
		if err != nil {
			return err
		}
	}
	return nil
}

// DisconnectBlock must be invoked when a block is being disconnected from the
// end of the main chain.  It keeps track of the state of each index it is
// managing, performs some sanity checks, and invokes each indexer to remove
// the index entries associated with the block.
//
// This is part of the blockchain.IndexManager interface.
func (m *Manager) DisconnectBlock(dbTx database.Tx, block *asiutil.Block,
	stxo []txo.SpentTxOut, vblock *asiutil.VBlock) error {

	// Call each of the currently active optional indexes with the block
	// being disconnected so they can update accordingly.
	for _, index := range m.enabledIndexes {
		err := dbIndexDisconnectBlock(dbTx, index, block, stxo, vblock)
		if err != nil {
			return err
		}
	}
	return nil
}

// NewManager returns a new index manager with the provided indexes enabled.
//
// The manager returned satisfies the blockchain.IndexManager interface and thus
// cleanly plugs into the normal blockchain processing path.
func NewManager(db database.Transactor, enabledIndexes []blockchain.Indexer) *Manager {
	return &Manager{
		db:             db,
		enabledIndexes: enabledIndexes,
	}
}
