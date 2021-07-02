// Copyright 2019 - See NOTICE file for copyright holders.
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

package client_test

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"perun.network/go-perun/channel/test"
	"perun.network/go-perun/client"
	clienttest "perun.network/go-perun/client/test"
	pkgtest "perun.network/go-perun/pkg/test"
	wallettest "perun.network/go-perun/wallet/test"
	"perun.network/go-perun/wire"
)

func TestNewLedgerChannelProposal(t *testing.T) {
	rng := pkgtest.Prng(t)
	base := clienttest.NewRandomLedgerChannelProposal(rng)

	// Without FundingAgreement it uses InitBals.
	prop, err := client.NewLedgerChannelProposal(base.ChallengeDuration, base.Participant, base.InitBals, base.Peers)
	require.NoError(t, err)
	assert.Equal(t, base.InitBals.Balances, prop.FundingAgreement)

	// FundingAgreements number of assets do not match InitBals.
	agreement := test.NewRandomBalances(rng, test.WithNumAssets(len(base.InitBals.Assets)+1))
	_, err = client.NewLedgerChannelProposal(base.ChallengeDuration, base.Participant, base.InitBals, base.Peers, client.WithFundingAgreement(agreement))
	assert.EqualError(t, err, "comparing FundingAgreement and initial balances sum: dimension mismatch")

	// FundingAgreements sum do not match InitBals sum.
	agreement = test.NewRandomBalances(rng, test.WithNumAssets(len(base.InitBals.Assets)))
	_, err = client.NewLedgerChannelProposal(base.ChallengeDuration, base.Participant, base.InitBals, base.Peers, client.WithFundingAgreement(agreement))
	assert.EqualError(t, err, "FundingAgreement and initial balances differ")
}

func TestChannelProposalReqSerialization(t *testing.T) {
	rng := pkgtest.Prng(t)
	for i := 0; i < 16; i++ {
		var app client.ProposalOpts
		if i&1 == 0 {
			app = client.WithApp(test.NewRandomAppAndData(rng))
		}
		var m wire.Msg
		if i&2 == 0 {
			m = clienttest.NewRandomLedgerChannelProposal(rng, client.WithNonceFrom(rng), app)
		} else {
			var err error
			m, err = clienttest.NewRandomSubChannelProposal(rng, client.WithNonceFrom(rng), app)
			require.NoError(t, err)
		}
		wire.TestMsg(t, m)
	}
}

func TestLedgerChannelProposalReqProposalID(t *testing.T) {
	rng := pkgtest.Prng(t)
	original := *client.NewRandomLedgerChannelProposal(rng)
	s := original.ProposalID()
	fake := *client.NewRandomLedgerChannelProposal(rng)

	assert.NotEqual(t, original.ChallengeDuration, fake.ChallengeDuration)
	assert.NotEqual(t, original.NonceShare, fake.NonceShare)
	assert.NotEqual(t, original.App, fake.App)

	c0 := original
	c0.ChallengeDuration = fake.ChallengeDuration
	assert.NotEqual(t, s, c0.ProposalID())

	c1 := original
	c1.NonceShare = fake.NonceShare
	assert.NotEqual(t, s, c1.ProposalID())

	c2 := original
	c2.Participant = fake.Participant
	assert.NotEqual(t, s, c2.ProposalID())

	c3 := original
	c3.App = fake.App
	assert.NotEqual(t, s, c3.ProposalID())

	c4 := original
	c4.InitData = fake.InitData
	assert.NotEqual(t, s, c4.ProposalID())

	c5 := original
	c5.InitBals = fake.InitBals
	assert.NotEqual(t, s, c5.ProposalID())

	c6 := original
	c6.Peers = fake.Peers
	assert.NotEqual(t, s, c6.ProposalID())
}

func TestSubChannelProposalReqProposalID(t *testing.T) {
	rng := pkgtest.Prng(t)
	original, err := clienttest.NewRandomSubChannelProposal(rng)
	require.NoError(t, err)
	s := original.ProposalID()
	fake, err := clienttest.NewRandomSubChannelProposal(rng)
	require.NoError(t, err)

	assert.NotEqual(t, original.ChallengeDuration, fake.ChallengeDuration)
	assert.NotEqual(t, original.NonceShare, fake.NonceShare)

	c0 := original
	c0.ChallengeDuration = fake.ChallengeDuration
	assert.NotEqual(t, s, c0.ProposalID())

	c1 := original
	c1.NonceShare = fake.NonceShare
	assert.NotEqual(t, s, c1.ProposalID())

	c2 := original
	c2.Parent = fake.Parent
	assert.NotEqual(t, s, c2.ProposalID())

	c3 := original
	c3.InitBals = fake.InitBals
	assert.NotEqual(t, s, c3.ProposalID())
}

func TestChannelProposalAccSerialization(t *testing.T) {
	rng := pkgtest.Prng(t)
	t.Run("ledger channel", func(t *testing.T) {
		for i := 0; i < 16; i++ {
			proposal := clienttest.NewRandomLedgerChannelProposal(rng)
			m := proposal.Accept(
				wallettest.NewRandomAddress(rng),
				client.WithNonceFrom(rng))
			wire.TestMsg(t, m)
		}
	})
	t.Run("sub channel", func(t *testing.T) {
		for i := 0; i < 16; i++ {
			var err error
			proposal, err := clienttest.NewRandomSubChannelProposal(rng)
			require.NoError(t, err)
			m := proposal.Accept(client.WithNonceFrom(rng))
			wire.TestMsg(t, m)
		}
	})
}

func TestChannelProposalRejSerialization(t *testing.T) {
	rng := pkgtest.Prng(t)
	for i := 0; i < 16; i++ {
		m := &client.ChannelProposalRej{
			ProposalID: newRandomProposalID(rng),
			Reason:     newRandomString(rng, 16, 16),
		}
		wire.TestMsg(t, m)
	}
}

func TestSubChannelProposalSerialization(t *testing.T) {
	rng := pkgtest.Prng(t)
	const repeatRandomizedTest = 16
	for i := 0; i < repeatRandomizedTest; i++ {
		prop, err := clienttest.NewRandomSubChannelProposal(rng)
		require.NoError(t, err)
		wire.TestMsg(t, prop)
	}
}

func newRandomProposalID(rng *rand.Rand) (id client.ProposalID) {
	rng.Read(id[:])
	return
}

// newRandomstring returns a random string of length between minLen and
// minLen+maxLenDiff.
func newRandomString(rng *rand.Rand, minLen, maxLenDiff int) string {
	r := make([]byte, minLen+rng.Intn(maxLenDiff))
	rng.Read(r)
	return string(r)
}
