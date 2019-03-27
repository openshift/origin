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

package main

import (
	"context"
	"encoding/base64"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math/big"
	"net/http"
	"path"
	"regexp"
	"time"

	ct "github.com/google/certificate-transparency-go"
	"github.com/google/certificate-transparency-go/client"
	"github.com/google/certificate-transparency-go/jsonclient"
	"github.com/google/certificate-transparency-go/scanner"
	"github.com/google/certificate-transparency-go/x509"
)

const (
	// matchesNothingRegex is a regex which cannot match any input.
	matchesNothingRegex = "a^"
)

var (
	logURI = flag.String("log_uri", "https://ct.googleapis.com/aviator", "CT log base URI")

	matchSubjectRegex = flag.String("match_subject_regex", ".*", "Regex to match CN/SAN")
	matchIssuerRegex  = flag.String("match_issuer_regex", "", "Regex to match in issuer CN")
	precertsOnly      = flag.Bool("precerts_only", false, "Only match precerts")
	serialNumber      = flag.String("serial_number", "", "Serial number of certificate of interest")
	sctTimestamp      = flag.Uint64("sct_timestamp_ms", 0, "Timestamp of logged SCT")

	parseErrors    = flag.Bool("parse_errors", false, "Only match certificates with parse errors")
	nfParseErrors  = flag.Bool("non_fatal_errors", false, "Treat non-fatal parse errors as also matching (with --parse_errors)")
	validateErrors = flag.Bool("validate_errors", false, "Only match certificates with validation errors")

	batchSize     = flag.Int("batch_size", 1000, "Max number of entries to request at per call to get-entries")
	numWorkers    = flag.Int("num_workers", 2, "Number of concurrent matchers")
	parallelFetch = flag.Int("parallel_fetch", 2, "Number of concurrent GetEntries fetches")
	startIndex    = flag.Int64("start_index", 0, "Log index to start scanning at")
	endIndex      = flag.Int64("end_index", 0, "Log index to end scanning at (non-inclusive, 0 = end of log)")

	printChains = flag.Bool("print_chains", false, "If true prints the whole chain rather than a summary")
	dumpDir     = flag.String("dump_dir", "", "Directory to store matched certificates in")
)

func dumpData(entry *ct.RawLogEntry) {
	if *dumpDir == "" {
		return
	}
	prefix := "unknown"
	suffix := "unknown"
	switch eType := entry.Leaf.TimestampedEntry.EntryType; eType {
	case ct.X509LogEntryType:
		prefix = "cert"
		suffix = "leaf"
	case ct.PrecertLogEntryType:
		prefix = "precert"
		suffix = "precert"
	default:
		log.Printf("Unknown log entry type %d", eType)
	}

	if len(entry.Cert.Data) > 0 {
		name := fmt.Sprintf("%s-%014d-%s.der", prefix, entry.Index, suffix)
		filename := path.Join(*dumpDir, name)
		if err := ioutil.WriteFile(filename, entry.Cert.Data, 0644); err != nil {
			log.Printf("Failed to dump data for %s at index %d: %v", prefix, entry.Index, err)
		}
	}

	for ii := 0; ii < len(entry.Chain); ii++ {
		name := fmt.Sprintf("%s-%014d-%02d.der", prefix, entry.Index, ii)
		filename := path.Join(*dumpDir, name)
		if err := ioutil.WriteFile(filename, entry.Chain[ii].Data, 0644); err != nil {
			log.Printf("Failed to dump data for CA at index %d: %v", entry.Index, err)
		}
	}
}

// Prints out a short bit of info about |cert|, found at |index| in the
// specified log
func logCertInfo(entry *ct.RawLogEntry) {
	parsedEntry, err := entry.ToLogEntry()
	if x509.IsFatal(err) || parsedEntry.X509Cert == nil {
		log.Printf("Process cert at index %d: <unparsed: %v>", entry.Index, err)
	} else {
		log.Printf("Process cert at index %d: CN: '%s'", entry.Index, parsedEntry.X509Cert.Subject.CommonName)
	}
	dumpData(entry)
}

// Prints out a short bit of info about |precert|, found at |index| in the
// specified log
func logPrecertInfo(entry *ct.RawLogEntry) {
	parsedEntry, err := entry.ToLogEntry()
	if x509.IsFatal(err) || parsedEntry.Precert == nil {
		log.Printf("Process precert at index %d: <unparsed: %v>", entry.Index, err)
	} else {
		log.Printf("Process precert at index %d: CN: '%s' Issuer: %s", entry.Index, parsedEntry.Precert.TBSCertificate.Subject.CommonName, parsedEntry.Precert.TBSCertificate.Issuer.CommonName)
	}
	dumpData(entry)
}

func chainToString(certs []ct.ASN1Cert) string {
	var output []byte

	for _, cert := range certs {
		output = append(output, cert.Data...)
	}

	return base64.StdEncoding.EncodeToString(output)
}

func logFullChain(entry *ct.RawLogEntry) {
	log.Printf("Index %d: Chain: %s", entry.Index, chainToString(entry.Chain))
}

func createRegexes(regexValue string) (*regexp.Regexp, *regexp.Regexp) {
	// Make a regex matcher
	var certRegex *regexp.Regexp
	precertRegex := regexp.MustCompile(regexValue)
	switch *precertsOnly {
	case true:
		certRegex = regexp.MustCompile(matchesNothingRegex)
	case false:
		certRegex = precertRegex
	}

	return certRegex, precertRegex
}

func createMatcherFromFlags(logClient *client.LogClient) (interface{}, error) {
	if *parseErrors {
		return scanner.CertParseFailMatcher{MatchNonFatalErrs: *nfParseErrors}, nil
	}
	if *validateErrors {
		matcher := scanner.CertVerifyFailMatcher{}
		matcher.PopulateRoots(context.TODO(), logClient)
		return matcher, nil
	}
	if *matchIssuerRegex != "" {
		certRegex, precertRegex := createRegexes(*matchIssuerRegex)
		return scanner.MatchIssuerRegex{
			CertificateIssuerRegex:    certRegex,
			PrecertificateIssuerRegex: precertRegex}, nil
	}
	if *serialNumber != "" {
		log.Printf("Using SerialNumber matcher on %s", *serialNumber)
		var sn big.Int
		_, success := sn.SetString(*serialNumber, 0)
		if !success {
			return nil, fmt.Errorf("Invalid serialNumber %s", *serialNumber)
		}
		return scanner.MatchSerialNumber{SerialNumber: sn}, nil
	}
	if *sctTimestamp != 0 {
		log.Printf("Using SCT Timestamp matcher on %d (%v)", *sctTimestamp, time.Unix(0, int64(*sctTimestamp*1000000)))
		return scanner.MatchSCTTimestamp{Timestamp: *sctTimestamp}, nil
	}
	certRegex, precertRegex := createRegexes(*matchSubjectRegex)
	return scanner.MatchSubjectRegex{
		CertificateSubjectRegex:    certRegex,
		PrecertificateSubjectRegex: precertRegex}, nil
}

func main() {
	flag.Parse()

	logClient, err := client.New(*logURI, &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSHandshakeTimeout:   30 * time.Second,
			ResponseHeaderTimeout: 30 * time.Second,
			MaxIdleConnsPerHost:   10,
			DisableKeepAlives:     false,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}, jsonclient.Options{})
	if err != nil {
		log.Fatal(err)
	}
	matcher, err := createMatcherFromFlags(logClient)
	if err != nil {
		log.Fatal(err)
	}

	opts := scanner.ScannerOptions{
		FetcherOptions: scanner.FetcherOptions{
			BatchSize:     *batchSize,
			ParallelFetch: *parallelFetch,
			StartIndex:    *startIndex,
			EndIndex:      *endIndex,
		},
		Matcher:    matcher,
		NumWorkers: *numWorkers,
	}
	scanner := scanner.NewScanner(logClient, opts)

	ctx := context.Background()
	if *printChains {
		scanner.Scan(ctx, logFullChain, logFullChain)
	} else {
		scanner.Scan(ctx, logCertInfo, logPrecertInfo)
	}
}
