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

package x509ext_test

import (
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/certificate-transparency-go"
	"github.com/google/certificate-transparency-go/gossip/minimal/x509ext"
	"github.com/google/certificate-transparency-go/tls"
	"github.com/google/certificate-transparency-go/x509"
	"github.com/google/certificate-transparency-go/x509/pkix"
)

var (
	// pilotPubKeyPEM is the public key for Google's Pilot log.
	pilotPubKeyPEM = []byte(`-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEfahLEimAoz2t01p3uMziiLOl/fHT
DM0YDOhBRuiBARsV4UvxG2LdNgoIGLrtCzWE0J5APC2em4JlvR8EEEFMoA==
-----END PUBLIC KEY-----`)
)

func TestSTHFromCert(t *testing.T) {
	rawPubKey, _ := pem.Decode(pilotPubKeyPEM)
	pubKey, _, _, err := ct.PublicKeyFromPEM(pilotPubKeyPEM)
	if err != nil {
		t.Fatalf("failed to decode test pubkey data: %v", err)
	}
	validSTH := x509ext.LogSTHInfo{
		LogURL:    []byte("http://ct.example.com/log"),
		Version:   0,
		TreeSize:  7834120,
		Timestamp: 1519395540364,
		SHA256RootHash: [...]byte{
			0xfe, 0xc0, 0xed, 0xe1, 0xbe, 0xf1, 0xa2, 0x25, 0xc3, 0x72, 0xa6, 0x44, 0x1b, 0xa2, 0xd5, 0xdd, 0x3b, 0xbb, 0x9b, 0x7b, 0xa9, 0x79, 0xd1, 0xa7, 0x03, 0xe7, 0xfe, 0x81, 0x49, 0x75, 0x85, 0xfb,
		},
		TreeHeadSignature: ct.DigitallySigned{
			Algorithm: tls.SignatureAndHashAlgorithm{Hash: tls.SHA256, Signature: tls.ECDSA},
			Signature: dehex("220164e031604aa2a0b68887ba668cefb3e0046e455d6323c3df38b8d50108895d70220146199ee1d759a029d8b37ce8701d2ca47a387bad8ac8ef1cb84b77bc0820ed"),
		},
	}
	sthData, err := tls.Marshal(validSTH)
	if err != nil {
		t.Fatalf("failed to marshal STH: %v", err)
	}

	var tests = []struct {
		name    string
		cert    x509.Certificate
		wantErr string
	}{
		{
			name: "ValidSTH",
			cert: x509.Certificate{
				NotBefore:               time.Now(),
				NotAfter:                time.Now().Add(24 * time.Hour),
				PublicKey:               pubKey,
				RawSubjectPublicKeyInfo: rawPubKey.Bytes,
				Subject: pkix.Name{
					CommonName: "Test STH holder",
				},
				Extensions: []pkix.Extension{
					{Id: x509ext.OIDExtensionCTSTH, Critical: false, Value: sthData},
				},
			},
		},
		{
			name: "MissingSTH",
			cert: x509.Certificate{
				NotBefore: time.Now(),
				NotAfter:  time.Now().Add(24 * time.Hour),
				Subject: pkix.Name{
					CommonName: "Test STH holder",
				},
			},
			wantErr: "no STH extension found",
		},
		{
			name: "TrailingData",
			cert: x509.Certificate{
				NotBefore: time.Now(),
				NotAfter:  time.Now().Add(24 * time.Hour),
				Subject: pkix.Name{
					CommonName: "Test STH holder",
				},
				Extensions: []pkix.Extension{
					{Id: x509ext.OIDExtensionCTSTH, Critical: false, Value: append(sthData, 0xff)},
				},
			},
			wantErr: "trailing data",
		},
		{
			name: "InvalidSTH",
			cert: x509.Certificate{
				NotBefore: time.Now(),
				NotAfter:  time.Now().Add(24 * time.Hour),
				Subject: pkix.Name{
					CommonName: "Test STH holder",
				},
				Extensions: []pkix.Extension{
					{Id: x509ext.OIDExtensionCTSTH, Critical: false, Value: []byte{0xff}},
				},
			},
			wantErr: "failed to unmarshal",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := x509ext.STHFromCert(&test.cert)
			if err != nil {
				if test.wantErr == "" {
					t.Errorf("STHFromCert(%+v)=nil,%v; want _,nil", test.cert, err)
				} else if !strings.Contains(err.Error(), test.wantErr) {
					t.Errorf("STHFromCert(%+v)=nil,%v; want nil,err containing %q", test.cert, err, test.wantErr)
				}
				return
			}
			if test.wantErr != "" {
				t.Errorf("STHFromCert(%+v)=_,nil; want nil,err containing %q", test.cert, test.wantErr)
			}
			t.Logf("retrieved STH %+v", got)
		})
	}
}

func dehex(h string) []byte {
	d, err := hex.DecodeString(h)
	if err != nil {
		panic(fmt.Sprintf("hard-coded data %q failed to decode! %v", h, err))
	}
	return d
}
