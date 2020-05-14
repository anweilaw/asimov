// Copyright (c) 2018-2020. The asimov developers
// Copyright (c) 2013-2017 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.
package blockchain

import (
	"bytes"
	"encoding/hex"
	"github.com/AsimovNetwork/asimov/protos"
	"testing"
	"github.com/AsimovNetwork/asimov/common"
)

// hexToBytes converts the passed hex string into bytes and will panic if there
// is an error.  This is only provided for the hard-coded constants so errors in
// the source code can be detected. It will only (and must only) be called with
// hard-coded values.
func hexToBytes(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic("invalid hex in source file: " + s)
	}
	return b
}

// TestVLQ ensures the variable length quantity serialization, deserialization,
// and size calculation works as expected.
func TestVLQ(t *testing.T) {
	t.Parallel()

	tests := []struct {
		val        uint64
		serialized []byte
	}{
		{0, hexToBytes("00")},
		{1, hexToBytes("01")},
		{127, hexToBytes("7f")},
		{128, hexToBytes("8000")},
		{129, hexToBytes("8001")},
		{255, hexToBytes("807f")},
		{256, hexToBytes("8100")},
		{16383, hexToBytes("fe7f")},
		{16384, hexToBytes("ff00")},
		{16511, hexToBytes("ff7f")}, // Max 2-byte value
		{16512, hexToBytes("808000")},
		{16513, hexToBytes("808001")},
		{16639, hexToBytes("80807f")},
		{32895, hexToBytes("80ff7f")},
		{2113663, hexToBytes("ffff7f")}, // Max 3-byte value
		{2113664, hexToBytes("80808000")},
		{270549119, hexToBytes("ffffff7f")}, // Max 4-byte value
		{270549120, hexToBytes("8080808000")},
		{2147483647, hexToBytes("86fefefe7f")},
		{2147483648, hexToBytes("86fefeff00")},
		{4294967295, hexToBytes("8efefefe7f")}, // Max uint32, 5 bytes
		// Max uint64, 10 bytes
		{18446744073709551615, hexToBytes("80fefefefefefefefe7f")},
	}

	for _, test := range tests {
		// Ensure the function to calculate the serialized size without
		// actually serializing the value is calculated properly.
		gotSize := serializeSizeVLQ(test.val)
		if gotSize != len(test.serialized) {
			t.Errorf("serializeSizeVLQ: did not get expected size for %d - got %d, want %d",
				test.val, gotSize, len(test.serialized))
			continue
		}

		// Ensure the value serializes to the expected bytes.
		gotBytes := make([]byte, gotSize)
		gotBytesWritten := putVLQ(gotBytes, test.val)
		if !bytes.Equal(gotBytes, test.serialized) {
			t.Errorf("putVLQUnchecked: did not get expected bytes for %d - got %x, want %x",
				test.val, gotBytes, test.serialized)
			continue
		}
		if gotBytesWritten != len(test.serialized) {
			t.Errorf("putVLQUnchecked: did not get expected number of bytes written for %d - got %d, want %d",
				test.val, gotBytesWritten, len(test.serialized))
			continue
		}

		// Ensure the serialized bytes deserialize to the expected
		// value.
		gotVal, gotBytesRead := deserializeVLQ(test.serialized)
		if gotVal != test.val {
			t.Errorf("deserializeVLQ: did not get expected value for %x - got %d, want %d",
				test.serialized, gotVal, test.val)
			continue
		}
		if gotBytesRead != len(test.serialized) {
			t.Errorf("deserializeVLQ: did not get expected number of bytes read for %d - got %d, want %d",
				test.serialized, gotBytesRead, len(test.serialized))
			continue
		}
	}
}


// TestScriptCompressionErrors ensures calling various functions related to
// script compression with incorrect data returns the expected results.
func TestScriptCompressionErrors(t *testing.T) {
	t.Parallel()

	// A nil script must result in a decoded size of 0.
	if gotSize := decodeCompressedScriptSize(nil); gotSize != 0 {
		t.Fatalf("decodeCompressedScriptSize with nil script did not return 0 - got %d", gotSize)
	}

	// A nil script must result in a nil decompressed script.
	if gotScript := decompressScript(nil); gotScript != nil {
		t.Fatalf("decompressScript with nil script did not return nil decompressed script - got %x", gotScript)
	}

	// A compressed script for a pay-to-pubkey (uncompressed) that results
	// in an invalid pubkey must result in a nil decompressed script.
	compressedScript := hexToBytes("04012d74d0cb94344c9569c2e77901573d8d" +
		"7903c3ebec3a957724895dca52c6b4")
	if gotScript := decompressScript(compressedScript); gotScript != nil {
		t.Fatalf("decompressScript with compressed pay-to-uncompressed-pubkey that is invalid did not return "+
			"nil decompressed script - got %x", gotScript)
	}
}

// TestCompressedTxOut ensures the transaction output serialization and
// deserialization works as expected.
func TestCompressedTxOut(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		amount     uint64
		pkScript   []byte
		compressed []byte
		asset      protos.Asset
	}{
		{
			name:       "nulldata with 0 BTC",
			amount:     0,
			pkScript:   hexToBytes("6a200102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20"),
			compressed: hexToBytes("00000000000000002c6a200102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f200c000000000000000000000000"),
			asset:      protos.Asset{0,0},
		},
		{
			name:       "pay-to-pubkey-hash with 546 xing",
			amount:     546,
			pkScript:   hexToBytes("76a915661018853670f9f3b0582c5b9ee8ce93764ac32b93c5ac"),
			compressed: hexToBytes("2202000000000000001018853670f9f3b0582c5b9ee8ce93764ac32b930c000000010000000000000002"),
			asset:      protos.Asset{1,2},
		},
		{
			name:       "pay-to-pubkey uncompressed 1 BTC",
			amount:     100000000,
			pkScript:   hexToBytes("4104192d74d0cb94344c9569c2e77901573d8d7903c3ebec3a957724895dca52c6b40d45264838c0bd96852662ce6a847b197376830160c6d2eb5e6a4c44d33f453eac"),
			compressed: hexToBytes("00e1f5050000000004192d74d0cb94344c9569c2e77901573d8d7903c3ebec3a957724895dca52c6b40c000000000000000000000000"),
			asset:      protos.Asset{0,0},
		},
		{
			name:       "pay-to-script-hash with 21000000 BTC",
			amount:     2100000000000000,
			pkScript:   hexToBytes("a915731018853670f9f3b0582c5b9ee8ce93764ac32b93c4"),
			compressed: hexToBytes("0040075af0750700011018853670f9f3b0582c5b9ee8ce93764ac32b930c000000010000000000000003"),
			asset:      protos.Asset{1,3},
		},
		{
			name:       "pay-to-contract with 0 BTC",
			amount:     0,
			pkScript:   hexToBytes("c215631018853670f9f3b0582c5b9ee8ce93764ac32b93"),
			compressed: hexToBytes("0000000000000000061018853670f9f3b0582c5b9ee8ce93764ac32b930c000000000000000000000000"),
			asset:      protos.Asset{0,0},
		},
		{
			name:       "vote-contract with 0 btc",
			amount:     0,
			pkScript:   hexToBytes("c615631018853670f9f3b0582c5b9ee8ce93764ac32b93"),
			compressed: hexToBytes("0000000000000000071018853670f9f3b0582c5b9ee8ce93764ac32b930c000000000000000000000000"),
			asset:      protos.Asset{0,0},
		},
		{
			name:       "0 btc, pkscript is nil",
			amount:     0,
			pkScript:   nil,
			compressed: hexToBytes("00000000000000000a0c000000000000000000000000"),
			asset:      protos.Asset{0,0},
		},
	}

	for i, test := range tests {
		// Ensure the function to calculate the serialized size without
		// actually serializing the txout is calculated properly.
		gotSize := compressedTxOutSize(test.amount, test.pkScript, &test.asset)
		if gotSize != len(test.compressed) {
			t.Errorf("compressedTxOutSize (%d, %s): did not get expected size - got %d, want %d",
				i, test.name, gotSize, len(test.compressed))
			continue
		}

		// Ensure the txout compresses to the expected value.
		gotCompressed := make([]byte, gotSize)
		gotBytesWritten := putCompressedTxOut(gotCompressed,
			test.amount, test.pkScript, &test.asset)
		if !bytes.Equal(gotCompressed, test.compressed) {
			t.Errorf("compressTxOut (%d, %s): did not get expected bytes - got %x, want %x",
				i, test.name, gotCompressed, test.compressed)
			continue
		}
		if gotBytesWritten != len(test.compressed) {
			t.Errorf("compressTxOut (%d, %s): did not get expected number of bytes written - got %d, want %d",
				i, test.name, gotBytesWritten, len(test.compressed))
			continue
		}

		// Ensure the serialized bytes are decoded back to the expected
		// uncompressed values.
		gotAmount, gotScript, assetNo, gotBytesRead, err := decodeCompressedTxOut(
			test.compressed)
		if err != nil {
			t.Errorf("decodeCompressedTxOut (%s): unexpected "+ "error: %v", test.name, err)
			continue
		}
		if gotAmount != test.amount {
			t.Errorf("decodeCompressedTxOut (%s): did not get expected amount - got %d, want %d",
				test.name, gotAmount, test.amount)
			continue
		}
		if !bytes.Equal(gotScript, test.pkScript) {
			t.Errorf("decodeCompressedTxOut (%s): did not get expected script - got %x, want %x",
				test.name, gotScript, test.pkScript)
			continue
		}
		if assetNo == nil || *assetNo != test.asset {
			t.Errorf("decodeCompressedTxOut (%s): did not get expected script - got %x, want %x",
				test.name, assetNo, test.asset)
			continue
		}
		if gotBytesRead != len(test.compressed) {
			t.Errorf("decodeCompressedTxOut (%s): did not get expected number of bytes read - got %d, want %d",
				test.name, gotBytesRead, len(test.compressed))
			continue
		}
	}
}

// TestTxOutCompressionErrors ensures calling various functions related to
// txout compression with incorrect data returns the expected results.
func TestTxOutCompressionErrors(t *testing.T) {
	t.Parallel()

	// A compressed txout with missing compressed script must error.
	compressedTxOut := hexToBytes("00")
	_, _, _, _, err := decodeCompressedTxOut(compressedTxOut)
	if !common.IsDeserializeErr(err) {
		t.Fatalf("decodeCompressedTxOut with missing compressed script "+
			"did not return expected error type - got %T, want DeserializeError", err)
	}

	// A compressed txout with short compressed script must error.
	compressedTxOut = hexToBytes("0010")
	_, _, _, _, err = decodeCompressedTxOut(compressedTxOut)
	if !common.IsDeserializeErr(err) {
		t.Fatalf("decodeCompressedTxOut with short compressed script "+
			"did not return expected error type - got %T, want DeserializeError", err)
	}
}
