// Copyright (c) 2018-2020 The asimov developers
// Copyright (c) 2013-2017 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package protos

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math/big"
	"strconv"
	"unsafe"

	"github.com/AsimovNetwork/asimov/common"
	"github.com/AsimovNetwork/asimov/common/serialization"
)

const (
	// TxVersion is the current latest supported transaction version.
	TxVersion = 1

	// MaxTxInSequenceNum is the maximum sequence number the sequence field
	// of a transaction input can be.
	MaxTxInSequenceNum uint32 = 0xffffffff

	// MaxPrevOutIndex is the maximum index the index field of a previous
	// outpoint can be.
	MaxPrevOutIndex uint32 = 0xffffffff

	// SequenceLockTimeDisabled is a flag that if set on a transaction
	// input's sequence number, the sequence number will not be interpreted
	// as a relative lock time.
	SequenceLockTimeDisabled = 1 << 31

	// SequenceLockTimeIsSeconds is a flag that if set on a transaction
	// input's sequence number, the relative locktime has units of 512
	// seconds.
	SequenceLockTimeIsSeconds = 1 << 22

	// SequenceLockTimeMask is a mask that extracts the relative locktime
	// when masked against the transaction input sequence number.
	SequenceLockTimeMask = 0x0000ffff

	// SequenceLockTimeGranularity is the defined time based granularity
	// for seconds-based relative time locks. When converting from seconds
	// to a sequence number, the value is right shifted by this amount,
	// therefore the granularity of relative time locks in 512 or 2^9
	// seconds. Enforced relative lock times are multiples of 512 seconds.
	SequenceLockTimeGranularity = 9

	// defaultTxInOutAlloc is the default size used for the backing array for
	// transaction inputs and outputs.  The array will dynamically grow as needed,
	// but this figure is intended to provide enough space for the number of
	// inputs and outputs in a typical transaction without needing to grow the
	// backing array multiple times.
	defaultTxInOutAlloc = 15

	// minTxInPayload is the minimum payload size for a transaction input.
	// PreviousOutPoint.Hash + PreviousOutPoint.Index 4 bytes + Varint for
	// SignatureScript length 1 byte + Sequence 4 bytes.
	minTxInPayload = 9 + common.HashLength

	// MaxTxInPerMessage is the maximum number of transactions inputs that
	// a transaction which fits into a message could possibly have.
	MaxTxInPerMessage = MaxMessagePayload / minTxInPayload

	// MinTxOutPayload is the minimum payload size for a transaction output.
	// Value 8 bytes + Asset 12 bytes + Varint for Asset 1 byte +
	// Varint for PkScript length 1 byte + Varint for Data 1 byte.
	minTxOutPayload = 23

	// MaxTxOutPerMessage is the maximum number of transactions outputs that
	// a transaction which fits into a message could possibly have.
	MaxTxOutPerMessage = MaxMessagePayload / minTxOutPayload

	// minTxPayload is the minimum payload size for a transaction.  Note
	// that any realistically usable transaction must have at least one
	// input or output, but that is a rule enforced at a higher layer, so
	// it is intentionally not included here.
	// Version 4 bytes + Varint number of transaction inputs 1 byte + Varint
	// number of transaction outputs 1 byte + LockTime 4 bytes + min input
	// payload + min output payload.
	minTxPayload = 10

	// freeListMaxScriptSize is the size of each buffer in the free list
	// that	is used for deserializing scripts from the protos before they are
	// concatenated into a single contiguous buffers.  This value was chosen
	// because it is slightly more than twice the size of the vast majority
	// of all "standard" scripts.  Larger scripts are still deserialized
	// properly as the free list will simply be bypassed for them.
	freeListMaxScriptSize = 512

	// freeListMaxItems is the number of buffers to keep in the free list
	// to use for script deserialization.  This value allows up to 100
	// scripts per transaction being simultaneously deserialized by 125
	// peers.  Thus, the peak usage of the free list is 12,500 * 512 =
	// 6,400,000 bytes.
	freeListMaxItems = 12500

	// DivisibleAsset is the default divisible property of asimov coin
	DivisibleAsset uint32 = 0

	// DefaultCoinId is the default coin id of asimov coin
	DefaultCoinId uint32 = 0

	// DefaultOrgId is the default organization id of asimov coin
	DefaultOrgId uint32 = 0
)

const (
	InDivisibleAsset = 1 << iota
)

// scriptFreeList defines a free list of byte slices (up to the maximum number
// defined by the freeListMaxItems constant) that have a cap according to the
// freeListMaxScriptSize common.  It is used to provide temporary buffers for
// deserializing scripts in order to greatly reduce the number of allocations
// required.
//
// The caller can obtain a buffer from the free list by calling the Borrow
// function and should return it via the Return function when done using it.
type scriptFreeList chan []byte

// Borrow returns a byte slice from the free list with a length according the
// provided size.  A new buffer is allocated if there are any items available.
//
// When the size is larger than the max size allowed for items on the free list
// a new buffer of the appropriate size is allocated and returned.  It is safe
// to attempt to return said buffer via the Return function as it will be
// ignored and allowed to go the garbage collector.
func (c scriptFreeList) Borrow(size uint64) []byte {
	if size > freeListMaxScriptSize {
		return make([]byte, size)
	}

	var buf []byte
	select {
	case buf = <-c:
	default:
		buf = make([]byte, freeListMaxScriptSize)
	}
	return buf[:size]
}

// Return puts the provided byte slice back on the free list when it has a cap
// of the expected length.  The buffer is expected to have been obtained via
// the Borrow function.  Any slices that are not of the appropriate size, such
// as those whose size is greater than the largest allowed free list item size
// are simply ignored so they can go to the garbage collector.
func (c scriptFreeList) Return(buf []byte) {
	// Ignore any buffers returned that aren't the expected size for the
	// free list.
	if cap(buf) != freeListMaxScriptSize {
		return
	}

	// Return the buffer to the free list when it's not full.  Otherwise let
	// it be garbage collected.
	select {
	case c <- buf:
	default:
		// Let it go to the garbage collector.
	}
}

// Create the concurrent safe free list to use for script deserialization.  As
// previously described, this free list is maintained to significantly reduce
// the number of allocations.
var scriptPool scriptFreeList = make(chan []byte, freeListMaxItems)

// OutPoint defines an asimov data type that is used to track previous
// transaction outputs.
type OutPoint struct {
	Hash  common.Hash
	Index uint32
}

// NewOutPoint returns a new asimov transaction outpoint point with the
// provided hash and index.
func NewOutPoint(hash *common.Hash, index uint32) *OutPoint {
	return &OutPoint{
		Hash:  *hash,
		Index: index,
	}
}

// String returns the OutPoint in the human-readable form "hash:index".
func (o OutPoint) String() string {
	// Allocate enough for hash string, colon, and 10 digits.  Although
	// at the time of writing, the number of digits can be no greater than
	// the length of the decimal representation of MaxTxOutPerMessage, the
	// maximum message payload may increase in the future and this
	// optimization may go unnoticed, so allocate space for 10 decimal
	// digits, which will fit any uint32.
	buf := make([]byte, 2*common.HashLength+1, 2*common.HashLength+1+10)
	copy(buf, o.Hash.UnprefixString())
	buf[2*common.HashLength] = ':'
	buf = strconv.AppendUint(buf, uint64(o.Index), 10)
	return string(buf)
}

// TxIn defines an asimov transaction input.
type TxIn struct {
	PreviousOutPoint OutPoint
	SignatureScript  []byte
	Sequence         uint32
}

func (ti *TxIn) toString() string {
	return fmt.Sprintf("\n	Sequence:%d\n	SignatureScript:%v\n	PreviousOutPoint:%v\n", ti.Sequence, ti.SignatureScript, ti.PreviousOutPoint)
}

// SerializeSize returns the number of bytes it would take to serialize the
// the transaction input.
func (ti *TxIn) SerializeSize() int {
	// Outpoint Hash 32 bytes + Outpoint Index 4 bytes + Sequence 4 bytes +
	// serialized varint size for the length of SignatureScript +
	// SignatureScript bytes.
	return 40 + serialization.VarIntSerializeSize(uint64(len(ti.SignatureScript))) +
		len(ti.SignatureScript)
}

// NewTxIn returns a new asimov transaction input with the provided
// previous outpoint point and signature script with a default sequence of
// MaxTxInSequenceNum.
func NewTxIn(prevOut *OutPoint, signatureScript []byte) *TxIn {
	return &TxIn{
		PreviousOutPoint: *prevOut,
		SignatureScript:  signatureScript,
		Sequence:         MaxTxInSequenceNum,
	}
}

// Asset defines an asimov data type that is used to distinguish different asset.
// Property: represents asset property. Currently 0th bit represents
//    divisible or not (1 indivisible),
//    the first bit of the second byte represents votable or not (1 votable);
// Id:[0-4) bytes represents the organization which creates this asset;
//    [4-8) bytes unique id distinguish different asset in the organization.
type Asset struct {
	Property uint32
	Id       uint64
}

func (a *Asset) String() string {
	return fmt.Sprintf("(%d, %d)", a.Property, a.Id)
}

// FixedBytes returns an asset with type of byte array
func (a *Asset) FixedBytes() [common.AssetLength]byte {
	var serialized [common.AssetLength]byte
	binary.BigEndian.PutUint32(serialized[:], a.Property)
	binary.BigEndian.PutUint64(serialized[4:], a.Id)
	return serialized
}

// Bytes returns an asset with type of byte slice
func (a *Asset) Bytes() []byte {
	serialized := a.FixedBytes()
	return serialized[:]
}

// IsFlowCoin checks if an asset is default asimov coin
func (a *Asset) IsFlowCoin() bool {
	return DefaultOrgId == uint32(a.Id >> 32)
}

// assetFields returns three parts of an asset.
// property + orgId + coinId
func (a *Asset) AssetFields() (uint32, uint32, uint32) {
	orgId := uint32(a.Id >> 32)
	coinId := uint32(a.Id & 0xFFFFFFFF)
	return a.Property, orgId, coinId
}

// IsIndivisible returns an asset is divisible or indivisible
func (a *Asset) IsIndivisible() bool {
	return (a.Property & InDivisibleAsset) == InDivisibleAsset
}

// Equal checks one asset if equal to another
func (a *Asset) Equal(other *Asset) bool {
	if other == nil && a == nil {
		return true
	}
	if other == nil || a == nil {
		return false
	}
	return a.Property == other.Property && a.Id == other.Id
}

// NewAsset returns struct of Asset, which combine of property, orgId and coinId
func NewAsset(property uint32, orgId uint32, coinId uint32) *Asset {
	return &Asset{
		Property: property,
		Id: (uint64(orgId) << 32) | uint64(coinId),
	}
}

// AssetFromInt returns struct of Asset from the given value
func AssetFromInt(val *big.Int) *Asset {
	asset := val.Bytes()
	paddingLength := common.AssetLength - len(asset)
	if paddingLength > 0 {
		padding := make([]byte, paddingLength)
		padding = append(padding, asset[:]...)
		asset = padding
	}
	property := binary.BigEndian.Uint32(asset[:4])
	id := binary.BigEndian.Uint64(asset[4:])
	return &Asset{
		Property: property,
		Id: id,
	}
}

// AssetFromBytes transfers byte asset to property asset
func AssetFromBytes(asset []byte) *Asset {
	paddingLength := common.AssetLength - len(asset)
	if paddingLength > 0 {
		padding := make([]byte, paddingLength)
		padding = append(padding, asset[:]...)
		asset = padding
	}
	property := binary.BigEndian.Uint32(asset[:4])
	id := binary.BigEndian.Uint64(asset[4:common.AssetLength])
	return &Asset{
		Property: property,
		Id: id,
	}
}

// AssetDetailFromBytes transfers byte asset to asset detail
func AssetDetailFromBytes(asset []byte) (uint32, uint32, uint32) {
	paddingLength := common.AssetLength - len(asset)
	if paddingLength > 0 {
		padding := make([]byte, paddingLength)
		padding = append(padding, asset[:]...)
		asset = padding
	}
	property := binary.BigEndian.Uint32(asset[:4])
	organizationId := binary.BigEndian.Uint32(asset[4:8])
	assetIndex := binary.BigEndian.Uint32(asset[8:common.AssetLength])

	return property, organizationId, assetIndex
}

// TxOut defines an asimov transaction output.
// Value: The amount of asset
// PkScript: The public key script
// Data: it may contains contract parameters, or it can be nil.
type TxOut struct {
	Value    int64
	PkScript []byte
	Asset    Asset
	Data     []byte
}

func (to *TxOut) toString() string {
	return fmt.Sprintf("(Value:%v, PkScript:%v, Asset:%v, Data:%v)", to.Value, to.PkScript, to.Asset, to.Data)
}

// SerializeSize returns the number of bytes it would take to serialize the
// the transaction output.
func (to *TxOut) SerializeSize() int {
	// Value 8 bytes + assetLength + Asset 12 byte
	// + serialized varint size for the length of PkScript +
	// PkScript bytes.
	// + serialized varint size for the length of Data +
	// Data bytes.
	return 21 +
		serialization.VarIntSerializeSize(uint64(len(to.PkScript))) + len(to.PkScript) +
		serialization.VarIntSerializeSize(uint64(len(to.Data))) + len(to.Data)
}

// NewTxOut returns a new asimov transaction output with the provided
// transaction value and public key script.
func NewTxOut(value int64, pkScript []byte, asset Asset) *TxOut {
	return &TxOut{
		Value:    value,
		PkScript: pkScript,
		Asset:    asset,
	}
}

// NewContractTxOut returns a new asimov contract transaction output
// with the provides transaction value, public key script and data
func NewContractTxOut(value int64, pkScript []byte, asset Asset, data []byte) *TxOut {
	return &TxOut{
		Value:    value,
		PkScript: pkScript,
		Asset:    asset,
		Data:     data,
	}
}

// Design on three steps.
// 1st, Add TxContract struct, done
// 2nd, Rm TxType done
// 3rd, Extend TxContract & refactor contract elements:
// when send a coin to contract, move out to extended TxContract,
// Standard TxOut only accept addresses with id 0x66 | 0x73
type TxContract struct {
	GasLimit uint32
}

// SerializeSize returns the number of bytes it would take to serialize the
// the transaction contract.
func (t *TxContract) SerializeSize() int {
	// serialized int size for GasLimit
	return 4
}

// MsgTx implements the Message interface and represents an asimov tx message.
// It is used to deliver transaction information in response to a getdata
// message (MsgGetData) for a given transaction.
//
// Use the AddTxIn and AddTxOut functions to build up the list of transaction
// inputs and outputs.
type MsgTx struct {
	Version    uint32 //对于虚拟交易，表示对应的真实交易的索引号。
	TxIn       []*TxIn
	TxOut      []*TxOut
	TxContract TxContract
	LockTime   uint32
}

func (msg *MsgTx) String() string {
	var txIn, txOut string
	for _, in := range msg.TxIn {
		txIn += in.toString()
		txIn += "\n"
	}

	for _, out := range msg.TxOut {
		txOut += out.toString()
		txOut += "\n"
	}

	return fmt.Sprintf("\nVersion:%v\n LockTime:%d\n TxIn:%s\n TxOut:%s\n", msg.Version, msg.LockTime, txIn, txOut)
}

// AddTxIn adds a transaction input to the message.
func (msg *MsgTx) AddTxIn(ti *TxIn) {
	msg.TxIn = append(msg.TxIn, ti)
}

// AddTxOut adds a transaction output to the message.
func (msg *MsgTx) AddTxOut(to *TxOut) {
	msg.TxOut = append(msg.TxOut, to)
}

// AddTxContract adds a transaction contract to the message.
func (msg *MsgTx) SetTxContract(tc *TxContract) {
	msg.TxContract = *tc
}

// TxHash generates the Hash for the transaction.
func (msg *MsgTx) TxHash() common.Hash {
	// Encode the transaction and calculate double sha256 on the result.
	// Ignore the error returns since the only way the encode could fail
	// is being out of memory or due to nil pointers, both of which would
	// cause a run-time panic.
	buf := bytes.NewBuffer(make([]byte, 0, msg.SerializeSize()))
	_ = msg.Serialize(buf)
	return common.DoubleHashH(buf.Bytes())
}

// Copy creates a deep copy of a transaction so that the original does not get
// modified when the copy is manipulated.
func (msg *MsgTx) Copy() *MsgTx {
	// Create new tx and start by copying primitive values and making space
	// for the transaction inputs and outputs.
	newTx := MsgTx{
		Version:  msg.Version,
		TxIn:     make([]*TxIn, 0, len(msg.TxIn)),
		TxOut:    make([]*TxOut, 0, len(msg.TxOut)),
		LockTime: msg.LockTime,
	}

	// Deep copy the old TxIn data.
	for _, oldTxIn := range msg.TxIn {
		// Deep copy the old previous outpoint.
		oldOutPoint := oldTxIn.PreviousOutPoint
		newOutPoint := OutPoint{}
		newOutPoint.Hash.SetBytes(oldOutPoint.Hash[:])
		newOutPoint.Index = oldOutPoint.Index

		// Deep copy the old signature script.
		var newScript []byte
		oldScript := oldTxIn.SignatureScript
		oldScriptLen := len(oldScript)
		if oldScriptLen > 0 {
			newScript = make([]byte, oldScriptLen)
			copy(newScript, oldScript[:oldScriptLen])
		}

		// Create new txIn with the deep copied data.
		newTxIn := TxIn{
			PreviousOutPoint: newOutPoint,
			SignatureScript:  newScript,
			Sequence:         oldTxIn.Sequence,
		}

		// Finally, append this fully copied txin.
		newTx.TxIn = append(newTx.TxIn, &newTxIn)
	}

	// Deep copy the old TxOut data.
	for _, oldTxOut := range msg.TxOut {
		// Deep copy the old PkScript
		var newScript []byte
		oldScript := oldTxOut.PkScript
		oldScriptLen := len(oldScript)
		if oldScriptLen > 0 {
			newScript = make([]byte, oldScriptLen)
			copy(newScript, oldScript[:oldScriptLen])
		}

		// Deep copy the old asset
		newAsset := Asset{}
		newAsset.Property = oldTxOut.Asset.Property
		newAsset.Id = oldTxOut.Asset.Id

		// Deep copy the old data
		var newData []byte
		oldData := oldTxOut.Data
		oldDataLen := len(oldData)
		if oldDataLen > 0 {
			newData = make([]byte, oldDataLen)
			copy(newData, oldData[:oldDataLen])
		}

		// Create new txOut with the deep copied data and append it to
		// new Tx.
		newTxOut := TxOut{
			Value:    oldTxOut.Value,
			PkScript: newScript,
			Asset:    newAsset,
			Data:     newData,
		}
		newTx.TxOut = append(newTx.TxOut, &newTxOut)
	}

	newTx.TxContract = msg.TxContract
	return &newTx
}

// VVSDecode decodes r using the asimov protocol encoding into the receiver.
// This is part of the Message interface implementation.
// See Deserialize for decoding transactions stored to disk, such as in a
// database, as opposed to decoding transactions from the protos.
func (msg *MsgTx) VVSDecode(r io.Reader, pver uint32, enc MessageEncoding) error {
	err := serialization.ReadUint32(r, &msg.Version)
	if err != nil {
		return err
	}

	count, err := serialization.ReadVarInt(r, pver)
	if err != nil {
		return err
	}

	// Prevent more input transactions than could possibly fit into a
	// message.  It would be possible to cause memory exhaustion and panics
	// without a sane upper bound on this count.
	if count > uint64(MaxTxInPerMessage) {
		str := fmt.Sprintf("too many input transactions to fit into "+
			"max message size [count %d, max %d]", count,
			MaxTxInPerMessage)
		return messageError("MsgTx.VVSDecode", str)
	}

	// returnScriptBuffers is a closure that returns any script buffers that
	// were borrowed from the pool when there are any deserialization
	// errors.  This is only valid to call before the final step which
	// replaces the scripts with the location in a contiguous buffer and
	// returns them.
	returnScriptBuffers := func() {
		for _, txIn := range msg.TxIn {
			if txIn == nil {
				continue
			}

			if txIn.SignatureScript != nil {
				scriptPool.Return(txIn.SignatureScript)
			}
		}
		for _, txOut := range msg.TxOut {
			if txOut == nil || txOut.PkScript == nil {
				continue
			}
			scriptPool.Return(txOut.PkScript)
		}
	}

	// Deserialize the inputs.
	var totalScriptSize uint64
	txIns := make([]TxIn, count)
	msg.TxIn = make([]*TxIn, count)
	for i := uint64(0); i < count; i++ {
		// The pointer is set now in case a script buffer is borrowed
		// and needs to be returned to the pool on error.
		ti := &txIns[i]
		msg.TxIn[i] = ti
		err = readTxIn(r, pver, ti)
		if err != nil {
			returnScriptBuffers()
			return err
		}

		totalScriptSize += uint64(len(ti.SignatureScript))
	}

	count, err = serialization.ReadVarInt(r, pver)
	if err != nil {
		returnScriptBuffers()
		return err
	}

	txoutCount := count
	// Prevent more output transactions than could possibly fit into a
	// message.  It would be possible to cause memory exhaustion and panics
	// without a sane upper bound on this count.
	if txoutCount > uint64(MaxTxOutPerMessage) {
		returnScriptBuffers()
		str := fmt.Sprintf("too many output transactions to fit into "+
			"max message size [count %d, max %d]", txoutCount,
			MaxTxOutPerMessage)
		return messageError("MsgTx.VVSDecode", str)
	}

	// Deserialize the outputs.
	txOuts := make([]TxOut, txoutCount)
	msg.TxOut = make([]*TxOut, txoutCount)
	for i := uint64(0); i < txoutCount; i++ {
		// The pointer is set now in case a script buffer is borrowed
		// and needs to be returned to the pool on error.
		to := &txOuts[i]
		msg.TxOut[i] = to
		err = readTxOut(r, pver, to)
		if err != nil {
			returnScriptBuffers()
			return err
		}
		totalScriptSize += uint64(len(to.PkScript))
	}

	// The pointer is set now in case a script buffer is borrowed
	// and needs to be returned to the pool on error.
	err = readTxContract(r, &msg.TxContract)
	if err != nil {
		returnScriptBuffers()
		return err
	}

	err = serialization.ReadUint32(r, &msg.LockTime)
	if err != nil {
		returnScriptBuffers()
		return err
	}

	// Create a single allocation to house all of the scripts and set each
	// input signature script and output public key script to the
	// appropriate subslice of the overall contiguous buffer.  Then, return
	// each individual script buffer back to the pool so they can be reused
	// for future deserializations.  This is done because it significantly
	// reduces the number of allocations the garbage collector needs to
	// track, which in turn improves performance and drastically reduces the
	// amount of runtime overhead that would otherwise be needed to keep
	// track of millions of small allocations.
	//
	// NOTE: It is no longer valid to call the returnScriptBuffers closure
	// after these blocks of code run because it is already done and the
	// scripts in the transaction inputs and outputs no longer point to the
	// buffers.
	var offset uint64
	scripts := make([]byte, totalScriptSize)
	for i := 0; i < len(msg.TxIn); i++ {
		// Copy the signature script into the contiguous buffer at the
		// appropriate offset.
		signatureScript := msg.TxIn[i].SignatureScript
		copy(scripts[offset:], signatureScript)

		// Reset the signature script of the transaction input to the
		// slice of the contiguous buffer where the script lives.
		scriptSize := uint64(len(signatureScript))
		end := offset + scriptSize
		msg.TxIn[i].SignatureScript = scripts[offset:end:end]
		offset += scriptSize

		// Return the temporary script buffer to the pool.
		scriptPool.Return(signatureScript)
	}
	for i := 0; i < len(msg.TxOut); i++ {
		// Copy the public key script into the contiguous buffer at the
		// appropriate offset.
		pkScript := msg.TxOut[i].PkScript
		copy(scripts[offset:], pkScript)

		// Reset the public key script of the transaction output to the
		// slice of the contiguous buffer where the script lives.
		scriptSize := uint64(len(pkScript))
		end := offset + scriptSize
		msg.TxOut[i].PkScript = scripts[offset:end:end]
		offset += scriptSize

		// Return the temporary script buffer to the pool.
		scriptPool.Return(pkScript)
	}

	return nil
}

// Deserialize decodes a transaction from r into the receiver using a format
// that is suitable for long-term storage such as a database while respecting
// the Version field in the transaction.  This function differs from VVSDecode
// in that VVSDecode decodes from the asimov protos protocol as it was sent
// across the network.  The protos encoding can technically differ depending on
// the protocol version and doesn't even really need to match the format of a
// stored transaction at all.  As of the time this comment was written, the
// encoded transaction is the same in both instances, but there is a distinct
// difference and separating the two allows the API to be flexible enough to
// deal with changes.
func (msg *MsgTx) Deserialize(r io.Reader) error {
	// At the current time, there is no difference between the protos encoding
	// at protocol version 0 and the stable long-term storage format.  As
	// a result, make use of VVSDecode.
	return msg.VVSDecode(r, 0, BaseEncoding)
}

// Serialize encodes the transaction to w using a format that suitable for
// long-term storage such as a database while respecting the Version field in
// the transaction.  This function differs from VVSEncode in that VVSEncode
// encodes the transaction to the asimov protos protocol in order to be sent
// across the network.  The protos encoding can technically differ depending on
// the protocol version and doesn't even really need to match the format of a
// stored transaction at all.  As of the time this comment was written, the
// encoded transaction is the same in both instances, but there is a distinct
// difference and separating the two allows the API to be flexible enough to
// deal with changes.
func (msg *MsgTx) Serialize(w io.Writer) error {
	// At the current time, there is no difference between the protos encoding
	// at protocol version 0 and the stable long-term storage format.  As
	// a result, make use of VVSEncode.
	//
	// Passing a encoding type of WitnessEncoding to VVSEncode for MsgTx
	// indicates that the transaction's witnesses (if any) should be
	// serialized according to the new serialization structure defined in
	// BIP0144.
	return msg.VVSEncode(w, 0, BaseEncoding)
}

// If a transaction has no witness data, then the witness hash,
// is the same as its txid.
func (msg *MsgTx) TransactionHash() common.Hash {
	return msg.TxHash()
}

// VVSEncode encodes the receiver to w using the asimov protocol encoding.
// This is part of the Message interface implementation.
// See Serialize for encoding transactions to be stored to disk, such as in a
// database, as opposed to encoding transactions for the protos.
func (msg *MsgTx) VVSEncode(w io.Writer, pver uint32, enc MessageEncoding) error {
	err := serialization.WriteUint32(w, msg.Version)
	if err != nil {
		return err
	}

	count := uint64(len(msg.TxIn))
	err = serialization.WriteVarInt(w, pver, count)
	if err != nil {
		return err
	}

	for _, ti := range msg.TxIn {
		err = writeTxIn(w, pver, ti)
		if err != nil {
			return err
		}
	}

	count = uint64(len(msg.TxOut))
	err = serialization.WriteVarInt(w, pver, count)
	if err != nil {
		return err
	}

	for _, to := range msg.TxOut {
		err = writeTxOut(w, pver, to)
		if err != nil {
			return err
		}
	}

	err = writeTxContract(w, &msg.TxContract)
	if err != nil {
		return err
	}

	return serialization.WriteUint32(w, msg.LockTime)
}

// SerializeSize returns the number of bytes it would take to serialize the
// the transaction.
func (msg *MsgTx) SerializeSize() int {
	// Version 4 bytes + LockTime 4 bytes + TxContract 4 bytes + Serialized varint size for the
	// number of transaction inputs and outputs.
	n := 12 + serialization.VarIntSerializeSize(uint64(len(msg.TxIn))) +
		serialization.VarIntSerializeSize(uint64(len(msg.TxOut)))

	for _, txIn := range msg.TxIn {
		n += txIn.SerializeSize()
	}

	for _, txOut := range msg.TxOut {
		n += txOut.SerializeSize()
	}

	return n
}

// Command returns the protocol command string for the message.  This is part
// of the Message interface implementation.
func (msg *MsgTx) Command() string {
	return CmdTx
}

// MaxPayloadLength returns the maximum length the payload can be for the
// receiver.  This is part of the Message interface implementation.
func (msg *MsgTx) MaxPayloadLength(pver uint32) uint32 {
	return MaxBlockPayload
}

// PkScriptLocs returns a slice containing the start of each public key script
// within the raw serialized transaction.  The caller can easily obtain the
// length of each script by using len on the script available via the
// appropriate transaction output entry.
func (msg *MsgTx) PkScriptLocs() []int {
	numTxOut := len(msg.TxOut)
	if numTxOut == 0 {
		return nil
	}

	// The starting offset in the serialized transaction of the first
	// transaction output is:
	//
	// Version 4 bytes + serialized varint size for the number of
	// transaction inputs and outputs + serialized size of each transaction
	// input.
	n := 4 + serialization.VarIntSerializeSize(uint64(len(msg.TxIn))) +
		serialization.VarIntSerializeSize(uint64(numTxOut))

	for _, txIn := range msg.TxIn {
		n += txIn.SerializeSize()
	}

	// Calculate and set the appropriate offset for each public key script.
	pkScriptLocs := make([]int, numTxOut)
	for i, txOut := range msg.TxOut {
		// The offset of the script in the transaction output is:
		//
		// Value 8 bytes + serialized varint size for the length of
		// PkScript.
		n += 8 + serialization.VarIntSerializeSize(uint64(len(txOut.PkScript)))
		pkScriptLocs[i] = n
		n += len(txOut.PkScript) + serialization.VarIntSerializeSize(uint64(common.AssetLength)) + common.AssetLength
		n += serialization.VarIntSerializeSize(uint64(len(txOut.Data))) + len(txOut.Data)
	}

	return pkScriptLocs
}

// DataLoc returns the start of the first txout's data
func (msg *MsgTx) DataLoc() int {
	numTxOut := len(msg.TxOut)
	if numTxOut == 0 {
		return 0
	}
	// The starting offset in the serialized transaction of the first
	// transaction output is:
	//
	// Version 4 bytes + serialized varint size for the number of
	// transaction inputs and outputs + serialized size of each transaction
	// input.
	n := 4 + serialization.VarIntSerializeSize(uint64(len(msg.TxIn))) +
		serialization.VarIntSerializeSize(uint64(numTxOut))

	for _, txIn := range msg.TxIn {
		n += txIn.SerializeSize()
	}

	txOut := msg.TxOut[0]
	// Value 8 bytes + serialized varint size for the length of PkScript
	// + the length of PkScript + serialized varint size for the length of Asset
	// + the length of Asset + serialized varint size for the length of Data
	n += 8 + serialization.VarIntSerializeSize(uint64(len(txOut.PkScript))) + len(txOut.PkScript)
	n += common.AssetLength + serialization.VarIntSerializeSize(uint64(common.AssetLength))
	n += serialization.VarIntSerializeSize(uint64(len(txOut.Data)))

	return n
}

// NewMsgTx returns a new asimov tx message that conforms to the Message
// interface.  The return instance has a default version of TxVersion and there
// are no transaction inputs or outputs.  Also, the lock time is set to zero
// to indicate the transaction is valid immediately as opposed to some time in
// future.
func NewMsgTx(version uint32) *MsgTx {
	return &MsgTx{
		Version: version,
		TxIn:    make([]*TxIn, 0, defaultTxInOutAlloc),
		TxOut:   make([]*TxOut, 0, defaultTxInOutAlloc),
	}
}

// readOutPoint reads the next sequence of bytes from r as an OutPoint.
func readOutPoint(r io.Reader, op *OutPoint) error {
	_, err := io.ReadFull(r, op.Hash[:])
	if err != nil {
		return err
	}

	return serialization.ReadUint32(r, &op.Index)
}

// writeOutPoint encodes op to the asimov protocol encoding for an OutPoint
// to w.
func writeOutPoint(w io.Writer, op *OutPoint) error {
	_, err := w.Write(op.Hash[:])
	if err != nil {
		return err
	}

	return serialization.WriteUint32(w, op.Index)
}

// readScript reads a variable length byte array that represents a transaction
// script.  It is encoded as a varInt containing the length of the array
// followed by the bytes themselves.  An error is returned if the length is
// greater than the passed maxAllowed parameter which helps protect against
// memory exhaustion attacks and forced panics through malformed messages.  The
// fieldName parameter is only used for the error message so it provides more
// context in the error.
func readScript(r io.Reader, pver uint32, maxAllowed uint32, fieldName string) ([]byte, error) {
	count, err := serialization.ReadVarInt(r, pver)
	if err != nil {
		return nil, err
	}

	// Prevent byte array larger than the max message size.  It would
	// be possible to cause memory exhaustion and panics without a sane
	// upper bound on this count.
	if count > uint64(maxAllowed) {
		str := fmt.Sprintf("%s is larger than the max allowed size "+
			"[count %d, max %d]", fieldName, count, maxAllowed)
		return nil, messageError("readScript", str)
	}

	b := scriptPool.Borrow(count)
	_, err = io.ReadFull(r, b)
	if err != nil {
		scriptPool.Return(b)
		return nil, err
	}
	return b, nil
}

// readTxIn reads the next sequence of bytes from r as a transaction input
// (TxIn).
func readTxIn(r io.Reader, pver uint32, ti *TxIn) error {
	err := readOutPoint(r, &ti.PreviousOutPoint)
	if err != nil {
		return err
	}

	ti.SignatureScript, err = readScript(r, pver, MaxMessagePayload,
		"transaction input signature script")
	if err != nil {
		return err
	}

	err = serialization.ReadUint32(r, &ti.Sequence)
	if err != nil {
		return err
	}

	return nil
}

// writeTxIn encodes ti to the asimov protocol encoding for a transaction
// input (TxIn) to w.
func writeTxIn(w io.Writer, pver uint32, ti *TxIn) error {
	err := writeOutPoint(w, &ti.PreviousOutPoint)
	if err != nil {
		return err
	}

	err = serialization.WriteVarBytes(w, pver, ti.SignatureScript)
	if err != nil {
		return err
	}

	err = serialization.WriteUint32(w, ti.Sequence)
	if err != nil {
		return nil
	}

	return nil
}

// readTxOut reads the next sequence of bytes from r as a transaction output
// (TxOut).
func readTxOut(r io.Reader, pver uint32, to *TxOut) error {
	err := serialization.ReadUint64(r, (*uint64)(unsafe.Pointer(&to.Value)))
	if err != nil {
		return err
	}

	to.PkScript, err = readScript(r, pver, MaxMessagePayload,
		"transaction output public key script")
	if err != nil {
		return err
	}

	_, err = serialization.ReadVarInt(r, 0)
	if err != nil {
		return err
	}

	err = serialization.ReadUint32B(r, (*uint32)(unsafe.Pointer(&to.Asset.Property)))
	if err != nil {
		return err
	}
	err = serialization.ReadUint64B(r, &to.Asset.Id)
	if err != nil {
		return err
	}

	to.Data, err = serialization.ReadVarBytes(r, pver, MaxMessagePayload, "transaction output data")
	if err != nil {
		return err
	}

	return nil
}

// writeTxOut encodes to into the asimov protocol encoding for a transaction
// output (TxOut) to w.
func writeTxOut(w io.Writer, pver uint32, to *TxOut) error {
	err := serialization.WriteUint64(w, uint64(to.Value))
	if err != nil {
		return err
	}

	err = serialization.WriteVarBytes(w, pver, to.PkScript)
	if err != nil {
		return err
	}

	err = serialization.WriteVarInt(w, 0, common.AssetLength)
	if err != nil {
		return err
	}

	err = serialization.WriteUint32B(w, to.Asset.Property)
	if err != nil {
		return err
	}
	err = serialization.WriteUint64B(w, to.Asset.Id)
	if err != nil {
		return err
	}

	return serialization.WriteVarBytes(w, pver, to.Data)
}

// readTxContract reads the next sequence of bytes from r as a transaction output
// (TxContract).
func readTxContract(r io.Reader, tc *TxContract) (err error) {
	err = serialization.ReadUint32(r, &tc.GasLimit)
	return
}

// writeTxContract encodes to into the asimov protocol encoding for a transaction
// contract (TxContract) to w.
func writeTxContract(w io.Writer, tc *TxContract) error {
	return serialization.WriteUint32(w, tc.GasLimit)
}
