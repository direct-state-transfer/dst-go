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

package client

import "github.com/hyperledger-labs/perun-node"

func NewClientForTest(pClient pClient, bus perun.WireBus, msgBusRegistry perun.Registerer) *client { // nolint: golint
	// it is okay to return an unexported type that satisfies an exported interface.
	return &client{
		pClient:        pClient,
		msgBus:         bus,
		msgBusRegistry: msgBusRegistry,
	}
}
