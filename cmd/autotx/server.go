// Copyright (c) 2018-2020. The asimov developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.
package main

import (
    "github.com/AsimovNetwork/asimov/chaincfg"
    "github.com/AsimovNetwork/asimov/crypto"
    "github.com/AsimovNetwork/asimov/protos"
    "github.com/AsimovNetwork/asimov/rpcs/rpcjson"
    "github.com/AsimovNetwork/asimov/rpcs/rpc"
    "sync"
    "sync/atomic"
    "time"
    "encoding/hex"
    "github.com/AsimovNetwork/asimov/asiutil"
)

const (
    DefaultMergeUtxoThreshold = 100
)

type Server struct {
    started         int32
    shutdown        int32
    wg              sync.WaitGroup
    quit            chan struct{}
    config          *chaincfg.FConfig
    timer           *time.Timer
    account         *crypto.Account
    contractAddr    string
    miningMemberAbi string
    signUpFunc      string
    mergeUtxosThd   int32
    wsConn          *rpc.Client
}

func (s *Server) Start() error {
    // Already started
    if atomic.AddInt32(&s.started, 1) != 1 {
        return nil
    }

    s.timer = time.NewTimer(2 * 100000000)

    //connect node using websocket:
    webErr := s.connectNode()
    if webErr != nil {
        return webErr
    }

    //get SystemContract info:
    err := s.getSystemContractInfo()
    if err != nil {
        return err
    }

    //get cur consensus info:
    preOutList, balance, err := s.updateSignUpStatus()
    if err != nil {
        return err
    }
    s.mergeUtxo(preOutList, balance)

    // Start the peer handler which in turn starts the address and block
    // managers.
    s.wg.Add(1)
    go s.watchConsensusStatus()

    return nil
}

func (s *Server) watchConsensusStatus() {
    quitCh := s.quit
mainloop:
    for {
        select {
        case <-s.timer.C:
            preOutList, balance, _ := s.updateSignUpStatus()
            s.mergeUtxo(preOutList, balance)
        case <-quitCh:
            break mainloop
        }
    }
    s.timer.Stop()
cleanup:
    for {
        select {
        case <-s.timer.C:
        default:
            break cleanup
        }
    }
    s.wg.Done()
}

// Stop gracefully shuts down the FlowServer by stopping and disconnecting all
// peers and the main listener.
func (s *Server) Stop() {
    // Make sure this only happens once.
    if atomic.AddInt32(&s.shutdown, 1) != 1 {
        mainLog.Infof("Server is already in the process of shutting down")
        return
    }
    mainLog.Warnf("Server shutting down")

    s.wsConn.Close() //close websocket client

    // Signal the remaining goroutines to quit.
    close(s.quit)
    return
}

// WaitForShutdown blocks until the main listener and peer handlers are stopped.
func (s *Server) WaitForShutdown() {
    s.wg.Wait()
    mainLog.Infof("Server shutdown complete")
}

type mergeUtxoData struct {
    Asset       protos.Asset
    Value       int64
    PreOutsList []protos.OutPoint
}

func (s *Server) connectNode() error {
    url := "ws://" + s.config.WSEndpoint
    wsConn, err := rpc.Dial(url)
    if err != nil {
        return err
    }
    s.wsConn = wsConn

    return nil
}

func (s *Server) getSystemContractInfo() error {
    var result rpcjson.GetConsensusMiningInfoResult
    err := s.wsConn.Call(&result, "asimov_getConsensusMiningInfo")
    if err != nil {
        return err
    }
    mainLog.Infof("getSystemContractInfo from server = %v", result)

    s.contractAddr = result.ContractAddr
    s.miningMemberAbi = result.MiningMemberAbi
    s.signUpFunc = result.SignUpFunc
    return nil
}

func (s *Server) updateSignUpStatus() ([]protos.OutPoint, int64, error) {
    var result rpcjson.GetSignUpStatusResult
    err := s.wsConn.Call(&result, "asimov_getSignUpStatus", s.account.Address.EncodeAddress())
    if err != nil {
        return nil, 0, err
    }
    mainLog.Infof("updateSignUpStatus: receive SignUpStatus from server = %v", result)

    if result.AutoSignUp && len(result.PreOutsList) > 0 {
        totalAmount := result.Balance
        preOutList := result.PreOutsList
        go s.createSignUpTx(&preOutList, totalAmount, s.account)
        return preOutList, totalAmount, nil
    }

    return nil, 0, nil
}

func (s *Server) mergeUtxo(signUpPreOutList []protos.OutPoint, signUpBalance int64) error {

    var mergeResult []rpcjson.GetMergeUtxoResult
    err := s.wsConn.Call(&mergeResult, "asimov_getMergeUtxoStatus", s.account.Address.EncodeAddress(), s.mergeUtxosThd)
    if err != nil {
        return err
    }
    mainLog.Infof("mergeUtxo: receive utxoInfo from server = %v", mergeResult)

    if len(mergeResult) > 0 {
        mergeList := make([]mergeUtxoData, 0)
        for _, result := range mergeResult {
            var tmpData mergeUtxoData
            assetByte, err := hex.DecodeString(result.Asset)
            if err != nil {
                return err
            }
            tmpData.Value = result.Value
            tmpData.Asset = *protos.AssetFromBytes(assetByte)
            tmpData.PreOutsList = result.PreOutsList
            mergeList = append(mergeList, tmpData)
        }

        //remove the utxo used by signUp:
        if signUpBalance > 0 {
            ascoinIdx := 0
            var mergePreOutList []protos.OutPoint
            for idx, utxoInfo := range mergeList {
                if utxoInfo.Asset == asiutil.AsimovAsset {
                    ascoinIdx = idx
                    mergePreOutList = utxoInfo.PreOutsList
                }
            }

            for _, signUpPreOut := range signUpPreOutList {
                for i := 0; i < len(mergePreOutList); {
                    if mergePreOutList[i] == signUpPreOut {
                        mergePreOutList = append(mergePreOutList[:i], mergePreOutList[i+1:]...)
                    } else {
                        i++
                    }
                }
            }

            mergeList[ascoinIdx].PreOutsList = mergePreOutList
            mergeList[ascoinIdx].Value = mergeList[ascoinIdx].Value - signUpBalance
        }

        //count utxo numbers for calc fees:
        utxoCnt := 0
        for _, utxoInfo := range mergeList {
            utxoCnt += len(utxoInfo.PreOutsList)
        }
        go s.autoMergeUtxo(mergeList, utxoCnt, s.account)
    }

    epochSize := chaincfg.ActiveNetParams.RoundSize
    nextWatch := time.Duration(epochSize*5) * time.Second
    s.timer.Reset(nextWatch)
    mainLog.Infof("WatchInterval = %v", nextWatch)

    return nil
}

func (s *Server) broadcastTx(txStr string) error {
    var result string
    err := s.wsConn.Call(&result, "asimov_sendRawTransaction", txStr)
    if err != nil {
        return err
    }
    mainLog.Infof("broadcastTx result from server: %v", result)

    return nil
}
