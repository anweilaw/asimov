// Copyright (c) 2018-2020. The asimov developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.
package ainterface

import (
	"github.com/AsimovNetwork/asimov/common"
)

type Consensus interface {
	Start() error
	Halt() error
	GetRoundInterval() int64
}

type BlockNode interface {
	Hash() common.Hash
	StateRoot() common.Hash
	Round() uint32
	Slot() uint16
	Height() int32
	GasUsed() uint64
	GasLimit() uint64
}

