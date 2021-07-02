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

package wire

import (
	"io"
	"time"

	perunio "perun.network/go-perun/pkg/io"
)

func init() {
	RegisterDecoder(Ping, func(r io.Reader) (Msg, error) { var m PingMsg; return &m, m.Decode(r) })
	RegisterDecoder(Pong, func(r io.Reader) (Msg, error) { var m PongMsg; return &m, m.Decode(r) })
	RegisterDecoder(Shutdown, func(r io.Reader) (Msg, error) { var m ShutdownMsg; return &m, m.Decode(r) })
}

// Since ping and pong messages are essentially the same, this is a common
// implementation for both.
type pingPongMsg struct {
	Created time.Time
}

func (m pingPongMsg) Encode(writer io.Writer) error {
	return perunio.Encode(writer, m.Created)
}

func (m *pingPongMsg) Decode(reader io.Reader) error {
	return perunio.Decode(reader, &m.Created)
}

func newPingPongMsg() pingPongMsg {
	// do not use `time.Now()` directly because it contains monotonic clock
	// data specific to the current process which breaks, e.g.,
	// `reflect.DeepEqual`, cf. "Marshal/Unmarshal functions are asymmetrical"
	// https://github.com/golang/go/issues/19502
	return pingPongMsg{Created: time.Unix(0, time.Now().UnixNano())}
}

// PingMsg is a ping request.
// It contains the time at which it was sent, so that the recipient can also
// measure the time it took to transmit the ping request.
type PingMsg struct {
	pingPongMsg
}

// Type returns Ping.
func (m *PingMsg) Type() Type {
	return Ping
}

// NewPingMsg creates a new Ping message.
func NewPingMsg() *PingMsg {
	return &PingMsg{newPingPongMsg()}
}

// PongMsg is the response to a ping message.
// It contains the time at which it was sent, so that the recipient knows how
// long the ping request took to be transmitted, and how quickly the response
// was sent.
type PongMsg struct {
	pingPongMsg
}

// Type returns Pong.
func (m *PongMsg) Type() Type {
	return Pong
}

// NewPongMsg creates a new Pong message.
func NewPongMsg() *PongMsg {
	return &PongMsg{newPingPongMsg()}
}

// ShutdownMsg is sent when orderly shutting down a connection.
type ShutdownMsg struct {
	Reason string
}

// Encode implements msg.Encode.
func (m *ShutdownMsg) Encode(w io.Writer) error {
	return perunio.Encode(w, m.Reason)
}

// Decode implements msg.Decode.
func (m *ShutdownMsg) Decode(r io.Reader) error {
	return perunio.Decode(r, &m.Reason)
}

// Type implements msg.Type.
func (m *ShutdownMsg) Type() Type {
	return Shutdown
}
