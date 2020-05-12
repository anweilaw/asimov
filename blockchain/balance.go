// Copyright (c) 2018-2020. The asimov developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.
package blockchain

import (
	"github.com/AsimovNetwork/asimov/asiutil"
	"github.com/AsimovNetwork/asimov/common"
	"github.com/AsimovNetwork/asimov/protos"
)

// fetch the balance of one assets for address,
// if it's erc721, return voucher id if there's voucherId in the balance and voucherId > 0.
// otherwise return the count of assets.
func (b *BlockChain) CalculateBalance(block *asiutil.Block, address common.Address, assets *protos.Assets, voucherId int64) (int64, error) {
	view := NewUtxoViewpoint()
	_, err := b.fetchAssetByAddress(block, view, address, assets)
	if err != nil {
		return 0, err
	}
	if assets == nil {
		return 0, nil
	}

	if !assets.IsIndivisible() {
		var balance int64
		for _, entry := range view.entries {
			if !entry.IsSpent() {
				balance += entry.Amount()
			}
		}

		return balance, nil

	} else {
		if voucherId > 0 {
			for _, entry := range view.entries {
				if entry.Amount() == voucherId && !entry.IsSpent() {
					return voucherId, nil
				}
			}
			return 0, nil

		} else {
			count := int64(0)
			for _, entry := range view.entries {
				if entry.Amount() > 0 && !entry.IsSpent() {
					count++
				}
			}

			return count, nil
		}
	}
}
