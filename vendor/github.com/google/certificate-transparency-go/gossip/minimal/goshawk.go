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

package minimal

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"
	"time"

	"github.com/golang/glog"
	"github.com/golang/protobuf/proto"
	"github.com/google/certificate-transparency-go"
	"github.com/google/certificate-transparency-go/gossip/minimal/configpb"
	"github.com/google/certificate-transparency-go/gossip/minimal/x509ext"
	"github.com/google/certificate-transparency-go/scanner"
	"github.com/google/certificate-transparency-go/x509"
	"github.com/google/trillian/merkle"
	"github.com/google/trillian/merkle/rfc6962"
)

// Goshawk is an agent that retrieves certificates from a destination log that
// have STH values embedded in them. Each STH is then checked for consistency
// against the source log.
type Goshawk struct {
	dest     *logConfig
	origins  map[string]*originLog // URL => log
	scanOpts scanner.ScannerOptions
}

type originLog struct {
	logConfig

	sths       chan *x509ext.LogSTHInfo
	mu         sync.RWMutex
	currentSTH *ct.SignedTreeHead
}

// NewGoshawkFromFile creates a Goshawk from the given filename, which should
// contain text-protobuf encoded configuration data, together with an optional
// http Client.
func NewGoshawkFromFile(ctx context.Context, filename string, hc *http.Client, scanOpts scanner.ScannerOptions) (*Goshawk, error) {
	cfgText, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var cfgProto configpb.GoshawkConfig
	if err := proto.UnmarshalText(string(cfgText), &cfgProto); err != nil {
		return nil, fmt.Errorf("%s: failed to parse gossip config: %v", filename, err)
	}
	cfg, err := NewGoshawk(ctx, &cfgProto, hc, scanOpts)
	if err != nil {
		return nil, fmt.Errorf("%s: config error: %v", filename, err)
	}
	return cfg, nil
}

// NewGoshawk creates a gossiper from the given configuration protobuf and optional http client.
func NewGoshawk(ctx context.Context, cfg *configpb.GoshawkConfig, hc *http.Client, scanOpts scanner.ScannerOptions) (*Goshawk, error) {
	if cfg.DestLog == nil {
		return nil, errors.New("no source log config found")
	}
	if cfg.SourceLog == nil || len(cfg.SourceLog) == 0 {
		return nil, errors.New("no source log config found")
	}

	dest, err := logConfigFromProto(cfg.DestLog, hc)
	if err != nil {
		return nil, fmt.Errorf("failed to parse dest log config: %v", err)
	}
	seenNames := make(map[string]bool)
	origins := make(map[string]*originLog)
	for _, lc := range cfg.SourceLog {
		base, err := logConfigFromProto(lc, hc)
		if err != nil {
			return nil, fmt.Errorf("failed to parse source log config: %v", err)
		}
		if _, ok := seenNames[base.Name]; ok {
			return nil, fmt.Errorf("duplicate source logs for name %s", base.Name)
		}
		seenNames[base.Name] = true

		if _, ok := origins[base.URL]; ok {
			return nil, fmt.Errorf("duplicate source logs for url %s", base.URL)
		}
		origins[base.URL] = &originLog{
			logConfig: *base,
			sths:      make(chan *x509ext.LogSTHInfo, cfg.BufferSize),
		}
	}

	hawk := Goshawk{dest: dest, origins: origins}
	scanOpts.Matcher = &hawk
	if scanOpts.Matcher.(scanner.Matcher) == nil {
		return nil, fmt.Errorf("hawk does not satisfy scanner.Matcher interface")
	}
	hawk.scanOpts = scanOpts
	return &hawk, nil
}

// Methods to ensure Goshawk implements the scanner.Matcher interface

// CertificateMatches identifies certificates in the log that have the STH extension.
func (hawk *Goshawk) CertificateMatches(cert *x509.Certificate) bool {
	return x509ext.HasSTHInfo(cert)
}

// PrecertificateMatches identifies those precertificates in the log that are of
// interest: none.
func (hawk *Goshawk) PrecertificateMatches(*ct.Precertificate) bool {
	return false
}

// Fly starts a collection of goroutines to perform log scanning and STH
// consistency checking. It should be terminated by cancelling the passed-in
// context.
func (hawk *Goshawk) Fly(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Add(1 + len(hawk.origins) + len(hawk.origins))

	go func() {
		// TODO: check scanner.Scan respects context cancellation
		// TODO: make Scan return scanned size and restart
		defer wg.Done()
		glog.Infof("starting Scanner(%s)", hawk.dest.Name)
		hawk.Scanner(ctx)
		glog.Infof("finished Scanner(%s)", hawk.dest.Name)
	}()
	for _, origin := range hawk.origins {
		go func(origin *originLog) {
			defer wg.Done()
			glog.Infof("starting STHRetriever(%s)", origin.Name)
			origin.STHRetriever(ctx)
			glog.Infof("finished STHRetriever(%s)", origin.Name)
		}(origin)
		go func(origin *originLog) {
			defer wg.Done()
			glog.Infof("starting Checker(%s)", origin.Name)
			origin.Checker(ctx)
			glog.Infof("finished Checker(%s)", origin.Name)
		}(origin)
	}
	wg.Wait()
}

// Scanner runs a continuous scan of the destination log.
func (hawk *Goshawk) Scanner(ctx context.Context) {
	ticker := time.NewTicker(hawk.dest.MinInterval)
	for {
		glog.V(1).Infof("Scanner(%s): run scan from %d to current size", hawk.dest.Name, hawk.scanOpts.StartIndex)
		s := scanner.NewScanner(hawk.dest.Log, hawk.scanOpts)
		size, err := s.ScanLog(ctx, hawk.foundCert, foundPrecert)
		if err != nil {
			glog.Errorf("Scanner(%s) terminated: %v", hawk.dest.Name, err)
			return
		}
		hawk.scanOpts.StartIndex = size

		glog.V(2).Infof("Scanner(%s): wait for a %s tick", hawk.dest.Name, hawk.dest.MinInterval)
		select {
		case <-ctx.Done():
			glog.Infof("Scanner(%s): termination requested", hawk.dest.Name)
			return
		case <-ticker.C:
		}
	}
}

func (hawk *Goshawk) foundCert(rawEntry *ct.RawLogEntry) {
	entry, err := rawEntry.ToLogEntry()
	if x509.IsFatal(err) {
		glog.Errorf("Scanner(%s): failed to parse cert from entry at %d: %v", hawk.dest.Name, rawEntry.Index, err)
		return
	}

	if entry.X509Cert == nil {
		glog.Errorf("Internal error: no X509Cert entry in %+v", entry)
		return
	}

	sthInfo, err := x509ext.LogSTHInfoFromCert(entry.X509Cert)
	if err != nil {
		glog.Errorf("Scanner(%s): failed to retrieve STH info from entry at %d: %v", hawk.dest.Name, entry.Index, err)
		return
	}
	url := string(sthInfo.LogURL)
	glog.Infof("Scanner(%s): process STHInfo for %s at index %d", hawk.dest.Name, url, entry.Index)

	origin, ok := hawk.origins[url]
	if !ok {
		glog.Errorf("Scanner(%s): found STH info for unrecognized log at %q in entry at %d", hawk.dest.Name, url, entry.Index)
	}
	origin.sths <- sthInfo
}

func foundPrecert(entry *ct.RawLogEntry) {
	glog.Errorf("Internal error: found pre-cert! %+v", entry)
}

func (o *originLog) Checker(ctx context.Context) {
	ticker := time.NewTicker(o.MinInterval)
	for {
		select {
		case sthInfo := <-o.sths:
			glog.Infof("Checker(%s): check STH size=%d timestamp=%d", o.Name, sthInfo.TreeSize, sthInfo.Timestamp)
			if err := o.validateSTH(ctx, sthInfo); err != nil {
				glog.Errorf("Checker(%s): failed to validate STH: %v", o.Name, err)
			}

		case <-ctx.Done():
			glog.Infof("Checker(%s): termination requested", o.Name)
			return
		}

		glog.V(2).Infof("Checker(%s): wait for a %s tick", o.Name, o.MinInterval)
		select {
		case <-ctx.Done():
			glog.Infof("Checker(%s): termination requested", o.Name)
			return
		case <-ticker.C:
		}
	}
}

func (o *originLog) validateSTH(ctx context.Context, sthInfo *x509ext.LogSTHInfo) error {
	// Validate the signature in sthInfo
	sth := ct.SignedTreeHead{
		Version:           ct.Version(sthInfo.Version),
		TreeSize:          sthInfo.TreeSize,
		Timestamp:         sthInfo.Timestamp,
		SHA256RootHash:    sthInfo.SHA256RootHash,
		TreeHeadSignature: sthInfo.TreeHeadSignature,
	}
	if err := o.Log.VerifySTHSignature(sth); err != nil {
		return fmt.Errorf("Checker(%s): failed to validate STH signature: %v", o.Name, err)
	}

	currentSTH := o.getLastSTH()
	if currentSTH == nil {
		glog.Warningf("Checker(%s): no current STH available", o.Name)
		return nil
	}
	first, second := sthInfo.TreeSize, currentSTH.TreeSize
	firstHash, secondHash := sthInfo.SHA256RootHash[:], currentSTH.SHA256RootHash[:]
	if first > second {
		glog.Warningf("Checker(%s): retrieved STH info (size=%d) > current STH (size=%d); reversing check", o.Name, first, second)
		first, second = second, first
		firstHash, secondHash = secondHash, firstHash
	}
	proof, err := o.Log.GetSTHConsistency(ctx, first, second)
	if err != nil {
		return err
	}

	verifier := merkle.NewLogVerifier(rfc6962.DefaultHasher)
	if err := verifier.VerifyConsistencyProof(int64(first), int64(second), firstHash, secondHash, proof); err != nil {
		return fmt.Errorf("Failed to VerifyConsistencyProof(%x @size=%d, %x @size=%d): %v", firstHash, first, secondHash, second, err)
	}
	glog.Infof("Checker(%s): verified that hash %x @%d + proof = hash %x @%d\n", o.Name, firstHash, first, secondHash, second)
	return nil
}

func (o *originLog) STHRetriever(ctx context.Context) {
	ticker := time.NewTicker(o.MinInterval)
	for {
		sth, err := o.Log.GetSTH(ctx)
		if err != nil {
			glog.Errorf("STHRetriever(%s): failed to get-sth: %v", o.Name, err)
		} else {
			o.updateSTH(sth)
		}

		// Wait before retrieving another STH.
		glog.V(2).Infof("STHRetriever(%s): wait for a %s tick", o.Name, o.MinInterval)
		select {
		case <-ctx.Done():
			glog.Infof("STHRetriever(%s): termination requested", o.Name)
			return
		case <-ticker.C:
		}
	}
}

func (o *originLog) updateSTH(sth *ct.SignedTreeHead) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.currentSTH == nil || sth.TreeSize > o.currentSTH.TreeSize {
		glog.V(1).Infof("STHRetriever(%s): update tip STH to size=%d timestamp=%d", o.Name, sth.TreeSize, sth.Timestamp)
		o.currentSTH = sth
	}
}

func (o *originLog) getLastSTH() *ct.SignedTreeHead {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.currentSTH
}
