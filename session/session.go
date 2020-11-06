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

package session

import (
	"context"
	"crypto/sha256"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/pkg/errors"
	pchannel "perun.network/go-perun/channel"
	pclient "perun.network/go-perun/client"
	psync "perun.network/go-perun/pkg/sync"
	pwallet "perun.network/go-perun/wallet"

	"github.com/hyperledger-labs/perun-node"
	"github.com/hyperledger-labs/perun-node/blockchain/ethereum"
	"github.com/hyperledger-labs/perun-node/client"
	"github.com/hyperledger-labs/perun-node/comm/tcp"
	"github.com/hyperledger-labs/perun-node/currency"
	"github.com/hyperledger-labs/perun-node/idprovider/local"
	"github.com/hyperledger-labs/perun-node/log"
)

// walletBackend for initializing user wallets and parsing off-chain addresses
// in incoming idProvider. A package level unexported variable is used so that a
// test wallet backend can be set using a function defined in export_test.go.
// Because real backend have large unlocking times and hence tests take very long.
var walletBackend perun.WalletBackend

func init() {
	// This can be overridden (only) in tests by calling the SetWalletBackend function.
	walletBackend = ethereum.NewWalletBackend()
}

type (
	session struct {
		log.Logger
		psync.Mutex

		id         string
		isOpen     bool
		timeoutCfg timeoutConfig
		user       perun.User
		chAsset    pchannel.Asset
		chClient   perun.ChClient
		idProvider perun.IDProvider

		chs map[string]*channel

		chProposalNotifier    perun.ChProposalNotifier
		chProposalNotifsCache []perun.ChProposalNotif
		chProposalResponders  map[string]chProposalResponderEntry
	}

	chProposalResponderEntry struct {
		proposal  pclient.ChannelProposal
		notif     perun.ChProposalNotif
		responder chProposalResponder
	}

	//go:generate mockery --name chProposalResponder --output ../internal/mocks

	// Proposal Responder defines the methods on proposal responder that will be used by the perun node.
	chProposalResponder interface {
		Accept(context.Context, *pclient.ChannelProposalAcc) (*pclient.Channel, error)
		Reject(ctx context.Context, reason string) error
	}
)

// New initializes a SessionAPI instance for the given configuration and returns an
// instance of it. All methods on it are safe for concurrent use.
func New(cfg Config) (*session, error) {
	user, err := NewUnlockedUser(walletBackend, cfg.User)
	if err != nil {
		return nil, err
	}

	if cfg.User.CommType != "tcp" {
		return nil, perun.ErrUnsupportedCommType
	}
	commBackend := tcp.NewTCPBackend(30 * time.Second)

	chAsset, err := walletBackend.ParseAddr(cfg.Asset)
	if err != nil {
		return nil, err
	}

	idProvider, err := initIDProvider(cfg.IDProviderType, cfg.IDProviderURL, walletBackend, user.Peer)
	if err != nil {
		return nil, err
	}

	chClientCfg := client.Config{
		Chain: client.ChainConfig{
			Adjudicator:      cfg.Adjudicator,
			Asset:            cfg.Asset,
			URL:              cfg.ChainURL,
			ConnTimeout:      cfg.ChainConnTimeout,
			OnChainTxTimeout: cfg.OnChainTxTimeout,
		},
		DatabaseDir:       cfg.DatabaseDir,
		PeerReconnTimeout: cfg.PeerReconnTimeout,
	}
	chClient, err := client.NewEthereumPaymentClient(chClientCfg, user, commBackend)
	if err != nil {
		return nil, err
	}

	sessionID := calcSessionID(user.OffChainAddr.Bytes())
	timeoutCfg := timeoutConfig{
		onChainTx: cfg.OnChainTxTimeout,
		response:  cfg.ResponseTimeout,
	}
	sess := &session{
		Logger:               log.NewLoggerWithField("session-id", sessionID),
		id:                   sessionID,
		isOpen:               true,
		timeoutCfg:           timeoutCfg,
		user:                 user,
		chAsset:              chAsset,
		chClient:             chClient,
		idProvider:           idProvider,
		chs:                  make(map[string]*channel),
		chProposalResponders: make(map[string]chProposalResponderEntry),
	}
	err = sess.chClient.RestoreChs(sess.handleRestoredCh)
	if err != nil {
		return nil, errors.WithMessage(err, "restoring channels")
	}
	chClient.Handle(sess, sess) // Init handlers
	return sess, nil
}

func initIDProvider(idProviderType, idProviderURL string, wb perun.WalletBackend, own perun.Peer) (
	perun.IDProvider, error) {
	if idProviderType != "yaml" {
		return nil, perun.ErrUnsupportedIDProviderType
	}
	idProvider, err := local.New(idProviderURL, wb)
	if err != nil {
		return nil, err
	}

	own.Alias = perun.OwnAlias
	err = idProvider.Write(perun.OwnAlias, own)
	if err != nil && !errors.Is(err, perun.ErrPeerExists) {
		return nil, errors.Wrap(err, "registering own user in idProvider")
	}
	return idProvider, nil
}

// calcSessionID calculates the sessionID as sha256 hash over the off-chain address of the user and
// the current UTC time.
//
// A time dependant parameter is required to ensure the same user is able to open multiple sessions
// with the same node and have unique session id for each.
func calcSessionID(userOffChainAddr []byte) string {
	h := sha256.New()
	_, _ = h.Write(userOffChainAddr)                  // nolint:errcheck		// this func does not err
	_, _ = h.Write([]byte(time.Now().UTC().String())) // nolint:errcheck		// this func does not err
	return fmt.Sprintf("%x", h.Sum(nil))
}

func (s *session) ID() string {
	return s.id
}

func (s *session) handleRestoredCh(pch *pclient.Channel) {
	s.Debugf("found channel in persistence: 0x%x", pch.ID())

	// Restore only those channels that are in acting phase.
	if pch.Phase() != pchannel.Acting {
		return
	}
	peers := pch.Peers()
	parts := make([]perun.Peer, len(peers))
	aliases := make([]string, len(peers))
	for i := range pch.Peers() {
		p, ok := s.idProvider.ReadByOffChainAddr(peers[i])
		if !ok {
			s.Info("Unknown peer address in a persisted channel, will not be restored", pch.Peers()[i].String())
			return
		}
		parts[i] = p
		aliases[i] = p.Alias
	}

	registerParts(parts, s.chClient)

	ch := newCh(pch, currency.ETH, aliases, s.timeoutCfg, pch.Params().ChallengeDuration)
	s.addCh(ch)
	s.Debugf("restored channel from persistence: %v", ch.getChInfo())
}

func (s *session) AddContact(peer perun.Peer) error {
	s.Debugf("Received request: session.AddContact. Params %+v", peer)
	s.Lock()
	defer s.Unlock()

	if !s.isOpen {
		return perun.ErrSessionClosed
	}

	err := s.idProvider.Write(peer.Alias, peer)
	if err != nil {
		s.Error(err)
	}
	return perun.GetAPIError(err)
}

func (s *session) GetContact(alias string) (perun.Peer, error) {
	s.Debugf("Received request: session.GetContact. Params %+v", alias)
	s.Lock()
	defer s.Unlock()

	if !s.isOpen {
		return perun.Peer{}, perun.ErrSessionClosed
	}

	peer, isPresent := s.idProvider.ReadByAlias(alias)
	if !isPresent {
		s.Error(perun.ErrUnknownAlias)
		return perun.Peer{}, perun.ErrUnknownAlias
	}
	return peer, nil
}

func (s *session) OpenCh(pctx context.Context, openingBalInfo perun.BalInfo, app perun.App, challengeDurSecs uint64) (
	perun.ChInfo, error) {
	s.Debugf("\nReceived request:session.OpenCh Params %+v,%+v,%+v", openingBalInfo, app, challengeDurSecs)
	// Session is locked only when adding the channel to session.

	if !s.isOpen {
		return perun.ChInfo{}, perun.ErrSessionClosed
	}

	sanitizeBalInfo(openingBalInfo)
	parts, err := retrieveParts(openingBalInfo.Parts, s.idProvider)
	if err != nil {
		s.Error(err, "retrieving channel participants using session idProvider")
		return perun.ChInfo{}, perun.GetAPIError(err)
	}
	registerParts(parts, s.chClient)

	allocations, err := makeAllocation(openingBalInfo, s.chAsset)
	if err != nil {
		s.Error(err, "making allocations")
		return perun.ChInfo{}, perun.GetAPIError(err)
	}

	proposal := pclient.NewLedgerChannelProposal(
		challengeDurSecs,
		s.user.OffChainAddr,
		allocations,
		makeOffChainAddrs(parts),
		pclient.WithApp(app.Def, app.Data),
		pclient.WithRandomNonce())
	ctx, cancel := context.WithTimeout(pctx, s.timeoutCfg.proposeCh(challengeDurSecs))
	defer cancel()
	pch, err := s.chClient.ProposeChannel(ctx, proposal)
	if err != nil {
		s.Error(err)
		// TODO: (mano) Use errors.Is here once a sentinel error value is defined in the SDK.
		if strings.Contains(err.Error(), "channel proposal rejected") {
			err = perun.ErrPeerRejected
		}
		return perun.ChInfo{}, perun.GetAPIError(err)
	}

	ch := newCh(pch, openingBalInfo.Currency, openingBalInfo.Parts, s.timeoutCfg, challengeDurSecs)

	s.addCh(ch)
	return ch.GetChInfo(), nil
}

// sanitizeBalInfo checks if the entry for ownAlias is at index 0,
// if not it rearranges the Aliases & Balance lists to make the index of ownAlias 0.
//
// BalanceInfo will be unchanged if there is no entry for ownAlias.
func sanitizeBalInfo(balInfo perun.BalInfo) {
	ownIdx := 0
	for idx := range balInfo.Parts {
		if balInfo.Parts[idx] == perun.OwnAlias {
			ownIdx = idx
		}
	}
	// Rearrange when ownAlias is not index 0.
	if ownIdx != 0 {
		balInfo.Parts[ownIdx] = balInfo.Parts[0]
		balInfo.Parts[0] = perun.OwnAlias

		ownAmount := balInfo.Bal[ownIdx]
		balInfo.Bal[ownIdx] = balInfo.Bal[0]
		balInfo.Bal[0] = ownAmount
	}
}

// retrieveParts retrieves the peers from corresponding to the aliases from the idprovider.
// The order of entries for parts list will be same as that of aliases. i.e aliases[i] = parts[i].Alias.
func retrieveParts(aliases []string, idProvider perun.IDProviderReader) ([]perun.Peer, error) {
	knownParts := make(map[string]perun.Peer, len(aliases))
	parts := make([]perun.Peer, len(aliases))
	missingParts := make([]string, 0, len(aliases))
	repeatedParts := make([]string, 0, len(aliases))
	foundOwnAlias := false
	for idx, alias := range aliases {
		if alias == perun.OwnAlias {
			foundOwnAlias = true
		}
		peer, isPresent := idProvider.ReadByAlias(alias)
		if !isPresent {
			missingParts = append(missingParts, alias)
			continue
		}
		if _, isPresent := knownParts[alias]; isPresent {
			repeatedParts = append(repeatedParts, alias)
		}
		knownParts[alias] = peer
		parts[idx] = peer
	}

	if len(missingParts) != 0 {
		return nil, errors.New(fmt.Sprintf("No peers found in idProvider for the following alias(es): %v", knownParts))
	}
	if len(repeatedParts) != 0 {
		return nil, errors.New(fmt.Sprintf("Repeated entries in aliases: %v", knownParts))
	}
	if !foundOwnAlias {
		return nil, errors.New("No entry for self found in aliases")
	}

	return parts, nil
}

// registerParts will register the given parts to the passed registry.
func registerParts(parts []perun.Peer, r perun.Registerer) {
	for idx := range parts {
		if parts[idx].Alias != perun.OwnAlias { // Skip own alias.
			r.Register(parts[idx].OffChainAddr, parts[idx].CommAddr)
		}
	}
}

// makeOffChainAddrs returns the list of off-chain addresses corresponding to the given list of peers.
func makeOffChainAddrs(parts []perun.Peer) []pwallet.Address {
	addrs := make([]pwallet.Address, len(parts))
	for i := range parts {
		addrs[i] = parts[i].OffChainAddr
	}
	return addrs
}

// makeAllocation makes an allocation using the BalanceInfo and the chAsset.
// Order of amounts in the balance is same as the order of Aliases in the Balance Info.
// It errors if any of the amounts cannot be parsed using the interpreter corresponding to the currency.
func makeAllocation(balInfo perun.BalInfo, chAsset pchannel.Asset) (*pchannel.Allocation, error) {
	if !currency.IsSupported(balInfo.Currency) {
		return nil, perun.ErrUnsupportedCurrency
	}

	balance := make([]*big.Int, len(balInfo.Bal))
	var err error
	for i := range balInfo.Bal {
		balance[i], err = currency.NewParser(balInfo.Currency).Parse(balInfo.Bal[i])
		if err != nil {
			return nil, errors.WithMessagef(err, "Parsing amount: %v", balInfo.Bal[i])
		}
	}

	return &pchannel.Allocation{
		Assets:   []pchannel.Asset{chAsset},
		Balances: [][]*big.Int{balance},
	}, nil
}

// addCh adds the channel to session. It locks the session mutex during the operation.
func (s *session) addCh(ch *channel) {
	ch.Logger = log.NewLoggerWithField("channel-id", ch.id)
	s.Lock()
	// TODO: (mano) use logger with multiple fields and use session-id, channel-id.
	s.chs[ch.id] = ch
	s.Unlock()
}

func (s *session) HandleProposal(chProposal pclient.ChannelProposal, responder *pclient.ProposalResponder) {
	s.Debugf("SDK Callback: HandleProposal. Params: %+v", chProposal)
	expiry := time.Now().UTC().Add(s.timeoutCfg.response).Unix()

	if !s.isOpen {
		// Code will not reach here during runtime as chClient is closed when closing a session.
		s.Error("Unexpected HandleProposal callback invoked on a closed session")
		return
	}

	parts := make([]string, len(chProposal.Proposal().PeerAddrs))
	for i := range chProposal.Proposal().PeerAddrs {
		p, ok := s.idProvider.ReadByOffChainAddr(chProposal.Proposal().PeerAddrs[i])
		if !ok {
			s.Info("Received channel proposal from unknonwn peer", chProposal.Proposal().PeerAddrs[i].String())
			// nolint: errcheck, gosec		// It is sufficient to just log this error.
			s.rejectChProposal(context.Background(), responder, "peer not found in session idProvider")
			expiry = 0
			break
		}
		parts[i] = p.Alias
	}

	notif := chProposalNotif(parts, currency.ETH, chProposal.Proposal(), expiry)
	entry := chProposalResponderEntry{
		proposal:  chProposal,
		notif:     notif,
		responder: responder,
	}

	s.Lock()
	defer s.Unlock()
	// Need not store entries for notification with expiry = 0, as these update requests have
	// already been rejected by the perun node. Hence no response is expected for these notifications.
	if expiry != 0 {
		s.chProposalResponders[notif.ProposalID] = entry
	}

	// Set ETH as the currency interpreter for incoming channel.
	// TODO: (mano) Provide an option for user to configure when more currency interpretters are supported.
	if s.chProposalNotifier == nil {
		s.chProposalNotifsCache = append(s.chProposalNotifsCache, notif)
		s.Debugf("HandleProposal: Notification cached", notif)
	} else {
		go s.chProposalNotifier(notif)
		s.Debugf("HandleProposal: Notification sent", notif)
	}
}

func chProposalNotif(parts []string, curr string, chProposal *pclient.BaseChannelProposal,
	expiry int64) perun.ChProposalNotif {
	return perun.ChProposalNotif{
		ProposalID:       fmt.Sprintf("%x", chProposal.ProposalID()),
		OpeningBalInfo:   makeBalInfoFromRawBal(parts, curr, chProposal.InitBals.Balances[0]),
		App:              makeApp(chProposal.Proposal().App, chProposal.InitData),
		ChallengeDurSecs: chProposal.ChallengeDuration,
		Expiry:           expiry,
	}
}

func (s *session) SubChProposals(notifier perun.ChProposalNotifier) error {
	s.Debug("Received request: session.SubChProposals")
	s.Lock()
	defer s.Unlock()

	if !s.isOpen {
		return perun.ErrSessionClosed
	}

	if s.chProposalNotifier != nil {
		return perun.ErrSubAlreadyExists
	}
	s.chProposalNotifier = notifier

	// Send all cached notifications.
	for i := len(s.chProposalNotifsCache); i > 0; i-- {
		go s.chProposalNotifier(s.chProposalNotifsCache[0])
		s.chProposalNotifsCache = s.chProposalNotifsCache[1:i]
	}
	return nil
}

func (s *session) UnsubChProposals() error {
	s.Debug("Received request: session.UnsubChProposals")
	s.Lock()
	defer s.Unlock()

	if !s.isOpen {
		return perun.ErrSessionClosed
	}

	if s.chProposalNotifier == nil {
		return perun.ErrNoActiveSub
	}
	s.chProposalNotifier = nil
	return nil
}

func (s *session) RespondChProposal(pctx context.Context, chProposalID string, accept bool) (perun.ChInfo, error) {
	s.Debugf("Received request: session.RespondChProposal. Params: %+v, %+v", chProposalID, accept)

	if !s.isOpen {
		return perun.ChInfo{}, perun.ErrSessionClosed
	}

	// Lock the session mutex only when retrieving the channel responder and deleting it.
	// It will again be locked when adding the channel to the session.
	s.Lock()
	entry, ok := s.chProposalResponders[chProposalID]
	if !ok {
		s.Info(perun.ErrUnknownProposalID)
		s.Unlock()
		return perun.ChInfo{}, perun.ErrUnknownProposalID
	}
	delete(s.chProposalResponders, chProposalID)
	s.Unlock()

	currTime := time.Now().UTC().Unix()
	if entry.notif.Expiry < currTime {
		s.Info("timeout:", entry.notif.Expiry, "received response at:", currTime)
		return perun.ChInfo{}, perun.ErrRespTimeoutExpired
	}

	var openedChInfo perun.ChInfo
	var err error
	switch accept {
	case true:
		openedChInfo, err = s.acceptChProposal(pctx, entry)
	case false:
		err = s.rejectChProposal(pctx, entry.responder, "rejected by user")
	}
	return openedChInfo, perun.GetAPIError(err)
}

func (s *session) acceptChProposal(pctx context.Context, entry chProposalResponderEntry) (perun.ChInfo, error) {
	ctx, cancel := context.WithTimeout(pctx, s.timeoutCfg.respChProposalAccept(entry.notif.ChallengeDurSecs))
	defer cancel()

	proposal := entry.proposal.Proposal()
	resp := proposal.NewChannelProposalAcc(s.user.OffChainAddr, pclient.WithRandomNonce())
	pch, err := entry.responder.Accept(ctx, resp)
	if err != nil {
		s.Error("Accepting channel proposal", err)
		return perun.ChInfo{}, err
	}

	// Set ETH as the currency interpreter for incoming channel.
	// TODO: (mano) Provide an option for user to configure when more currency interpreters are supported.
	ch := newCh(pch, currency.ETH, entry.notif.OpeningBalInfo.Parts, s.timeoutCfg, entry.notif.ChallengeDurSecs)
	s.addCh(ch)
	return ch.getChInfo(), nil
}

func (s *session) rejectChProposal(pctx context.Context, responder chProposalResponder, reason string) error {
	ctx, cancel := context.WithTimeout(pctx, s.timeoutCfg.respChProposalReject())
	defer cancel()
	err := responder.Reject(ctx, reason)
	if err != nil {
		s.Error("Rejecting channel proposal", err)
	}
	return err
}

func (s *session) GetChsInfo() []perun.ChInfo {
	s.Debug("Received request: session.GetChInfos")
	s.Lock()
	defer s.Unlock()

	openChsInfo := make([]perun.ChInfo, len(s.chs))
	i := 0
	for _, ch := range s.chs {
		openChsInfo[i] = ch.GetChInfo()
		i++
	}
	return openChsInfo
}

func (s *session) GetCh(chID string) (perun.ChAPI, error) {
	s.Debugf("Internal call to get channel instance. Params: %+v", chID)

	s.Lock()
	ch, ok := s.chs[chID]
	s.Unlock()
	if !ok {
		s.Info(perun.ErrUnknownChID, "not found in session")
		return nil, perun.ErrUnknownChID
	}

	return ch, nil
}

func (s *session) HandleUpdate(chUpdate pclient.ChannelUpdate, responder *pclient.UpdateResponder) {
	s.Debugf("SDK Callback: HandleUpdate. Params: %+v", chUpdate)
	s.Lock()
	defer s.Unlock()

	if !s.isOpen {
		// Code will not reach here during runtime as chClient is closed when closing a session.
		s.Error("Unexpected HandleUpdate callback invoked on a closed session")
		return
	}

	chID := fmt.Sprintf("%x", chUpdate.State.ID)
	ch, ok := s.chs[chID]
	if !ok {
		s.Info("Received update for unknown channel", chID)
		err := responder.Reject(context.Background(), "unknown channel for this session")
		s.Info("Error rejecting incoming update for unknown channel with id %s: %v", chID, err)
		return
	}
	go ch.HandleUpdate(chUpdate, responder)
}

func (s *session) Close(force bool) ([]perun.ChInfo, error) {
	s.Debug("Received request: session.Close")
	s.Lock()
	defer s.Unlock()

	if !s.isOpen {
		return nil, perun.ErrSessionClosed
	}

	openChsInfo := []perun.ChInfo{}
	unexpectedPhaseChIDs := []string{}

	for _, ch := range s.chs {
		// Acquire channel mutex to ensure any ongoing operation on the channel is finished.
		ch.Lock()

		// Calling Phase() also waits for the mutex on pchannel that ensures any handling of Registered event
		// in the Watch routine is also completed. But if the event was received after acquiring channel mutex
		// and completed before pc.Phase() returned, this event will not yet be serviced by perun-node.
		// A solution to this is to add a provision (that is currenlt missing) to suspend the Watcher (only
		// for open channels) before acquiring channel mutex and restoring it later if force option is false.
		//
		// TODO (mano): Add a provision in go-perun to suspend the watcher and it use it here.
		//
		// Since there will be no ongoing operations in perun-node, the pchannel is should be in one of the two
		// stable phases knonwn to perun node (see state diagram in the docs for details) : Acting or Withdrawn.
		phase := ch.pch.Phase()
		if phase != pchannel.Acting && phase != pchannel.Withdrawn {
			unexpectedPhaseChIDs = append(unexpectedPhaseChIDs, ch.ID())
		}
		if ch.status == open {
			openChsInfo = append(openChsInfo, ch.getChInfo())
		}
	}
	if len(unexpectedPhaseChIDs) != 0 {
		err := fmt.Errorf("chs in unexpected phase during session close: %v", unexpectedPhaseChIDs)
		s.Error(err.Error())
		s.unlockAllChs()
		return nil, perun.GetAPIError(errors.WithStack(err))
	}
	if !force && len(openChsInfo) != 0 {
		err := fmt.Errorf("%w: %v", perun.ErrOpenCh, openChsInfo)
		s.Error(err.Error())
		s.unlockAllChs()
		return openChsInfo, perun.GetAPIError(errors.WithStack(err))
	}

	s.isOpen = false
	return openChsInfo, s.close()
}

func (s *session) unlockAllChs() {
	for _, ch := range s.chs {
		ch.Unlock()
	}
}

func (s *session) close() error {
	s.user.OnChain.Wallet.LockAll()
	s.user.OffChain.Wallet.LockAll()
	return errors.WithMessage(s.chClient.Close(), "closing session")
}
