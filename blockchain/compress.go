// Copyright (c) 2018-2020. The asimov developers
// Copyright (c) 2013-2017 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.
package blockchain

import (
	"encoding/binary"
	"github.com/AsimovNetwork/asimov/common"
	"github.com/AsimovNetwork/asimov/crypto"
	"github.com/AsimovNetwork/asimov/protos"
	"github.com/AsimovNetwork/asimov/txscript"
)

// -----------------------------------------------------------------------------
// A variable length quantity (VLQ) is an encoding that uses an arbitrary number
// of binary octets to represent an arbitrarily large integer.  The scheme
// employs a most significant byte (MSB) base-128 encoding where the high bit in
// each byte indicates whether or not the byte is the final one.  In addition,
// to ensure there are no redundant encodings, an offset is subtracted every
// time a group of 7 bits is shifted out.  Therefore each integer can be
// represented in exactly one way, and each representation stands for exactly
// one integer.
//
// Another nice property of this encoding is that it provides a compact
// representation of values that are typically used to indicate sizes.  For
// example, the values 0 - 127 are represented with a single byte, 128 - 16511
// with two bytes, and 16512 - 2113663 with three bytes.
//
// While the encoding allows arbitrarily large integers, it is artificially
// limited in this code to an unsigned 64-bit integer for efficiency purposes.
//
// Example encodings:
//           0 -> [0x00]
//         127 -> [0x7f]                 * Max 1-byte value
//         128 -> [0x80 0x00]
//         129 -> [0x80 0x01]
//         255 -> [0x80 0x7f]
//         256 -> [0x81 0x00]
//       16511 -> [0xff 0x7f]            * Max 2-byte value
//       16512 -> [0x80 0x80 0x00]
//       32895 -> [0x80 0xff 0x7f]
//     2113663 -> [0xff 0xff 0x7f]       * Max 3-byte value
//   270549119 -> [0xff 0xff 0xff 0x7f]  * Max 4-byte value
//      2^64-1 -> [0x80 0xfe 0xfe 0xfe 0xfe 0xfe 0xfe 0xfe 0xfe 0x7f]
//
// References:
//   https://en.wikipedia.org/wiki/Variable-length_quantity
//   http://www.codecodex.com/wiki/Variable-Length_Integers
// -----------------------------------------------------------------------------

// serializeSizeVLQ returns the number of bytes it would take to serialize the
// passed number as a variable-length quantity according to the format described
// above.
func serializeSizeVLQ(n uint64) int {
	size := 1
	for ; n > 0x7f; n = (n >> 7) - 1 {
		size++
	}

	return size
}

// putVLQ serializes the provided number to a variable-length quantity according
// to the format described above and returns the number of bytes of the encoded
// value.  The result is placed directly into the passed byte slice which must
// be at least large enough to handle the number of bytes returned by the
// serializeSizeVLQ function or it will panic.
func putVLQ(target []byte, n uint64) int {
	offset := 0
	for ; ; offset++ {
		// The high bit is set when another byte follows.
		highBitMask := byte(0x80)
		if offset == 0 {
			highBitMask = 0x00
		}

		target[offset] = byte(n&0x7f) | highBitMask
		if n <= 0x7f {
			break
		}
		n = (n >> 7) - 1
	}

	// Reverse the bytes so it is MSB-encoded.
	for i, j := 0, offset; i < j; i, j = i+1, j-1 {
		target[i], target[j] = target[j], target[i]
	}

	return offset + 1
}

// deserializeVLQ deserializes the provided variable-length quantity according
// to the format described above.  It also returns the number of bytes
// deserialized.
func deserializeVLQ(serialized []byte) (uint64, int) {
	var n uint64
	var size int
	for _, val := range serialized {
		size++
		n = (n << 7) | uint64(val&0x7f)
		if val&0x80 != 0x80 {
			break
		}
		n++
	}

	return n, size
}

// -----------------------------------------------------------------------------
// In order to reduce the size of stored scripts, a domain specific compression
// algorithm is used which recognizes standard scripts and stores them using
// less bytes than the original script.  The compression algorithm used here was
// obtained from Bitcoin Core, so all credits for the algorithm go to it.
//
// The general serialized format is:
//
//   <script size or type><script data>
//
//   Field                 Type     Size
//   script size or type   VLQ      variable
//   script data           []byte   variable
//
// The specific serialized format for each recognized standard script is:
//
// - Pay-to-pubkey-hash: (21 bytes) - <0><20-byte pubkey hash>
// - Pay-to-script-hash: (21 bytes) - <1><20-byte script hash>
// - Pay-to-pubkey**:    (33 bytes) - <2, 3, 4, or 5><32-byte pubkey X value>
//   2, 3 = compressed pubkey with bit 0 specifying the y coordinate to use
//   4, 5 = uncompressed pubkey with bit 0 specifying the y coordinate to use
//   ** Only valid public keys starting with 0x02, 0x03, and 0x04 are supported.
//
// Any scripts which are not recognized as one of the aforementioned standard
// scripts are encoded using the general serialized format and encode the script
// size as the sum of the actual size of the script and the number of special
// cases.
// -----------------------------------------------------------------------------

// The following constants specify the special constants used to identify a
// special script type in the domain-specific compressed script encoding.
//
// NOTE: This section specifically does not use iota since these values are
// serialized and must be stable for long-term storage.
const (
	// cstPayToPubKeyHash identifies a compressed pay-to-pubkey-hash script.
	cstPayToPubKeyHash = 0

	// cstPayToScriptHash identifies a compressed pay-to-script-hash script.
	cstPayToScriptHash = 1

	// cstPayToPubKeyComp2 identifies a compressed pay-to-pubkey script to
	// a compressed pubkey.  Bit 0 specifies which y-coordinate to use
	// to reconstruct the full uncompressed pubkey.
	cstPayToPubKeyComp2 = 2

	// cstPayToPubKeyComp3 identifies a compressed pay-to-pubkey script to
	// a compressed pubkey.  Bit 0 specifies which y-coordinate to use
	// to reconstruct the full uncompressed pubkey.
	cstPayToPubKeyComp3 = 3

	// cstPayToPubKeyUncomp4 identifies a compressed pay-to-pubkey script to
	// an uncompressed pubkey.  Bit 0 specifies which y-coordinate to use
	// to reconstruct the full uncompressed pubkey.
	cstPayToPubKeyUncomp4 = 4

	// cstPayToPubKeyUncomp5 identifies a compressed pay-to-pubkey script to
	// an uncompressed pubkey.  Bit 0 specifies which y-coordinate to use
	// to reconstruct the full uncompressed pubkey.
	cstPayToPubKeyUncomp5 = 5

	//payto call contract, contains create & template
	cstPayToContract = 6

	//vote contract
	cstVoteContract = 7

	// numSpecialScripts is the number of special scripts recognized by the
	// domain-specific script compression algorithm.
	numSpecialScripts = 10
)

// isPubKeyHash returns whether or not the passed public key script is a
// standard pay-to-pubkey-hash script along with the pubkey hash it is paying to
// if it is.
func isPubKeyHash(script []byte) (bool, []byte) {
	if len(script) == 26 && script[0] == txscript.OP_DUP &&
		script[1] == txscript.OP_HASH160 &&
		script[2] == txscript.OP_DATA_21 &&
		script[3] == common.PubKeyHashAddrID &&
		script[24] == txscript.OP_IFLAG_EQUALVERIFY &&
		script[25] == txscript.OP_CHECKSIG {
		return true, script[4:24]
	}

	return false, nil
}

// isScriptHash returns whether or not the passed public key script is a
// standard pay-to-script-hash script along with the script hash it is paying to
// if it is.
func isScriptHash(script []byte) (bool, []byte) {
	if len(script) == 24 && script[0] == txscript.OP_HASH160 &&
		script[1] == txscript.OP_DATA_21 &&
		script[2] == common.ScriptHashAddrID &&
		script[23] == txscript.OP_IFLAG_EQUAL {
		return true, script[3:23]
	}

	return false, nil
}

// return whether or not  the passed public key is a standard contract script.
func isContract(script []byte) (bool, []byte) {
	if len(script) == 23 &&
		script[0] == txscript.OP_CALL &&
		script[1] == txscript.OP_DATA_21 &&
		script[2] == common.ContractHashAddrID {
		return true, script[3:23]
	}

	return false, nil
}

// return whether or not  the passed public key is a standard vote script.
func isVote(script []byte) (bool, []byte) {
	if len(script) == 23 &&
		script[0] == txscript.OP_VOTE &&
		script[1] == txscript.OP_DATA_21 &&
		script[2] == common.ContractHashAddrID {
		return true, script[3:23]
	}

	return false, nil
}

// isPubKey returns whether or not the passed public key script is a standard
// pay-to-pubkey script that pays to a valid compressed or uncompressed public
// key along with the serialized pubkey it is paying to if it is.
//
// NOTE: This function ensures the public key is actually valid since the
// compression algorithm requires valid pubkeys.  It does not support hybrid
// pubkeys.  This means that even if the script has the correct form for a
// pay-to-pubkey script, this function will only return true when it is paying
// to a valid compressed or uncompressed pubkey.
func isPubKey(script []byte) (bool, []byte) {
	// Pay-to-compressed-pubkey script.
	if len(script) == 35 && script[0] == txscript.OP_DATA_33 &&
		script[34] == txscript.OP_CHECKSIG && (script[1] == 0x02 ||
		script[1] == 0x03) {

		// Ensure the public key is valid.
		serializedPubKey := script[1:34]
		_, err := crypto.ParsePubKey(serializedPubKey, crypto.S256())
		if err == nil {
			return true, serializedPubKey
		}
	}

	// Pay-to-uncompressed-pubkey script.
	if len(script) == 67 && script[0] == txscript.OP_DATA_65 &&
		script[66] == txscript.OP_CHECKSIG && script[1] == 0x04 {

		// Ensure the public key is valid.
		serializedPubKey := script[1:66]
		_, err := crypto.ParsePubKey(serializedPubKey, crypto.S256())
		if err == nil {
			return true, serializedPubKey
		}
	}

	return false, nil
}

// compressedScriptSize returns the number of bytes the passed script would take
// when encoded with the domain specific compression algorithm described above.
func compressedScriptSize(pkScript []byte) int {
	// Pay-to-pubkey-hash script.
	if valid, _ := isPubKeyHash(pkScript); valid {
		return 21
	}

	// Pay-to-script-hash script.
	if valid, _ := isScriptHash(pkScript); valid {
		return 21
	}

	if valid, _ := isContract(pkScript); valid {
		return 21
	}

	// Pay-to-pubkey (compressed or uncompressed) script.
	if valid, _ := isPubKey(pkScript); valid {
		return 33
	}

	// Pay-to-pubkey (compressed or uncompressed) script.
	if valid, _ := isVote(pkScript); valid {
		return 21
	}

	// When none of the above special cases apply, encode the script as is
	// preceded by the sum of its size and the number of special cases
	// encoded as a variable length quantity.
	return serializeSizeVLQ(uint64(len(pkScript)+numSpecialScripts)) +
		len(pkScript)
}

// decodeCompressedScriptSize treats the passed serialized bytes as a compressed
// script, possibly followed by other data, and returns the number of bytes it
// occupies taking into account the special encoding of the script size by the
// domain specific compression algorithm described above.
func decodeCompressedScriptSize(serialized []byte) int {
	scriptSize, bytesRead := deserializeVLQ(serialized)
	if bytesRead == 0 {
		return 0
	}

	switch scriptSize {
	case cstPayToPubKeyHash:
		return 21

	case cstPayToScriptHash:
		return 21

	case cstPayToContract, cstVoteContract:
		return 21

	case cstPayToPubKeyComp2, cstPayToPubKeyComp3, cstPayToPubKeyUncomp4,
		cstPayToPubKeyUncomp5:
		return 33
	}

	scriptSize -= numSpecialScripts
	scriptSize += uint64(bytesRead)
	return int(scriptSize)
}

func decodeCompressedAssetSize(serialized []byte) int {
	assetSize, bytesRead := deserializeVLQ(serialized)
	if bytesRead == 0 {
		return 0
	}
	assetSize += uint64(bytesRead)
	return int(assetSize)
}

func putCompressedAsset(target []byte, asset *protos.Asset) int {
	encodedSize := uint64(common.AssetLength)
	vlqSizeLen := putVLQ(target, encodedSize)
	copy(target[vlqSizeLen:], asset.Bytes())
	return vlqSizeLen + common.AssetLength
}

// putCompressedScript compresses the passed script according to the domain
// specific compression algorithm described above directly into the passed
// target byte slice.  The target byte slice must be at least large enough to
// handle the number of bytes returned by the compressedScriptSize function or
// it will panic.
func putCompressedScript(target, pkScript []byte) int {
	// Pay-to-pubkey-hash script.
	if valid, hash := isPubKeyHash(pkScript); valid {
		target[0] = cstPayToPubKeyHash
		copy(target[1:21], hash)
		return 21
	}

	// Pay-to-script-hash script.
	if valid, hash := isScriptHash(pkScript); valid {
		target[0] = cstPayToScriptHash
		copy(target[1:21], hash)
		return 21
	}

	if valid, hash := isContract(pkScript); valid {
		target[0] = cstPayToContract
		copy(target[1:21], hash)
		return 21
	}

	if valid, hash := isVote(pkScript); valid {
		target[0] = cstVoteContract
		copy(target[1:21], hash)
		return 21
	}

	// Pay-to-pubkey (compressed or uncompressed) script.
	if valid, serializedPubKey := isPubKey(pkScript); valid {
		pubKeyFormat := serializedPubKey[0]
		switch pubKeyFormat {
		case 0x02, 0x03:
			target[0] = pubKeyFormat
			copy(target[1:33], serializedPubKey[1:33])
			return 33
		case 0x04:
			// Encode the oddness of the serialized pubkey into the
			// compressed script type.
			target[0] = pubKeyFormat | (serializedPubKey[64] & 0x01)
			copy(target[1:33], serializedPubKey[1:33])
			return 33
		}
	}

	// When none of the above special cases apply, encode the unmodified
	// script preceded by the sum of its size and the number of special
	// cases encoded as a variable length quantity.
	encodedSize := uint64(len(pkScript) + numSpecialScripts)
	vlqSizeLen := putVLQ(target, encodedSize)
	copy(target[vlqSizeLen:], pkScript)
	return vlqSizeLen + len(pkScript)
}

// decompressScript returns the original script obtained by decompressing the
// passed compressed script according to the domain specific compression
// algorithm described above.
//
// NOTE: The script parameter must already have been proven to be long enough
// to contain the number of bytes returned by decodeCompressedScriptSize or it
// will panic.  This is acceptable since it is only an internal function.
func decompressScript(compressedPkScript []byte) []byte {
	// In practice this function will not be called with a zero-length or
	// nil script since the nil script encoding includes the length, however
	// the code below assumes the length exists, so just return nil now if
	// the function ever ends up being called with a nil script in the
	// future.
	if len(compressedPkScript) == 0 {
		return nil
	}

	// Decode the script size and examine it for the special cases.
	encodedScriptSize, bytesRead := deserializeVLQ(compressedPkScript)
	switch encodedScriptSize {
	// Pay-to-pubkey-hash script.  The resulting script is:
	// <OP_DUP><OP_HASH160><20 byte hash><OP_EQUALVERIFY><OP_CHECKSIG>
	case cstPayToPubKeyHash:
		pkScript := make([]byte, 26)
		pkScript[0] = txscript.OP_DUP
		pkScript[1] = txscript.OP_HASH160
		pkScript[2] = txscript.OP_DATA_21
		pkScript[3] = common.PubKeyHashAddrID
		copy(pkScript[4:], compressedPkScript[bytesRead:bytesRead+20])
		pkScript[24] = txscript.OP_IFLAG_EQUALVERIFY
		pkScript[25] = txscript.OP_CHECKSIG
		return pkScript

	// Pay-to-script-hash script.  The resulting script is:
	// <OP_HASH160><20 byte script hash><OP_EQUAL>
	case cstPayToScriptHash:
		pkScript := make([]byte, 24)
		pkScript[0] = txscript.OP_HASH160
		pkScript[1] = txscript.OP_DATA_21
		pkScript[2] = common.ScriptHashAddrID
		copy(pkScript[3:], compressedPkScript[bytesRead:bytesRead+20])
		pkScript[23] = txscript.OP_IFLAG_EQUAL
		return pkScript

	// Pay-to-compressed-pubkey script.  The resulting script is:
	// <OP_DATA_33><33 byte compressed pubkey><OP_CHECKSIG>
	case cstPayToPubKeyComp2, cstPayToPubKeyComp3:
		pkScript := make([]byte, 35)
		pkScript[0] = txscript.OP_DATA_33
		pkScript[1] = byte(encodedScriptSize)
		copy(pkScript[2:], compressedPkScript[bytesRead:bytesRead+32])
		pkScript[34] = txscript.OP_CHECKSIG
		return pkScript

	// Pay-to-uncompressed-pubkey script.  The resulting script is:
	// <OP_DATA_65><65 byte uncompressed pubkey><OP_CHECKSIG>
	case cstPayToPubKeyUncomp4, cstPayToPubKeyUncomp5:
		// Change the leading byte to the appropriate compressed pubkey
		// identifier (0x02 or 0x03) so it can be decoded as a
		// compressed pubkey.  This really should never fail since the
		// encoding ensures it is valid before compressing to this type.
		compressedKey := make([]byte, 33)
		compressedKey[0] = byte(encodedScriptSize - 2)
		copy(compressedKey[1:], compressedPkScript[1:])
		key, err := crypto.ParsePubKey(compressedKey, crypto.S256())
		if err != nil {
			return nil
		}

		pkScript := make([]byte, 67)
		pkScript[0] = txscript.OP_DATA_65
		copy(pkScript[1:], key.SerializeUncompressed())
		pkScript[66] = txscript.OP_CHECKSIG
		return pkScript

	case cstPayToContract:
		pkScript := make([]byte, 23)
		pkScript[0] = txscript.OP_CALL
		pkScript[1] = txscript.OP_DATA_21
		pkScript[2] = common.ContractHashAddrID
		copy(pkScript[3:], compressedPkScript[bytesRead:bytesRead+20])
		return pkScript

	case cstVoteContract:
		pkScript := make([]byte, 23)
		pkScript[0] = txscript.OP_VOTE
		pkScript[1] = txscript.OP_DATA_21
		pkScript[2] = common.ContractHashAddrID
		copy(pkScript[3:], compressedPkScript[bytesRead:bytesRead+20])
		return pkScript
	}

	// When none of the special cases apply, the script was encoded using
	// the general format, so reduce the script size by the number of
	// special cases and return the unmodified script.
	scriptSize := int(encodedScriptSize - numSpecialScripts)
	pkScript := make([]byte, scriptSize)
	copy(pkScript, compressedPkScript[bytesRead:bytesRead+scriptSize])
	return pkScript
}

func decompressAsset(compressedAsset []byte) *protos.Asset {
	if len(compressedAsset) == 0 {
		return nil
	}

	// Decode the asset size and examine it for the special cases.
	encodedAssetSize, bytesRead := deserializeVLQ(compressedAsset)
	if encodedAssetSize == common.AssetLength {
		assetSize := int(encodedAssetSize)
		return protos.AssetFromBytes(compressedAsset[bytesRead : bytesRead+assetSize])
	}
	return nil
}

// -----------------------------------------------------------------------------
// Compressed transaction outputs consist of an amount and a public key script
// both compressed using the domain specific compression algorithms previously
// described.
//
// The serialized format is:
//
//   <compressed amount><compressed script>
//
//   Field                 Type     Size
//     compressed amount   VLQ      variable
//     compressed script   []byte   variable
// -----------------------------------------------------------------------------

// compressedTxOutSize returns the number of bytes the passed transaction output
// fields would take when encoded with the format described above.
func compressedTxOutSize(amount uint64, pkScript []byte, asset *protos.Asset) int {
	return common.AmountSize +
		compressedScriptSize(pkScript) + serializeSizeVLQ(uint64(common.AssetLength)) +
		common.AssetLength
}

// putCompressedTxOut compresses the passed amount and script according to their
// domain specific compression algorithms and encodes them directly into the
// passed target byte slice with the format described above.  The target byte
// slice must be at least large enough to handle the number of bytes returned by
// the compressedTxOutSize function or it will panic.
func putCompressedTxOut(target []byte, amount uint64, pkScript []byte, asset *protos.Asset) int {
	binary.LittleEndian.PutUint64(target, amount)
	offset := common.AmountSize
	offset += putCompressedScript(target[offset:], pkScript)
	offset += putCompressedAsset(target[offset:], asset)
	return offset
}

// decodeCompressedTxOut decodes the passed compressed txout, possibly followed
// by other data, into its uncompressed amount and script and returns them along
// with the number of bytes they occupied prior to decompression.
func decodeCompressedTxOut(serialized []byte) (uint64, []byte, *protos.Asset, int, error) {
	bytesRead := common.AmountSize
	if len(serialized) < bytesRead {
		return 0, nil, nil, 0, common.DeserializeError("unexpected end of " +
			"data after compressed amount")
	}
	amount := binary.LittleEndian.Uint64(serialized)

	// Decode the compressed script size and ensure there are enough bytes
	// left in the slice for it.
	scriptSize := decodeCompressedScriptSize(serialized[bytesRead:])
	if len(serialized[bytesRead:]) < scriptSize {
		return 0, nil, nil, bytesRead, common.DeserializeError("unexpected end of " +
			"data after script size")
	}

	// Decode the compressed asset size and ensure there are enough bytes
	// left in the slice for it.
	assetSize := decodeCompressedAssetSize(serialized[bytesRead+scriptSize:])
	if len(serialized[bytesRead+scriptSize:]) < assetSize {
		return 0, nil, nil, bytesRead, common.DeserializeError("unexpected end of " +
			"data after asset size")
	}

	// Decompress and return the amount and script and asset
	script := decompressScript(serialized[bytesRead : bytesRead+scriptSize])
	asset := decompressAsset(serialized[bytesRead+scriptSize:])
	return amount, script, asset, bytesRead + scriptSize + assetSize, nil
}
