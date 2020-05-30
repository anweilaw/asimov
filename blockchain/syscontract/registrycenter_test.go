package syscontract

import (
	"github.com/AsimovNetwork/asimov/asiutil"
	"github.com/AsimovNetwork/asimov/protos"
	fvm "github.com/AsimovNetwork/asimov/vm/fvm/core/vm"
	"testing"
)

func TestIsLimit(t *testing.T) {
	limitValue := 0
	m := &Manager{
		assetsUnrestrictedCache:make(map[protos.Asset]struct{}),
		checkLimit: func(block *asiutil.Block, stateDB fvm.StateDB, asset *protos.Asset) int {
			return limitValue
		},
	}
	var sign [protos.HashSignLen]byte
	sign[0] = 1

	unsignedBlock := asiutil.NewBlock(&protos.MsgBlock{
	})

	signedBlock := asiutil.NewBlock(&protos.MsgBlock{
		Header:protos.BlockHeader{
			SigData:sign,
		},
	})

	if m.IsLimit(unsignedBlock,nil,&asiutil.AsimovAsset) != -1 ||
		m.IsLimit(signedBlock,nil,&asiutil.AsimovAsset) != -1 {
		t.Errorf("IsLimit error when asset is main asset")
	}

	limitValue = -1
	asset1 := protos.NewAsset(1,1,1)
	if m.IsLimit(unsignedBlock,nil,asset1) != -1 ||
		m.IsLimit(signedBlock,nil,asset1) != -1 ||
		m.IsLimitInCache(asset1) {
		t.Errorf("IsLimit error when limit value -1")
	}

	limitValue = 1
	asset2 := protos.NewAsset(1,1,2)
	if m.IsLimit(unsignedBlock,nil,asset2) != 1 ||
		m.IsLimit(signedBlock,nil,asset2) != 1 ||
		m.IsLimitInCache(asset2) {
		t.Errorf("IsLimit error when asset limit value 1")
	}

	limitValue = 0
	asset3 := protos.NewAsset(1,1,3)
	if m.IsLimit(unsignedBlock,nil,asset3) != 0 ||
		m.IsLimitInCache(asset3) {
		t.Errorf("IsLimit error when unsigned block and limit value 0")
	}

	if m.IsLimit(signedBlock,nil,asset3) != 0 ||
		!m.IsLimitInCache(asset3) {
		t.Errorf("IsLimit error when signed block and limit value 0")
	}

	m.DisconnectBlock(signedBlock)

	if m.IsLimitInCache(asset3) {
		t.Errorf("DisconnectBlock error")
	}

}