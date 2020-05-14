package virtualtx

import (
	"github.com/AsimovNetwork/asimov/common"
	"github.com/AsimovNetwork/asimov/protos"
	"math/big"
	"testing"
)

var (
	//sender common.Address, to common.Address, amount *big.Int, assets []byte
	add1 = common.BytesToAddress([]byte{0x00, 0x00, 0x00, 0x01})
	add2 = common.BytesToAddress([]byte{0x00, 0x00, 0x00, 0x02})
	add3 = common.BytesToAddress([]byte{0x00, 0x00, 0x00, 0x03})
	add4 = common.BytesToAddress([]byte{0x00, 0x00, 0x00, 0x04})

	ass1 = protos.NewAsset(0, 0, 100)
	ass2 = protos.NewAsset(1, 0, 200)
)

func TestAppendVTransfer(t *testing.T) {
	vtx := NewVirtualTransaction()
	vtx.AppendVTransfer(add1, add2, big.NewInt(int64(1)), ass1) //-1, 1
	vtx.AppendVTransfer(add1, add3, big.NewInt(int64(1)), ass1) //-2, 1, 1

	item := vtx.getItem(ass1)
	if !item.Erc20 {
		t.Error("error erc20")
	}
	if item.Mint {
		t.Error("error mint")
	}
	if item.Create {
		t.Error("error create")
	}

	if item.Erc20Change[string(add1.Bytes())] != -2 {
		t.Error("asset change num not equal.")
	}
	if item.Erc20Change[string(add2.Bytes())] != 1 {
		t.Error("asset change num not equal.")
	}
	if item.Erc20Change[string(add3.Bytes())] != 1 {
		t.Error("asset change num not equal.")
	}

	vtx.AppendVTransfer(add2, add3, big.NewInt(int64(1)), ass1) //-2, 0, 2
	if item.Erc20Change[string(add1.Bytes())] != -2 {
		t.Error("asset change num not equal.")
	}
	if item.Erc20Change[string(add2.Bytes())] != 0 {
		t.Error("asset change num not equal.")
	}
	if item.Erc20Change[string(add3.Bytes())] != 2 {
		t.Error("asset change num not equal.")
	}

	vtx.AppendVTransfer(add1, add2, big.NewInt(int64(123)), ass2)
	vtx.AppendVTransfer(add1, add3, big.NewInt(int64(124)), ass2)
	vtx.AppendVTransfer(add2, add3, big.NewInt(int64(100)), ass2)
	vtx.AppendVTransfer(add3, add4, big.NewInt(int64(200)), ass2)
	item = vtx.getItem(ass2)
	if item.Erc20 {
		t.Error("error erc20")
	}
	if item.Mint {
		t.Error("error mint")
	}
	if item.Create {
		t.Error("error create")
	}

	if _, has := item.Erc721Change[string(add1.Bytes())][int64(-123)]; !has {
		t.Error("error erc20")
	}
	if _, has := item.Erc721Change[string(add1.Bytes())][int64(-124)]; !has {
		t.Error("error erc20")
	}
	if _, has := item.Erc721Change[string(add2.Bytes())][int64(123)]; !has {
		t.Error("error erc20")
	}
	if _, has := item.Erc721Change[string(add3.Bytes())][int64(124)]; !has {
		t.Error("error erc20")
	}
	if _, has := item.Erc721Change[string(add2.Bytes())][int64(-100)]; !has {
		t.Error("error erc20")
	}
	if _, has := item.Erc721Change[string(add3.Bytes())][int64(100)]; !has {
		t.Error("error erc20")
	}

	if _, has := item.Erc721Change[string(add3.Bytes())][int64(-200)]; !has {
		t.Error("error erc20")
	}
	if _, has := item.Erc721Change[string(add4.Bytes())][int64(200)]; !has {
		t.Error("error erc20")
	}

	vtx.AppendVTransfer(add3, add4, big.NewInt(int64(100)), ass2)
	if _, has := item.Erc721Change[string(add3.Bytes())][int64(100)]; has {
		t.Error("error erc20")
	}
	if _, has := item.Erc721Change[string(add4.Bytes())][int64(100)]; !has {
		t.Error("error erc20")
	}

}

func TestAppendVCreation(t *testing.T) {
	vtx := NewVirtualTransaction()
	vtx.AppendVCreation(add1, big.NewInt(int64(1)), ass1, VTransferTypeCreation)
	vtx.AppendVCreation(add2, big.NewInt(int64(1)), ass1, VTransferTypeMint)
	item := vtx.getItem(ass1)
	if !item.Erc20 {
		t.Error("error erc20")
	}
	if !item.Mint {
		t.Error("error mint")
	}
	if !item.Create {
		t.Error("error create")
	}

	if item.Erc20Change[string(add1.Bytes())] != 1 {
		t.Error("asset change num not equal.")
	}
	if item.Erc20Change[string(add2.Bytes())] != 1 {
		t.Error("asset change num not equal.")
	}

}

func TestGetIncoming(t *testing.T) {
	vtx := NewVirtualTransaction()
	vtx.AppendVTransfer(add1, add2, big.NewInt(int64(1)), ass1) //-1, 1
	vtx.AppendVTransfer(add1, add3, big.NewInt(int64(1)), ass1) //-2, 1, 1

	if vtx.GetIncoming(add1, ass1, 0).Int64() != -2 {
		t.Error("asset change num not equal.")
	}
	if vtx.GetIncoming(add2, ass1, 0).Int64() != 1 {
		t.Error("asset change num not equal.")
	}
	if vtx.GetIncoming(add3, ass1, 0).Int64() != 1 {
		t.Error("asset change num not equal.")
	}

	if vtx.GetIncoming(add1, ass2, 0).Int64() != 0 {
		t.Error("asset change num not equal.")
	}
	if vtx.GetIncoming(add2, ass2, 0).Int64() != 0 {
		t.Error("asset change num not equal.")
	}
	if vtx.GetIncoming(add3, ass2, 0).Int64() != 0 {
		t.Error("asset change num not equal.")
	}
}