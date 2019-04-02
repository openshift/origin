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

// Package core provides transport-agnostic implementation of Migrillian tool.
package core

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"sync"

	"github.com/golang/glog"

	ct "github.com/google/certificate-transparency-go"
	"github.com/google/certificate-transparency-go/client"
	"github.com/google/certificate-transparency-go/scanner"
	"github.com/google/certificate-transparency-go/trillian/migrillian/election"

	"github.com/google/trillian/merkle"
	_ "github.com/google/trillian/merkle/rfc6962" // Register hasher.
	"github.com/google/trillian/monitoring"
	"github.com/google/trillian/types"
)

var (
	metrics     treeMetrics
	metricsOnce sync.Once
)

// treeMetrics holds metrics keyed by Tree ID.
type treeMetrics struct {
	masterRuns     monitoring.Counter
	masterCancels  monitoring.Counter
	isMaster       monitoring.Gauge
	entriesFetched monitoring.Counter
	entriesSeen    monitoring.Counter
	entriesStored  monitoring.Counter
	// TODO(pavelkalinnikov): Add latency histograms, latest STH, tree size, etc.
}

// initMetrics creates metrics using the factory, if not yet created.
func initMetrics(mf monitoring.MetricFactory) {
	const treeID = "tree_id"
	metricsOnce.Do(func() {
		metrics = treeMetrics{
			masterRuns:     mf.NewCounter("master_runs", "Number of mastership runs.", treeID),
			masterCancels:  mf.NewCounter("master_cancels", "Number of unexpected mastership cancelations.", treeID),
			isMaster:       mf.NewGauge("is_master", "The instance is currently the master.", treeID),
			entriesFetched: mf.NewCounter("entries_fetched", "Entries fetched from the source log.", treeID),
			entriesSeen:    mf.NewCounter("entries_seen", "Entries seen by the submitters.", treeID),
			entriesStored:  mf.NewCounter("entries_stored", "Entries successfully submitted to Trillian.", treeID),
		}
	})
}

// Options holds configuration for a Controller.
type Options struct {
	scanner.FetcherOptions
	Submitters          int
	BatchesPerSubmitter int
}

// Controller coordinates migration from a CT log to a Trillian tree.
//
// TODO(pavelkalinnikov):
// - Schedule a distributed fetch to increase throughput.
// - Store CT STHs in Trillian or make this tool stateful on its own.
// - Make fetching stateful to reduce master resigning aftermath.
type Controller struct {
	opts     Options
	batches  chan scanner.EntryBatch
	ctClient *client.LogClient
	plClient *PreorderedLogClient
	ef       election.Factory
	label    string
}

// NewController creates a Controller configured by the passed in options, CT
// and Trillian clients, and a master election factory.
//
// The passed in MetricFactory is used to create per-tree metrics, and it
// should be the same for all instances. However, it is used only once.
func NewController(
	opts Options,
	ctClient *client.LogClient,
	plClient *PreorderedLogClient,
	ef election.Factory,
	mf monitoring.MetricFactory,
) *Controller {
	initMetrics(mf)
	l := strconv.FormatInt(plClient.tree.TreeId, 10)
	return &Controller{opts: opts, ctClient: ctClient, plClient: plClient, ef: ef, label: l}
}

// RunWhenMaster is a master-elected version of Run method. It executes Run
// whenever this instance captures mastership of the tree ID. As soon as the
// instance stops being the master, Run is canceled. The method returns if a
// severe error occurs, the passed in context is canceled, or fetching is
// completed (in non-Continuous mode). Releases mastership when terminates.
func (c *Controller) RunWhenMaster(ctx context.Context) error {
	treeID := c.plClient.tree.TreeId

	el, err := c.ef.NewElection(ctx, treeID)
	if err != nil {
		return err
	}
	defer func(ctx context.Context) {
		metrics.isMaster.Set(0, c.label)
		if err := el.Close(ctx); err != nil {
			glog.Warningf("%d: Election.Close(): %v", treeID, err)
		}
	}(ctx)

	for {
		if err := el.Await(ctx); err != nil {
			return err
		}
		metrics.isMaster.Set(1, c.label)

		mctx, err := el.Observe(ctx)
		if err != nil {
			return err
		} else if err := mctx.Err(); err != nil {
			return err
		}

		glog.Infof("%d: running as master", treeID)
		metrics.masterRuns.Inc(c.label)

		// Run while still master (or until an error).
		err = c.Run(mctx)
		if ctx.Err() != nil {
			// We have been externally canceled, so return the current error (which
			// could be nil or a cancelation-related error).
			return err
		} else if mctx.Err() == nil {
			// We are still the master, so emit the real error.
			return err
		}

		// Otherwise the mastership has been canceled, retry.
		metrics.isMaster.Set(0, c.label)
		metrics.masterCancels.Inc(c.label)
	}
}

// Run transfers CT log entries obtained via the CT log client to a Trillian
// pre-ordered log via Trillian client. If Options.Continuous is true then the
// migration process runs continuously trying to keep up with the target CT
// log. Returns if an error occurs, the context is canceled, or all the entries
// have been transferred (in non-Continuous mode).
func (c *Controller) Run(ctx context.Context) error {
	treeID := c.plClient.tree.TreeId

	root, err := c.plClient.getVerifiedRoot(ctx)
	if err != nil {
		return err
	}
	if c.opts.Continuous { // Ignore range parameters in Continuous mode.
		// TODO(pavelkalinnikov): Restore fetching state from storage in a better
		// way than "take the current tree size".
		c.opts.StartIndex, c.opts.EndIndex = int64(root.TreeSize), 0
		glog.Warningf("%d: updated entry range to [%d, INF)", treeID, c.opts.StartIndex)
	}

	fetcher := scanner.NewFetcher(c.ctClient, &c.opts.FetcherOptions)
	sth, err := fetcher.Prepare(ctx)
	if err != nil {
		return err
	}
	if err := c.verifyConsistency(ctx, root, sth); err != nil {
		return err
	}

	var wg sync.WaitGroup
	bufferSize := c.opts.Submitters * c.opts.BatchesPerSubmitter
	c.batches = make(chan scanner.EntryBatch, bufferSize)
	defer func() {
		close(c.batches)
		wg.Wait()
	}()

	// TODO(pavelkalinnikov): Share the submitters pool between multiple trees.
	for w, cnt := 0, c.opts.Submitters; w < cnt; w++ {
		wg.Add(1)
		go func() {
			c.runSubmitter(ctx)
			wg.Done()
		}()
	}

	handler := func(b scanner.EntryBatch) {
		metrics.entriesFetched.Add(float64(len(b.Entries)), c.label)
		c.batches <- b
	}
	return fetcher.Run(ctx, handler)
}

// verifyConsistency checks that the provided verified Trillian root is
// consistent with the CT log's STH.
func (c *Controller) verifyConsistency(ctx context.Context, root *types.LogRootV1, sth *ct.SignedTreeHead) error {
	h := c.plClient.verif.Hasher
	if root.TreeSize == 0 {
		if got, want := root.RootHash, h.EmptyRoot(); !bytes.Equal(got, want) {
			return fmt.Errorf("invalid empty tree hash %x, want %x", got, want)
		}
		return nil
	}

	resp, err := c.ctClient.GetEntryAndProof(ctx, root.TreeSize-1, sth.TreeSize)
	if err != nil {
		return err
	}
	leafHash, err := h.HashLeaf(resp.LeafInput)
	if err != nil {
		return err
	}

	hash, err := merkle.NewLogVerifier(h).VerifiedPrefixHashFromInclusionProof(
		int64(root.TreeSize), int64(sth.TreeSize),
		resp.AuditPath, sth.SHA256RootHash[:], leafHash)
	if err != nil {
		return err
	}

	if got := root.RootHash; !bytes.Equal(got, hash) {
		return fmt.Errorf("inconsistent root hash %x, want %x", got, hash)
	}
	return nil
}

// runSubmitter obtaines CT log entry batches from the controller's channel and
// submits them through Trillian client. Returns when the channel is closed.
func (c *Controller) runSubmitter(ctx context.Context) {
	treeID := c.plClient.tree.TreeId
	for b := range c.batches {
		entries := float64(len(b.Entries))
		metrics.entriesSeen.Add(entries, c.label)

		end := b.Start + int64(len(b.Entries))
		// TODO(pavelkalinnikov): Retry with backoff on errors.
		if err := c.plClient.addSequencedLeaves(ctx, &b); err != nil {
			glog.Errorf("%d: failed to add batch [%d, %d): %v", treeID, b.Start, end, err)
		} else {
			glog.Infof("%d: added batch [%d, %d)", treeID, b.Start, end)
			metrics.entriesStored.Add(entries, c.label)
		}
	}
}
