// Code generated by mockery v0.0.0-dev. DO NOT EDIT.

package mocks

import (
	context "context"

	perun "github.com/hyperledger-labs/perun-node"
	mock "github.com/stretchr/testify/mock"
)

// SessionAPI is an autogenerated mock type for the SessionAPI type
type SessionAPI struct {
	mock.Mock
}

// AddPeerID provides a mock function with given fields: _a0
func (_m *SessionAPI) AddPeerID(_a0 perun.Peer) error {
	ret := _m.Called(_a0)

	var r0 error
	if rf, ok := ret.Get(0).(func(perun.Peer) error); ok {
		r0 = rf(_a0)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Close provides a mock function with given fields: force
func (_m *SessionAPI) Close(force bool) ([]perun.ChInfo, error) {
	ret := _m.Called(force)

	var r0 []perun.ChInfo
	if rf, ok := ret.Get(0).(func(bool) []perun.ChInfo); ok {
		r0 = rf(force)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]perun.ChInfo)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(bool) error); ok {
		r1 = rf(force)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetCh provides a mock function with given fields: _a0
func (_m *SessionAPI) GetCh(_a0 string) (perun.ChAPI, error) {
	ret := _m.Called(_a0)

	var r0 perun.ChAPI
	if rf, ok := ret.Get(0).(func(string) perun.ChAPI); ok {
		r0 = rf(_a0)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(perun.ChAPI)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(_a0)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetChsInfo provides a mock function with given fields:
func (_m *SessionAPI) GetChsInfo() []perun.ChInfo {
	ret := _m.Called()

	var r0 []perun.ChInfo
	if rf, ok := ret.Get(0).(func() []perun.ChInfo); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]perun.ChInfo)
		}
	}

	return r0
}

// GetPeerID provides a mock function with given fields: alias
func (_m *SessionAPI) GetPeerID(alias string) (perun.Peer, error) {
	ret := _m.Called(alias)

	var r0 perun.Peer
	if rf, ok := ret.Get(0).(func(string) perun.Peer); ok {
		r0 = rf(alias)
	} else {
		r0 = ret.Get(0).(perun.Peer)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(alias)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ID provides a mock function with given fields:
func (_m *SessionAPI) ID() string {
	ret := _m.Called()

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

// OpenCh provides a mock function with given fields: _a0, _a1, _a2, _a3
func (_m *SessionAPI) OpenCh(_a0 context.Context, _a1 perun.BalInfo, _a2 perun.App, _a3 uint64) (perun.ChInfo, error) {
	ret := _m.Called(_a0, _a1, _a2, _a3)

	var r0 perun.ChInfo
	if rf, ok := ret.Get(0).(func(context.Context, perun.BalInfo, perun.App, uint64) perun.ChInfo); ok {
		r0 = rf(_a0, _a1, _a2, _a3)
	} else {
		r0 = ret.Get(0).(perun.ChInfo)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, perun.BalInfo, perun.App, uint64) error); ok {
		r1 = rf(_a0, _a1, _a2, _a3)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// RespondChProposal provides a mock function with given fields: _a0, _a1, _a2
func (_m *SessionAPI) RespondChProposal(_a0 context.Context, _a1 string, _a2 bool) (perun.ChInfo, error) {
	ret := _m.Called(_a0, _a1, _a2)

	var r0 perun.ChInfo
	if rf, ok := ret.Get(0).(func(context.Context, string, bool) perun.ChInfo); ok {
		r0 = rf(_a0, _a1, _a2)
	} else {
		r0 = ret.Get(0).(perun.ChInfo)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string, bool) error); ok {
		r1 = rf(_a0, _a1, _a2)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// SubChProposals provides a mock function with given fields: _a0
func (_m *SessionAPI) SubChProposals(_a0 perun.ChProposalNotifier) error {
	ret := _m.Called(_a0)

	var r0 error
	if rf, ok := ret.Get(0).(func(perun.ChProposalNotifier) error); ok {
		r0 = rf(_a0)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// UnsubChProposals provides a mock function with given fields:
func (_m *SessionAPI) UnsubChProposals() error {
	ret := _m.Called()

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}
