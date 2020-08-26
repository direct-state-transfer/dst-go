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

package grpc

import (
	"context"

	psync "perun.network/go-perun/pkg/sync"

	"github.com/pkg/errors"

	"github.com/hyperledger-labs/perun-node"
	"github.com/hyperledger-labs/perun-node/api/grpc/pb"
	"github.com/hyperledger-labs/perun-node/app/payment"
)

// PayChServer represents a grpc server that implements payment channel API.
type PayChServer struct {
	n perun.NodeAPI

	// The mutex should be used when accessing the map data structures.
	psync.Mutex

	// These maps are used to hold an signal channel for each active subscription.
	// When a subscription is registered, subscribe function will add an entry to the
	// map corresponding to the subscription type.
	// The unsubscribe call should retrieve the channel from the map and close it, which
	// will signal the subscription routine to end.
	//
	// chProposalsNotif & chCloseNotifs work on per session basis and hence this is a map
	// of session id to signaling channel.
	// chUpdatesNotif work on a per channel basis and hence this is a map of session id to
	// channel id to signaling channel.

	chProposalsNotif map[string]chan bool
	chUpdatesNotif   map[string]map[string]chan bool
	chClosesNotif    map[string]chan bool
}

// NewPayChServer returns a new grpc server that can server the payment channel API.
func NewPayChServer(n perun.NodeAPI) *PayChServer {
	return &PayChServer{
		n:                n,
		chProposalsNotif: make(map[string]chan bool),
		chUpdatesNotif:   make(map[string]map[string]chan bool),
		chClosesNotif:    make(map[string]chan bool),
	}
}

// GetConfig wraps node.GetConfig.
func (a *PayChServer) GetConfig(context.Context, *pb.GetConfigReq) (*pb.GetConfigResp, error) {
	cfg := a.n.GetConfig()
	return &pb.GetConfigResp{
		ChainAddress:       cfg.ChainURL,
		AdjudicatorAddress: cfg.Adjudicator,
		AssetAddress:       cfg.Asset,
		CommTypes:          cfg.CommTypes,
		ContactTypes:       cfg.ContactTypes,
	}, nil
}

// Time wraps node.Time.
func (a *PayChServer) Time(context.Context, *pb.TimeReq) (*pb.TimeResp, error) {
	return &pb.TimeResp{
		Time: a.n.Time(),
	}, nil
}

// Help wraps node.Help.
func (a *PayChServer) Help(context.Context, *pb.HelpReq) (*pb.HelpResp, error) {
	return &pb.HelpResp{
		Apis: a.n.Help(),
	}, nil
}

// OpenSession wraps node.OpenSession.
func (a *PayChServer) OpenSession(ctx context.Context, req *pb.OpenSessionReq) (*pb.OpenSessionResp, error) {
	errResponse := func(err error) *pb.OpenSessionResp {
		return &pb.OpenSessionResp{
			Response: &pb.OpenSessionResp_Error{
				Error: &pb.MsgError{
					Error: err.Error(),
				},
			},
		}
	}

	sessionID, err := a.n.OpenSession(req.ConfigFile)
	if err != nil {
		return errResponse(err), nil
	}

	a.Lock()
	a.chUpdatesNotif[sessionID] = make(map[string]chan bool)
	a.Unlock()

	return &pb.OpenSessionResp{
		Response: &pb.OpenSessionResp_MsgSuccess_{
			MsgSuccess: &pb.OpenSessionResp_MsgSuccess{
				SessionID: sessionID,
			},
		},
	}, nil
}

// AddContact wraps session.AddContact.
func (a *PayChServer) AddContact(ctx context.Context, req *pb.AddContactReq) (*pb.AddContactResp, error) {
	errResponse := func(err error) *pb.AddContactResp {
		return &pb.AddContactResp{
			Response: &pb.AddContactResp_Error{
				Error: &pb.MsgError{
					Error: err.Error(),
				},
			},
		}
	}

	sess, err := a.n.GetSession(req.SessionID)
	if err != nil {
		return errResponse(err), nil
	}
	err = sess.AddContact(perun.Peer{
		Alias:              req.Peer.Alias,
		OffChainAddrString: req.Peer.OffChainAddress,
		CommAddr:           req.Peer.CommAddress,
		CommType:           req.Peer.CommType,
	})
	if err != nil {
		return errResponse(err), nil
	}

	return &pb.AddContactResp{
		Response: &pb.AddContactResp_MsgSuccess_{
			MsgSuccess: &pb.AddContactResp_MsgSuccess{
				Success: true,
			},
		},
	}, nil
}

// GetContact wraps session.GetContact.
func (a *PayChServer) GetContact(ctx context.Context, req *pb.GetContactReq) (*pb.GetContactResp, error) {
	errResponse := func(err error) *pb.GetContactResp {
		return &pb.GetContactResp{
			Response: &pb.GetContactResp_Error{
				Error: &pb.MsgError{
					Error: err.Error(),
				},
			},
		}
	}

	sess, err := a.n.GetSession(req.SessionID)
	if err != nil {
		return errResponse(err), nil
	}
	peer, err := sess.GetContact(req.Alias)
	if err != nil {
		return errResponse(err), nil
	}

	return &pb.GetContactResp{
		Response: &pb.GetContactResp_MsgSuccess_{
			MsgSuccess: &pb.GetContactResp_MsgSuccess{
				Peer: &pb.Peer{
					Alias:           peer.Alias,
					OffChainAddress: peer.OffChainAddrString,
					CommAddress:     peer.CommAddr,
					CommType:        peer.CommType,
				},
			},
		},
	}, nil
}

// OpenPayCh wraps session.OpenPayCh.
func (a *PayChServer) OpenPayCh(ctx context.Context, req *pb.OpenPayChReq) (*pb.OpenPayChResp, error) {
	errResponse := func(err error) *pb.OpenPayChResp {
		return &pb.OpenPayChResp{
			Response: &pb.OpenPayChResp_Error{
				Error: &pb.MsgError{Error: err.Error()},
			},
		}
	}

	sess, err := a.n.GetSession(req.SessionID)
	if err != nil {
		return errResponse(err), nil
	}
	balInfo := FromGrpcBalInfo(req.OpeningBalance)
	payChInfo, err := payment.OpenPayCh(ctx, sess, req.PeerAlias, balInfo, req.ChallengeDurSecs)
	if err != nil {
		return errResponse(err), nil
	}

	return &pb.OpenPayChResp{
		Response: &pb.OpenPayChResp_MsgSuccess_{
			MsgSuccess: &pb.OpenPayChResp_MsgSuccess{
				Channel: &pb.PaymentChannel{
					ChannelID:   payChInfo.ChannelID,
					Balanceinfo: ToGrpcBalInfo(payChInfo.BalInfo),
					Version:     payChInfo.Version,
				},
			},
		},
	}, nil
}

// GetPayChs wraps session.GetPayChs.
func (a *PayChServer) GetPayChs(ctx context.Context, req *pb.GetPayChsReq) (*pb.GetPayChsResp, error) {
	errResponse := func(err error) *pb.GetPayChsResp {
		return &pb.GetPayChsResp{
			Response: &pb.GetPayChsResp_Error{
				Error: &pb.MsgError{Error: err.Error()},
			},
		}
	}

	sess, err := a.n.GetSession(req.SessionID)
	if err != nil {
		return errResponse(err), nil
	}
	payChInfos := payment.GetPayChs(sess)
	if err != nil {
		return errResponse(err), nil
	}
	payChInfosGrpc := make([]*pb.PaymentChannel, len(payChInfos))
	for i := 0; i < len(payChInfosGrpc); i++ {
		payChInfosGrpc[i] = &pb.PaymentChannel{
			ChannelID:   payChInfos[i].ChannelID,
			Balanceinfo: ToGrpcBalInfo(payChInfos[i].BalInfo),
			Version:     payChInfos[i].Version,
		}
	}

	return &pb.GetPayChsResp{
		Response: &pb.GetPayChsResp_MsgSuccess_{
			MsgSuccess: &pb.GetPayChsResp_MsgSuccess{
				OpenChannels: payChInfosGrpc,
			},
		},
	}, nil
}

// SubPayChProposals wraps session.SubPayChProposals.
func (a *PayChServer) SubPayChProposals(req *pb.SubPayChProposalsReq,
	srv pb.Payment_API_SubPayChProposalsServer) error {
	sess, err := a.n.GetSession(req.SessionID)
	if err != nil {
		// TODO: (mano) Return a error response and not a protocol error
		return errors.WithMessage(err, "cannot register subscription")
	}

	notifier := func(notif payment.PayChProposalNotif) {
		// nolint: govet	// err does not shadow prev declarations as this runs in a different context.
		err := srv.Send(&pb.SubPayChProposalsResp{Response: &pb.SubPayChProposalsResp_Notify_{
			Notify: &pb.SubPayChProposalsResp_Notify{
				ProposalID:       notif.ProposalID,
				OpeningBalance:   ToGrpcBalInfo(notif.OpeningBals),
				ChallengeDurSecs: notif.ChallengeDurSecs,
				Expiry:           notif.Expiry,
			},
		}})
		_ = err
		// if err != nil {
		// TODO: (mano) Handle error while sending.
		// }
	}
	err = payment.SubPayChProposals(sess, notifier)
	if err != nil {
		// TODO: (mano) Return a error response and not a protocol error
		return errors.WithMessage(err, "cannot register subscription")
	}

	signal := make(chan bool)
	a.Lock()
	a.chProposalsNotif[req.SessionID] = signal
	a.Unlock()

	<-signal
	return nil
}

// UnsubPayChProposals wraps session.UnsubPayChProposals.
func (a *PayChServer) UnsubPayChProposals(ctx context.Context, req *pb.UnsubPayChProposalsReq) (
	*pb.UnsubPayChProposalsResp, error) {
	errResponse := func(err error) *pb.UnsubPayChProposalsResp {
		return &pb.UnsubPayChProposalsResp{
			Response: &pb.UnsubPayChProposalsResp_Error{
				Error: &pb.MsgError{
					Error: err.Error(),
				},
			},
		}
	}

	sess, err := a.n.GetSession(req.SessionID)
	if err != nil {
		return errResponse(err), nil
	}
	err = payment.UnsubPayChProposals(sess)
	if err != nil {
		return errResponse(err), nil
	}

	a.Lock()
	signal := a.chProposalsNotif[req.SessionID]
	a.Unlock()
	close(signal)

	return &pb.UnsubPayChProposalsResp{
		Response: &pb.UnsubPayChProposalsResp_MsgSuccess_{
			MsgSuccess: &pb.UnsubPayChProposalsResp_MsgSuccess{
				Success: true,
			},
		},
	}, nil
}

// RespondPayChProposal wraps session.RespondPayChProposal.
func (a *PayChServer) RespondPayChProposal(ctx context.Context, req *pb.RespondPayChProposalReq) (
	*pb.RespondPayChProposalResp, error) {
	errResponse := func(err error) *pb.RespondPayChProposalResp {
		return &pb.RespondPayChProposalResp{
			Response: &pb.RespondPayChProposalResp_Error{
				Error: &pb.MsgError{
					Error: err.Error(),
				},
			},
		}
	}

	sess, err := a.n.GetSession(req.SessionID)
	if err != nil {
		return errResponse(err), nil
	}
	err = payment.RespondPayChProposal(ctx, sess, req.ProposalID, req.Accept)
	if err != nil {
		return errResponse(err), nil
	}

	return &pb.RespondPayChProposalResp{
		Response: &pb.RespondPayChProposalResp_MsgSuccess_{
			MsgSuccess: &pb.RespondPayChProposalResp_MsgSuccess{
				Success: true,
			},
		},
	}, nil
}

// SubPayChCloses wraps session.SubPayChCloses.
func (a *PayChServer) SubPayChCloses(req *pb.SubPayChClosesReq, srv pb.Payment_API_SubPayChClosesServer) error {
	sess, err := a.n.GetSession(req.SessionID)
	if err != nil {
		// TODO: (mano) Return a error response and not a protocol error
		return errors.WithMessage(err, "cannot register subscription")
	}

	notifier := func(notif payment.PayChCloseNotif) {
		// nolint: govet	// err does not shadow prev declarations as this runs in a different context.
		err := srv.Send(&pb.SubPayChClosesResp{Response: &pb.SubPayChClosesResp_Notify_{
			Notify: &pb.SubPayChClosesResp_Notify{
				ClosingState: &pb.PaymentChannel{
					ChannelID:   notif.ClosingState.ChannelID,
					Balanceinfo: ToGrpcBalInfo(notif.ClosingState.BalInfo),
					Version:     notif.ClosingState.Version,
				},
				Error: notif.Error,
			},
		}})
		_ = err
		// if err != nil {
		// TODO: (mano) Handle error while sending.
		// }
	}
	err = payment.SubPayChCloses(sess, notifier)
	if err != nil {
		// TODO: (mano) Return a error response and not a protocol error
		return errors.WithMessage(err, "cannot register subscription")
	}

	signal := make(chan bool)
	a.Lock()
	a.chClosesNotif[req.SessionID] = signal
	a.Unlock()

	<-signal
	return nil
}

// UnsubPayChClose wraps session.UnsubPayChClose.
func (a *PayChServer) UnsubPayChClose(ctx context.Context, req *pb.UnsubPayChClosesReq) (
	*pb.UnsubPayChClosesResp, error) {
	errResponse := func(err error) *pb.UnsubPayChClosesResp {
		return &pb.UnsubPayChClosesResp{
			Response: &pb.UnsubPayChClosesResp_Error{
				Error: &pb.MsgError{
					Error: err.Error(),
				},
			},
		}
	}
	sess, err := a.n.GetSession(req.SessionID)
	if err != nil {
		return errResponse(err), nil
	}
	err = payment.UnsubPayChCloses(sess)
	if err != nil {
		return errResponse(err), nil
	}

	a.Lock()
	signal := a.chClosesNotif[req.SessionID]
	a.Unlock()
	close(signal)

	return &pb.UnsubPayChClosesResp{
		Response: &pb.UnsubPayChClosesResp_MsgSuccess_{
			MsgSuccess: &pb.UnsubPayChClosesResp_MsgSuccess{
				Success: true,
			},
		},
	}, nil
}

// CloseSession wraps session.CloseSession. For now, this is a stub.
func (a *PayChServer) CloseSession(context.Context, *pb.CloseSessionReq) (*pb.CloseSessionResp, error) {
	return nil, nil
}

// SendPayChUpdate wraps channel.SendPayChUpdate.
func (a *PayChServer) SendPayChUpdate(ctx context.Context, req *pb.SendPayChUpdateReq) (
	*pb.SendPayChUpdateResp, error) {
	errResponse := func(err error) *pb.SendPayChUpdateResp {
		return &pb.SendPayChUpdateResp{
			Response: &pb.SendPayChUpdateResp_Error{
				Error: &pb.MsgError{
					Error: err.Error(),
				},
			},
		}
	}

	sess, err := a.n.GetSession(req.SessionID)
	if err != nil {
		return errResponse(err), nil
	}
	channel, err := sess.GetCh(req.ChannelID)
	if err != nil {
		return errResponse(err), nil
	}
	err = payment.SendPayChUpdate(ctx, channel, req.Payee, req.Amount)
	if err != nil {
		return errResponse(err), nil
	}

	return &pb.SendPayChUpdateResp{
		Response: &pb.SendPayChUpdateResp_MsgSuccess_{
			MsgSuccess: &pb.SendPayChUpdateResp_MsgSuccess{
				Success: true,
			},
		},
	}, nil
}

// SubPayChUpdates wraps channel.SubPayChUpdates.
func (a *PayChServer) SubPayChUpdates(req *pb.SubpayChUpdatesReq, srv pb.Payment_API_SubPayChUpdatesServer) error {
	sess, err := a.n.GetSession(req.SessionID)
	if err != nil {
		// TODO: (mano) Return a error response and not a protocol error.
		return errors.WithMessage(err, "cannot register subscription")
	}
	channel, err := sess.GetCh(req.ChannelID)
	if err != nil {
		return errors.WithMessage(err, "cannot register subscription")
	}

	notifier := func(notif payment.PayChUpdateNotif) {
		// nolint: govet	// err does not shadow prev declarations as this runs in a different context.
		err := srv.Send(&pb.SubPayChUpdatesResp{Response: &pb.SubPayChUpdatesResp_Notify_{
			Notify: &pb.SubPayChUpdatesResp_Notify{
				ProposedBalance: ToGrpcBalInfo(notif.ProposedBals),
				UpdateID:        notif.UpdateID,
				Final:           notif.Final,
				Expiry:          notif.Expiry,
			},
		}})
		_ = err
		// if err != nil {
		// 	// TODO: (mano) Error handling when sending notification.
		// }
	}
	err = payment.SubPayChUpdates(channel, notifier)
	if err != nil {
		// TODO: (mano) Error handling when sending notification.
		return errors.WithMessage(err, "cannot register subscription")
	}

	signal := make(chan bool)
	a.Lock()
	a.chUpdatesNotif[req.SessionID][req.ChannelID] = signal
	a.Unlock()

	<-signal
	return nil
}

// UnsubPayChUpdates wraps channel.UnsubPayChUpdates.
func (a *PayChServer) UnsubPayChUpdates(ctx context.Context, req *pb.UnsubPayChUpdatesReq) (
	*pb.UnsubPayChUpdatesResp, error) {
	errResponse := func(err error) *pb.UnsubPayChUpdatesResp {
		return &pb.UnsubPayChUpdatesResp{
			Response: &pb.UnsubPayChUpdatesResp_Error{
				Error: &pb.MsgError{
					Error: err.Error(),
				},
			},
		}
	}
	sess, err := a.n.GetSession(req.SessionID)
	if err != nil {
		return errResponse(err), nil
	}
	channel, err := sess.GetCh(req.ChannelID)
	if err != nil {
		return errResponse(err), nil
	}
	err = payment.UnsubPayChUpdates(channel)
	if err != nil {
		return errResponse(err), nil
	}

	a.Lock()
	signal := a.chUpdatesNotif[req.SessionID][req.ChannelID]
	a.Unlock()
	close(signal)

	return &pb.UnsubPayChUpdatesResp{
		Response: &pb.UnsubPayChUpdatesResp_MsgSuccess_{
			MsgSuccess: &pb.UnsubPayChUpdatesResp_MsgSuccess{
				Success: true,
			},
		},
	}, nil
}

// RespondPayChUpdate wraps channel.RespondPayChUpdate.
func (a *PayChServer) RespondPayChUpdate(ctx context.Context, req *pb.RespondPayChUpdateReq) (
	*pb.RespondPayChUpdateResp, error) {
	errResponse := func(err error) *pb.RespondPayChUpdateResp {
		return &pb.RespondPayChUpdateResp{
			Response: &pb.RespondPayChUpdateResp_Error{
				Error: &pb.MsgError{
					Error: err.Error(),
				},
			},
		}
	}

	sess, err := a.n.GetSession(req.SessionID)
	if err != nil {
		return errResponse(err), nil
	}
	channel, err := sess.GetCh(req.ChannelID)
	if err != nil {
		return errResponse(err), nil
	}
	err = payment.RespondPayChUpdate(ctx, channel, req.UpdateID, req.Accept)
	if err != nil {
		return errResponse(err), nil
	}

	return &pb.RespondPayChUpdateResp{
		Response: &pb.RespondPayChUpdateResp_MsgSuccess_{
			MsgSuccess: &pb.RespondPayChUpdateResp_MsgSuccess{
				Success: true,
			},
		},
	}, nil
}

// GetPayChBalance wraps channel.GetPayChBalance.
func (a *PayChServer) GetPayChBalance(ctx context.Context, req *pb.GetPayChBalanceReq) (
	*pb.GetPayChBalanceResp, error) {
	errResponse := func(err error) *pb.GetPayChBalanceResp {
		return &pb.GetPayChBalanceResp{
			Response: &pb.GetPayChBalanceResp_Error{
				Error: &pb.MsgError{
					Error: err.Error(),
				},
			},
		}
	}

	sess, err := a.n.GetSession(req.SessionID)
	if err != nil {
		return errResponse(err), nil
	}
	channel, err := sess.GetCh(req.ChannelID)
	if err != nil {
		return errResponse(err), nil
	}
	balInfo := payment.GetBalInfo(channel)
	if err != nil {
		return errResponse(err), nil
	}

	return &pb.GetPayChBalanceResp{
		Response: &pb.GetPayChBalanceResp_MsgSuccess_{
			MsgSuccess: &pb.GetPayChBalanceResp_MsgSuccess{
				CurrentBalance: ToGrpcBalInfo(balInfo),
				CurrentVersion: "",
			},
		},
	}, nil
}

// ClosePayCh wraps channel.ClosePayCh.
func (a *PayChServer) ClosePayCh(ctx context.Context, req *pb.ClosePayChReq) (*pb.ClosePayChResp, error) {
	errResponse := func(err error) *pb.ClosePayChResp {
		return &pb.ClosePayChResp{
			Response: &pb.ClosePayChResp_Error{
				Error: &pb.MsgError{
					Error: err.Error(),
				},
			},
		}
	}

	sess, err := a.n.GetSession(req.SessionID)
	if err != nil {
		return errResponse(err), nil
	}
	channel, err := sess.GetCh(req.ChannelID)
	if err != nil {
		return errResponse(err), nil
	}
	payChInfo, err := payment.ClosePayCh(ctx, channel)
	if err != nil {
		return errResponse(err), nil
	}

	return &pb.ClosePayChResp{
		Response: &pb.ClosePayChResp_MsgSuccess_{
			MsgSuccess: &pb.ClosePayChResp_MsgSuccess{
				ClosingBalance: ToGrpcBalInfo(payChInfo.BalInfo),
				ClosingVersion: payChInfo.Version,
			},
		},
	}, nil
}

// FromGrpcBalInfo is a helper function to convert BalInfo struct defined in grpc package
// to BalInfo struct defined in perun-node. It is exported for use in tests.
func FromGrpcBalInfo(src *pb.BalanceInfo) perun.BalInfo {
	balInfo := perun.BalInfo{
		Currency: src.Currency,
		Bals:     make(map[string]string, len(src.Balances)),
	}
	for _, aliasBalance := range src.Balances {
		for key, value := range aliasBalance.Value {
			balInfo.Bals[key] = value
		}
	}
	return balInfo
}

// ToGrpcBalInfo is a helper function to convert BalInfo struct defined in perun-node
// to BalInfo struct defined in grpc package. It is exported for use in tests.
func ToGrpcBalInfo(src perun.BalInfo) *pb.BalanceInfo {
	balInfo := &pb.BalanceInfo{
		Currency: src.Currency,
		Balances: make([]*pb.BalanceInfo_AliasBalance, len(src.Bals)),
	}
	i := 0
	for key, value := range src.Bals {
		balInfo.Balances[i] = &pb.BalanceInfo_AliasBalance{
			Value: make(map[string]string),
		}
		balInfo.Balances[i].Value[key] = value
		i++
	}
	return balInfo
}