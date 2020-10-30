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
	"encoding/hex"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/google/certificate-transparency-go"
	"github.com/google/certificate-transparency-go/gossip/minimal/x509ext"
	"github.com/google/certificate-transparency-go/tls"
	"github.com/google/certificate-transparency-go/x509"
	"github.com/google/certificate-transparency-go/x509util"
)

func TestCertForSTH(t *testing.T) {
	ctx := context.Background()
	g := testGossiper(ctx, t)
	var tests = []struct {
		name    string
		sth     ct.SignedTreeHead
		wantErr string
	}{
		{
			name:    "MarshalFailure",
			sth:     ct.SignedTreeHead{Version: 256},
			wantErr: "value 256 too large",
		},
		{
			name: "ValidSTH",
			sth: ct.SignedTreeHead{
				Version:   0,
				TreeSize:  7834120,
				Timestamp: 1519395540364,
				SHA256RootHash: [32]byte{
					0xfe, 0xc0, 0xed, 0xe1, 0xbe, 0xf1, 0xa2, 0x25, 0xc3, 0x72, 0xa6, 0x44, 0x1b, 0xa2, 0xd5, 0xdd, 0x3b, 0xbb, 0x9b, 0x7b, 0xa9, 0x79, 0xd1, 0xa7, 0x03, 0xe7, 0xfe, 0x81, 0x49, 0x75, 0x85, 0xfb,
				},
				TreeHeadSignature: ct.DigitallySigned{
					Algorithm: tls.SignatureAndHashAlgorithm{Hash: tls.SHA256, Signature: tls.ECDSA},
					Signature: dehex("220164e031604aa2a0b68887ba668cefb3e0046e455d6323c3df38b8d50108895d70220146199ee1d759a029d8b37ce8701d2ca47a387bad8ac8ef1cb84b77bc0820ed"),
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			src := g.srcs["theSourceOfAllSTHs"]
			got, err := src.CertForSTH(&test.sth, g)
			if err != nil {
				if test.wantErr == "" {
					t.Errorf("CertForSTH(%+v)=nil,%v; want _,nil", test.sth, err)
				} else if !strings.Contains(err.Error(), test.wantErr) {
					t.Errorf("CertForSTH(%+v)=nil,%v; want nil,err containing %q", test.sth, err, test.wantErr)
				}
				return
			}
			if test.wantErr != "" {
				t.Errorf("CertForSTH(%+v)=_,nil; want nil,err containing %q", test.sth, test.wantErr)
			}
			leaf, _ := x509.ParseCertificate(got.Data)
			t.Logf("created Leaf:\n%s", x509util.CertificateToString(leaf))
			// Check we can extract the same STH
			gotSTH, err := x509ext.STHFromCert(leaf)
			if err != nil {
				t.Errorf("STHFromCert(leaf)=nil,%v; want _,nil", err)
				return
			}
			if !reflect.DeepEqual(gotSTH, &test.sth) {
				t.Errorf("STHFromCert(leaf)=%+v; want %+v", gotSTH, test.sth)
			}
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
