# Changelog

## [0.6.0](https://github.com/hyperledger-labs/perun-node/releases/tag/v0.6.0) – 2021-07-08

perun-node v0.6.0 is a regular development release.

Newly added:

  1. Implemented a new app with text based UI (TUI) for trying out
     perun-node.
  2. Validate contracts during node initialization.
  3. Added an example for directly using the payment API by importing
     `perun-node` module. See [app/payment/payment_test.go](app/payment/payment_test.go).
  4. Updated the payment API to allow use of ERC20 token as currency. 
     * A channel can be opened with balances in more than one
       currency.
     * Multiple payments involving different currencies can be made in a
       single API call.
     * Asset contracts for ERC20 tokens can be specified in node config
       file or registered via API.

Improvements:

  1. Improved error handling. The errors returned by API provide a rich
     context and necessary data in structured format for easy error
     handling.
  2. Use structured logging. Request and response (success or error) are
     logged in a structured format for each API call.
  3. Use weak (scrypt) encryption parameters for ethereum keystore
     wallets by default. This makes the `open session` action in tests and
     in CLI/TUI apps to be much faster.

For detailed information on changes covered in this release, see
milestone [12](https://github.com/hyperledger-labs/perun-node/milestone/12).

## [0.5.0](https://github.com/hyperledger-labs/perun-node/releases/tag/v0.5.0) – 2020-12-18

perun-node v0.5.0 is a regular development release.

Newly added:

  1. Support for payment requests. User can send (decrease own balance) or
     request (increase own balance) money when sending state channel updates.
  2. Support for running integration tests in Circle CI continuous integration
     pipeline.

Improvements:

  1. Added unit tests in session package. Previously only integration tests were
     implemented in this package.
  2. Channel logger is now derived from session logger. So each log entry for
     the channel also includes the fields registered on session logger.
     (currently it is only session-id).
  3. Re-organized definition of configuration parameters used in tests for
     easier maintenance and update.

For detailed information on changes covered in the this release, see
milestone [11](https://github.com/hyperledger-labs/perun-node/milestone/11).

## [0.4.0](https://github.com/hyperledger-labs/perun-node/releases/tag/v0.4.0) – 2020-10-19

perun-node v0.4.0 is the first regular development release after complete re-implementation done in v0.3.0.

Newly added:

  1. `perunnode` executable binary for running an instance of perun node.
  2. `perunnodecli` interactive application as reference client implementation
     for connecting to perun node.
  3. Implemented option to close a session.
  4. Support for persistence. Close a session with open channels, restore it
     later and transact on the restored channels.

Improvements:

  1. Updated go-perun dependency to include the following changes:
     * Channel nonce is created from randomness contributed from both
       participants of the channel.
     * Use of transactor interface abstraction for initializing contract
       backend.
  2. Updated payment channel API to use consistent data formats.
  3. Combined channel close subscription and channel update subscription into
     one.

For detailed information on changes covered in the this release, see
milestone [10](https://github.com/hyperledger-labs/perun-node/milestone/10).

## [0.3.0](https://github.com/hyperledger-labs/perun-node/releases/tag/v0.3.0) – 2020-08-26

perun-node v0.3.0 is a complete re-implementation of the previous version
(project name changed from dst-go to perun-node). This version uses the
[go-perun](https://github.com/hyperledger-labs/go-perun) SDK that implements a
state channel client based on perun protocol and builds other functionalities on
top of that.

The following features are included:

  1. Ethereum backend for blockchain.
  2. Key management using ethereum keystore.
  3. YAML file based contacts provider.
  4. Session (as an abstraction) for enabling multiple users use the same node
     with dedicated contacts provider and key manager.
  5. Two party payment channel API for the user over gRPC protocol.

For detailed information on changes covered in this release, see milestones
[3](https://github.com/hyperledger-labs/perun-node/milestone/3),
[4](https://github.com/hyperledger-labs/perun-node/milestone/4),
[5](https://github.com/hyperledger-labs/perun-node/milestone/5),
[6](https://github.com/hyperledger-labs/perun-node/milestone/6),
[7](https://github.com/hyperledger-labs/perun-node/milestone/7),
[8](https://github.com/hyperledger-labs/perun-node/milestone/8), and
[9](https://github.com/hyperledger-labs/perun-node/milestone/9).
