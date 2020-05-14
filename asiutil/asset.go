// Copyright (c) 2018-2020. The asimov developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.
package asiutil

import "github.com/AsimovNetwork/asimov/protos"


// assetList defines o list of asset
type AssetsList []protos.Asset

func (al AssetsList) Swap(i, j int) {
	al[i], al[j] = al[j], al[i]
}

func (al AssetsList) Len() int {
	return len(al)
}

func (al AssetsList) Less(i, j int) bool {
	return al[i].Id < al[j].Id || (al[i].Id == al[j].Id && al[i].Property < al[j].Property)
}
