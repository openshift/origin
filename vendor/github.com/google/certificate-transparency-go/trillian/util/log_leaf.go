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

package util

import (
	"crypto/sha256"

	"github.com/golang/glog"
	ct "github.com/google/certificate-transparency-go"
	"github.com/google/certificate-transparency-go/tls"
	"github.com/google/trillian"
)

// BuildLogLeaf returns a Trillian LogLeaf structure for a (pre-)cert and the
// chain of certificates leading it up to a known root.
func BuildLogLeaf(logPrefix string,
	merkleLeaf ct.MerkleTreeLeaf, leafIndex int64,
	cert ct.ASN1Cert, chain []ct.ASN1Cert, isPrecert bool,
) (trillian.LogLeaf, error) {
	leafData, err := tls.Marshal(merkleLeaf)
	if err != nil {
		glog.Warningf("%s: Failed to serialize Merkle leaf: %v", logPrefix, err)
		return trillian.LogLeaf{}, err
	}

	extraData, err := ExtraDataForChain(cert, chain, isPrecert)
	if err != nil {
		glog.Warningf("%s: Failed to serialize chain for ExtraData: %v", logPrefix, err)
		return trillian.LogLeaf{}, err
	}

	// leafIDHash allows Trillian to detect duplicate entries, so this should be
	// a hash over the cert data.
	leafIDHash := sha256.Sum256(cert.Data)

	return trillian.LogLeaf{
		LeafValue:        leafData,
		ExtraData:        extraData,
		LeafIndex:        leafIndex,
		LeafIdentityHash: leafIDHash[:],
	}, nil
}

// ExtraDataForChain creates the extra data associated with a log entry as
// described in RFC6962 section 4.6.
func ExtraDataForChain(cert ct.ASN1Cert, chain []ct.ASN1Cert, isPrecert bool) ([]byte, error) {
	var extra interface{}
	if isPrecert {
		// For a pre-cert, the extra data is a TLS-encoded PrecertChainEntry.
		extra = ct.PrecertChainEntry{
			PreCertificate:   cert,
			CertificateChain: chain,
		}
	} else {
		// For a certificate, the extra data is a TLS-encoded:
		//   ASN.1Cert certificate_chain<0..2^24-1>;
		// containing the chain after the leaf.
		extra = ct.CertificateChain{Entries: chain}
	}
	return tls.Marshal(extra)
}
