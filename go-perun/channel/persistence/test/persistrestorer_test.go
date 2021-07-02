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

package test_test

import (
	"context"
	"testing"

	_ "perun.network/go-perun/backend/sim" // backend init
	"perun.network/go-perun/channel/persistence/test"
	pkgtest "perun.network/go-perun/pkg/test"
)

func TestPersistRestorer_Generic(t *testing.T) {
	pr := test.NewPersistRestorer(t)
	test.GenericPersistRestorerTest(
		context.Background(),
		t,
		pkgtest.Prng(t),
		pr,
		8,
		8,
	)
}
