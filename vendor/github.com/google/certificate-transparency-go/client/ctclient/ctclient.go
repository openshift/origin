// Copyright 2016 Google Inc. All Rights Reserved.
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

// ctclient is a command-line utility for interacting with CT logs.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/pem"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	ct "github.com/google/certificate-transparency-go"
	"github.com/google/certificate-transparency-go/client"
	"github.com/google/certificate-transparency-go/dnsclient"
	"github.com/google/certificate-transparency-go/jsonclient"
	"github.com/google/certificate-transparency-go/loglist"
	"github.com/google/certificate-transparency-go/x509"
	"github.com/google/certificate-transparency-go/x509util"
	"github.com/google/trillian/merkle"
	"github.com/google/trillian/merkle/rfc6962"
)

var (
	dnsBase   = flag.String("dns_base", "", "Base DNS name for queries; if non-empty, DNS queries rather than HTTP will be used")
	useDNS    = flag.Bool("dns", false, "Use DNS access points for inclusion checking (requires --log_name or --dns_base)")
	logName   = flag.String("log_name", "", "Name of log to retrieve information from --log_list for")
	logList   = flag.String("log_list", loglist.LogListURL, "Location of master log list (URL or filename)")
	logURI    = flag.String("log_uri", "https://ct.googleapis.com/rocketeer", "CT log base URI")
	logMMD    = flag.Duration("log_mmd", 24*time.Hour, "Log's maximum merge delay")
	pubKey    = flag.String("pub_key", "", "Name of file containing log's public key")
	certChain = flag.String("cert_chain", "", "Name of file containing certificate chain as concatenated PEM files")
	textOut   = flag.Bool("text", true, "Display certificates as text")
	getFirst  = flag.Int64("first", -1, "First entry to get")
	getLast   = flag.Int64("last", -1, "Last entry to get")
	treeSize  = flag.Int64("size", -1, "Tree size to query at")
	treeHash  = flag.String("tree_hash", "", "Tree hash to check against (as hex string)")
	prevSize  = flag.Int64("prev_size", -1, "Previous tree size to get consistency against")
	prevHash  = flag.String("prev_hash", "", "Previous tree hash to check against (as hex string)")
	leafHash  = flag.String("leaf_hash", "", "Leaf hash to retrieve (as hex string)")
)

func signatureToString(signed *ct.DigitallySigned) string {
	return fmt.Sprintf("Signature: Hash=%v Sign=%v Value=%x", signed.Algorithm.Hash, signed.Algorithm.Signature, signed.Signature)
}

func getSTH(ctx context.Context, logClient client.CheckLogClient) {
	sth, err := logClient.GetSTH(ctx)
	if err != nil {
		log.Fatal(err)
	}
	// Display the STH
	when := ct.TimestampToTime(sth.Timestamp)
	fmt.Printf("%v (timestamp %d): Got STH for %v log (size=%d) at %v, hash %x\n", when, sth.Timestamp, sth.Version, sth.TreeSize, logClient.BaseURI(), sth.SHA256RootHash)
	fmt.Printf("%v\n", signatureToString(&sth.TreeHeadSignature))
}

func addChain(ctx context.Context, logClient *client.LogClient) {
	if *certChain == "" {
		log.Fatalf("No certificate chain file specified with -cert_chain")
	}
	rest, err := ioutil.ReadFile(*certChain)
	if err != nil {
		log.Fatalf("Failed to read certificate file: %v", err)
	}
	var chain []ct.ASN1Cert
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		if block.Type == "CERTIFICATE" {
			chain = append(chain, ct.ASN1Cert{Data: block.Bytes})
		}
	}
	if len(chain) == 0 {
		log.Fatalf("No certificates found in %s", *certChain)
	}

	// Examine the leaf to see if it looks like a pre-certificate.
	isPrecert := false
	leaf, err := x509.ParseCertificate(chain[0].Data)
	if err == nil {
		count, _ := x509util.OIDInExtensions(x509.OIDExtensionCTPoison, leaf.Extensions)
		if count > 0 {
			isPrecert = true
			fmt.Print("Uploading pre-certificate to log\n")
		}
	}

	var sct *ct.SignedCertificateTimestamp
	if isPrecert {
		sct, err = logClient.AddPreChain(ctx, chain)
	} else {
		sct, err = logClient.AddChain(ctx, chain)
	}
	if err != nil {
		if err, ok := err.(client.RspError); ok {
			log.Fatalf("Upload failed: %q, detail:\n  %s", err, string(err.Body))
		}
		log.Fatalf("Upload failed: %q", err)
	}
	// Calculate the leaf hash
	leafEntry := ct.CreateX509MerkleTreeLeaf(chain[0], sct.Timestamp)
	leafHash, err := ct.LeafHashForLeaf(leafEntry)
	if err != nil {
		log.Fatalf("Failed to create hash of leaf: %v", err)
	}

	// Display the SCT
	when := ct.TimestampToTime(sct.Timestamp)
	fmt.Printf("Uploaded chain of %d certs to %v log at %v, timestamp: %d (%v)\n", len(chain), sct.SCTVersion, logClient.BaseURI(), sct.Timestamp, when)
	fmt.Printf("LogID: %x\n", sct.LogID.KeyID[:])
	fmt.Printf("LeafHash: %x\n", leafHash)
	fmt.Printf("Signature: %v\n", signatureToString(&sct.Signature))

	age := time.Now().Sub(when)
	if age > *logMMD {
		// SCT's timestamp is old enough that the certificate should be included.
		getInclusionProofForHash(ctx, logClient, leafHash[:])
	}
}

func getRoots(ctx context.Context, logClient *client.LogClient) {
	roots, err := logClient.GetAcceptedRoots(ctx)
	if err != nil {
		log.Fatal(err)
	}
	for _, root := range roots {
		showRawCert(root)
	}
}

func getEntries(ctx context.Context, logClient *client.LogClient) {
	if *getFirst == -1 {
		log.Fatal("No -first option supplied")
	}
	if *getLast == -1 {
		*getLast = *getFirst
	}
	rsp, err := logClient.GetRawEntries(ctx, *getFirst, *getLast)
	if err != nil {
		log.Fatal(err)
	}

	for i, rawEntry := range rsp.Entries {
		index := *getFirst + int64(i)
		rle, err := ct.RawLogEntryFromLeaf(index, &rawEntry)
		if err != nil {
			fmt.Printf("Index=%d Failed to unmarshal leaf entry: %v", index, err)
			continue
		}

		ts := rle.Leaf.TimestampedEntry
		when := ct.TimestampToTime(ts.Timestamp)
		fmt.Printf("Index=%d Timestamp=%d (%v) ", rle.Index, ts.Timestamp, when)

		switch ts.EntryType {
		case ct.X509LogEntryType:
			fmt.Printf("X.509 certificate:\n")
			showRawCert(*ts.X509Entry)
		case ct.PrecertLogEntryType:
			fmt.Printf("pre-certificate from issuer with keyhash %x:\n", ts.PrecertEntry.IssuerKeyHash)
			showRawCert(rle.Cert) // As-submitted: with signature and poison.
		default:
			fmt.Printf("Unhandled log entry type %d\n", ts.EntryType)
		}
	}
}

func getInclusionProof(ctx context.Context, logClient client.CheckLogClient) {
	hash, err := hex.DecodeString(*leafHash)
	if err != nil || len(hash) != sha256.Size {
		log.Fatal("No valid --leaf_hash supplied in hex")
	}
	getInclusionProofForHash(ctx, logClient, hash)
}

func getInclusionProofForHash(ctx context.Context, logClient client.CheckLogClient, hash []byte) {
	var sth *ct.SignedTreeHead
	size := *treeSize
	if size <= 0 {
		var err error
		sth, err = logClient.GetSTH(ctx)
		if err != nil {
			log.Fatalf("Failed to get current STH: %v", err)
		}
		size = int64(sth.TreeSize)
	}
	// Display the inclusion proof.
	rsp, err := logClient.GetProofByHash(ctx, hash, uint64(size))
	if err != nil {
		log.Fatalf("Failed to get-proof-by-hash: %v", err)
	}
	fmt.Printf("Inclusion proof for index %d in tree of size %d:\n", rsp.LeafIndex, size)
	for _, e := range rsp.AuditPath {
		fmt.Printf("  %x\n", e)
	}
	if sth != nil {
		// If we retrieved an STH we can verify the proof.
		verifier := merkle.NewLogVerifier(rfc6962.DefaultHasher)
		if err := verifier.VerifyInclusionProof(rsp.LeafIndex, int64(sth.TreeSize), rsp.AuditPath, sth.SHA256RootHash[:], hash); err != nil {
			log.Fatalf("Failed to VerifyInclusionProof(%d, %d)=%v", rsp.LeafIndex, sth.TreeSize, err)
		}
		fmt.Printf("Verified that hash %x + proof = root hash %x\n", hash, sth.SHA256RootHash)
	}
}

func getConsistencyProof(ctx context.Context, logClient client.CheckLogClient) {
	if *treeSize <= 0 {
		log.Fatal("No valid --size supplied")
	}
	if *prevSize <= 0 {
		log.Fatal("No valid --prev_size supplied")
	}
	var hash1, hash2 []byte
	if *prevHash != "" {
		var err error
		hash1, err = hex.DecodeString(*prevHash)
		if err != nil {
			log.Fatalf("Invalid --prev_hash: %v", err)
		}
		if l := len(hash1); l != sha256.Size {
			log.Fatalf("Invalid --prev_hash length: %d", l)
		}
	}
	if *treeHash != "" {
		var err error
		hash2, err = hex.DecodeString(*treeHash)
		if err != nil {
			log.Fatalf("Invalid --tree_hash: %v", err)
		}
		if l := len(hash2); l != sha256.Size {
			log.Fatalf("invalid --tree_hash length: %d", l)
		}
	}
	if (hash1 != nil) != (hash2 != nil) {
		log.Fatalf("Need both --prev_hash and --tree_hash or neither")
	}
	getConsistencyProofBetween(ctx, logClient, *prevSize, *treeSize, hash1, hash2)
}

func getConsistencyProofBetween(ctx context.Context, logClient client.CheckLogClient, first, second int64, prevHash, treeHash []byte) {
	proof, err := logClient.GetSTHConsistency(ctx, uint64(first), uint64(second))
	if err != nil {
		if err, ok := err.(client.RspError); ok {
			log.Fatalf("get-sth-consistency failed: %q, detail:\n  %s", err, string(err.Body))
		}
		log.Fatalf("Failed to get-sth-consistency: %v", err)
	}
	fmt.Printf("Consistency proof from size %d to size %d:\n", first, second)
	for _, e := range proof {
		fmt.Printf("  %x\n", e)
	}
	if prevHash == nil || treeHash == nil {
		return
	}
	// We have tree hashes so we can verify the proof.
	verifier := merkle.NewLogVerifier(rfc6962.DefaultHasher)
	if err := verifier.VerifyConsistencyProof(first, second, prevHash, treeHash, proof); err != nil {
		log.Fatalf("Failed to VerifyConsistencyProof(%x @size=%d, %x @size=%d): %v", prevHash, first, treeHash, second, err)
	}
	fmt.Printf("Verified that hash %x @%d + proof = hash %x @%d\n", prevHash, first, treeHash, second)
}

func showRawCert(cert ct.ASN1Cert) {
	if *textOut {
		c, err := x509.ParseCertificate(cert.Data)
		if err != nil {
			log.Printf("Error parsing certificate: %q", err.Error())
		}
		if c == nil {
			return
		}
		showParsedCert(c)
	} else {
		showPEMData(cert.Data)
	}
}

func showParsedCert(cert *x509.Certificate) {
	if *textOut {
		fmt.Printf("%s\n", x509util.CertificateToString(cert))
	} else {
		showPEMData(cert.Raw)
	}
}

func showPEMData(data []byte) {
	if err := pem.Encode(os.Stdout, &pem.Block{Type: "CERTIFICATE", Bytes: data}); err != nil {
		log.Printf("Failed to PEM encode cert: %q", err.Error())
	}
}

func dieWithUsage(msg string) {
	fmt.Fprintln(os.Stderr, msg)
	fmt.Fprintf(os.Stderr, "Usage: ctclient [options] <cmd>\n"+
		"where cmd is one of:\n"+
		"   sth           retrieve signed tree head\n"+
		"   upload        upload cert chain and show SCT (needs -cert_chain)\n"+
		"   getroots      show accepted roots\n"+
		"   getentries    get log entries (needs -first and -last)\n"+
		"   inclusion     get inclusion proof (needs -leaf_hash and optionally -size)\n"+
		"   consistency   get consistency proof (needs -size and -prev_size, optionally -tree_hash and -prev_hash)\n")
	os.Exit(1)
}

func main() {
	flag.Parse()
	ctx := context.Background()
	httpClient := &http.Client{
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
	}
	var opts jsonclient.Options
	if *pubKey != "" {
		pubkey, err := ioutil.ReadFile(*pubKey)
		if err != nil {
			log.Fatal(err)
		}
		opts.PublicKey = string(pubkey)
	}

	uri := *logURI
	dns := *dnsBase
	if *logName != "" {
		llData, err := x509util.ReadFileOrURL(*logList, httpClient)
		if err != nil {
			log.Fatalf("Failed to read log list: %v", err)
		}
		ll, err := loglist.NewFromJSON(llData)
		if err != nil {
			log.Fatalf("Failed to build log list: %v", err)
		}

		logs := ll.FindLogByName(*logName)
		if len(logs) == 0 {
			log.Fatalf("No log with name like %q found in loglist %q", *logName, *logList)
		}
		if len(logs) > 1 {
			logNames := make([]string, len(logs))
			for i, log := range logs {
				logNames[i] = fmt.Sprintf("%q", log.Description)
			}
			log.Fatalf("Multiple logs with name like %q found in loglist: %s", *logName, strings.Join(logNames, ","))
		}
		uri = "https://" + logs[0].URL
		if *useDNS {
			dns = logs[0].DNSAPIEndpoint
		}
		if opts.PublicKey == "" {
			opts.PublicKeyDER = logs[0].Key
		}
	}
	if *useDNS && dns == "" {
		log.Fatal("DNS access requested (with --dns) but no DNS base name known")
	}

	var err error
	var logClient *client.LogClient
	var checkClient client.CheckLogClient
	if dns != "" {
		checkClient, err = dnsclient.New(dns, opts)
	} else {
		logClient, err = client.New(uri, httpClient, opts)
		checkClient = logClient
	}
	if err != nil {
		log.Fatal(err)
	}

	args := flag.Args()
	if len(args) != 1 {
		dieWithUsage("Need command argument")
	}
	cmd := args[0]
	switch cmd {
	case "sth":
		getSTH(ctx, checkClient)
	case "upload":
		if logClient == nil {
			log.Fatal("Cannot upload over DNS")
		}
		addChain(ctx, logClient)
	case "getroots", "get_roots", "get-roots":
		if logClient == nil {
			log.Fatal("Cannot retrieve roots over DNS")
		}
		getRoots(ctx, logClient)
	case "getentries", "get_entries", "get-entries":
		if logClient == nil {
			log.Fatal("Cannot get-entries over DNS")
		}
		getEntries(ctx, logClient)
	case "inclusion", "inclusion-proof":
		getInclusionProof(ctx, checkClient)
	case "consistency":
		getConsistencyProof(ctx, checkClient)
	default:
		dieWithUsage(fmt.Sprintf("Unknown command '%s'", cmd))
	}
}
