// Copyright 2014 Google Inc. All Rights Reserved.
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

// Package scanner holds code for iterating through the contents of a CT log.
package scanner

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/golang/glog"
	ct "github.com/google/certificate-transparency-go"
	"github.com/google/certificate-transparency-go/client"
	"github.com/google/certificate-transparency-go/x509"
)

// ScannerOptions holds configuration options for the Scanner.
type ScannerOptions struct { // nolint:golint
	FetcherOptions

	// Custom matcher for x509 Certificates, functor will be called for each
	// Certificate found during scanning. Should be a Matcher or LeafMatcher
	// implementation.
	Matcher interface{}

	// Match precerts only (Matcher still applies to precerts).
	PrecertOnly bool

	// Number of concurrent matchers to run.
	NumWorkers int

	// Number of fetched entries to buffer on their way to the callbacks.
	BufferSize int
}

// DefaultScannerOptions returns a new ScannerOptions with sensible defaults.
func DefaultScannerOptions() *ScannerOptions {
	return &ScannerOptions{
		FetcherOptions: *DefaultFetcherOptions(),
		Matcher:        &MatchAll{},
		PrecertOnly:    false,
		NumWorkers:     1,
	}
}

// Scanner is a tool to scan all the entries in a CT Log.
type Scanner struct {
	fetcher *Fetcher

	// Configuration options for this Scanner instance.
	opts ScannerOptions

	// Counters of the number of certificates scanned and matched.
	certsProcessed int64
	certsMatched   int64

	// Counter of the number of precertificates encountered during the scan.
	precertsSeen int64

	unparsableEntries         int64
	entriesWithNonFatalErrors int64
}

// entryInfo represents information about a log entry.
type entryInfo struct {
	// The index of the entry containing the LeafInput in the log.
	index int64
	// The log entry returned by the log server.
	entry ct.LeafEntry
}

// Takes the error returned by either x509.ParseCertificate() or
// x509.ParseTBSCertificate() and determines if it's non-fatal or otherwise.
// In the case of non-fatal errors, the error will be logged,
// entriesWithNonFatalErrors will be incremented, and the return value will be
// false.
// Fatal errors will cause the function to return true.
// When err is nil, this method does nothing.
func (s *Scanner) isCertErrorFatal(err error, logEntry *ct.LogEntry, index int64) bool {
	if err == nil {
		// No error to handle.
		return false
	} else if !x509.IsFatal(err) {
		atomic.AddInt64(&s.entriesWithNonFatalErrors, 1)
		// We'll make a note, but continue.
		glog.V(1).Infof("Non-fatal error in %v at index %d: %v", logEntry.Leaf.TimestampedEntry.EntryType, index, err)
		return false
	}
	return true
}

// Processes the given entry in the specified log.
func (s *Scanner) processEntry(info entryInfo, foundCert func(*ct.RawLogEntry), foundPrecert func(*ct.RawLogEntry)) error {
	atomic.AddInt64(&s.certsProcessed, 1)

	switch matcher := s.opts.Matcher.(type) {
	case Matcher:
		return s.processMatcherEntry(matcher, info, foundCert, foundPrecert)
	case LeafMatcher:
		return s.processMatcherLeafEntry(matcher, info, foundCert, foundPrecert)
	default:
		return fmt.Errorf("Unexpected matcher type %T", matcher)
	}
}

func (s *Scanner) processMatcherEntry(matcher Matcher, info entryInfo, foundCert func(*ct.RawLogEntry), foundPrecert func(*ct.RawLogEntry)) error {
	rawLogEntry, err := ct.RawLogEntryFromLeaf(info.index, &info.entry)
	if err != nil {
		return fmt.Errorf("failed to build raw log entry %d: %v", info.index, err)
	}
	// Matcher instances need the parsed [pre-]certificate.
	logEntry, err := rawLogEntry.ToLogEntry()
	if s.isCertErrorFatal(err, logEntry, info.index) {
		return fmt.Errorf("failed to parse [pre-]certificate in MerkleTreeLeaf[%d]: %v", info.index, err)
	}

	switch {
	case logEntry.X509Cert != nil:
		if s.opts.PrecertOnly {
			// Only interested in precerts and this is an X.509 cert, early-out.
			return nil
		}
		if matcher.CertificateMatches(logEntry.X509Cert) {
			atomic.AddInt64(&s.certsMatched, 1)
			foundCert(rawLogEntry)
		}
	case logEntry.Precert != nil:
		if matcher.PrecertificateMatches(logEntry.Precert) {
			atomic.AddInt64(&s.certsMatched, 1)
			foundPrecert(rawLogEntry)
		}
		atomic.AddInt64(&s.precertsSeen, 1)
	default:
		return fmt.Errorf("saw unknown entry type: %v", logEntry.Leaf.TimestampedEntry.EntryType)
	}
	return nil
}

func (s *Scanner) processMatcherLeafEntry(matcher LeafMatcher, info entryInfo, foundCert func(*ct.RawLogEntry), foundPrecert func(*ct.RawLogEntry)) error {
	if !matcher.Matches(&info.entry) {
		return nil
	}

	rawLogEntry, err := ct.RawLogEntryFromLeaf(info.index, &info.entry)
	if rawLogEntry == nil {
		return fmt.Errorf("failed to build raw log entry %d: %v", info.index, err)
	}
	switch eType := rawLogEntry.Leaf.TimestampedEntry.EntryType; eType {
	case ct.X509LogEntryType:
		if s.opts.PrecertOnly {
			// Only interested in precerts and this is an X.509 cert, early-out.
			return nil
		}
		foundCert(rawLogEntry)
	case ct.PrecertLogEntryType:
		foundPrecert(rawLogEntry)
		atomic.AddInt64(&s.precertsSeen, 1)
	default:
		return fmt.Errorf("saw unknown entry type: %v", eType)
	}
	return nil
}

// Worker function to match certs.
// Accepts MatcherJobs over the entries channel, and processes them.
// Returns true over the done channel when the entries channel is closed.
func (s *Scanner) matcherJob(entries <-chan entryInfo, foundCert func(*ct.RawLogEntry), foundPrecert func(*ct.RawLogEntry)) {
	for e := range entries {
		if err := s.processEntry(e, foundCert, foundPrecert); err != nil {
			atomic.AddInt64(&s.unparsableEntries, 1)
			glog.Errorf("Failed to parse entry at index %d: %s", e.index, err.Error())
		}
	}
}

// Pretty prints the passed in duration into a human readable string.
func humanTime(dur time.Duration) string {
	hours := int(dur / time.Hour)
	dur %= time.Hour
	minutes := int(dur / time.Minute)
	dur %= time.Minute
	seconds := int(dur / time.Second)
	s := ""
	if hours > 0 {
		s += fmt.Sprintf("%d hours ", hours)
	}
	if minutes > 0 {
		s += fmt.Sprintf("%d minutes ", minutes)
	}
	if seconds > 0 || len(s) == 0 {
		s += fmt.Sprintf("%d seconds ", seconds)
	}
	return s
}

func (s *Scanner) logThroughput(treeSize int64, stop <-chan bool) {
	const wndSize = 15
	wnd := make([]int64, wndSize)
	wndTotal := int64(0)

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for slot, filled, prevCnt := 0, 0, int64(0); ; slot = (slot + 1) % wndSize {
		select {
		case <-stop:
			return
		case <-ticker.C:
			certsCnt := atomic.LoadInt64(&s.certsProcessed)
			certsMatched := atomic.LoadInt64(&s.certsMatched)

			slotValue := certsCnt - prevCnt
			wndTotal += slotValue - wnd[slot]
			wnd[slot], prevCnt = slotValue, certsCnt

			if filled < wndSize {
				filled++
			}

			throughput := float64(wndTotal) / float64(filled)
			remainingCerts := treeSize - int64(s.opts.StartIndex) - certsCnt
			remainingSeconds := int(float64(remainingCerts) / throughput)
			remainingString := humanTime(time.Duration(remainingSeconds) * time.Second)
			glog.V(1).Infof("Processed: %d certs (to index %d), matched %d (%2.2f%%). Throughput (last %ds): %3.2f ETA: %s\n",
				certsCnt, s.opts.StartIndex+certsCnt, certsMatched,
				(100.0*float64(certsMatched))/float64(certsCnt),
				filled, throughput, remainingString)
		}
	}
}

// Scan performs a scan against the Log. Blocks until the scan is complete.
//
// For each x509 certificate found, calls foundCert with the corresponding
// LogEntry, which includes the index of the entry and the certificate.
// For each precert found, calls foundPrecert with the corresponding LogEntry,
// which includes the index of the entry and the precert.
func (s *Scanner) Scan(ctx context.Context, foundCert func(*ct.RawLogEntry), foundPrecert func(*ct.RawLogEntry)) error {
	_, err := s.ScanLog(ctx, foundCert, foundPrecert)
	return err
}

// ScanLog performs a scan against the Log, returning the count of scanned entries.
func (s *Scanner) ScanLog(ctx context.Context, foundCert func(*ct.RawLogEntry), foundPrecert func(*ct.RawLogEntry)) (int64, error) {
	glog.V(1).Infof("Starting up Scanner...")
	s.certsProcessed = 0
	s.certsMatched = 0
	s.precertsSeen = 0
	s.unparsableEntries = 0
	s.entriesWithNonFatalErrors = 0

	sth, err := s.fetcher.Prepare(ctx)
	if err != nil {
		return -1, err
	}

	startTime := time.Now()
	stop := make(chan bool)
	go s.logThroughput(int64(sth.TreeSize), stop)
	defer func() {
		stop <- true
		close(stop)
	}()

	// Start matcher workers.
	var wg sync.WaitGroup
	entries := make(chan entryInfo, s.opts.BufferSize)
	for w, cnt := 0, s.opts.NumWorkers; w < cnt; w++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			s.matcherJob(entries, foundCert, foundPrecert)
			glog.V(1).Infof("Matcher %d finished", idx)
		}(w)
	}

	flatten := func(b EntryBatch) {
		for i, e := range b.Entries {
			entries <- entryInfo{index: b.Start + int64(i), entry: e}
		}
	}
	err = s.fetcher.Run(ctx, flatten)
	close(entries) // Causes matcher workers to terminate.
	wg.Wait()      // Wait until they terminate.
	if err != nil {
		return -1, err
	}

	glog.V(1).Infof("Completed %d certs in %s", atomic.LoadInt64(&s.certsProcessed), humanTime(time.Since(startTime)))
	glog.V(1).Infof("Saw %d precerts", atomic.LoadInt64(&s.precertsSeen))
	glog.V(1).Infof("Saw %d unparsable entries", atomic.LoadInt64(&s.unparsableEntries))
	glog.V(1).Infof("Saw %d non-fatal errors", atomic.LoadInt64(&s.entriesWithNonFatalErrors))

	return int64(s.fetcher.opts.EndIndex), nil
}

// NewScanner creates a Scanner instance using client to talk to the log,
// taking configuration options from opts.
func NewScanner(client *client.LogClient, opts ScannerOptions) *Scanner {
	var scanner Scanner
	scanner.opts = opts
	scanner.fetcher = NewFetcher(client, &scanner.opts.FetcherOptions)

	// Set a default match-everything regex if none was provided.
	if opts.Matcher == nil {
		opts.Matcher = &MatchAll{}
	}
	return &scanner
}
