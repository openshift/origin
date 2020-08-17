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
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/certificate-transparency-go/client"
	"github.com/google/certificate-transparency-go/jsonclient"
	"github.com/google/certificate-transparency-go/x509"
	"github.com/google/certificate-transparency-go/x509util"
)

func testGossiper(ctx context.Context, t *testing.T) *Gossiper {
	t.Helper()
	g, err := NewGossiperFromFile(ctx, "testdata/test.cfg", nil)
	if err != nil {
		t.Fatalf("failed to create Gossiper for test: %v", err)
	}
	return g
}

func TestCheckRootIncluded(t *testing.T) {
	ctx := context.Background()
	g := testGossiper(ctx, t)

	var tests = []struct {
		name    string
		handler http.HandlerFunc
		wantErr string
	}{
		{
			name: "ValidResponse",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				fmt.Fprintln(w, fmt.Sprintf(`{"certificates":["%s"]}`, base64.StdEncoding.EncodeToString(g.root.Raw)))
			},
		},
		{
			name: "ErrorResponse",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				http.Error(w, "Or coffee", http.StatusTeapot)
			},
			wantErr: "teapot",
		},
		{
			name: "MalformedResponse",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				fmt.Fprintln(w, "Malformed response")
			},
			wantErr: "failed to get accepted roots",
		},
		{
			name: "ValidEmptyResponse",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				fmt.Fprintln(w, `{"certificates":[]}`)
			},
			wantErr: "gossip root not found",
		},
		{
			name: "ValidResponseWithoutRoot",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				fmt.Fprintln(w, `{"certificates":["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"]}`)
			},
			wantErr: "gossip root not found",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			s := httptest.NewServer(test.handler)
			defer s.Close()

			// Override the default client
			dest := g.dests["theDestinationOfAllSTHs"]
			client, err := client.New(s.URL+"/ct/v1/get-roots", nil, jsonclient.Options{})
			if err != nil {
				t.Fatalf("failed to create log client for %q: %v", dest.Name, err)
			}
			dest.Log = client

			if err = g.CheckRootIncluded(ctx); err != nil {
				if test.wantErr == "" {
					t.Errorf("CertRootIncluded()=nil,%v; want _,nil", err)
				} else if !strings.Contains(err.Error(), test.wantErr) {
					t.Errorf("CertRootIncluded()=%v; want err containing %q", err, test.wantErr)
				}
				return
			}
			if test.wantErr != "" {
				t.Errorf("CertRootIncluded()=nil; want err containing %q", test.wantErr)
			}
		})
	}
}

func TestGetSTHAsCert(t *testing.T) {
	ctx := context.Background()
	g := testGossiper(ctx, t)

	var tests = []struct {
		name    string
		handler http.HandlerFunc
		wantErr string
	}{
		{
			name: "ValidResponse",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				fmt.Fprintln(w, `{"tree_size":7834486,"timestamp":1519488735960,"sha256_root_hash":"GCO5SOm/jjQYiBC9PUFFX0pIhS8+vP2YzAUWejiLVxI=","tree_head_signature":"BAMARzBFAiEAx74xiiZY0Erq07ASVH5hEfGtaLiGScFVy253YWJYSEUCICh59Qp0pokKQE4oJgbXGBTRbYrroaKDuoFWb5tcf2sU"}`)
			},
		},
		{
			name: "ErrorResponse",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				http.Error(w, "Or coffee", http.StatusTeapot)
			},
			wantErr: "teapot",
		},
		{
			name: "MalformedResponse",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				fmt.Fprintln(w, "Malformed response")
			},
			wantErr: "invalid",
		},
		{
			name: "ValidEmptyResponse",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				fmt.Fprintln(w, `{}`)
			},
			wantErr: "invalid length",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			s := httptest.NewServer(test.handler)
			defer s.Close()

			// Override the default client
			src, ok := g.srcs["theSourceOfAllSTHs"]
			if !ok {
				t.Fatalf("failed to find destination log")
			}
			client, err := client.New(s.URL+"/ct/v1/get-sth", nil, jsonclient.Options{})
			if err != nil {
				t.Fatalf("failed to create log client for %q: %v", src.Name, err)
			}
			src.Log = client

			gotRaw, err := src.GetSTHAsCert(ctx, g)
			if err != nil {
				if test.wantErr == "" {
					t.Errorf("GetSTHAsCert()=nil,%v; want _,nil", err)
				} else if !strings.Contains(err.Error(), test.wantErr) {
					t.Errorf("GetSTHAsCert()=%v; want err containing %q", err, test.wantErr)
				}
				return
			}
			if test.wantErr != "" {
				t.Errorf("GetSTHAsCert()=nil; want err containing %q", test.wantErr)
			}
			gotCert, err := x509.ParseCertificate(gotRaw.Data)
			if err != nil {
				t.Errorf("GetSTHAsCert() gave unparseable cert: %v", err)
				return
			}
			t.Logf("retrieved:\n%s", x509util.CertificateToString(gotCert))

			// Second retrieval should give nothing as the result is the same
			gotRaw2, err := src.GetSTHAsCert(ctx, g)
			if err != nil || gotRaw2 != nil {
				t.Errorf("GetSTHAsCert(second)=%v,%v; want nil,nil", gotRaw2, err)
				return
			}
		})
	}
}
