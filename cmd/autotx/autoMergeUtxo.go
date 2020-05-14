// Copyright (c) 2018-2020. The asimov developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.
package main

import (
    "bytes"
    "encoding/hex"
    "github.com/AsimovNetwork/asimov/blockchain"
    "github.com/AsimovNetwork/asimov/chaincfg"
    "github.com/AsimovNetwork/asimov/common"
    "github.com/AsimovNetwork/asimov/crypto"
    "github.com/AsimovNetwork/asimov/protos"
    "github.com/AsimovNetwork/asimov/txscript"
    "github.com/AsimovNetwork/asimov/asiutil"
)

func (s *Server) autoMergeUtxo(mergeList []mergeUtxoData, utxoCnt int, account *crypto.Account) error {
    gasLimit := chaincfg.DefaultMergeLimit
    tmpGasLimit := utxoCnt * 150 * blockchain.NormalGas * 11 / 10
    if tmpGasLimit > gasLimit {
        gasLimit = tmpGasLimit
    }
    fees := int64(gasLimit)

    senderPkScript, err := txscript.PayToAddrScript(account.Address)
    if err != nil {
        mainLog.Errorf("get senderPkScript error: %v", err)
        return err
    }

    txMsg := protos.NewMsgTx(protos.TxVersion)
    for _, data := range mergeList {
        outValue := data.Value
        if data.Asset == asiutil.AsimovAsset {
            outValue = outValue - fees
        }
        if outValue < 0 {
            return nil
        }
        for _, prevOut := range data.PreOutsList {
            txMsg.AddTxIn(&protos.TxIn{
                PreviousOutPoint: prevOut,
                SignatureScript:  nil,
                Sequence:         protos.MaxTxInSequenceNum - 1,
            })
        }
        if outValue > 0 {
            txMsg.AddTxOut(&protos.TxOut{
                Value:    outValue,
                PkScript: senderPkScript,
                Asset:    data.Asset,
                Data:     nil,
            })
        }
    }

    txMsg.TxContract.GasLimit = uint32(gasLimit)

    // Sign the txMsg transaction.
    lookupKey := func(a common.IAddress) (*crypto.PrivateKey, bool, error) {
        return &account.PrivateKey, true, nil
    }
    for i := 0; i < len(txMsg.TxIn); i++ {
        sigScript, err := txscript.SignTxOutput(txMsg, i,
            senderPkScript, txscript.SigHashAll,
            txscript.KeyClosure(lookupKey), nil, nil)
        if err != nil {
            mainLog.Errorf("autoMergeUtxoTx error: %v", err)
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
    mainLog.Infof("autoMergeUtxoTx broadcast!")
    return nil
}