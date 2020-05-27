// Copyright (c) 2018-2020. The asimov developers
// Copyright (c) 2013-2017 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.
package blockchain

import (
	"github.com/AsimovNetwork/asimov/blockchain/txo"
	"github.com/AsimovNetwork/asimov/database"
	"github.com/AsimovNetwork/asimov/asiutil"
)

// Indexer provides a generic interface for an indexer that is managed by an
// index manager such as the Manager type provided by this package.
type Indexer interface {
	// Key returns the key of the index as a byte slice.
	Key() []byte

	// Name returns the human-readable name of the index.
	Name() string

	// Init is invoked when the index manager is first initializing the
	// index.  This differs from the Create method in that it is called on
	// every load, including the case the index was just created.
	Init() error

	// Create is invoked when the indexer manager determines the index needs
	// to be created for the first time.
	Create(dbTx database.Tx) error

	//Check is invoked each time when node started. Checking if there is a new bucket
	//added to this indexer
	Check(dbTx database.Tx) error

	// ConnectBlock is invoked when a new block has been connected to the
	// main chain. The set of output spent within a block is also passed in
	// so indexers can access the pevious output scripts input spent if
	// required.
	ConnectBlock(database.Tx, *asiutil.Block, []txo.SpentTxOut, *asiutil.VBlock) error

	// DisconnectBlock is invoked when a block has been disconnected from
	// the main chain. The set of outputs scripts that were spent within
	// this block is also returned so indexers can clean up the prior index
	// state for this block
	DisconnectBlock(database.Tx, *asiutil.Block, []txo.SpentTxOut, *asiutil.VBlock) error

	// FetchBlockRegion returns the block region for the provided key
	// from the index.  The block region can in turn be used to load the
	// raw object bytes.  When there is no entry for the provided hash, nil
	// will be returned for the both the entry and the error.
	//
	// This function is safe for concurrent access.
	FetchBlockRegion([]byte) (*database.BlockRegion, error)
}

// IndexManager provides a generic interface that the is called when blocks are
// connected and disconnected to and from the tip of the main chain for the
// purpose of supporting optional indexes.
type IndexManager interface {
	// Init is invoked during chain initialize in order to allow the index
	// manager to initialize itself and any indexes it is managing.  The
	// channel parameter specifies a channel the caller can close to signal
	// that the process should be interrupted.  It can be nil if that
	// behavior is not desired.
	Init(*BlockChain, <-chan struct{}) error

	// ConnectBlock is invoked when a new block has been connected to the
	// main chain. The set of output spent within a block is also passed in
	// so indexers can access the previous output scripts input spent if
	// required.
	ConnectBlock(database.Tx, *asiutil.Block, []txo.SpentTxOut, *asiutil.VBlock) error

	// DisconnectBlock is invoked when a block has been disconnected from
	// the main chain. The set of outputs scripts that were spent within
	// this block is also returned so indexers can clean up the prior index
	// state for this block.
	DisconnectBlock(database.Tx, *asiutil.Block, []txo.SpentTxOut, *asiutil.VBlock) error
}
