// Copyright (c) 2018-2020 The asimov developers
// Copyright (c) 2013-2017 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package protos

import (
	"fmt"
	"io"

	"github.com/AsimovNetwork/asimov/common"
	"github.com/AsimovNetwork/asimov/common/serialization"
)

// RejectCode represents a numeric value by which a remote peer indicates
// why a message was rejected.
type RejectCode uint8

// These constants define the various supported reject codes.
const (
	RejectMalformed       RejectCode = 0x01
	RejectInvalid         RejectCode = 0x10
	RejectObsolete        RejectCode = 0x11
	RejectDuplicate       RejectCode = 0x12
	RejectForbidden       RejectCode = 0x13
	RejectNonstandard     RejectCode = 0x40
	RejectLowGasPrice     RejectCode = 0x41
	RejectCheckpoint      RejectCode = 0x43
	RejectFeeUnsupport    RejectCode = 0x44
)

// Map of reject codes back strings for pretty printing.
var rejectCodeStrings = map[RejectCode]string{
	RejectMalformed:       "REJECT_MALFORMED",
	RejectInvalid:         "REJECT_INVALID",
	RejectObsolete:        "REJECT_OBSOLETE",
	RejectDuplicate:       "REJECT_DUPLICATE",
	RejectForbidden:       "REJECT_FORBIDDEN",
	RejectNonstandard:     "REJECT_NONSTANDARD",
	RejectLowGasPrice:     "REJECT_LOWGASPRICE",
	RejectCheckpoint:      "REJECT_CHECKPOINT",
	RejectFeeUnsupport:    "REJECT_FEEUNSUPPORT",
}

// String returns the RejectCode in human-readable form.
func (code RejectCode) String() string {
	if s, ok := rejectCodeStrings[code]; ok {
		return s
	}

	return fmt.Sprintf("Unknown RejectCode (%d)", uint8(code))
}

// MsgReject implements the Message interface and represents a bitcoin reject
// message.
//
// This message was not added until protocol version RejectVersion.
type MsgReject struct {
	// Cmd is the command for the message which was rejected such as
	// as CmdBlock or CmdTx.  This can be obtained from the Command function
	// of a Message.
	Cmd string

	// RejectCode is a code indicating why the command was rejected.  It
	// is encoded as a uint8 on the protos.
	Code RejectCode

	// Reason is a human-readable string with specific details (over and
	// above the reject code) about why the command was rejected.
	Reason string

	// Hash identifies a specific block or transaction that was rejected
	// and therefore only applies the MsgBlock and MsgTx messages.
	Hash common.Hash
}

// VVSDecode decodes r using the bitcoin protocol encoding into the receiver.
// This is part of the Message interface implementation.
func (msg *MsgReject) VVSDecode(r io.Reader, pver uint32, enc MessageEncoding) error {
	// Command that was rejected.
	cmd, err := serialization.ReadVarString(r, pver)
	if err != nil {
		return err
	}
	msg.Cmd = cmd

	// Code indicating why the command was rejected.
	err = serialization.ReadUint8(r, (*uint8)(&msg.Code))
	if err != nil {
		return err
	}

	// Human readable string with specific details (over and above the
	// reject code above) about why the command was rejected.
	reason, err := serialization.ReadVarString(r, pver)
	if err != nil {
		return err
	}
	msg.Reason = reason

	// CmdBlock and CmdTx messages have an additional hash field that
	// identifies the specific block or transaction.
	if msg.Cmd == CmdBlock || msg.Cmd == CmdTx {
		err := serialization.ReadNBytes(r, msg.Hash[:], common.HashLength)
		if err != nil {
			return err
		}
	}

	return nil
}

// VVSEncode encodes the receiver to w using the bitcoin protocol encoding.
// This is part of the Message interface implementation.
func (msg *MsgReject) VVSEncode(w io.Writer, pver uint32, enc MessageEncoding) error {
	// Command that was rejected.
	err := serialization.WriteVarString(w, pver, msg.Cmd)
	if err != nil {
		return err
	}

	// Code indicating why the command was rejected.
	err = serialization.WriteUint8(w, uint8(msg.Code))
	if err != nil {
		return err
	}

	// Human readable string with specific details (over and above the
	// reject code above) about why the command was rejected.
	err = serialization.WriteVarString(w, pver, msg.Reason)
	if err != nil {
		return err
	}

	// CmdBlock and CmdTx messages have an additional hash field that
	// identifies the specific block or transaction.
	if msg.Cmd == CmdBlock || msg.Cmd == CmdTx {
		err := serialization.WriteNBytes(w, msg.Hash[:])
		if err != nil {
			return err
		}
	}

	return nil
}

// Command returns the protocol command string for the message.  This is part
// of the Message interface implementation.
func (msg *MsgReject) Command() string {
	return CmdReject
}

// MaxPayloadLength returns the maximum length the payload can be for the
// receiver.  This is part of the Message interface implementation.
func (msg *MsgReject) MaxPayloadLength(pver uint32) uint32 {
	plen := uint32(MaxMessagePayload)
	return plen
}

// NewMsgReject returns a new bitcoin reject message that conforms to the
// Message interface.  See MsgReject for details.
func NewMsgReject(command string, code RejectCode, reason string) *MsgReject {
	return &MsgReject{
		Cmd:    command,
		Code:   code,
		Reason: reason,
	}
}
