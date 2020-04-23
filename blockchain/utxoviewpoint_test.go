// Copyright (c) 2018-2020. The asimov developers
// Copyright (c) 2013-2017 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.
package blockchain

import (
	"bytes"
	"github.com/AsimovNetwork/asimov/asiutil"
	"github.com/AsimovNetwork/asimov/blockchain/txo"
	"github.com/AsimovNetwork/asimov/common"
	"github.com/AsimovNetwork/asimov/database"
	"github.com/AsimovNetwork/asimov/protos"
	"github.com/AsimovNetwork/asimov/txscript"
	"reflect"
	"testing"
)

func TestFetchUtxoViewByAddress(t *testing.T) {
	parivateKeyList := []string{
		"0xd0f0461b7b4d26cf370e6c73b58ef7fa26e8e30853a8cee901ed42cf0879cb6e",  //privateKey0
	}
	acc, _, chain, teardownFunc, err := createFakeChainByPrivateKeys(parivateKeyList, 3)
	if err != nil || chain == nil {
		t.Errorf("create block error %v", err)
	}
	defer teardownFunc()

	var testBookkeepers = []common.Address {
		{0x66,0xe3,0x05,0x4b,0x41,0x10,0x51,0xda,0x54,0x92,0xae,0xc7,0xa8,0x23,0xb0,0x0c,0xb3,0xad,0xd7,0x72,0xd7,},
		{0x66,0xe3,0x05,0x4b,0x41,0x10,0x51,0xda,0x54,0x92,0xae,0xc7,0xa8,0x23,0xb0,0x0c,0xb3,0xad,0xd7,0x72,0x00,},
		{0x66,0xe3,0x05,0x4b,0x41,0x10,0x51,0xda,0x54,0x92,0xae,0xc7,0xa8,0x23,},
	}

	var testAsset = []protos.Assets{
		{0,0},
		{1,1},
	}
	var testAmount = []int64{
		200000000,
		111111111,
	}

	resultEntryList := make([]EntryPair,0)
	var coinbasePkscript []byte
	//create 2 block:
	for i:=0; i<2; i++ {
		block, err := createTestBlock(chain,1, uint16(i), 0, testAsset[i], testAmount[i],
			acc[0].Address,nil, chain.bestChain.Tip())
		if err != nil {
			t.Errorf("create block error %v", err)
		}
		view := NewUtxoViewpoint()
		coinBase := asiutil.NewTx(block.MsgBlock().Transactions[0])
		_ = view.AddTxOuts(coinBase, block.Height())
		if i == 0 {
			coinbasePkscript = coinBase.MsgTx().TxOut[1].PkScript
		} else {
			coinbasePkscript = coinBase.MsgTx().TxOut[0].PkScript
		}

		//add utxo to db
		err = chain.db.Update(func(dbTx database.Tx) error {
			// Update the balance using the state of the utxo view.
			err = dbPutBalance(dbTx, view)
			if err != nil {
				return err
			}
			return nil
		})

		for outpoint, entry := range view.entries {
			// No need to update the database if the entry was not modified.
			if entry == nil || !entry.IsModified() {
				continue
			}
			pkscript := entry.PkScript()
			if bytes.Equal(pkscript, coinbasePkscript) {
				log.Infof("pk = %v",pkscript)
				var tmpEntry EntryPair
				tmpEntry.Key = outpoint
				tmpEntry.Value = entry
				resultEntryList = append(resultEntryList,tmpEntry)
			}
		}

		nodeErr := chain.addBlockNode(block)
		if nodeErr != nil {
			t.Errorf("tests error %v", err)
		}
	}

	tests := []struct {
		addr 		common.Address
		result 		EntryPairList
	} {
		//input addr with utxo
		{
			testBookkeepers[0],
			resultEntryList,
		},
		//input addr with no utxo
		{
			testBookkeepers[1],
			nil,
		},
		//input error addr
		{
			testBookkeepers[2],
			nil,
		},
	}

	for i, test := range tests {
		t.Logf("==========test case %d==========", i)

		utxoViewPoint := NewUtxoViewpoint()
		_, err := chain.FetchUtxoViewByAddress(utxoViewPoint, test.addr.ScriptAddress())
		if err != nil {
			t.Errorf("tests #%d error %v", i, err)
		} else {
			wantLength := len(test.result)
			gotLength := len(utxoViewPoint.entries)
			if wantLength != gotLength {
				t.Errorf("tests #%d error: the number of result is not correct", i)
			}

			for tmp:=0; tmp<len(test.result);tmp++ {
				wantMap := test.result[tmp]
				gotMap := utxoViewPoint.entries
				if _, ok := gotMap[wantMap.Key]; ok {
					if wantMap.Value.Assets().Id != gotMap[wantMap.Key].Assets().Id ||
						wantMap.Value.Assets().Property != gotMap[wantMap.Key].Assets().Property{
						t.Errorf("tests #%d error: the assets of result utxoViewPoint is not correct: " +
							"want: %v, got: %v", i, wantMap.Value.Assets(), gotMap[wantMap.Key].Assets())
					}
					if !bytes.Equal(wantMap.Value.PkScript(), gotMap[wantMap.Key].PkScript()) {
						t.Errorf("tests #%d error: the pkScript of result utxoViewPoint is not correct: " +
							"want: %v, got: %v", i, wantMap.Value.PkScript(), gotMap[wantMap.Key].PkScript())
					}
					if wantMap.Value.Amount() != gotMap[wantMap.Key].Amount() {
						t.Errorf("tests #%d error: the amount of result utxoViewPoint is not correct: " +
							"want: %v, got: %v", i, wantMap.Value.Amount(), gotMap[wantMap.Key].Amount())
					}
					if wantMap.Value.BlockHeight() != gotMap[wantMap.Key].BlockHeight() {
						t.Errorf("tests #%d error: the blockHeight of result utxoViewPoint is not correct: " +
							"want: %v, got: %v", i, wantMap.Value.BlockHeight(), gotMap[wantMap.Key].BlockHeight())
					}
					if gotMap[wantMap.Key].LockItem() != nil {
						t.Errorf("tests #%d error: the lockItem of result utxoViewPoint is not correct: " +
							"want: nil, got: %v", i,gotMap[wantMap.Key].LockItem())
					}
				} else {
					t.Errorf("tests #%d error: the map of result is not correct", i)
				}
			}
		}
	}
}


func TestConstructLockItems(t *testing.T) {
	t.Parallel()

	voteId1 := txo.VoteId{1,2,3,4,5,6,7,8,9,0,1,2,3,4,5,6,7,8,9,0,1,2,3,4,5,6,7,8,0,0,0,1}
	voteId2 := txo.VoteId{1,2,3,4,5,6,7,8,9,0,1,2,3,4,5,6,7,8,9,0,1,2,3,4,5,6,7,8,0,0,0,2}
	voteId3 := txo.VoteId{1,2,3,4,5,6,7,8,9,0,1,2,3,4,5,6,7,8,9,0,1,2,3,4,5,6,7,8,0,0,0,3}
	voteId4 := txo.VoteId{1,2,3,4,5,6,7,8,9,0,1,2,3,4,5,6,7,8,9,0,1,2,3,4,5,6,7,8,0,0,0,4}
	assets1 := protos.Assets{0,1}
	assets2 := protos.Assets{0,2}
	assets3 := protos.Assets{0,3}
	indivisibleAssets := protos.Assets{1,1}

	//test coinbase tx:
	msgCoinbase := protos.MsgTx{
		TxIn: []*protos.TxIn {
			protos.NewTxIn(&protos.OutPoint{
				Hash:common.Hash{}, Index:protos.MaxPrevOutIndex,
			}, nil),
		},
		TxOut: []*protos.TxOut {
			protos.NewTxOut(10000000000, []byte{txscript.OP_1}, asiutil.FlowCoinAsset),
		},
	}
	txCoinbase := asiutil.NewTx(&msgCoinbase)
	viewCoinbase := NewUtxoViewpoint()
	lockItemCoinbaseWant := []*txo.LockItem {
		nil,
	}
	lockItemsCoinbase, feeLockItemsCoinbase := viewCoinbase.ConstructLockItems(txCoinbase, 10)
	if len(feeLockItemsCoinbase) != 0 {
		t.Errorf("the length of feeLockItemsCoinbase expect to be 0")
		return
	}
	if !reflect.DeepEqual(lockItemsCoinbase, lockItemCoinbaseWant) {
		t.Errorf("lock item: returned item is unexpected -- got (%v), want (%v)",
			lockItemsCoinbase, lockItemCoinbaseWant)
		return
	}

	//test no outPut tx:
	msgNoOutPut := protos.MsgTx{
		TxIn: []*protos.TxIn {
			protos.NewTxIn(&protos.OutPoint{
				Hash:common.Hash([common.HashLength]byte{
					0,1,2,3,4,5,6,7,8,9,0,1,2,3,4,5,6,7,8,9,0,1,2,3,4,5,6,7,8,9,0,0,
				}), Index:0,
			}, nil),
		},
	}
	txNoOutPut := asiutil.NewTx(&msgNoOutPut)
	viewNoOutPut := NewUtxoViewpoint()
	lockItemsNoOutPut, feeLockItemsNoOutPut := viewNoOutPut.ConstructLockItems(txNoOutPut, 10)
	if !(lockItemsNoOutPut == nil && feeLockItemsNoOutPut == nil) {
		t.Errorf("lockItems1 and feeLockItems1 both expected nil")
		return
	}

	//test VoteTy tx with indivisible assets
	indivisibleAssetMsg := protos.MsgTx{
		TxIn: []*protos.TxIn {
			protos.NewTxIn(&protos.OutPoint{
				Hash:common.Hash([common.HashLength]byte{
					0,1,2,3,4,5,6,7,8,9,0,1,2,3,4,5,6,7,8,9,0,1,2,3,4,5,6,7,8,9,0,0,
				}), Index:0,
			}, nil),
			protos.NewTxIn(&protos.OutPoint{
				Hash:common.Hash([common.HashLength]byte{
					0,1,2,3,4,5,6,7,8,9,0,1,2,3,4,5,6,7,8,9,0,1,2,3,4,5,6,7,8,9,0,10,
				}), Index:0,
			}, nil),
		},
		TxOut: []*protos.TxOut {
			protos.NewContractTxOut(200000000, []byte{txscript.OP_VOTE, txscript.OP_DATA_21,
				0x63, 0,1,2,3,4,5,6,7,8,9,0,1,2,3,4,5,6,7,8,9,}, indivisibleAssets, []byte{
				0,1,2,3,4,5,
				0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,2,
				0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,200,
			}),
			protos.NewTxOut(100000000, []byte{txscript.OP_3}, asiutil.FlowCoinAsset),
		},
	}
	indivisibleTx := asiutil.NewTx(&indivisibleAssetMsg)
	indivisibleView := NewUtxoViewpoint()
	indivisibleView.entries[indivisibleAssetMsg.TxIn[0].PreviousOutPoint] = txo.NewUtxoEntry(
		200000000,
		nil,
		0,
		false,
		&indivisibleAssets,
		nil)
	indivisibleView.entries[indivisibleAssetMsg.TxIn[1].PreviousOutPoint] = txo.NewUtxoEntry(
		100000000,
		nil,
		0,
		false,
		&asiutil.FlowCoinAsset,
		nil)


	indivisibleAssetLockItems, indivisibleAssetFeeLockItems := indivisibleView.ConstructLockItems(indivisibleTx, 10)
	if len(indivisibleAssetFeeLockItems) != 0 {
		t.Errorf("the length of feeLockItemsCoinbase expect to be 0")
		return
	}
	if len(indivisibleAssetLockItems) != len(indivisibleAssetMsg.TxOut) {
		t.Errorf("the length of indivisibleAssetLockItems is not equal to indivisibleAssetMsg.TxOut")
		return
	}
	for k:=0; k<len(indivisibleAssetLockItems); k++ {
		if indivisibleAssetLockItems[k] != nil {
			t.Errorf("the  indivisibleAssetLockItems[%d] is expect to be nil", k)
			return
		}
	}

	//test OP_VOTE tx:
	msg := protos.MsgTx{
		TxIn: []*protos.TxIn {
			protos.NewTxIn(&protos.OutPoint{
				Hash:common.Hash([common.HashLength]byte{
					0,1,2,3,4,5,6,7,8,9,0,1,2,3,4,5,6,7,8,9,0,1,2,3,4,5,6,7,8,9,0,0,
				}), Index:0,
			}, nil),
			protos.NewTxIn(&protos.OutPoint{
				Hash:common.Hash([common.HashLength]byte{
					0,1,2,3,4,5,6,7,8,9,0,1,2,3,4,5,6,7,8,9,0,1,2,3,4,5,6,7,8,9,0,1,
				}), Index:1,
			}, nil),
			protos.NewTxIn(&protos.OutPoint{
				Hash:common.Hash([common.HashLength]byte{
					0,1,2,3,4,5,6,7,8,9,0,1,2,3,4,5,6,7,8,9,0,1,2,3,4,5,6,7,8,9,0,2,
				}), Index:2,
			}, nil),
			protos.NewTxIn(&protos.OutPoint{
				Hash:common.Hash([common.HashLength]byte{
					0,1,2,3,4,5,6,7,8,9,0,1,2,3,4,5,6,7,8,9,0,1,2,3,4,5,6,7,8,9,0,3,
				}), Index:3,
			}, nil),
			protos.NewTxIn(&protos.OutPoint{
				Hash:common.Hash([common.HashLength]byte{
					0,1,2,3,4,5,6,7,8,9,0,1,2,3,4,5,6,7,8,9,0,1,2,3,4,5,6,7,8,9,0,4,
				}), Index:4,
			}, nil),
			protos.NewTxIn(&protos.OutPoint{
				Hash:common.Hash([common.HashLength]byte{
					0,1,2,3,4,5,6,7,8,9,0,1,2,3,4,5,6,7,8,9,0,1,2,3,4,5,6,7,8,9,0,5,
				}), Index:5,
			}, nil),
		},
		TxOut: []*protos.TxOut {
			protos.NewContractTxOut(200000000, []byte{txscript.OP_VOTE, txscript.OP_DATA_21,
				0x63, 0,1,2,3,4,5,6,7,8,9,0,1,2,3,4,5,6,7,8,9,}, asiutil.FlowCoinAsset, []byte{
				0,1,2,3,4,5,
				0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,2,
				0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,200,
			}),
			protos.NewTxOut(1600000000, []byte{txscript.OP_1}, assets1),
			protos.NewTxOut(100000000, []byte{txscript.OP_2}, assets1),
			protos.NewTxOut(2100000000, []byte{txscript.OP_3}, assets2),
			protos.NewTxOut(4000000000, []byte{txscript.OP_3}, assets3),
			protos.NewTxOut(0, []byte{txscript.OP_3}, assets3),
			protos.NewTxOut(100000000, []byte{txscript.OP_3}, asiutil.FlowCoinAsset),
		},
	}
	tx := asiutil.NewTx(&msg)
	firstOut := tx.MsgTx().TxOut[0]
	_, addrs, _, _ := txscript.ExtractPkScriptAddrs(firstOut.PkScript)
	id := pickVoteArgument(firstOut.Data)
	voteIdFirst := txo.NewVoteId(addrs[0].StandardAddress(), id)

	view := NewUtxoViewpoint()
	view.entries[msg.TxIn[0].PreviousOutPoint] = txo.NewUtxoEntry(
		300000000,
		nil,
		0,
		false,
		&asiutil.FlowCoinAsset,
		nil)
	view.entries[msg.TxIn[1].PreviousOutPoint] = txo.NewUtxoEntry(
		1500000000,
		nil,
		0,
		false,
		&assets1,
		&txo.LockItem{
			map[txo.VoteId]*txo.LockEntry{
				voteId1:&txo.LockEntry{
					voteId1, 500000000,
				},
				voteId2:&txo.LockEntry{
					voteId2, 800000000,
				},
			},
		})
	view.entries[msg.TxIn[2].PreviousOutPoint] = txo.NewUtxoEntry(
		500000000,
		nil,
		0,
		false,
		&assets1,
		&txo.LockItem{
			map[txo.VoteId]*txo.LockEntry{
				voteId1:&txo.LockEntry{
					voteId1, 500000000,
				},
				voteId2:&txo.LockEntry{
					voteId2, 1200000000,
				},
			},
		})
	view.entries[msg.TxIn[3].PreviousOutPoint] = txo.NewUtxoEntry(
		220000000,
		nil,
		0,
		false,
		&assets2,
		&txo.LockItem{
			map[txo.VoteId]*txo.LockEntry{
				voteId1:&txo.LockEntry{
					voteId1, 3000000000,
				},
				voteId3:&txo.LockEntry{
					voteId3, 300000000,
				},
			},
		})
	view.entries[msg.TxIn[4].PreviousOutPoint] = txo.NewUtxoEntry(
		4000000000,
		nil,
		0,
		false,
		&assets3,
		&txo.LockItem{
			map[txo.VoteId]*txo.LockEntry{
				voteId4:&txo.LockEntry{
					voteId4, 4000000000,
				},
			},
		})
	view.entries[msg.TxIn[5].PreviousOutPoint] = txo.NewUtxoEntry(
		100000000,
		nil,
		0,
		false,
		&asiutil.FlowCoinAsset,
		&txo.LockItem{
			map[txo.VoteId]*txo.LockEntry{
				*voteIdFirst:&txo.LockEntry{
					*voteIdFirst, 100000000,
				},
			},
		})


	//result that we want:
	lockItemWant := []*txo.LockItem {
		&txo.LockItem{
			map[txo.VoteId]*txo.LockEntry{
				*voteIdFirst:&txo.LockEntry{
					*voteIdFirst, 200000000,
				},
			},
		},
		&txo.LockItem{
			map[txo.VoteId]*txo.LockEntry{
				voteId1:&txo.LockEntry{
					voteId1, 1000000000,
				},
				voteId2:&txo.LockEntry{
					voteId2, 1600000000,
				},
			},
		},
		&txo.LockItem{
			map[txo.VoteId]*txo.LockEntry{
				voteId2:&txo.LockEntry{
					voteId2, 100000000,
				},
			},
		},
		&txo.LockItem{
			map[txo.VoteId]*txo.LockEntry{
				voteId1:&txo.LockEntry{
					voteId1, 2100000000,
				},
				voteId3:&txo.LockEntry{
					voteId3, 300000000,
				},
			},
		},
		&txo.LockItem{
			map[txo.VoteId]*txo.LockEntry{
				voteId4:&txo.LockEntry{
					voteId4, 4000000000,
				},
			},
		},
		nil,
		&txo.LockItem{
			map[txo.VoteId]*txo.LockEntry{
				*voteIdFirst:&txo.LockEntry{
					*voteIdFirst, 100000000,
				},
			},
		},
	}
	feeLockItemsWant := map[protos.Assets]*txo.LockItem {
		protos.Assets{0,1}:&txo.LockItem{
			map[txo.VoteId]*txo.LockEntry{
				voteId2:&txo.LockEntry{
					voteId2, 300000000,
				},
			},
		},
		protos.Assets{0,2}:&txo.LockItem{
			map[txo.VoteId]*txo.LockEntry{
				voteId1:&txo.LockEntry{
					voteId1, 900000000,
				},
			},
		},
	}

	lockItems, feeLockItems := view.ConstructLockItems(tx, 10)
	if len(lockItems) != len(msg.TxOut) {
		t.Errorf("lockItems length expected %d, get %d", len(msg.TxOut), len(lockItems))
		return
	}

	if !reflect.DeepEqual(lockItems, lockItemWant) {
		t.Errorf("lock item: returned item is unexpected -- got (%v), want (%v)",
			lockItems, lockItemWant)
		return
	}
	if !reflect.DeepEqual(feeLockItems, feeLockItemsWant) {
		t.Errorf("lock item fee: returned item is unexpected -- got (%v), want (%v)",
			feeLockItems, feeLockItemsWant)
		return
	}
}
