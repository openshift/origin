// Copyright 2017 Google Inc. All Rights Reserved.
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

package integration

import (
	"context"
	"crypto"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/google/certificate-transparency-go/client"
	"github.com/google/certificate-transparency-go/jsonclient"
	"github.com/google/certificate-transparency-go/tls"
	"github.com/google/certificate-transparency-go/trillian/ctfe/configpb"
	"github.com/google/certificate-transparency-go/x509"

	ct "github.com/google/certificate-transparency-go"
)

func TestHammer_NotAfter(t *testing.T) {
	keys := loadTestKeys(t)

	s, lc := newFakeCTServer(t)
	defer s.close()

	now := time.Now()
	notAfterStart := now.Add(-24 * time.Hour)
	notAfterOverride := now.Add(23 * time.Hour)
	notAfterLimit := now.Add(24 * time.Hour)

	ctx := context.Background()
	addChain := func(hs *hammerState) error { return hs.addChain(ctx) }
	addPreChain := func(hs *hammerState) error { return hs.addPreChain(ctx) }

	tests := []struct {
		desc                                           string
		fn                                             func(hs *hammerState) error
		notAfterOverride, notAfterStart, notAfterLimit time.Time
		// wantNotAfter is only checked if not zeroed
		wantNotAfter time.Time
	}{
		{
			desc: "nonTemporalAddChain",
			fn:   addChain,
		},
		{
			desc: "nonTemporalAddPreChain",
			fn:   addPreChain,
		},
		{
			desc:             "nonTemporalFixedAddChain",
			fn:               addChain,
			notAfterOverride: notAfterOverride,
			wantNotAfter:     notAfterOverride,
		},
		{
			desc:             "nonTemporalFixedAddPreChain",
			fn:               addPreChain,
			notAfterOverride: notAfterOverride,
			wantNotAfter:     notAfterOverride,
		},
		{
			desc:          "temporalAddChain",
			fn:            addChain,
			notAfterStart: notAfterStart,
			notAfterLimit: notAfterLimit,
		},
		{
			desc:          "temporalAddPreChain",
			fn:            addPreChain,
			notAfterStart: notAfterStart,
			notAfterLimit: notAfterLimit,
		},
		{
			desc:             "temporalFixedAddChain",
			fn:               addChain,
			notAfterOverride: notAfterOverride,
			notAfterStart:    notAfterStart,
			notAfterLimit:    notAfterLimit,
			wantNotAfter:     notAfterOverride,
		},
		{
			desc:             "temporalFixedAddPreChain",
			fn:               addPreChain,
			notAfterOverride: notAfterOverride,
			notAfterStart:    notAfterStart,
			notAfterLimit:    notAfterLimit,
			wantNotAfter:     notAfterOverride,
		},
	}

	for _, test := range tests {
		s.reset()

		var startPB, limitPB *timestamp.Timestamp
		if ts := test.notAfterStart; ts.UnixNano() > 0 {
			startPB, _ = ptypes.TimestampProto(ts)
		}
		if ts := test.notAfterLimit; ts.UnixNano() > 0 {
			limitPB, _ = ptypes.TimestampProto(ts)
		}
		hs, err := newHammerState(&HammerConfig{
			CACert:     keys.caCert,
			ClientPool: RandomPool{lc},
			LeafChain:  keys.leafChain,
			LeafCert:   keys.leafCert,
			LogCfg: &configpb.LogConfig{
				NotAfterStart: startPB,
				NotAfterLimit: limitPB,
			},
			Signer:           keys.signer,
			NotAfterOverride: test.notAfterOverride,
		})
		if err != nil {
			t.Errorf("%v: newHammerState() returned err = %v", test.desc, err)
			continue
		}

		if err := test.fn(hs); err != nil {
			t.Errorf("%v: addChain() returned err = %v", test.desc, err)
			continue
		}
		if got, want := len(s.addedCerts), 1; got != want {
			t.Errorf("%v: unexpected number of certs added to server, got = %v, want = %v", test.desc, got, want)
			continue
		}
		temporal := startPB != nil || limitPB != nil
		for i, cert := range s.addedCerts {
			switch got, fixed := cert.NotAfter, test.wantNotAfter.UnixNano() > 0; {
			case fixed && got.Unix() != test.wantNotAfter.Unix(): // second precision is OK
				t.Errorf("%v: cert #%v has NotAfter = %v, want = %v", test.desc, i, got, test.wantNotAfter)
			case !fixed && temporal && (got.Before(test.notAfterStart) || got.After(test.notAfterLimit)):
				t.Errorf("%v: cert #%v has NotAfter = %v, want %v <= NotAfter <= %v", test.desc, i, got, test.notAfterStart, test.notAfterLimit)
			}
		}
	}
}

// fakeCTServer is a fake HTTP server that mimics a CT frontend.
// It supports add-chain and add-pre-chain methods and saves the first certificate of the chain in
// the addCerts field.
// Callers should call reset() before usage to reset internal state and defer-call close() to ensure
// the server is stopped and resources are freed.
type fakeCTServer struct {
	lis    net.Listener
	server *http.Server

	addedCerts []*x509.Certificate
}

func (s *fakeCTServer) addChain(w http.ResponseWriter, req *http.Request) {
	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}

	addReq := &ct.AddChainRequest{}
	if err := json.Unmarshal(body, addReq); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}

	cert, err := x509.ParseCertificate(addReq.Chain[0])
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	s.addedCerts = append(s.addedCerts, cert)

	dsBytes, err := tls.Marshal(tls.DigitallySigned{})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	resp := &ct.AddChainResponse{
		SCTVersion: ct.V1,
		Signature:  dsBytes,
	}
	respBytes, err := json.Marshal(resp)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(respBytes)
}

func (s *fakeCTServer) close() {
	if s.server != nil {
		s.server.Close()
	}
	if s.lis != nil {
		s.lis.Close()
	}
}

func (s *fakeCTServer) reset() {
	s.addedCerts = nil
}

func (s *fakeCTServer) serve() {
	s.server.Serve(s.lis)
}

func writeErr(w http.ResponseWriter, status int, err error) {
	w.WriteHeader(status)
	io.WriteString(w, err.Error())
}

// newFakeCTServer creates and starts a fakeCTServer.
// It returns the started server and a client to the same server.
func newFakeCTServer(t *testing.T) (*fakeCTServer, *client.LogClient) {
	s := &fakeCTServer{}

	var err error
	s.lis, err = net.Listen("tcp", "")
	if err != nil {
		s.close()
		t.Fatalf("net.Listen() returned err = %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/ct/v1/add-chain", s.addChain)
	mux.HandleFunc("/ct/v1/add-pre-chain", s.addChain)

	s.server = &http.Server{Handler: mux}
	go s.serve()

	lc, err := client.New(fmt.Sprintf("http://%s", s.lis.Addr()), nil, jsonclient.Options{})
	if err != nil {
		t.Fatalf("client.New() returned err = %v", err)
	}

	return s, lc
}

// testKeys contains all keys and associated signer required for hammer tests.
type testKeys struct {
	caChain, leafChain []ct.ASN1Cert
	caCert, leafCert   *x509.Certificate
	signer             crypto.Signer
}

// loadTestKeys loads the test keys from the testdata/ directory.
func loadTestKeys(t *testing.T) *testKeys {
	t.Helper()

	const testdataPath = "../testdata/"

	caChain, err := GetChain(testdataPath, "int-ca.cert")
	if err != nil {
		t.Fatalf("GetChain() returned err = %v", err)
	}
	leafChain, err := GetChain(testdataPath, "leaf01.chain")
	if err != nil {
		t.Fatalf("GetChain() returned err = %v", err)
	}
	caCert, err := x509.ParseCertificate(caChain[0].Data)
	if err != nil {
		t.Fatalf("x509.ParseCertificate() returned err = %v", err)
	}
	leafCert, err := x509.ParseCertificate(leafChain[0].Data)
	if err != nil {
		t.Fatalf("x509.ParseCertificate() returned err = %v", err)
	}
	signer, err := MakeSigner(testdataPath)
	if err != nil {
		t.Fatalf("MakeSigner() returned err = %v", err)
	}

	return &testKeys{
		caChain:   caChain,
		leafChain: leafChain,
		caCert:    caCert,
		leafCert:  leafCert,
		signer:    signer,
	}
}
