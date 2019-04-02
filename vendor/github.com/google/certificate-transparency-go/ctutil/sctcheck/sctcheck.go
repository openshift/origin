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

// sctcheck is a utility to show and check embedded SCTs (Signed Certificate
// Timestamps) in certificates.
package main

import (
	"context"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/golang/glog"
	"github.com/google/certificate-transparency-go"
	"github.com/google/certificate-transparency-go/ctutil"
	"github.com/google/certificate-transparency-go/loglist"
	"github.com/google/certificate-transparency-go/x509"
	"github.com/google/certificate-transparency-go/x509util"
)

var (
	logList  = flag.String("log_list", loglist.LogListURL, "Location of master CT log list (URL or filename)")
	deadline = flag.Duration("deadline", 30*time.Second, "Timeout deadline for HTTP requests")
	useDNS   = flag.Bool("dns", false, "Use DNS access points for inclusion checking")
)

type logInfoFactory func(*loglist.Log, *http.Client) (*ctutil.LogInfo, error)

func main() {
	flag.Parse()
	ctx := context.Background()
	hc := &http.Client{Timeout: *deadline}

	llData, err := x509util.ReadFileOrURL(*logList, hc)
	if err != nil {
		glog.Exitf("Failed to read log list: %v", err)
	}
	ll, err := loglist.NewFromJSON(llData)
	if err != nil {
		glog.Exitf("Failed to parse log list: %v", err)
	}

	lf := ctutil.NewLogInfo
	if *useDNS {
		lf = ctutil.NewLogInfoOverDNSWrapper
	}

	totalInvalid := 0
	for _, arg := range flag.Args() {
		var chain []*x509.Certificate
		var valid, invalid int
		if strings.HasPrefix(arg, "https://") {
			// Get chain served online for TLS connection to site, and check any SCTs
			// provided alongside on the connection along the way.
			chain, valid, invalid, err = getAndCheckSiteChain(ctx, lf, arg, ll, hc)
			if err != nil {
				glog.Errorf("%s: failed to get cert chain: %v", arg, err)
				continue
			}
			glog.Errorf("Found %d external SCTs for %q, of which %d were validated", (valid + invalid), arg, valid)
			totalInvalid += invalid
		} else {
			// Treat the argument as a certificate file to load.
			data, err := ioutil.ReadFile(arg)
			if err != nil {
				glog.Errorf("%s: failed to read data: %v", arg, err)
				continue
			}
			chain, err = x509util.CertificatesFromPEM(data)
			if err != nil {
				glog.Errorf("%s: failed to read cert data: %v", arg, err)
				continue
			}
		}
		if len(chain) == 0 {
			glog.Errorf("%s: no certificates found", arg)
			continue
		}
		// Check the chain for embedded SCTs.
		valid, invalid = checkChain(ctx, lf, chain, ll, hc)
		glog.Errorf("Found %d embedded SCTs for %q, of which %d were validated", (valid + invalid), arg, valid)
		totalInvalid += invalid
	}
	if totalInvalid > 0 {
		os.Exit(1)
	}
}

// checkChain iterates over any embedded SCTs in the leaf certificate of the chain
// and checks those SCTs.  Returns the counts of valid and invalid embedded SCTs found.
func checkChain(ctx context.Context, lf logInfoFactory, chain []*x509.Certificate, ll *loglist.LogList, hc *http.Client) (int, int) {
	leaf := chain[0]
	if len(leaf.SCTList.SCTList) == 0 {
		return 0, 0
	}

	var issuer *x509.Certificate
	if len(chain) < 2 {
		glog.Info("No issuer in chain; attempting online retrieval")
		var err error
		issuer, err = x509util.GetIssuer(leaf, hc)
		if err != nil {
			glog.Errorf("Failed to get issuer online: %v", err)
		}
	} else {
		issuer = chain[1]
	}

	// Build a Merkle leaf that corresponds to the embedded SCTs.  We can use the same
	// leaf for all of the SCTs, as long as the timestamp field gets updated.
	merkleLeaf, err := ct.MerkleTreeLeafForEmbeddedSCT([]*x509.Certificate{leaf, issuer}, 0)
	if err != nil {
		glog.Errorf("Failed to build Merkle leaf: %v", err)
		return 0, len(leaf.SCTList.SCTList)
	}

	var valid, invalid int
	for i, sctData := range leaf.SCTList.SCTList {
		subject := fmt.Sprintf("embedded SCT[%d]", i)
		if checkSCT(ctx, lf, subject, merkleLeaf, &sctData, ll, hc) {
			valid++
		} else {
			invalid++
		}
	}
	return valid, invalid
}

// getAndCheckSiteChain retrieves and returns the chain of certificates presented
// for an HTTPS site.  Along the way it checks any external SCTs that are served
// up on the connection alongside the chain.  Returns the chain and counts of
// valid and invalid external SCTs found.
func getAndCheckSiteChain(ctx context.Context, lf logInfoFactory, target string, ll *loglist.LogList, hc *http.Client) ([]*x509.Certificate, int, int, error) {
	u, err := url.Parse(target)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("failed to parse URL: %v", err)
	}
	if u.Scheme != "https" {
		return nil, 0, 0, errors.New("non-https URL provided")
	}
	host := u.Host
	if !strings.Contains(host, ":") {
		host += ":443"
	}

	glog.Infof("Retrieve certificate chain from TLS connection to %q", host)
	dialer := net.Dialer{Timeout: hc.Timeout}
	conn, err := tls.DialWithDialer(&dialer, "tcp", host, &tls.Config{InsecureSkipVerify: true})
	if err != nil {
		return nil, 0, 0, fmt.Errorf("failed to dial %q: %v", host, err)
	}
	defer conn.Close()

	goChain := conn.ConnectionState().PeerCertificates
	glog.Infof("Found chain of length %d", len(goChain))

	// Convert base crypto/x509.Certificates to our forked x509.Certificate type.
	chain := make([]*x509.Certificate, len(goChain))
	for i, goCert := range goChain {
		cert, err := x509.ParseCertificate(goCert.Raw)
		if err != nil {
			return nil, 0, 0, fmt.Errorf("failed to convert Go Certificate [%d]: %v", i, err)
		}
		chain[i] = cert
	}

	// Check externally-provided SCTs.
	var valid, invalid int
	scts := conn.ConnectionState().SignedCertificateTimestamps
	if len(scts) > 0 {
		merkleLeaf, err := ct.MerkleTreeLeafFromChain(chain, ct.X509LogEntryType, 0 /* timestamp added later */)
		if err != nil {
			glog.Errorf("Failed to build Merkle tree leaf: %v", err)
			return chain, 0, len(scts), nil
		}
		for i, sctData := range scts {
			subject := fmt.Sprintf("external SCT[%d]", i)
			if checkSCT(ctx, lf, subject, merkleLeaf, &x509.SerializedSCT{Val: sctData}, ll, hc) {
				valid++
			} else {
				invalid++
			}

		}
	}

	return chain, valid, invalid, nil
}

// checkSCT performs checks on an SCT and Merkle tree leaf, performing both
// signature validation and online log inclusion checking.  Returns whether
// the SCT is valid.
func checkSCT(ctx context.Context, liFactory logInfoFactory, subject string, merkleLeaf *ct.MerkleTreeLeaf, sctData *x509.SerializedSCT, ll *loglist.LogList, hc *http.Client) bool {
	sct, err := x509util.ExtractSCT(sctData)
	if err != nil {
		glog.Errorf("Failed to deserialize %s data: %v", subject, err)
		glog.Errorf("Data: %x", sctData.Val)
		return false
	}
	glog.Infof("Examine %s with timestamp: %d (%v) from logID: %x", subject, sct.Timestamp, ct.TimestampToTime(sct.Timestamp), sct.LogID.KeyID[:])
	log := ll.FindLogByKeyHash(sct.LogID.KeyID)
	if log == nil {
		glog.Warningf("Unknown logID: %x, cannot validate %s", sct.LogID, subject)
		return false
	}
	logInfo, err := liFactory(log, hc)
	if err != nil {
		glog.Errorf("Failed to build log info for %q log: %v", log.Description, err)
		return false
	}

	glog.Infof("Validate %s against log %q...", subject, logInfo.Description)
	if err := logInfo.VerifySCTSignature(*sct, *merkleLeaf); err != nil {
		glog.Errorf("Failed to verify %s signature from log %q: %v", subject, log.Description, err)
	} else {
		glog.Infof("Validate %s against log %q... validated", subject, log.Description)
	}

	glog.Infof("Check %s inclusion against log %q...", subject, log.Description)
	index, err := logInfo.VerifyInclusion(ctx, *merkleLeaf, sct.Timestamp)
	if err != nil {
		age := time.Now().Sub(ct.TimestampToTime(sct.Timestamp))
		if age < logInfo.MMD {
			glog.Warningf("Failed to verify inclusion proof (%v) but %s timestamp is only %v old, less than log's MMD of %d seconds", err, subject, age, log.MaximumMergeDelay)
		} else {
			glog.Errorf("Failed to verify inclusion proof for %s: %v", subject, err)
		}
		return false
	}
	glog.Infof("Check %s inclusion against log %q... included at %d", subject, log.Description, index)
	return true
}
