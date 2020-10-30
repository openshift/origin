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

package core

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/certificate-transparency-go/scanner"
	"github.com/google/trillian"
	"github.com/google/trillian/client"
	"github.com/google/trillian/crypto"
	"github.com/google/trillian/types"
)

// PreorderedLogClient is a means of communicating with a single Trillian
// pre-ordered log tree.
type PreorderedLogClient struct {
	cli    trillian.TrillianLogClient
	verif  *client.LogVerifier
	tree   *trillian.Tree
	prefix string // TODO(pavelkalinnikov): Get rid of this.
}

// NewPreorderedLogClient creates and initializes a pre-ordered log client.
func NewPreorderedLogClient(
	cli trillian.TrillianLogClient, tree *trillian.Tree, prefix string,
) (*PreorderedLogClient, error) {
	if tree == nil {
		return nil, errors.New("missing Tree")
	}
	if got, want := tree.TreeType, trillian.TreeType_PREORDERED_LOG; got != want {
		return nil, fmt.Errorf("tree %d is %v, want %v", tree.TreeId, got, want)
	}
	v, err := client.NewLogVerifierFromTree(tree)
	if err != nil {
		return nil, err
	}
	return &PreorderedLogClient{cli: cli, verif: v, tree: tree, prefix: prefix}, nil
}

// getVerifiedRoot returns the current root of the Trillian tree. Verifies the
// log's signature.
func (c *PreorderedLogClient) getVerifiedRoot(ctx context.Context) (*types.LogRootV1, error) {
	req := trillian.GetLatestSignedLogRootRequest{LogId: c.tree.TreeId}
	rsp, err := c.cli.GetLatestSignedLogRoot(ctx, &req)
	if err != nil {
		return nil, err
	} else if rsp == nil || rsp.SignedLogRoot == nil {
		return nil, errors.New("missing SignedLogRoot")
	}
	return crypto.VerifySignedLogRoot(c.verif.PubKey, c.verif.SigHash, rsp.SignedLogRoot)
}

// addSequencedLeaves converts a batch of CT log entries into Trillian log
// leaves and submits them to Trillian via AddSequencedLeaves API.
func (c *PreorderedLogClient) addSequencedLeaves(ctx context.Context, b *scanner.EntryBatch) error {
	// TODO(pavelkalinnikov): Verify range inclusion against the remote STH.
	leaves := make([]*trillian.LogLeaf, len(b.Entries))
	for i, e := range b.Entries {
		var err error
		if leaves[i], err = buildLogLeaf(c.prefix, b.Start+int64(i), &e); err != nil {
			return err
		}
	}

	req := trillian.AddSequencedLeavesRequest{LogId: c.tree.TreeId, Leaves: leaves}
	rsp, err := c.cli.AddSequencedLeaves(ctx, &req)
	if err != nil {
		return fmt.Errorf("AddSequencedLeaves(): %v", err)
	} else if rsp == nil {
		return errors.New("missing AddSequencedLeaves response")
	}
	// TODO(pavelkalinnikov): Check rsp.Results statuses.
	return nil
}
