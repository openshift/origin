// Copyright 2015 Google Inc. All Rights Reserved.
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

package main

import (
	"compress/zlib"
	"context"
	"encoding/gob"
	"flag"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/golang/glog"
	ct "github.com/google/certificate-transparency-go"
	"github.com/google/certificate-transparency-go/client"
	"github.com/google/certificate-transparency-go/jsonclient"
	"github.com/google/certificate-transparency-go/preload"
	"github.com/google/certificate-transparency-go/scanner"
	"github.com/google/certificate-transparency-go/x509"
)

var (
	sourceLogURI          = flag.String("source_log_uri", "https://ct.googleapis.com/aviator", "CT log base URI to fetch entries from")
	targetLogURI          = flag.String("target_log_uri", "https://example.com/ct", "CT log base URI to add entries to")
	targetTemporalLogCfg  = flag.String("target_temporal_log_cfg", "", "File holding temporal log configuration")
	batchSize             = flag.Int("batch_size", 1000, "Max number of entries to request at per call to get-entries")
	numWorkers            = flag.Int("num_workers", 2, "Number of concurrent matchers")
	parallelFetch         = flag.Int("parallel_fetch", 2, "Number of concurrent GetEntries fetches")
	parallelSubmit        = flag.Int("parallel_submit", 2, "Number of concurrent add-[pre]-chain requests")
	startIndex            = flag.Int64("start_index", 0, "Log index to start scanning at")
	sctInputFile          = flag.String("sct_file", "", "File to save SCTs & leaf data to")
	precertsOnly          = flag.Bool("precerts_only", false, "Only match precerts")
	tlsTimeout            = flag.Duration("tls_timeout", 30*time.Second, "TLS handshake timeout (see http.Transport)")
	rspHeaderTimeout      = flag.Duration("response_header_timeout", 30*time.Second, "Response header timeout (see http.Transport)")
	maxIdlePerHost        = flag.Int("max_idle_conns_per_host", 10, "Maximum number of idle connections per host (see http.Transport)")
	maxIdleConns          = flag.Int("max_idle_conns", 100, "Maximum number of idle connections (see http.Transport)")
	idleTimeout           = flag.Duration("idle_conn_timeout", 90*time.Second, "Idle connections with no use within this period will be closed (see http.Transport)")
	disableKeepAlive      = flag.Bool("disable_keepalive", false, "Disable HTTP Keep-Alive (see http.Transport)")
	expectContinueTimeout = flag.Duration("expect_continue_timeout", time.Second, "Amount of time to wait for a response if request uses Expect: 100-continue (see http.Transport")
)

func recordSct(addedCerts chan<- *preload.AddedCert, certDer ct.ASN1Cert, sct *ct.SignedCertificateTimestamp) {
	addedCert := preload.AddedCert{
		CertDER:                    certDer,
		SignedCertificateTimestamp: *sct,
		AddedOk:                    true,
	}
	addedCerts <- &addedCert
}

func recordFailure(addedCerts chan<- *preload.AddedCert, certDer ct.ASN1Cert, addError error) {
	addedCert := preload.AddedCert{
		CertDER:      certDer,
		AddedOk:      false,
		ErrorMessage: addError.Error(),
	}
	addedCerts <- &addedCert
}

func sctDumper(addedCerts <-chan *preload.AddedCert, sctWriter io.Writer) {
	encoder := gob.NewEncoder(sctWriter)

	numAdded := 0
	numFailed := 0

	for c := range addedCerts {
		if c.AddedOk {
			numAdded++
		} else {
			numFailed++
		}
		if encoder != nil {
			err := encoder.Encode(c)
			if err != nil {
				glog.Fatalf("failed to encode to %s: %v", *sctInputFile, err)
			}
		}
	}
	glog.Infof("Added %d certs, %d failed, total: %d\n", numAdded, numFailed, numAdded+numFailed)
}

func certSubmitter(ctx context.Context, addedCerts chan<- *preload.AddedCert, logClient client.AddLogClient, certs <-chan *ct.LogEntry) {
	for c := range certs {
		chain := make([]ct.ASN1Cert, len(c.Chain)+1)
		chain[0] = ct.ASN1Cert{Data: c.X509Cert.Raw}
		copy(chain[1:], c.Chain)
		sct, err := logClient.AddChain(ctx, chain)
		if err != nil {
			glog.Errorf("failed to add chain with CN %s: %v\n", c.X509Cert.Subject.CommonName, err)
			recordFailure(addedCerts, chain[0], err)
			continue
		}
		recordSct(addedCerts, chain[0], sct)
		glog.V(2).Infof("Added chain for CN '%s', SCT: %s\n", c.X509Cert.Subject.CommonName, sct)
	}
}

func precertSubmitter(ctx context.Context, addedCerts chan<- *preload.AddedCert, logClient client.AddLogClient, precerts <-chan *ct.LogEntry) {
	for c := range precerts {
		chain := make([]ct.ASN1Cert, len(c.Chain)+1)
		chain[0] = c.Precert.Submitted
		copy(chain[1:], c.Chain)
		sct, err := logClient.AddPreChain(ctx, chain)
		if err != nil {
			glog.Errorf("failed to add pre-chain with CN %s: %v", c.Precert.TBSCertificate.Subject.CommonName, err)
			recordFailure(addedCerts, chain[0], err)
			continue
		}
		recordSct(addedCerts, chain[0], sct)
		glog.V(2).Infof("Added precert chain for CN '%s', SCT: %s\n", c.Precert.TBSCertificate.Subject.CommonName, sct)
	}
}

func main() {
	flag.Parse()
	glog.CopyStandardLogTo("WARNING")

	var sctFileWriter io.Writer
	var err error
	if *sctInputFile != "" {
		sctFile, err := os.Create(*sctInputFile)
		if err != nil {
			glog.Exitf("Failed to create SCT file: %v", err)
		}
		defer sctFile.Close()
		sctFileWriter = sctFile
	} else {
		sctFileWriter = ioutil.Discard
	}

	sctWriter := zlib.NewWriter(sctFileWriter)
	defer func() {
		err := sctWriter.Close()
		if err != nil {
			glog.Exitf("Failed to close SCT file: %v", err)
		}
	}()

	transport := &http.Transport{
		TLSHandshakeTimeout:   *tlsTimeout,
		ResponseHeaderTimeout: *rspHeaderTimeout,
		MaxIdleConnsPerHost:   *maxIdlePerHost,
		DisableKeepAlives:     *disableKeepAlive,
		MaxIdleConns:          *maxIdleConns,
		IdleConnTimeout:       *idleTimeout,
		ExpectContinueTimeout: *expectContinueTimeout,
	}

	fetchLogClient, err := client.New(*sourceLogURI, &http.Client{
		Timeout:   10 * time.Second,
		Transport: transport,
	}, jsonclient.Options{})
	if err != nil {
		glog.Exitf("Failed to create client for source log: %v", err)
	}

	opts := scanner.ScannerOptions{
		FetcherOptions: scanner.FetcherOptions{
			BatchSize:     *batchSize,
			ParallelFetch: *parallelFetch,
			StartIndex:    *startIndex,
		},
		Matcher:     scanner.MatchAll{},
		PrecertOnly: *precertsOnly,
		NumWorkers:  *numWorkers,
	}
	scanner := scanner.NewScanner(fetchLogClient, opts)

	bufferSize := 10 * *parallelSubmit
	certs := make(chan *ct.LogEntry, bufferSize)
	precerts := make(chan *ct.LogEntry, bufferSize)
	addedCerts := make(chan *preload.AddedCert, bufferSize)

	var sctWriterWG sync.WaitGroup
	sctWriterWG.Add(1)
	go func() {
		defer sctWriterWG.Done()
		sctDumper(addedCerts, sctWriter)
	}()

	var submitLogClient client.AddLogClient
	if *targetTemporalLogCfg != "" {
		cfg, err := client.TemporalLogConfigFromFile(*targetTemporalLogCfg)
		if err != nil {
			glog.Exitf("Failed to load temporal log config: %v", err)
		}
		submitLogClient, err = client.NewTemporalLogClient(*cfg, &http.Client{Transport: transport})
		if err != nil {
			glog.Exitf("Failed to create client for destination temporal log: %v", err)
		}
	} else {
		submitLogClient, err = client.New(*targetLogURI, &http.Client{Transport: transport}, jsonclient.Options{})
		if err != nil {
			glog.Exitf("Failed to create client for destination log: %v", err)
		}
	}

	ctx := context.Background()
	var submitterWG sync.WaitGroup
	for w := 0; w < *parallelSubmit; w++ {
		submitterWG.Add(2)
		go func() {
			defer submitterWG.Done()
			certSubmitter(ctx, addedCerts, submitLogClient, certs)
		}()
		go func() {
			defer submitterWG.Done()
			precertSubmitter(ctx, addedCerts, submitLogClient, precerts)
		}()
	}

	addChainFunc := func(rawEntry *ct.RawLogEntry) {
		entry, err := rawEntry.ToLogEntry()
		if x509.IsFatal(err) {
			glog.Errorf("Failed to parse cert at %d: %v", rawEntry.Index, err)
			return
		}
		certs <- entry
	}
	addPreChainFunc := func(rawEntry *ct.RawLogEntry) {
		entry, err := rawEntry.ToLogEntry()
		if x509.IsFatal(err) {
			glog.Errorf("Failed to parse precert at %d: %v", rawEntry.Index, err)
			return
		}
		precerts <- entry
	}
	scanner.Scan(ctx, addChainFunc, addPreChainFunc)

	close(certs)
	close(precerts)
	submitterWG.Wait()
	close(addedCerts)
	sctWriterWG.Wait()
}
