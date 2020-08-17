// Copyright 2018 Google Inc. All Rights Reserved.
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

// Package election provides master election tools, and interfaces for plugging
// in a custom underlying mechanism.
// TODO(pavelkalinnikov): Migrate this package to Trillian.
package election

import "context"

// Election controls an instance's participation in master election process.
// Note: Implementations are not intended to be thread-safe.
type Election interface {
	// Await blocks until the instance captures mastership. Returns immediately
	// if it is already the master. Returns an error if capturing fails, or the
	// passed in context is canceled before mastership is captured. If an error
	// is returned, the instance might still have become the master. Idempotent,
	// might be useful to retry in case of an error.
	Await(ctx context.Context) error

	// Observe returns a "mastership context" which remains active until the
	// instance stops being the master, or the passed in context is canceled. If
	// the instance is not the master during this call, returns an already
	// canceled context. In particular, this will happen if Observe is called
	// without a preceding Await.
	//
	// The resources used for maintaining the mastership context are released
	// when the latter gets canceled. This happens when the instance loses
	// mastership, calls Resign, an error occurs in mastership monitoring, or the
	// context passed in to Observe is explicitly canceled.
	Observe(ctx context.Context) (context.Context, error)

	// Resign releases mastership for this instance. The instance can be elected
	// again using Await. Idempotent, might be useful to retry if fails.
	//
	// Note: Resign does not guarantee immediate cancelation of the context
	// returned from Observe. However, the latter will happen *eventually* if
	// resigning is successful. The caller can force mastership context
	// cancelation by explicitly canceling the context passed in to Observe.
	//
	// The caller is advised to tear down mastership-related work before invoking
	// Resign to have best protection against double-master situations.
	Resign(ctx context.Context) error

	// Close permanently stops participating in election, and releases the
	// resources. It does best effort on resigning despite potential cancelation
	// of the passed in context, so that other instances can overtake mastership
	// faster. No other method should be called after Close.
	//
	// Note: Does not guarantee immediate mastership context cancelation, see
	// Resign comment for details.
	Close(ctx context.Context) error
}

// Factory encapsulates the creation of an Election instance for a treeID.
type Factory interface {
	NewElection(ctx context.Context, treeID int64) (Election, error)
}
