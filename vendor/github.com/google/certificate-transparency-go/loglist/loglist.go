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

// Package loglist allows parsing and searching of the master CT Log list.
package loglist

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/google/certificate-transparency-go/tls"
)

const (
	// LogListURL has the master URL for Google Chrome's log list.
	LogListURL = "https://www.gstatic.com/ct/log_list/log_list.json"
	// LogListSignatureURL has the URL for the signature over Google Chrome's log list.
	LogListSignatureURL = "https://www.gstatic.com/ct/log_list/log_list.sig"
)

// Manually mapped from https://www.gstatic.com/ct/log_list/log_list_schema.json

// LogList holds a collection of logs and their operators
type LogList struct {
	Logs      []Log      `json:"logs"`
	Operators []Operator `json:"operators"`
}

// Operator describes a log operator
type Operator struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// Log describes a log.
type Log struct {
	Description       string `json:"description"`
	Key               []byte `json:"key"`
	MaximumMergeDelay int    `json:"maximum_merge_delay"` // seconds
	OperatedBy        []int  `json:"operated_by"`         // List of log operators
	URL               string `json:"url"`
	FinalSTH          *STH   `json:"final_sth,omitempty"`
	DisqualifiedAt    int    `json:"disqualified_at,omitempty"`
	DNSAPIEndpoint    string `json:"dns_api_endpoint,omitempty"` // DNS API endpoint for the log
}

// STH describes a signed tree head from a log.
type STH struct {
	TreeSize          int    `json:"tree_size"`
	Timestamp         int    `json:"timestamp"`
	SHA256RootHash    []byte `json:"sha256_root_hash"`
	TreeHeadSignature []byte `json:"tree_head_signature"`
}

// NewFromJSON creates a LogList from JSON encoded data.
func NewFromJSON(llData []byte) (*LogList, error) {
	var ll LogList
	if err := json.Unmarshal(llData, &ll); err != nil {
		return nil, fmt.Errorf("failed to parse log list: %v", err)
	}
	return &ll, nil
}

// NewFromSignedJSON creates a LogList from JSON encoded data, checking a
// signature along the way. The signature data should be provided as the
// raw signature data.
func NewFromSignedJSON(llData, rawSig []byte, pubKey crypto.PublicKey) (*LogList, error) {
	sigAlgo := tls.Anonymous
	switch pkType := pubKey.(type) {
	case *rsa.PublicKey:
		sigAlgo = tls.RSA
	case *ecdsa.PublicKey:
		sigAlgo = tls.ECDSA
	default:
		return nil, fmt.Errorf("Unsupported public key type %v", pkType)
	}
	tlsSig := tls.DigitallySigned{
		Algorithm: tls.SignatureAndHashAlgorithm{
			Hash:      tls.SHA256,
			Signature: sigAlgo,
		},
		Signature: rawSig,
	}
	if err := tls.VerifySignature(pubKey, llData, tlsSig); err != nil {
		return nil, fmt.Errorf("failed to verify signature: %v", err)
	}
	return NewFromJSON(llData)
}

// FindLogByName returns all logs whose names contain the given string.
func (ll *LogList) FindLogByName(name string) []*Log {
	name = strings.ToLower(name)
	var results []*Log
	for _, log := range ll.Logs {
		if strings.Contains(strings.ToLower(log.Description), name) {
			log := log
			results = append(results, &log)
		}
	}
	return results
}

// FindLogByURL finds the log with the given URL.
func (ll *LogList) FindLogByURL(url string) *Log {
	for _, log := range ll.Logs {
		// Don't count trailing slashes
		if strings.TrimRight(log.URL, "/") == strings.TrimRight(url, "/") {
			return &log
		}
	}
	return nil
}

// FindLogByKeyHash finds the log with the given key hash.
func (ll *LogList) FindLogByKeyHash(keyhash [sha256.Size]byte) *Log {
	for _, log := range ll.Logs {
		h := sha256.Sum256(log.Key)
		if bytes.Equal(h[:], keyhash[:]) {
			return &log
		}
	}
	return nil
}

// FindLogByKeyHashPrefix finds all logs whose key hash starts with the prefix.
func (ll *LogList) FindLogByKeyHashPrefix(prefix string) []*Log {
	var results []*Log
	for _, log := range ll.Logs {
		h := sha256.Sum256(log.Key)
		hh := hex.EncodeToString(h[:])
		if strings.HasPrefix(hh, prefix) {
			log := log
			results = append(results, &log)
		}
	}
	return results
}

// FindLogByKey finds the log with the given DER-encoded key.
func (ll *LogList) FindLogByKey(key []byte) *Log {
	for _, log := range ll.Logs {
		if bytes.Equal(log.Key[:], key) {
			return &log
		}
	}
	return nil
}

var hexDigits = regexp.MustCompile("^[0-9a-fA-F]+$")

// FuzzyFindLog tries to find logs that match the given unspecified input,
// whose format is unspecified.  This generally returns a single log, but
// if text input that matches multiple log descriptions is provided, then
// multiple logs may be returned.
func (ll *LogList) FuzzyFindLog(input string) []*Log {
	input = strings.Trim(input, " \t")
	if logs := ll.FindLogByName(input); len(logs) > 0 {
		return logs
	}
	if log := ll.FindLogByURL(input); log != nil {
		return []*Log{log}
	}
	// Try assuming the input is binary data of some form.  First base64:
	if data, err := base64.StdEncoding.DecodeString(input); err == nil {
		if len(data) == sha256.Size {
			var hash [sha256.Size]byte
			copy(hash[:], data)
			if log := ll.FindLogByKeyHash(hash); log != nil {
				return []*Log{log}
			}
		}
		if log := ll.FindLogByKey(data); log != nil {
			return []*Log{log}
		}
	}
	// Now hex, but strip all internal whitespace first.
	input = stripInternalSpace(input)
	if data, err := hex.DecodeString(input); err == nil {
		if len(data) == sha256.Size {
			var hash [sha256.Size]byte
			copy(hash[:], data)
			if log := ll.FindLogByKeyHash(hash); log != nil {
				return []*Log{log}
			}
		}
		if log := ll.FindLogByKey(data); log != nil {
			return []*Log{log}
		}
	}
	// Finally, allow hex strings with an odd number of digits.
	if hexDigits.MatchString(input) {
		if logs := ll.FindLogByKeyHashPrefix(input); len(logs) > 0 {
			return logs
		}
	}

	return nil
}

func stripInternalSpace(input string) string {
	return strings.Map(func(r rune) rune {
		if !unicode.IsSpace(r) {
			return r
		}
		return -1
	}, input)
}
