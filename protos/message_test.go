// Copyright (c) 2018-2020 The asimov developers
// Copyright (c) 2013-2017 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package protos

import (
	"bytes"
	"encoding/binary"
	"github.com/AsimovNetwork/asimov/common"
	"io"
	"net"
	"reflect"
	"testing"
	"time"
)

// makeHeader is a convenience function to make a message header in the form of
// a byte slice.  It is used to force errors when reading messages.
func makeHeader(btcnet common.AsimovNet, command string,
	payloadLen uint32, checksum uint32) []byte {

	// The length of a bitcoin message header is 24 bytes.
	// 4 byte magic number of the bitcoin network + 12 byte command + 4 byte
	// payload length + 4 byte checksum.
	buf := make([]byte, 20)
	//binary.LittleEndian.PutUint32(buf, uint32(btcnet))
	copy(buf[0:], []byte(command))
	binary.LittleEndian.PutUint32(buf[12:], payloadLen)
	binary.LittleEndian.PutUint32(buf[16:], checksum)
	return buf
}

// TestMessage tests the Read/WriteMessage and Read/WriteMessageN API.
func TestMessage(t *testing.T) {
	pver := common.ProtocolVersion

	// Create the various types of messages to test.

	// MsgVersion.
	addrYou := &net.TCPAddr{IP: net.ParseIP("192.168.0.1"), Port: 8333}
	you := NewNetAddress(addrYou, common.SFNodeNetwork)
	you.Timestamp = time.Time{}.Local() // Version message has zero value timestamp.
	addrMe := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8333}
	me := NewNetAddress(addrMe, common.SFNodeNetwork)
	me.Timestamp = time.Time{}.Local() // Version message has zero value timestamp.
	msgVersion := NewMsgVersion(me, you, 123123, 0, common.MainNet)

	baseBlockHdr := &BlockHeader{
		Version:       1,
		PrevBlock:     mainNetGenesisHash,
		MerkleRoot:    mainNetGenesisMerkleRoot,
		Timestamp:     0x495fab29, // 2009-01-03 12:15:05 -0600 CST
		Height:        1,
		SigData:       [65]byte{0x63, 0xe3, 0x83, 0x03, 0xb3, 0xf3, 0xb3, 0x73, 0x83},
	}

	msgVerack := NewMsgVerAck()
	msgGetAddr := NewMsgGetAddr()
	msgAddr := NewMsgAddr()
	//msgGetBlocks := NewMsgGetBlocks(&common.Hash{})
	msgBlock := &blockOne
	msgInv := NewMsgInv()
	msgGetData := NewMsgGetData()
	msgNotFound := NewMsgNotFound()

	msgTx := NewMsgTx(1)
	msgPing := NewMsgPing(123123)
	msgPong := NewMsgPong(123123)
	msgGetHeaders := NewMsgGetHeaders()
	msgHeaders := NewMsgHeaders()
	//msgAlert := NewMsgAlert([]byte("payload"), []byte("signature"))
	msgMemPool := NewMsgMemPool()
	msgFilterAdd := NewMsgFilterAdd([]byte{0x01})
	msgFilterClear := NewMsgFilterClear()
	msgFilterLoad := NewMsgFilterLoad([]byte{0x01}, 10, 0, BloomUpdateNone)
	//bh := NewBlockHeader(1, &common.Hash{}, &common.Hash{}, 0)
	bh := baseBlockHdr
	msgMerkleBlock := NewMsgMerkleBlock(bh)
	msgReject := NewMsgReject("block", RejectDuplicate, "duplicate block")
	msgGetCFilters := NewMsgGetCFilters(GCSFilterRegular, 0, &common.Hash{})
	msgGetCFHeaders := NewMsgGetCFHeaders(GCSFilterRegular, 0, &common.Hash{})
	msgGetCFCheckpt := NewMsgGetCFCheckpt(GCSFilterRegular, &common.Hash{})
	msgCFilter := NewMsgCFilter(GCSFilterRegular, &common.Hash{},
		[]byte("payload"))
	msgCFHeaders := NewMsgCFHeaders()
	msgCFCheckpt := NewMsgCFCheckpt(GCSFilterRegular, &common.Hash{}, 0)

	tests := []struct {
		in     Message          // Value to encode
		out    Message          // Expected decoded value
		pver   uint32           // Protocol version for protos encoding
		btcnet common.AsimovNet // Network to use for protos encoding
		bytes  int              // Expected num bytes read/written
	}{
		{msgVersion, msgVersion, pver, common.MainNet, 153},
		{msgVerack, msgVerack, pver, common.MainNet, 20},
		{msgGetAddr, msgGetAddr, pver, common.MainNet, 20},
		{msgAddr, msgAddr, pver, common.MainNet, 21},
		//{msgGetBlocks, msgGetBlocks, pver, types.MainNet, 57},
		{msgBlock, msgBlock, pver, common.MainNet, 716},
		{msgInv, msgInv, pver, common.MainNet, 21},
		{msgGetData, msgGetData, pver, common.MainNet, 21},
		{msgNotFound, msgNotFound, pver, common.MainNet, 21},
		{msgTx, msgTx, pver, common.MainNet, 34},
		{msgPing, msgPing, pver, common.MainNet, 28},
		{msgPong, msgPong, pver, common.MainNet, 28},
		{msgGetHeaders, msgGetHeaders, pver, common.MainNet, 57},
		{msgHeaders, msgHeaders, pver, common.MainNet, 21},
		//{msgAlert, msgAlert, pver, MainNet, 42},
		{msgMemPool, msgMemPool, pver, common.MainNet, 20},
		{msgFilterAdd, msgFilterAdd, pver, common.MainNet, 22},
		{msgFilterClear, msgFilterClear, pver, common.MainNet, 20},
		{msgFilterLoad, msgFilterLoad, pver, common.MainNet, 31},
		{msgMerkleBlock, msgMerkleBlock, pver, common.MainNet, 280},
		{msgReject, msgReject, pver, common.MainNet, 75},
		{msgGetCFilters, msgGetCFilters, pver, common.MainNet, 57},
		{msgGetCFHeaders, msgGetCFHeaders, pver, common.MainNet, 57},
		{msgGetCFCheckpt, msgGetCFCheckpt, pver, common.MainNet, 53},
		{msgCFilter, msgCFilter, pver, common.MainNet, 61},
		{msgCFHeaders, msgCFHeaders, pver, common.MainNet, 86},
		{msgCFCheckpt, msgCFCheckpt, pver, common.MainNet, 54},
	}

	t.Logf("Running %d tests", len(tests))
	for i, test := range tests {
		// Encode to protos format.
		var buf bytes.Buffer
		nw, err := WriteMessageN(&buf, test.in, test.pver)
		if err != nil {
			t.Errorf("WriteMessage #%d error %v", i, err)
			continue
		}

		// Ensure the number of bytes written match the expected value.
		if nw != test.bytes {
			t.Errorf("WriteMessage #%d unexpected num bytes "+
				"written - got %d, want %d", i, nw, test.bytes)
		}

		// Decode from protos format.
		rbuf := bytes.NewReader(buf.Bytes())
		nr, msg, _, err := ReadMessageN(rbuf, test.pver)
		if err != nil {
			t.Errorf("ReadMessage #%d error %v, msg %v", i, err,
				msg)
			continue
		}
		if !reflect.DeepEqual(msg, test.out) {
			t.Errorf("ReadMessage #%d\n got: %v want: %v", i,
				msg, test.out)
			continue
		}

		// Ensure the number of bytes read match the expected value.
		if nr != test.bytes {
			t.Errorf("ReadMessage #%d unexpected num bytes read - "+
				"got %d, want %d", i, nr, test.bytes)
		}
	}

	// Do the same thing for Read/WriteMessage, but ignore the bytes since
	// they don't return them.
	t.Logf("Running %d tests", len(tests))
	for i, test := range tests {
		// Encode to protos format.
		var buf bytes.Buffer
		err := WriteMessage(&buf, test.in, test.pver)
		if err != nil {
			t.Errorf("WriteMessage #%d error %v", i, err)
			continue
		}

		// Decode from protos format.
		rbuf := bytes.NewReader(buf.Bytes())
		msg, _, err := ReadMessage(rbuf, test.pver)
		if err != nil {
			t.Errorf("ReadMessage #%d error %v, msg %v", i, err,
				msg)
			continue
		}
		if !reflect.DeepEqual(msg, test.out) {
			t.Errorf("ReadMessage #%d\n got: %v want: %v", i,
				msg, test.out)
			continue
		}
	}
}

// TestReadMessageWireErrors performs negative tests against protos decoding into
// concrete messages to confirm error paths work correctly.
func TestReadMessageWireErrors(t *testing.T) {
	pver := common.ProtocolVersion
	btcnet := common.MainNet

	// Ensure message errors are as expected with no function specified.
	wantErr := "something bad happened"
	testErr := MessageError{Description: wantErr}
	if testErr.Error() != wantErr {
		t.Errorf("MessageError: wrong error - got %v, want %v",
			testErr.Error(), wantErr)
	}

	// Ensure message errors are as expected with a function specified.
	wantFunc := "foo"
	testErr = MessageError{Func: wantFunc, Description: wantErr}
	if testErr.Error() != wantFunc+": "+wantErr {
		t.Errorf("MessageError: wrong error - got %v, want %v",
			testErr.Error(), wantErr)
	}

	// Wire encoded bytes for main and testnet3 networks magic identifiers.
	testNet3Bytes := makeHeader(common.TestNet, "", 0, 0)

	// Wire encoded bytes for a message that exceeds max overall message
	// length.
	mpl := uint32(MaxMessagePayload)
	exceedMaxPayloadBytes := makeHeader(btcnet, "getaddr", mpl+1, 0)

	// Wire encoded bytes for a command which is invalid utf-8.
	badCommandBytes := makeHeader(btcnet, "bogus", 0, 0)
	badCommandBytes[4] = 0x81

	// Wire encoded bytes for a command which is valid, but not supported.
	unsupportedCommandBytes := makeHeader(btcnet, "bogus", 0, 0)

	// Wire encoded bytes for a message which exceeds the max payload for
	// a specific message type.
	exceedTypePayloadBytes := makeHeader(btcnet, "getaddr", 1, 0)

	// Wire encoded bytes for a message which does not deliver the full
	// payload according to the header length.
	shortPayloadBytes := makeHeader(btcnet, "version", 115, 0)

	// Wire encoded bytes for a message with a bad checksum.
	badChecksumBytes := makeHeader(btcnet, "version", 2, 0xbeef)
	badChecksumBytes = append(badChecksumBytes, []byte{0x0, 0x0}...)

	// Wire encoded bytes for a message which has a valid header, but is
	// the wrong format.  An addr starts with a varint of the number of
	// contained in the message.  Claim there is two, but don't provide
	// them.  At the same time, forge the header fields so the message is
	// otherwise accurate.
	badMessageBytes := makeHeader(btcnet, "addr", 1, 0xeaadc31c)
	badMessageBytes = append(badMessageBytes, 0x2)

	// Wire encoded bytes for a message which the header claims has 15k
	// bytes of data to discard.
	discardBytes := makeHeader(btcnet, "bogus", 15*1024, 0)

	tests := []struct {
		buf     []byte           // Wire encoding
		pver    uint32           // Protocol version for protos encoding
		btcnet  common.AsimovNet // Bitcoin network for protos encoding
		max     int              // Max size of fixed buffer to induce errors
		readErr error            // Expected read error
		bytes   int              // Expected num bytes read
	}{
		// Latest protocol version with intentional read errors.

		// Short header.
		{
			[]byte{},
			pver,
			btcnet,
			0,
			io.EOF,
			0,
		},

		// Wrong network.  Want MainNet, but giving TestNet.
		{
			testNet3Bytes,
			pver,
			btcnet,
			len(testNet3Bytes),
			&MessageError{},
			20,
		},

		// Exceed max overall message payload length.
		{
			exceedMaxPayloadBytes,
			pver,
			btcnet,
			len(exceedMaxPayloadBytes),
			&MessageError{},
			20,
		},

		// Invalid UTF-8 command.
		{
			badCommandBytes,
			pver,
			btcnet,
			len(badCommandBytes),
			&MessageError{},
			20,
		},

		// Valid, but unsupported command.
		{
			unsupportedCommandBytes,
			pver,
			btcnet,
			len(unsupportedCommandBytes),
			&MessageError{},
			20,
		},

		// Exceed max allowed payload for a message of a specific type.
		{
			exceedTypePayloadBytes,
			pver,
			btcnet,
			len(exceedTypePayloadBytes),
			&MessageError{},
			20,
		},

		// Message with a payload shorter than the header indicates.
		{
			shortPayloadBytes,
			pver,
			btcnet,
			len(shortPayloadBytes),
			io.EOF,
			20,
		},

		// Message with a bad checksum.
		{
			badChecksumBytes,
			pver,
			btcnet,
			len(badChecksumBytes),
			&MessageError{},
			22,
		},

		// Message with a valid header, but wrong format.
		{
			badMessageBytes,
			pver,
			btcnet,
			len(badMessageBytes),
			io.EOF,
			21,
		},

		// 15k bytes of data to discard.
		{
			discardBytes,
			pver,
			btcnet,
			len(discardBytes),
			&MessageError{},
			20,
		},
	}

	t.Logf("Running %d tests", len(tests))
	for i, test := range tests {
		// Decode from protos format.
		r := newFixedReader(test.max, test.buf)
		nr, _, _, err := ReadMessageN(r, test.pver)
		if reflect.TypeOf(err) != reflect.TypeOf(test.readErr) {
			t.Errorf("ReadMessage #%d wrong error got: %v <%T>, "+
				"want: %T", i, err, err, test.readErr)
			continue
		}

		// Ensure the number of bytes written match the expected value.
		if nr != test.bytes {
			t.Errorf("ReadMessage #%d unexpected num bytes read - "+
				"got %d, want %d", i, nr, test.bytes)
		}

		// For errors which are not of type MessageError, check them for
		// equality.
		if _, ok := err.(*MessageError); !ok {
			if err != test.readErr {
				t.Errorf("ReadMessage #%d wrong error got: %v <%T>, "+
					"want: %v <%T>", i, err, err,
					test.readErr, test.readErr)
				continue
			}
		}
	}
}

// TestWriteMessageWireErrors performs negative tests against protos encoding from
// concrete messages to confirm error paths work correctly.
func TestWriteMessageWireErrors(t *testing.T) {
	pver := common.ProtocolVersion
	btcnet := common.MainNet
	wireErr := &MessageError{}

	// Fake message with a command that is too long.
	badCommandMsg := &fakeMessage{command: "somethingtoolong"}

	// Fake message with a problem during encoding
	encodeErrMsg := &fakeMessage{forceEncodeErr: true}

	// Fake message that has payload which exceeds max overall message size.
	exceedOverallPayload := make([]byte, MaxMessagePayload+1)
	exceedOverallPayloadErrMsg := &fakeMessage{payload: exceedOverallPayload}

	// Fake message that has payload which exceeds max allowed per message.
	exceedPayload := make([]byte, 1)
	exceedPayloadErrMsg := &fakeMessage{payload: exceedPayload, forceLenErr: true}

	// Fake message that is used to force errors in the header and payload
	// writes.
	bogusPayload := []byte{0x01, 0x02, 0x03, 0x04}
	bogusMsg := &fakeMessage{command: "bogus", payload: bogusPayload}

	tests := []struct {
		msg    Message          // Message to encode
		pver   uint32           // Protocol version for protos encoding
		btcnet common.AsimovNet // Bitcoin network for protos encoding
		max    int              // Max size of fixed buffer to induce errors
		err    error            // Expected error
		bytes  int              // Expected num bytes written
	}{
		// Command too long.
		{badCommandMsg, pver, btcnet, 0, wireErr, 0},
		// Force error in payload encode.
		{encodeErrMsg, pver, btcnet, 0, wireErr, 0},
		// Force error due to exceeding max overall message payload size.
		{exceedOverallPayloadErrMsg, pver, btcnet, 0, wireErr, 0},
		// Force error due to exceeding max payload for message type.
		{exceedPayloadErrMsg, pver, btcnet, 0, wireErr, 0},
		// Force error in header write.
		{bogusMsg, pver, btcnet, 0, io.ErrShortWrite, 0},
		// Force error in payload write.
		{bogusMsg, pver, btcnet, 24, nil, 24},
	}

	t.Logf("Running %d tests", len(tests))
	for i, test := range tests {
		// Encode protos format.
		w := newFixedWriter(test.max)
		nw, err := WriteMessageN(w, test.msg, test.pver)
		if reflect.TypeOf(err) != reflect.TypeOf(test.err) {
			t.Errorf("WriteMessage #%d wrong error got: %v <%T>, "+
				"want: %T", i, err, err, test.err)
			continue
		}

		// Ensure the number of bytes written match the expected value.
		if nw != test.bytes {
			t.Errorf("WriteMessage #%d unexpected num bytes "+
				"written - got %d, want %d", i, nw, test.bytes)
		}

		// For errors which are not of type MessageError, check them for
		// equality.
		if _, ok := err.(*MessageError); !ok {
			if err != test.err {
				t.Errorf("ReadMessage #%d wrong error got: %v <%T>, "+
					"want: %v <%T>", i, err, err,
					test.err, test.err)
				continue
			}
		}
	}
}
