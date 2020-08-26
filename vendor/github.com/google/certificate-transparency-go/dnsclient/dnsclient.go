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

// Package dnsclient is a client library for performing CT operations
// over DNS.  The DNS mechanism is experimental and subject to change.
package dnsclient

import (
	"context"
	"crypto/sha256"
	"encoding/base32"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"

	ct "github.com/google/certificate-transparency-go"
	"github.com/google/certificate-transparency-go/jsonclient"
	"github.com/google/certificate-transparency-go/tls"
)

// DNSClient represents a DNS client for a given CT Log instance
type DNSClient struct {
	base     string
	Verifier *ct.SignatureVerifier // nil for no verification (e.g. no public key available)
	resolve  func(ctx context.Context, name string) ([]string, error)
}

// New constructs a new DNSClient instance.  The base parameter gives the
// top-level domain name; opts can be used to provide a custom logger
// interface and a public key for signature verification.
func New(base string, opts jsonclient.Options) (*DNSClient, error) {
	pubkey, err := opts.ParsePublicKey()
	if err != nil {
		return nil, fmt.Errorf("invalid public key: %v", err)
	}

	var verifier *ct.SignatureVerifier
	if pubkey != nil {
		var err error
		verifier, err = ct.NewSignatureVerifier(pubkey)
		if err != nil {
			return nil, err
		}
	}
	if len(base) > 0 && base[len(base)-1] != '.' {
		base += "."
	}
	return &DNSClient{
		base:     base,
		Verifier: verifier,
		resolve:  func(ctx context.Context, name string) ([]string, error) { return net.LookupTXT(name) },
	}, nil
}

const (
	// Match base64 data (although note that this will also match invalid data with
	// padding in the middle rather than the end).
	base64RE = "[A-Za-z0-9+/=]+"
	// DNS results for get-sth have 5 matches: 1 for the overall match, plus 4
	// dot-delimited fields
	sthMatchCount = 1 + 4
)

var (
	sthTXT = regexp.MustCompile(`^(\d+)\.(\d+)\.(` + base64RE + `)\.(` + base64RE + `)`)
)

// BaseURI returns a base dns: URI (cf. RFC 4501) that DNS queries will be built on.
func (c *DNSClient) BaseURI() string {
	return fmt.Sprintf("dns:%s", c.base)
}

// GetSTH retrieves the current STH from the log.
func (c *DNSClient) GetSTH(ctx context.Context) (*ct.SignedTreeHead, error) {
	name := "sth." + c.base
	results, err := c.resolve(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("lookup for %q failed: %v", name, err)
	}
	result := strings.Join(results, "")
	matches := sthTXT.FindStringSubmatch(result)
	if matches == nil || len(matches) < sthMatchCount {
		return nil, fmt.Errorf("failed to parse result %q", result)
	}
	treeSize, err := strconv.ParseUint(matches[1], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse tree size %q", matches[1])
	}
	timestamp, err := strconv.ParseUint(matches[2], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse timestamp %q", matches[2])
	}
	rootHash, err := base64.StdEncoding.DecodeString(matches[3])
	if err != nil {
		return nil, fmt.Errorf("failed to parse root hash %q", matches[3])
	}
	if len(rootHash) != sha256.Size {
		return nil, fmt.Errorf("wrong size root hash for %q: %d not %d", matches[3], len(rootHash), sha256.Size)
	}
	signature, err := base64.StdEncoding.DecodeString(matches[4])
	if err != nil {
		return nil, fmt.Errorf("failed to decode signature %q", matches[4])
	}

	sth := ct.SignedTreeHead{
		TreeSize:  treeSize,
		Timestamp: timestamp,
	}
	copy(sth.SHA256RootHash[:], rootHash)

	var ds ct.DigitallySigned
	if rest, err := tls.Unmarshal(signature, &ds); err != nil {
		return nil, fmt.Errorf("failed to parse signature: %v", err)
	} else if len(rest) > 0 {
		return nil, errors.New("trailing data in parse signature")
	}
	sth.TreeHeadSignature = ds
	if err := c.verifySTHSignature(sth); err != nil {
		return nil, fmt.Errorf("signature validation failed: %v", err)
	}
	return &sth, nil
}

// verifySTHSignature checks the signature in sth, returning any error
// encountered or nil if verification is successful.
func (c *DNSClient) verifySTHSignature(sth ct.SignedTreeHead) error {
	if c.Verifier == nil {
		// Can't verify signatures without a verifier
		return nil
	}
	return c.Verifier.VerifySTHSignature(sth)
}

// GetSTHConsistency retrieves the consistency proof between two snapshots.
func (c *DNSClient) GetSTHConsistency(ctx context.Context, first, second uint64) ([][]byte, error) {
	return c.getProof(ctx, fmt.Sprintf("%d.%d.sth-consistency.%s", first, second, c.base))
}

// GetProofByHash returns an audit path for the hash of an SCT.
func (c *DNSClient) GetProofByHash(ctx context.Context, hash []byte, treeSize uint64) (*ct.GetProofByHashResponse, error) {
	// First get the index of the entry.
	hash32 := base32.StdEncoding.EncodeToString(hash)
	// Drop any trailing padding.
	hash32 = strings.TrimRight(hash32, "=")
	name := fmt.Sprintf("%s.hash.%s", hash32, c.base)
	results, err := c.resolve(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("lookup failed for %q: %v", name, err)
	}
	result := strings.Join(results, "")
	leafIndex, err := strconv.ParseUint(result, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse result %q", result)
	}

	proof, err := c.getProof(ctx, fmt.Sprintf("%d.%d.tree.%s", leafIndex, treeSize, c.base))
	if err != nil {
		return nil, err
	}

	return &ct.GetProofByHashResponse{
		LeafIndex: int64(leafIndex),
		AuditPath: proof,
	}, nil
}

// TODO(drysdale): add an expectedProofSize parameter and pre-calculate the sizes
// of proof that are expected.
func (c *DNSClient) getProof(ctx context.Context, base string) ([][]byte, error) {
	var proof [][]byte
	for index := 0; index <= 255; {
		name := fmt.Sprintf("%d.%s", index, base)
		results, err := c.resolve(ctx, name)
		if err != nil {
			if index == 0 {
				return nil, fmt.Errorf("lookup for %q failed: %v", name, err)
			}
			// Assume that a failure to retrieve any more means the proof is complete.
			break
		}
		result := []byte(strings.Join(results, ""))
		// We expect the result to be a concatenation of hashes, and so should be a multiple of the hash size.
		if len(result)%sha256.Size != 0 {
			return nil, fmt.Errorf("unexpected length of data %d, not multiple of %d: %x", len(result), sha256.Size, result)
		}
		if len(result) == 0 {
			break
		}
		for start := 0; start < len(result); start += sha256.Size {
			s := result[start : start+sha256.Size]
			proof = append(proof, s)
			index++
		}
	}
	return proof, nil
}
