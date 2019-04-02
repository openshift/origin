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

package ctutil

import (
	"encoding/base64"
	"testing"

	ct "github.com/google/certificate-transparency-go"
	"github.com/google/certificate-transparency-go/testdata"
	"github.com/google/certificate-transparency-go/tls"
	"github.com/google/certificate-transparency-go/x509util"
)

func TestLeafHash(t *testing.T) {
	tests := []struct {
		desc     string
		chainPEM string
		sct      []byte
		embedded bool
		want     string
	}{
		{
			desc:     "cert",
			chainPEM: testdata.TestCertPEM + testdata.CACertPEM,
			sct:      testdata.TestCertProof,
			want:     testdata.TestCertB64LeafHash,
		},
		{
			desc:     "precert",
			chainPEM: testdata.TestPreCertPEM + testdata.CACertPEM,
			sct:      testdata.TestPreCertProof,
			want:     testdata.TestPreCertB64LeafHash,
		},
		{
			desc:     "cert with embedded SCT",
			chainPEM: testdata.TestEmbeddedCertPEM + testdata.CACertPEM,
			sct:      testdata.TestPreCertProof,
			embedded: true,
			want:     testdata.TestPreCertB64LeafHash,
		},
	}

	for _, test := range tests {
		// Parse chain
		chain, err := x509util.CertificatesFromPEM([]byte(test.chainPEM))
		if err != nil {
			t.Errorf("%s: error parsing certificate chain: %s", test.desc, err)
			continue
		}

		// Parse SCT
		var sct ct.SignedCertificateTimestamp
		if _, err = tls.Unmarshal(test.sct, &sct); err != nil {
			t.Errorf("%s: error tls-unmarshalling sct: %s", test.desc, err)
			continue
		}

		// Test LeafHash()
		wantSl, err := base64.StdEncoding.DecodeString(test.want)
		if err != nil {
			t.Fatalf("%s: error base64-decoding leaf hash %q: %s", test.desc, test.want, err)
		}
		var want [32]byte
		copy(want[:], wantSl)

		got, err := LeafHash(chain, &sct, test.embedded)
		if got != want || err != nil {
			t.Errorf("%s: LeafHash(_,_) = %v, %v, want %v, nil", test.desc, got, err, want)
		}

		// Test LeafHashB64()
		gotB64, err := LeafHashB64(chain, &sct, test.embedded)
		if gotB64 != test.want || err != nil {
			t.Errorf("%s: LeafHashB64(_,_) = %v, %v, want %v, nil", test.desc, gotB64, err, test.want)
		}
	}
}

func TestLeafHashErrors(t *testing.T) {
	tests := []struct {
		desc     string
		chainPEM string
		sct      []byte
		embedded bool
	}{
		{
			desc:     "empty chain",
			chainPEM: "",
			sct:      testdata.TestCertProof,
		},
		{
			desc:     "nil SCT",
			chainPEM: testdata.TestCertPEM + testdata.CACertPEM,
			sct:      nil,
		},
		{
			desc:     "no SCTs embedded in cert, embedded true",
			chainPEM: testdata.TestCertPEM + testdata.CACertPEM,
			sct:      testdata.TestInvalidProof,
			embedded: true,
		},
		{
			desc:     "cert contains embedded SCTs, but not the SCT provided",
			chainPEM: testdata.TestEmbeddedCertPEM + testdata.CACertPEM,
			sct:      testdata.TestInvalidProof,
			embedded: true,
		},
	}

	for _, test := range tests {
		// Parse chain
		chain, err := x509util.CertificatesFromPEM([]byte(test.chainPEM))
		if err != nil {
			t.Errorf("%s: error parsing certificate chain: %s", test.desc, err)
			continue
		}

		// Parse SCT
		var sct *ct.SignedCertificateTimestamp
		if test.sct != nil {
			sct = &ct.SignedCertificateTimestamp{}
			if _, err = tls.Unmarshal(test.sct, sct); err != nil {
				t.Errorf("%s: error tls-unmarshalling sct: %s", test.desc, err)
				continue
			}
		}

		// Test LeafHash()
		got, err := LeafHash(chain, sct, test.embedded)
		if got != emptyHash || err == nil {
			t.Errorf("%s: LeafHash(_,_) = %s, %v, want %v, error", test.desc, got, err, emptyHash)
		}

		// Test LeafHashB64()
		gotB64, err := LeafHashB64(chain, sct, test.embedded)
		if gotB64 != "" || err == nil {
			t.Errorf("%s: LeafHashB64(_,_) = %s, %v, want \"\", error", test.desc, gotB64, err)
		}
	}
}

func TestVerifySCT(t *testing.T) {
	tests := []struct {
		desc     string
		chainPEM string
		sct      []byte
		embedded bool
		wantErr  bool
	}{
		{
			desc:     "cert",
			chainPEM: testdata.TestCertPEM + testdata.CACertPEM,
			sct:      testdata.TestCertProof,
		},
		{
			desc:     "precert",
			chainPEM: testdata.TestPreCertPEM + testdata.CACertPEM,
			sct:      testdata.TestPreCertProof,
		},
		{
			desc:     "invalid SCT",
			chainPEM: testdata.TestPreCertPEM + testdata.CACertPEM,
			sct:      testdata.TestCertProof,
			wantErr:  true,
		},
		{
			desc:     "cert with embedded SCT",
			chainPEM: testdata.TestEmbeddedCertPEM + testdata.CACertPEM,
			sct:      testdata.TestPreCertProof,
			embedded: true,
		},
		{
			desc:     "cert with invalid embedded SCT",
			chainPEM: testdata.TestInvalidEmbeddedCertPEM + testdata.CACertPEM,
			sct:      testdata.TestInvalidProof,
			embedded: true,
			wantErr:  true,
		},
	}

	for _, test := range tests {
		// Parse chain
		chain, err := x509util.CertificatesFromPEM([]byte(test.chainPEM))
		if err != nil {
			t.Errorf("%s: error parsing certificate chain: %s", test.desc, err)
			continue
		}

		// Parse SCT
		var sct ct.SignedCertificateTimestamp
		if _, err = tls.Unmarshal(test.sct, &sct); err != nil {
			t.Errorf("%s: error tls-unmarshalling sct: %s", test.desc, err)
			continue
		}

		// Test VerifySCT()
		pk, err := ct.PublicKeyFromB64(testdata.LogPublicKeyB64)
		if err != nil {
			t.Errorf("%s: error parsing public key: %s", test.desc, err)
		}

		err = VerifySCT(pk, chain, &sct, test.embedded)
		if gotErr := (err != nil); gotErr != test.wantErr {
			t.Errorf("%s: VerifySCT(_,_,_, %t) = %v, want error? %t", test.desc, test.embedded, err, test.wantErr)
		}
	}
}

func TestContainsSCT(t *testing.T) {
	tests := []struct {
		desc    string
		certPEM string
		sct     []byte
		want    bool
	}{
		{
			desc:    "cert doesn't contain any SCTs",
			certPEM: testdata.TestCertPEM,
			sct:     testdata.TestPreCertProof,
			want:    false,
		},
		{
			desc:    "cert contains SCT but not specified SCT",
			certPEM: testdata.TestEmbeddedCertPEM,
			sct:     testdata.TestInvalidProof,
			want:    false,
		},
		{
			desc:    "cert contains SCT",
			certPEM: testdata.TestEmbeddedCertPEM,
			sct:     testdata.TestPreCertProof,
			want:    true,
		},
	}

	for _, test := range tests {
		// Parse cert
		cert, err := x509util.CertificateFromPEM([]byte(test.certPEM))
		if err != nil {
			t.Errorf("%s: error parsing certificate: %s", test.desc, err)
			continue
		}

		// Parse SCT
		var sct ct.SignedCertificateTimestamp
		if _, err = tls.Unmarshal(test.sct, &sct); err != nil {
			t.Errorf("%s: error tls-unmarshalling sct: %s", test.desc, err)
			continue
		}

		// Test ContainsSCT()
		got, err := ContainsSCT(cert, &sct)
		if err != nil {
			t.Errorf("%s: ContainsSCT(_,_) = false, %s, want no error", test.desc, err)
			continue
		}

		if got != test.want {
			t.Errorf("%s: ContainsSCT(_,_) = %t, nil, want %t, nil", test.desc, got, test.want)
		}
	}
}
