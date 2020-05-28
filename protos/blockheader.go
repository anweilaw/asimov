// Copyright (c) 2018-2020 The asimov developers
// Copyright (c) 2013-2017 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package protos

import (
	"bytes"
	"github.com/AsimovNetwork/asimov/common"
	"github.com/AsimovNetwork/asimov/common/serialization"
	"io"
	"time"
	"unsafe"
)

const (
	// blockHeaderFixedPayload is a constant that represents the part fixed number of bytes for a block
	// header.
	// Version 4 bytes + Timestamp 8 bytes
	// GasLimit 8 bytes + GasUsed 8 bytes + round 4 bytes + slot 2 bytes +
	// Weight 2 bytes + Round 4 bytes + Coinbase 21 bytes
	// PrevBlock, MerkleRoot, stateRoot and PoaHash hashes
	blockHeaderFixedPayload = 61 + (common.HashLength * 4)

	// max sign len
	HashSignLen = 65

	// BlockHeaderPayload is the number of bytes a block header can be.
	// blockHeaderFixedPayload + SigData
	BlockHeaderPayload = blockHeaderFixedPayload + HashSignLen
)

// BlockHeader defines information about a block and is used in the asimov
// block (MsgBlock) and headers (MsgHeaders) messages.
type BlockHeader struct {
	// Version of the block.  This is not the same as the protocol version.
	Version int32

	// Hash of the previous block header in the block chain.
	PrevBlock common.Hash

	// Merkle tree reference to hash of all transactions for the block.
	MerkleRoot common.Hash

	// Time the block was created.
	Timestamp int64

	// VM fields
	StateRoot common.Hash
	GasLimit uint64
	GasUsed  uint64

	// Consensus fields
	Round     uint32
	SlotIndex uint16

	// The weight of block, it is the sum of all weight of signatures.
	Weight    uint16
	// Hash of ReceiptHash Bloom and PreBlockSigs
	PoaHash   common.Hash
	Height    int32

	CoinBase  common.Address
	SigData   [HashSignLen]byte
}

// BlockHash computes the block identifier hash for the given block header.
func (bh *BlockHeader) BlockHash() common.Hash {
	// Encode the header and double sha256 everything prior to the number of
	// transactions.  Ignore the error returns since there is no way the
	// encode could fail except being out of memory which would cause a
	// run-time panic.
	buflen := blockHeaderFixedPayload

	buf := bytes.NewBuffer(make([]byte, 0, buflen))
	err := bh.SerializeNode(buf)
	if err != nil {
		panic(err)
	}

	return common.DoubleHashH(buf.Bytes())
}

// Deserialize decodes a block header from r into the receiver using a format
// that is suitable for long-term storage such as a database while respecting
// the Version field.
func (bh *BlockHeader) Deserialize(r io.Reader) error {
	// At the current time, there is no difference between the protos encoding
	// at protocol version 0 and the stable long-term storage format.  As
	// a result, make use of readBlockHeader.
	if err := serialization.ReadUint32(r, (*uint32)(unsafe.Pointer(&bh.Version))); err != nil {
		return err
	}
	if err := serialization.ReadNBytes(r, bh.PrevBlock[:], common.HashLength); err != nil {
		return err
	}
	if err := serialization.ReadNBytes(r, bh.MerkleRoot[:], common.HashLength); err != nil {
		return err
	}
	if err := serialization.ReadUint64(r, (*uint64)(unsafe.Pointer(&bh.Timestamp))); err != nil {
		return err
	}

	if err := serialization.ReadNBytes(r, bh.StateRoot[:], common.HashLength); err != nil {
		return err
	}
	if err := serialization.ReadUint64(r, &bh.GasLimit); err != nil {
		return err
	}
	if err := serialization.ReadUint64(r, &bh.GasUsed); err != nil {
		return err
	}

	if err := serialization.ReadUint32(r, &bh.Round); err != nil {
		return err
	}

	if err := serialization.ReadUint16(r, &bh.SlotIndex); err != nil {
		return err
	}

	if err := serialization.ReadUint16(r, &bh.Weight); err != nil {
		return err
	}

	if err := serialization.ReadNBytes(r, bh.PoaHash[:], common.HashLength); err != nil {
		return err
	}

	if err := serialization.ReadUint32(r, (*uint32)(unsafe.Pointer(&bh.Height))); err != nil {
		return err
	}

	if err := serialization.ReadNBytes(r, bh.CoinBase[:], common.AddressLength); err != nil {
		return err
	}

	err := serialization.ReadNBytes(r, bh.SigData[:], HashSignLen)
	if err != nil {
		return err
	}

	return nil
}

// Serialize encodes a block header from w into the receiver using a format
// that is suitable for long-term storage such as a database while respecting
// the Version field.
func (bh *BlockHeader) Serialize(w io.Writer) error {
	// At the current time, there is no difference between the protos encoding
	// at protocol version 0 and the stable long-term storage format.  As
	// a result, make use of writeBlockHeader.
	err := bh.SerializeNode(w)
	if err != nil {
		return err
	}

	err = serialization.WriteNBytes(w, bh.SigData[:])
	if err != nil {
		return err
	}

	return nil
}

// NewBlockHeader returns a new BlockHeader using the provided version, previous
// block hash, merkle root hash, difficulty bits, and nonce used to generate the
// block with defaults for the remaining fields.
func NewBlockHeader(version int32, prevHash *common.Hash) *BlockHeader {

	// Limit the timestamp to one second precision since the protocol
	// doesn't support better.
	return &BlockHeader{
		Version:   version,
		PrevBlock: *prevHash,
		Timestamp: time.Now().Unix(),
	}
}

// readBlockHeader reads a bitcoin block header from r.  See Deserialize for
// decoding block headers stored to disk, such as in a database, as opposed to
// decoding from the protos.
func readBlockHeader(r io.Reader, bh *BlockHeader) error {
	return bh.Deserialize(r)
}

// writeBlockHeader writes a bitcoin block header to w.  See Serialize for
// encoding block headers to be stored to disk, such as in a database, as
// opposed to encoding for the protos.
func writeBlockHeader(w io.Writer, bh *BlockHeader) error {
	return bh.Serialize(w)
}

// SerializeNode encodes BlockHeader to w using the asimov protocol encoding.
func (bh *BlockHeader) SerializeNode(w io.Writer) error {
	if err := serialization.WriteUint32(w, uint32(bh.Version)); err != nil {
		return err
	}
	if err := serialization.WriteNBytes(w, bh.PrevBlock[:]); err != nil {
		return err
	}
	if err := serialization.WriteNBytes(w, bh.MerkleRoot[:]); err != nil {
		return err
	}
	if err := serialization.WriteUint64(w, uint64(bh.Timestamp)); err != nil {
		return err
	}
	if err := serialization.WriteNBytes(w, bh.StateRoot[:]); err != nil {
		return err
	}
	if err := serialization.WriteUint64(w, bh.GasLimit); err != nil {
		return err
	}
	if err := serialization.WriteUint64(w, bh.GasUsed); err != nil {
		return err
	}
	if err := serialization.WriteUint32(w, bh.Round); err != nil {
		return err
	}
	if err := serialization.WriteUint16(w, bh.SlotIndex); err != nil {
		return err
	}
	if err := serialization.WriteUint16(w, bh.Weight); err != nil {
		return err
	}
	if err := serialization.WriteNBytes(w, bh.PoaHash[:]); err != nil {
		return err
	}
	if err := serialization.WriteUint32(w, uint32(bh.Height)); err != nil {
		return err
	}
	if err := serialization.WriteNBytes(w, bh.CoinBase[:]); err != nil {
		return err
	}
	return nil
}
