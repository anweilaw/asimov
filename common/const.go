// Copyright (c) 2018-2020. The asimov developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package common

const (
	// XingPerAsimov is the number of xing in one asimov coin (1 ASC).
	XingPerAsimov int64 = 1e8

	// MaxXing is the maximum transaction amount allowed in xing.
	// 10 billion
	MaxXing = 1e10 * XingPerAsimov

	// MaxPrice is the maximum gas price limit.
	MaxPrice = 1000

	// Considering that evm costs 21000 gas, we suppose the average transaction's
	// length is 1k bytes, hence each byte cost 21 gas.
	GasPerByte = 21

	// CoinbaseTxGas use 100000, constantly.
	CoinbaseTxGas = 100000

	// CommandSize is the fixed size of all commands in the common bitcoin message
	// header.  Shorter commands must be zero padded.
	CommandSize = 12

	// MaxBlockSize is the maximum number of bytes within a block
	MaxBlockSize = 2 * 1024 * 1024

	// CoreTeamPercent is the percent of block reward
	CoreTeamPercent = 0.2

	// The gas is used when consensus invoke some readonly function in contract.
	ReadOnlyGas = 2300

	// The gas is used when checking whether an address is in whitelist on limit assets.
	SupportCheckGas = 6000

	// The gas is used when consensus invoke some readonly function in system contract.
	SystemContractReadOnlyGas = 30000000

	// max block height depth which can be included in header.
	// such as block N can include block sign of N-1, ... N-BlockSignDepth
	BlockSignDepth = 10

	// Target gas floor for mined blocks.
	GasFloor = 160000000

	// Target gas ceiling for mined blocks.
	GasCeil = 160000000

	// Min interval time seconds of blocks
	MinBlockInterval = 3

	// Max interval time seconds of blocks
	MaxBlockInterval = 15

	// The default interval time seconds of blocks
	DefaultBlockInterval = 5

	// LockTimeThreshold is the number below which a lock time is
	// interpreted to be a block number.  Since an average of one block
	// is generated per 5 seconds, this allows blocks for about 237
	// years.
	LockTimeThreshold = 1.5e8 // Tue Jul 14 02:40:00 2017 UTC
)
