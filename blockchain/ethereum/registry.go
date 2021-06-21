// Copyright (c) 2021 - for information on the respective copyright owner
// see the NOTICE file and/or the repository at
// https://github.com/hyperledger-labs/perun-node
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ethereum

import (
	"sync"

	pwallet "perun.network/go-perun/wallet"

	"github.com/hyperledger-labs/perun-node"
	"github.com/hyperledger-labs/perun-node/blockchain"
)

// Use a slice to keep track of registered symbols because iterating over map
// to retrieve the symbols each time will result in different ordering of
// symbols in the list.
type contractRegistry struct {
	mtx         sync.RWMutex
	chain       perun.ROChainBackend // read-only chain backend for validating contracts.
	adjudicator pwallet.Address
	assetETH    pwallet.Address
	assets      map[string]pwallet.Address
}

// NewContractRegistry initializes a contract registry and sets the adjudicator
// and asset ETH contract addresses.
//
// If it returns an error, it could be one of the following:
// - InvalidContractError if the contract at given adjudicator or asset ETH
// address is invalid.
// - Standard error if the adjudicator address in the asset ETH contract does not
// match the passed value.
func NewContractRegistry(chain perun.ROChainBackend, adjudicator, assetETH pwallet.Address) (
	perun.ContractRegistry, error) {
	err := chain.ValidateAdjudicator(adjudicator)
	if err != nil {
		return nil, err
	}
	err = chain.ValidateAssetETH(adjudicator, assetETH)
	if err != nil {
		return nil, err
	}

	r := contractRegistry{
		chain:       chain,
		adjudicator: adjudicator,
		assetETH:    assetETH,
		assets:      make(map[string]pwallet.Address),
	}
	return &r, nil
}

// RegisterAsset registers the currency symbol and the asset contract in the
// registry.
//
// If it returns an error, it could be one of the following:
// - AssetERC20RegisteredError if an the same asset contract was already
// registered for another symbol or if another asset contract is registered for
// the symbol of this token contract. For this, symbol is read from blockchain.
// - InvalidContractError if the contract at given asset address is invalid.
// - Standard error if the symbol & decimals could not be read from the
// blockchain or if the token address in the asset contract does not match the
// passed value.
func (r *contractRegistry) RegisterAssetERC20(token, asset pwallet.Address) (
	string, uint8, error) {
	r.mtx.Lock()
	defer r.mtx.Unlock()

	if symbol, found := r.isAssetRegistered(asset); found {
		return "", 0, blockchain.NewAssetERC20RegisteredError(asset.String(), symbol)
	}

	symbol, maxDecimals, err := r.chain.ValidateAssetERC20(r.adjudicator, token, asset)
	if err != nil {
		return "", 0, err
	}

	if alreadyRegisteredAsset, found := r.isSymbolRegistered(symbol); found {
		return "", 0, blockchain.NewAssetERC20RegisteredError(alreadyRegisteredAsset.String(), symbol)
	}

	r.assets[symbol] = asset
	return symbol, maxDecimals, nil
}

// Adjudicator returns adjudicator contract address.
func (r *contractRegistry) Adjudicator() pwallet.Address {
	r.mtx.RLock()
	adjudicator := r.adjudicator
	r.mtx.RUnlock()
	return adjudicator
}

// AssetETH returns asset ETH contract address.
func (r *contractRegistry) AssetETH() pwallet.Address {
	r.mtx.RLock()
	assetETH := r.assetETH
	r.mtx.RUnlock()
	return assetETH
}

// Assets returns a list of all the asset contract addresses
// registered in this module with the addresses in string format.
func (r *contractRegistry) Assets() map[string]string {
	r.mtx.RLock()
	assetsCopy := make(map[string]string, len(r.assets))
	assetsCopy["ETH"] = r.assetETH.String()
	for symbol, asset := range r.assets {
		assetsCopy[symbol] = asset.String()
	}
	r.mtx.RUnlock()
	return assetsCopy
}

// Asset returns asset contract address for the given symbol.
func (r *contractRegistry) Asset(symbol string) (asset pwallet.Address, found bool) {
	r.mtx.RLock()
	asset, found = r.isSymbolRegistered(symbol)
	r.mtx.RUnlock()
	return asset, found
}

// Symbol returns symbol for the given asset contract address.
func (r *contractRegistry) Symbol(asset pwallet.Address) (symbol string, found bool) {
	r.mtx.RLock()
	symbol, found = r.isAssetRegistered(asset)
	r.mtx.RUnlock()
	return symbol, found
}

func (r *contractRegistry) isSymbolRegistered(symbol string) (asset pwallet.Address, found bool) {
	asset, found = r.assets[symbol]
	return asset, found && asset != nil
}

func (r *contractRegistry) isAssetRegistered(asset pwallet.Address) (symbol string, found bool) {
	for symbol, gotAsset := range r.assets {
		if gotAsset.Equals(asset) {
			return symbol, true
		}
	}
	return "", false
}