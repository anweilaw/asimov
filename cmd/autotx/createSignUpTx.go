// Copyright (c) 2018-2020. The asimov developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.
package main

import (
    "bytes"
    "encoding/hex"
    "github.com/AsimovNetwork/asimov/asiutil"
    "github.com/AsimovNetwork/asimov/chaincfg"
    "github.com/AsimovNetwork/asimov/common"
    "github.com/AsimovNetwork/asimov/crypto"
    "github.com/AsimovNetwork/asimov/mempool"
    "github.com/AsimovNetwork/asimov/protos"
    "github.com/AsimovNetwork/asimov/txscript"
    "github.com/AsimovNetwork/asimov/vm/fvm"
)

func (s *Server) createSignUpTx(prevOuts *[]protos.OutPoint, totalAmount int64, account *crypto.Account) error {
    fees := int64(chaincfg.DefaultAutoSignUpGasLimit)
    if fees < 0 {
        mainLog.Errorf("createSignUpTx error: fees is smaller than 0, can not gen signUpTx!")
        errStr := "createSignUpTx: fees is smaller than 0, can not gen signUpTx!"
        return mempool.TxRuleError{protos.RejectInvalid, errStr}
    }

    senderPkScript, err := txscript.PayToAddrScript(account.Address)
    if err != nil {
        mainLog.Errorf("get senderPkScript error: %v", err)
        return err
    }

    //get contractPkScript of autoSignUp:
    contractAddrStr := s.contractAddr
    contractAddr := common.HexToAddress(contractAddrStr)
    if contractAddr[0] != common.ContractHashAddrID {
        mainLog.Errorf("invalid contractAddr error: %v", contractAddr)
        return err
    }
    contractPkScript, err := txscript.PayToAddrScript(&contractAddr)
    if err != nil {
        mainLog.Errorf("get contractPkScript error: %v", err)
        return err
    }

    txMsg := protos.NewMsgTx(protos.TxVersion)
    for _, prevOut := range *prevOuts {
        txMsg.AddTxIn(&protos.TxIn{
            PreviousOutPoint: prevOut,
            SignatureScript:  nil,
            Sequence:         protos.MaxTxInSequenceNum,
        })
    }

    Data, DataErr := fvm.PackFunctionArgs(s.miningMemberAbi, s.signUpFunc)
    if DataErr != nil {
        mainLog.Errorf("PackFunctionArgs error: %v", DataErr)
        return DataErr
    }
    txMsg.AddTxOut(&protos.TxOut{
        Value:    int64(0),
        PkScript: contractPkScript,
        Asset:    asiutil.AsimovAsset,
        Data:     Data,
    })

    changeValue := totalAmount - fees
    if changeValue > 0 {
        txMsg.AddTxOut(&protos.TxOut{
            Value:    changeValue,
            PkScript: senderPkScript,
            Asset:    asiutil.AsimovAsset,
            Data:     nil,
        })
    }

    txMsg.TxContract.GasLimit = chaincfg.DefaultAutoSignUpGasLimit

    // Sign the txMsg transaction.
    lookupKey := func(a common.IAddress) (*crypto.PrivateKey, bool, error) {
        return &account.PrivateKey, true, nil
    }
    for i := 0; i < len(txMsg.TxIn); i++ {
        sigScript, err := txscript.SignTxOutput(txMsg, i,
            senderPkScript, txscript.SigHashAll,
            txscript.KeyClosure(lookupKey), nil, nil)
        if err != nil {
            mainLog.Errorf("sigScript error: %v", err)
            return err
        }
        txMsg.TxIn[i].SignatureScript = sigScript
    }

    //encode and broadcast tx:
    var buf bytes.Buffer
    if err := txMsg.VVSEncode(&buf, 0, protos.BaseEncoding); err != nil {
        mainLog.Errorf("txMsg encode error: %v", err)
        return err
    }
    txHex := hex.EncodeToString(buf.Bytes())
    err = s.broadcastTx(txHex)
    if err != nil {
        return err
    }
    mainLog.Infof("auto createSignUpTx broadcast!")
    return nil
}
