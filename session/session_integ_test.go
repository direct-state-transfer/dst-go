// Copyright (c) 2020 - for information on the respective copyright owner
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

// +build integration

package session_test

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"testing"

	copyutil "github.com/otiai10/copy"
	"github.com/phayes/freeport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hyperledger-labs/perun-node"
	"github.com/hyperledger-labs/perun-node/blockchain/ethereum/ethereumtest"
	"github.com/hyperledger-labs/perun-node/idprovider/idprovidertest"
	"github.com/hyperledger-labs/perun-node/session"
	"github.com/hyperledger-labs/perun-node/session/sessiontest"
)

func init() {
	session.SetWalletBackend(ethereumtest.NewTestWalletBackend())
}

func Test_Integ_New(t *testing.T) {
	prng := rand.New(rand.NewSource(1729))
	peerIDs := newPeerIDs(t, prng, uint(2))

	prng = rand.New(rand.NewSource(1729))
	cfg := sessiontest.NewConfigT(t, prng, peerIDs...)

	t.Run("happy", func(t *testing.T) {
		sess, err := session.New(cfg)
		require.NoError(t, err)
		assert.NotNil(t, sess)
	})
	t.Run("persistence_already_in_user", func(t *testing.T) {
		_, err := session.New(cfg)
		require.Error(t, err)
		t.Log(err)
	})
	t.Run("invalid_chain_addr", func(t *testing.T) {
		cfgCopy := cfg
		cfgCopy.DatabaseDir = newDatabaseDir(t)
		cfgCopy.ChainURL = "invalid-url"
		_, err := session.New(cfgCopy)
		assert.Error(t, err)
		t.Log(err)
	})
	t.Run("zero_chainConnTimeout", func(t *testing.T) {
		cfgCopy := cfg
		cfgCopy.DatabaseDir = newDatabaseDir(t)
		cfgCopy.ChainConnTimeout = 0
		_, err := session.New(cfgCopy)
		assert.Error(t, err)
		t.Log(err)
	})
	t.Run("invalid_user_onchain_addr", func(t *testing.T) {
		cfgCopy := cfg
		cfgCopy.DatabaseDir = newDatabaseDir(t)
		cfgCopy.User.OnChainAddr = ethereumtest.NewRandomAddress(prng).String()
		_, err := session.New(cfgCopy)
		assert.Error(t, err)
		t.Log(err)
	})
	t.Run("invalid_asset_address", func(t *testing.T) {
		cfgCopy := cfg
		cfgCopy.DatabaseDir = newDatabaseDir(t)
		cfgCopy.Asset = "invalid_addr"
		_, err := session.New(cfgCopy)
		assert.Error(t, err)
		t.Log(err)
	})
	t.Run("invalid_adjudicator_contract", func(t *testing.T) {
		cfgCopy := cfg
		cfgCopy.DatabaseDir = newDatabaseDir(t)
		cfgCopy.Adjudicator = ethereumtest.NewRandomAddress(prng).String()
		_, err := session.New(cfgCopy)
		assert.Error(t, err)
		t.Log(err)
	})
	t.Run("invalid_asset_contract", func(t *testing.T) {
		cfgCopy := cfg
		cfgCopy.DatabaseDir = newDatabaseDir(t)
		cfgCopy.Asset = ethereumtest.NewRandomAddress(prng).String()
		_, err := session.New(cfgCopy)
		assert.Error(t, err)
		t.Log(err)
	})
	t.Run("unsupported_comm_backend", func(t *testing.T) {
		cfgCopy := cfg
		cfgCopy.DatabaseDir = newDatabaseDir(t)
		cfgCopy.User.CommType = "unsupported"
		_, err := session.New(cfgCopy)
		assert.Error(t, err)
		t.Log(err)
	})
	t.Run("unsupported_idprovider_type", func(t *testing.T) {
		cfgCopy := cfg
		cfgCopy.DatabaseDir = newDatabaseDir(t)
		cfgCopy.IDProviderType = "unsupported"
		_, err := session.New(cfgCopy)
		assert.Error(t, err)
		t.Log(err)
	})
	t.Run("invalid_idprovider_init_error", func(t *testing.T) {
		cfgCopy := cfg
		cfgCopy.DatabaseDir = newDatabaseDir(t)
		cfgCopy.IDProviderURL = newCorruptedYAMLFile(t)
		_, err := session.New(cfgCopy)
		assert.Error(t, err)
		t.Log(err)
	})
	t.Run("invalid_idprovider_has_entry_for_self", func(t *testing.T) {
		ownPeer := perun.PeerID{
			Alias: perun.OwnAlias,
		}
		cfgCopy := cfg
		cfgCopy.DatabaseDir = newDatabaseDir(t)
		cfgCopy.IDProviderURL = idprovidertest.NewIDProviderT(t, ownPeer)
		_, err := session.New(cfgCopy)
		t.Log(err)
		assert.Error(t, err)
	})
}

func Test_Integ_Persistence(t *testing.T) {
	prng := rand.New(rand.NewSource(1729))

	aliceCfg := sessiontest.NewConfigT(t, prng)
	// Use idprovider and databaseDir from a session that was persisted already.
	aliceCfg.DatabaseDir = copyDirToTmp(t, "../testdata/session/persistence/alice-database")
	aliceCfg.IDProviderURL = "../testdata/session/persistence/alice-idprovider.yaml"

	alice, err := session.New(aliceCfg)
	require.NoErrorf(t, err, "initializing alice session")
	t.Logf("alice session id: %s\n", alice.ID())
	t.Logf("alice database dir is: %s\n", aliceCfg.DatabaseDir)

	t.Run("GetChannelInfos", func(t *testing.T) {
		t.Run("happy", func(t *testing.T) {
			require.Equal(t, 3, len(alice.GetChsInfo()))
		})
	})
}

func newPeerIDs(t *testing.T, prng *rand.Rand, n uint) []perun.PeerID {
	peerIDs := make([]perun.PeerID, n)
	for i := range peerIDs {
		port, err := freeport.GetFreePort()
		require.NoError(t, err)
		peerIDs[i].Alias = fmt.Sprintf("%d", i)
		peerIDs[i].OffChainAddrString = ethereumtest.NewRandomAddress(prng).String()
		peerIDs[i].CommType = "tcp"
		peerIDs[i].CommAddr = fmt.Sprintf("127.0.0.1:%d", port)
	}
	return peerIDs
}

func newCorruptedYAMLFile(t *testing.T) string {
	// First line has yaml syntax error (two colons).
	corruptedYaml := `
Alice: alias: Alice
    offchain_address: 0x9282681723920798983380581376586951466585
    comm_address: 127.0.0.1:5751
    comm_type: tcpip
Bob:
    alias: Bob
    offchain_address: 0x3369783337071807248093730889602727505701
    comm_address: 127.0.0.1:5750
    comm_type: tcpip`

	tempFile, err := ioutil.TempFile("", "")
	require.NoError(t, err)
	t.Cleanup(func() {
		if err = os.Remove(tempFile.Name()); err != nil {
			t.Log("Error in test cleanup: removing file - " + tempFile.Name())
		}
	})
	_, err = tempFile.WriteString(corruptedYaml)
	require.NoErrorf(t, tempFile.Close(), "closing temporary file")
	require.NoError(t, err)
	return tempFile.Name()
}

func copyDirToTmp(t *testing.T, src string) (tempDirName string) {
	var err error
	tempDirName, err = ioutil.TempDir("", "")
	require.NoError(t, err)
	require.NoError(t, copyutil.Copy(src, tempDirName))
	t.Cleanup(func() {
		if err := os.RemoveAll(tempDirName); err != nil {
			t.Logf("Error in removing the file in test cleanup - %v", err)
		}
	})
	return tempDirName
}

func newDatabaseDir(t *testing.T) (dir string) {
	databaseDir, err := ioutil.TempDir("", "")
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := os.RemoveAll(databaseDir); err != nil {
			t.Logf("Error in removing the file in test cleanup - %v", err)
		}
	})
	return databaseDir
}
