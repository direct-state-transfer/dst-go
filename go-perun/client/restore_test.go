// Copyright 2020 - See NOTICE file for copyright holders.
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

package client

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"perun.network/go-perun/channel"
	"perun.network/go-perun/channel/persistence"
	"perun.network/go-perun/channel/test"
	"perun.network/go-perun/log"
	"perun.network/go-perun/pkg/sync"
	pkgtest "perun.network/go-perun/pkg/test"
	"perun.network/go-perun/wallet"
	wallettest "perun.network/go-perun/wallet/test"
	"perun.network/go-perun/wire"
)

func patchChFromSource(
	c *Client,
	ch *persistence.Channel,
	parent *Channel,
	peers ...wire.Address,
) (*Channel, error) {
	acc, _ := wallettest.RandomWallet().Unlock(ch.ParamsV.Parts[ch.IdxV])
	machine, _ := channel.NewStateMachine(acc, *ch.ParamsV)
	pmachine := persistence.FromStateMachine(machine, nil)

	_ch := &Channel{parent: parent, machine: pmachine, OnCloser: new(sync.Closer)}
	_ch.conn = new(channelConn)
	_ch.conn.r = wire.NewRelay()
	return _ch, nil
}

func TestReconstructChannel(t *testing.T) {
	rng := pkgtest.Prng(t)
	db := map[channel.ID]*persistence.Channel{}

	restParent := mkRndChan(rng)
	db[restParent.ID()] = restParent

	restChild := mkRndChan(rng)
	parentID := restParent.ID()
	restChild.Parent = &parentID
	db[restChild.ID()] = restChild

	c := &Client{log: log.Get()}

	t.Run("parent first", func(t *testing.T) {
		chans := map[channel.ID]*Channel{}
		parent := c.reconstructChannel(patchChFromSource, restParent, db, chans)
		child := c.reconstructChannel(patchChFromSource, restChild, db, chans)
		assert.Same(t, child.parent, parent)
	})
	t.Run("child first", func(t *testing.T) {
		chans := map[channel.ID]*Channel{}
		child := c.reconstructChannel(patchChFromSource, restChild, db, chans)
		parent := c.reconstructChannel(patchChFromSource, restParent, db, chans)
		assert.Same(t, child.parent, parent)
	})
}

func TestRestoreChannelCollection(t *testing.T) {
	rng := pkgtest.Prng(t)

	// Generate multiple trees of channels into one collection.
	db := make(map[channel.ID]*persistence.Channel)
	for i := 0; i < 3; i++ {
		mkRndChanTree(rng, 3, 1, 3, db)
	}

	// Remember channels that have been published.
	witnessedChans := make(map[channel.ID]struct{})
	c := &Client{log: log.Get(), channels: makeChanRegistry()}
	c.OnNewChannel(func(ch *Channel) {
		_, ok := witnessedChans[ch.ID()]
		require.False(t, ok)
		_, ok = db[ch.ID()]
		require.True(t, ok)
		witnessedChans[ch.ID()] = struct{}{}
	})

	// Restore all channels into the client and check the published channels.
	c.restoreChannelCollection(db, patchChFromSource)
	require.Equal(t, len(witnessedChans), len(db), "channel count mismatch")

	// Duplicates should be ignored and there should be no missing channels.
	c.OnNewChannel(func(*Channel) {
		t.Fatal("must not add duplicate or new channels")
	})
	c.restoreChannelCollection(db, patchChFromSource)
}

// mkRndChan creates a single random channel.
func mkRndChan(rng *rand.Rand) *persistence.Channel {
	parts := make([]wallet.Address, channel.MaxNumParts)
	for i := range parts {
		parts[i] = wallettest.NewRandomAccount(rng).Address()
	}
	ch := persistence.NewChannel()
	ch.IdxV = channel.Index(rng.Intn(channel.MaxNumParts))
	ch.ParamsV = test.NewRandomParams(rng, test.WithParts(parts...))
	sigs := make([]bool, channel.MaxNumParts)
	opts := test.WithParams(ch.ParamsV)
	ch.StagingTXV = *test.NewRandomTransaction(rng, sigs, opts)
	ch.CurrentTXV = *test.NewRandomTransaction(rng, sigs, opts)
	ch.PhaseV = test.NewRandomPhase(rng)
	return ch
}

// mkRndChanTree creates a tree of up to depth layers, with each layer having
// one minimum and maximum child less per node than the previous layer. The
// generated channels are entered into db and the root channel is returned.
func mkRndChanTree(
	rng *rand.Rand,
	depth, minChildren, maxChildren int,
	db map[channel.ID]*persistence.Channel) (root *persistence.Channel) {
	root = mkRndChan(rng)
	db[root.ID()] = root

	if depth > 0 && maxChildren > 0 {
		children := minChildren + rng.Intn(maxChildren-minChildren+1)
		if minChildren > 0 {
			minChildren--
		}
		for i := 0; i < children; i++ {
			t := mkRndChanTree(rng, depth-1, minChildren, maxChildren-1, db)
			t.Parent = new(channel.ID)
			*t.Parent = root.ID()
		}
	}
	return
}
