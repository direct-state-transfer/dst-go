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

package channel

import (
	"io"

	"github.com/pkg/errors"

	perunio "perun.network/go-perun/pkg/io"
)

type (
	// State is the full state of a state channel (app).
	// It does not include the channel parameters. However, the included channel ID
	// should be a commitment to the channel parameters by hash-digesting them.
	// The state is the piece of data that is signed and sent to the adjudicator
	// during disputes.
	State struct {
		// id is the immutable id of the channel this state belongs to
		ID ID
		// version counter
		Version uint64
		// App identifies the application that this channel is running.
		// We do not want a deep copy here, since the Apps are just an immutable reference.
		// They are only included in the State to support serialization of the `Data` field.
		App App `cloneable:"shallow"`
		// Allocation is the current allocation of channel assets to
		// the channel participants and apps running inside this channel.
		Allocation
		// Data is the app-specific data.
		Data Data
		// IsFinal indicates that the channel is in its final state. Such a state
		// can immediately be settled on the blockchain or a funding channel, in
		// case of sub- or virtual channels.
		// A final state cannot be further progressed.
		IsFinal bool
	}

	// Data is the data of the application running in this app channel.
	// Decoding happens with App.DecodeData.
	Data interface {
		perunio.Encoder
		// Clone should return a deep copy of the Data object.
		// It should return nil if the Data object is nil.
		Clone() Data
	}
)

var _ perunio.Serializer = new(State)

// newState creates a new state, checking that the parameters and allocation are
// compatible. This function is not exported because a user of the channel
// package would usually not create a State directly. The user receives the
// initial state from the machine instead.
func newState(params *Params, initBals Allocation, initData Data) (*State, error) {
	// sanity checks
	n := len(params.Parts)
	for _, asset := range initBals.Balances {
		if n != len(asset) {
			return nil, errors.New("number of participants in parameters and initial balances don't match")
		}
	}
	if err := initBals.Valid(); err != nil {
		return nil, err
	}
	return &State{
		ID:         params.ID(),
		Version:    0,
		App:        params.App,
		Allocation: initBals,
		Data:       initData,
	}, nil
}

// Clone makes a deep copy of the State object.
// If it is nil, it returns nil.
// App implementations should use this method when creating the next state from
// an old one.
func (s *State) Clone() *State {
	if s == nil {
		return nil
	}

	clone := *s
	clone.Allocation = s.Allocation.Clone()
	clone.Data = s.Data.Clone()
	// Shallow copy the app.
	clone.App = s.App
	return &clone
}

// Encode encodes a state into an `io.Writer` or returns an `error`.
func (s State) Encode(w io.Writer) error {
	err := perunio.Encode(w, s.ID, s.Version, s.Allocation, s.IsFinal, OptAppEnc{s.App}, s.Data)
	return errors.WithMessage(err, "state encode")
}

// Decode decodes a state from an `io.Reader` or returns an `error`.
func (s *State) Decode(r io.Reader) error {
	// Decode ID, Version, Allocation, IsFinal, App
	err := perunio.Decode(r, &s.ID, &s.Version, &s.Allocation, &s.IsFinal, OptAppDec{&s.App})
	if err != nil {
		return errors.WithMessage(err, "id or version decode")
	}
	// Decode app data
	s.Data, err = s.App.DecodeData(r)
	return errors.WithMessage(err, "app decode data")
}

// Equal returns whether two `State` objects are equal.
// The App is compared by its definition, and the Apps Data by its encoding.
func (s *State) Equal(t *State) error {
	if s == t {
		return nil
	}
	if s.ID != t.ID {
		return errors.New("different IDs")
	}
	if s.Version != t.Version {
		return errors.New("different Versions")
	}
	if err := AppShouldEqual(s.App, t.App); err != nil {
		return err
	}
	if err := s.Allocation.Equal(&t.Allocation); err != nil {
		return errors.WithMessage(err, "different Allocations")
	}
	if ok, err := perunio.EqualEncoding(s.Data, t.Data); err != nil {
		return errors.WithMessage(err, "comparing App data encoding")
	} else if !ok {
		return errors.Errorf("different App data")
	}
	if s.IsFinal != t.IsFinal {
		return errors.New("different IsFinal flags")
	}
	return nil
}

// ToSubAlloc creates a SubAlloc from the state that can be added to the
// parent channel's locked funds.
func (s *State) ToSubAlloc() *SubAlloc {
	return NewSubAlloc(s.ID, s.Allocation.Sum(), nil)
}
