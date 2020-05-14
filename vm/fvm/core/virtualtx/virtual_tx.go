package virtualtx

// Virtual transaction (VTX) transfers asset state change to chain

import (
	"github.com/AsimovNetwork/asimov/common"
	"github.com/AsimovNetwork/asimov/protos"
	"math/big"
)

const (
	// transfer asset
	VTransferTypeNormal   uint8 = 1
	// create new asset
	VTransferTypeCreation uint8 = 2
	// mint existing asset
	VTransferTypeMint     uint8 = 3
)

type VTransfer struct {
	From          []byte
	To            []byte
	Amount        int64
	Asset         *protos.Asset
	VTransferType uint8
}

type VirtualTransaction struct {
	VTransfer []*VTransfer

	// asset --> result
	items map[protos.Asset]*TransferResultItem
}

func NewVirtualTransaction() *VirtualTransaction {
	return &VirtualTransaction{
		VTransfer: make([]*VTransfer, 0, 0),
		items:     make(map[protos.Asset]*TransferResultItem),
	}
}

type TransferResultItem struct {
	Asset  []byte // not used
	Erc20  bool
	Create bool // true if a new asset is created
	Mint   bool // true if asset is mint

	// 721 asset mint record
	Erc721Mint map[int64]struct{}

	// asset transfer record
	Erc20Change  map[common.Address]int64 // 20 asset transfer record
	Erc721Change map[common.Address]map[int64]struct{} // 721 asset transfer record
}

func (vtx *VirtualTransaction) GetAllTransfers() map[protos.Asset]*TransferResultItem {
	return vtx.items
}

func (vtx *VirtualTransaction) AppendVTransfer(sender common.Address, to common.Address, amount *big.Int, assets *protos.Asset) {
	vTransfer := VTransfer{
		From:          sender.Bytes(),
		To:            to.Bytes(),
		Amount:        amount.Int64(),
		Asset:         assets,
		VTransferType: VTransferTypeNormal,
	}
	vtx.VTransfer = append(vtx.VTransfer, &vTransfer)

	assetItem := vtx.getItem(assets)
	vtx.addChange(assetItem, sender, -amount.Int64())
	vtx.addChange(assetItem, to, amount.Int64())
}

func (vtx *VirtualTransaction) AppendVCreation(to common.Address, amount *big.Int, assets *protos.Asset, vTransferType uint8) {
	vTransfer := VTransfer{
		From:          nil,
		To:            to.Bytes(),
		Amount:        amount.Int64(),
		Asset:         assets,
		VTransferType: vTransferType,
	}
	vtx.VTransfer = append(vtx.VTransfer, &vTransfer)

	assetItem := vtx.getItem(assets)
	if vTransferType == VTransferTypeCreation {
		assetItem.Create = true
	}

	if amount.Int64() > 0 {
		assetItem.Mint = true
		vtx.mint(assetItem, to, amount.Int64())
	}
}

// return total incoming for 20 asset
// return voucher id for 721 asset; return 0 if voucher id not in change list
func (vtx *VirtualTransaction) GetIncoming(addr common.Address, assets *protos.Asset, voucherId int64) *big.Int {
	income := big.NewInt(0)
	assetItem, ok := vtx.items[*assets]
	if ok {
		if assetItem.Erc20 {
			change, ok := assetItem.Erc20Change[addr]
			if ok {
				income.Add(income, big.NewInt(change))
			}

		} else {
			addressInfo, ok := assetItem.Erc721Change[addr]
			if ok {
				if voucherId > 0 {
					if _, has := addressInfo[voucherId]; has {
						return big.NewInt(voucherId)
					}
				} else {
					return big.NewInt(int64(len(addressInfo)))
				}
			}
		}
	}

	return income
}

func (vtx *VirtualTransaction) getItem(assets *protos.Asset) *TransferResultItem {
	assetItem, ok := vtx.items[*assets]
	if !ok {
		vtx.items[*assets] = &TransferResultItem{
			Erc20:       true,
			Create:      false,
			Mint:        false,
			Erc20Change: map[common.Address]int64{},
		}
		assetItem, _ = vtx.items[*assets]
		if assets.IsIndivisible() {
			assetItem.Erc20 = false
			assetItem.Erc721Change = make(map[common.Address]map[int64]struct{})
			assetItem.Erc721Mint = map[int64]struct{}{}
		}
	}

	return assetItem
}

func (vtx *VirtualTransaction) addChange(assetItem *TransferResultItem, addr common.Address, amount int64) {
	if assetItem.Erc20 {
		vtx.addErc20(assetItem, addr, amount)

	} else {
		vtx.addErc721(assetItem, addr, amount)
	}
}

func (vtx *VirtualTransaction) mint(assetItem *TransferResultItem, addr common.Address, amount int64) {
	if assetItem.Erc20 {
		vtx.addErc20(assetItem, addr, amount)

	} else {
		vtx.mintErc721(assetItem, addr, amount)
	}
}

func (vtx *VirtualTransaction) mintErc721(assetItem *TransferResultItem, addr common.Address, amount int64) {
	assetItem.Erc721Mint[amount] = struct{}{}

	dic, ok := assetItem.Erc721Change[addr]
	if !ok {
		assetItem.Erc721Change[addr] = map[int64]struct{}{}
		dic, _ = assetItem.Erc721Change[addr]
	}

	dic[amount] = struct{}{}
}

func (vtx *VirtualTransaction) addErc20(assetItem *TransferResultItem, addr common.Address, amount int64) {
	_, ok := assetItem.Erc20Change[addr]
	if !ok {
		assetItem.Erc20Change[addr] = 0
	}

	balance, _ := assetItem.Erc20Change[addr]
	if amount+balance > common.MaxXing || amount+balance < -common.MaxXing {
		return
	}

	assetItem.Erc20Change[addr] += amount
}

func (vtx *VirtualTransaction) addErc721(assetItem *TransferResultItem, addr common.Address, amount int64) {
	dic, ok := assetItem.Erc721Change[addr]
	if !ok {
		assetItem.Erc721Change[addr] = map[int64]struct{}{}
		dic, _ = assetItem.Erc721Change[addr]
	}

	voucherId := -amount
	if _, has := dic[voucherId]; has {
		delete(dic, voucherId)
	} else {
		dic[amount] = struct{}{}
	}
}
