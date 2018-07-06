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

package ctfe

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang/glog"
	"github.com/golang/mock/gomock"
	"github.com/google/certificate-transparency-go"
	"github.com/google/certificate-transparency-go/tls"
	cttestonly "github.com/google/certificate-transparency-go/trillian/ctfe/testonly"
	"github.com/google/certificate-transparency-go/trillian/mockclient"
	"github.com/google/certificate-transparency-go/trillian/testdata"
	"github.com/google/certificate-transparency-go/trillian/util"
	"github.com/google/certificate-transparency-go/x509"
	"github.com/google/trillian"
	"github.com/google/trillian/crypto"
	"github.com/google/trillian/crypto/keys/pem"
	"github.com/google/trillian/monitoring"
	"github.com/kylelemons/godebug/pretty"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Arbitrary time for use in tests
var fakeTime = time.Date(2016, 7, 22, 11, 01, 13, 0, time.UTC)
var fakeTimeMillis = uint64(fakeTime.UnixNano() / millisPerNano)

// The deadline should be the above bumped by 500ms
var fakeDeadlineTime = time.Date(2016, 7, 22, 11, 01, 13, 500*1000*1000, time.UTC)
var fakeTimeSource = util.NewFixedTimeSource(fakeTime)

const caCertB64 string = `MIIC0DCCAjmgAwIBAgIBADANBgkqhkiG9w0BAQUFADBVMQswCQYDVQQGEwJHQjEk
MCIGA1UEChMbQ2VydGlmaWNhdGUgVHJhbnNwYXJlbmN5IENBMQ4wDAYDVQQIEwVX
YWxlczEQMA4GA1UEBxMHRXJ3IFdlbjAeFw0xMjA2MDEwMDAwMDBaFw0yMjA2MDEw
MDAwMDBaMFUxCzAJBgNVBAYTAkdCMSQwIgYDVQQKExtDZXJ0aWZpY2F0ZSBUcmFu
c3BhcmVuY3kgQ0ExDjAMBgNVBAgTBVdhbGVzMRAwDgYDVQQHEwdFcncgV2VuMIGf
MA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQDVimhTYhCicRmTbneDIRgcKkATxtB7
jHbrkVfT0PtLO1FuzsvRyY2RxS90P6tjXVUJnNE6uvMa5UFEJFGnTHgW8iQ8+EjP
KDHM5nugSlojgZ88ujfmJNnDvbKZuDnd/iYx0ss6hPx7srXFL8/BT/9Ab1zURmnL
svfP34b7arnRsQIDAQABo4GvMIGsMB0GA1UdDgQWBBRfnYgNyHPmVNT4DdjmsMEk
tEfDVTB9BgNVHSMEdjB0gBRfnYgNyHPmVNT4DdjmsMEktEfDVaFZpFcwVTELMAkG
A1UEBhMCR0IxJDAiBgNVBAoTG0NlcnRpZmljYXRlIFRyYW5zcGFyZW5jeSBDQTEO
MAwGA1UECBMFV2FsZXMxEDAOBgNVBAcTB0VydyBXZW6CAQAwDAYDVR0TBAUwAwEB
/zANBgkqhkiG9w0BAQUFAAOBgQAGCMxKbWTyIF4UbASydvkrDvqUpdryOvw4BmBt
OZDQoeojPUApV2lGOwRmYef6HReZFSCa6i4Kd1F2QRIn18ADB8dHDmFYT9czQiRy
f1HWkLxHqd81TbD26yWVXeGJPE3VICskovPkQNJ0tU4b03YmnKliibduyqQQkOFP
OwqULg==`

const intermediateCertB64 string = `MIIC3TCCAkagAwIBAgIBCTANBgkqhkiG9w0BAQUFADBVMQswCQYDVQQGEwJHQjEk
MCIGA1UEChMbQ2VydGlmaWNhdGUgVHJhbnNwYXJlbmN5IENBMQ4wDAYDVQQIEwVX
YWxlczEQMA4GA1UEBxMHRXJ3IFdlbjAeFw0xMjA2MDEwMDAwMDBaFw0yMjA2MDEw
MDAwMDBaMGIxCzAJBgNVBAYTAkdCMTEwLwYDVQQKEyhDZXJ0aWZpY2F0ZSBUcmFu
c3BhcmVuY3kgSW50ZXJtZWRpYXRlIENBMQ4wDAYDVQQIEwVXYWxlczEQMA4GA1UE
BxMHRXJ3IFdlbjCBnzANBgkqhkiG9w0BAQEFAAOBjQAwgYkCgYEA12pnjRFvUi5V
/4IckGQlCLcHSxTXcRWQZPeSfv3tuHE1oTZe594Yy9XOhl+GDHj0M7TQ09NAdwLn
o+9UKx3+m7qnzflNxZdfxyn4bxBfOBskNTXPnIAPXKeAwdPIRADuZdFu6c9S24rf
/lD1xJM1CyGQv1DVvDbzysWo2q6SzYsCAwEAAaOBrzCBrDAdBgNVHQ4EFgQUllUI
BQJ4R56Hc3ZBMbwUOkfiKaswfQYDVR0jBHYwdIAUX52IDchz5lTU+A3Y5rDBJLRH
w1WhWaRXMFUxCzAJBgNVBAYTAkdCMSQwIgYDVQQKExtDZXJ0aWZpY2F0ZSBUcmFu
c3BhcmVuY3kgQ0ExDjAMBgNVBAgTBVdhbGVzMRAwDgYDVQQHEwdFcncgV2VuggEA
MAwGA1UdEwQFMAMBAf8wDQYJKoZIhvcNAQEFBQADgYEAIgbascZrcdzglcP2qi73
LPd2G+er1/w5wxpM/hvZbWc0yoLyLd5aDIu73YJde28+dhKtjbMAp+IRaYhgIyYi
hMOqXSGR79oQv5I103s6KjQNWUGblKSFZvP6w82LU9Wk6YJw6tKXsHIQ+c5KITix
iBEUO5P6TnqH3TfhOF8sKQg=`

const caAndIntermediateCertsPEM string = "-----BEGIN CERTIFICATE-----\n" +
	caCertB64 +
	"\n-----END CERTIFICATE-----\n" +
	"\n-----BEGIN CERTIFICATE-----\n" +
	intermediateCertB64 +
	"\n-----END CERTIFICATE-----\n"

type handlerTestInfo struct {
	mockCtrl      *gomock.Controller
	roots         *PEMCertPool
	notAfterStart time.Time
	notAfterEnd   time.Time
	client        *mockclient.MockTrillianLogClient
	c             *LogContext
}

// setupTest creates mock objects and contexts.  Caller should invoke info.mockCtrl.Finish().
func setupTest(t *testing.T, pemRoots []string, signer *crypto.Signer) handlerTestInfo {
	t.Helper()
	info := handlerTestInfo{
		mockCtrl: gomock.NewController(t),
		roots:    NewPEMCertPool(),
	}

	info.client = mockclient.NewMockTrillianLogClient(info.mockCtrl)
	vOpts := CertValidationOpts{
		trustedRoots:  info.roots,
		rejectExpired: false,
		extKeyUsages:  []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
	}
	iOpts := InstanceOptions{Deadline: time.Millisecond * 500, MetricFactory: monitoring.InertMetricFactory{}, RequestLog: new(DefaultRequestLog)}
	info.c = NewLogContext(0x42, "test", vOpts, info.client, signer, iOpts, fakeTimeSource)

	for _, pemRoot := range pemRoots {
		if !info.roots.AppendCertsFromPEM([]byte(pemRoot)) {
			glog.Fatal("failed to load cert pool")
		}
	}

	return info
}

func (info handlerTestInfo) getHandlers() map[string]AppHandler {
	return map[string]AppHandler{
		"get-sth":             {Context: info.c, Handler: getSTH, Name: "GetSTH", Method: http.MethodGet},
		"get-sth-consistency": {Context: info.c, Handler: getSTHConsistency, Name: "GetSTHConsistency", Method: http.MethodGet},
		"get-proof-by-hash":   {Context: info.c, Handler: getProofByHash, Name: "GetProofByHash", Method: http.MethodGet},
		"get-entries":         {Context: info.c, Handler: getEntries, Name: "GetEntries", Method: http.MethodGet},
		"get-roots":           {Context: info.c, Handler: getRoots, Name: "GetRoots", Method: http.MethodGet},
		"get-entry-and-proof": {Context: info.c, Handler: getEntryAndProof, Name: "GetEntryAndProof", Method: http.MethodGet},
	}
}

func (info handlerTestInfo) postHandlers() map[string]AppHandler {
	return map[string]AppHandler{
		"add-chain":     {Context: info.c, Handler: addChain, Name: "AddChain", Method: http.MethodPost},
		"add-pre-chain": {Context: info.c, Handler: addPreChain, Name: "AddPreChain", Method: http.MethodPost},
	}
}

func TestPostHandlersRejectGet(t *testing.T) {
	info := setupTest(t, []string{cttestonly.FakeCACertPEM}, nil)
	defer info.mockCtrl.Finish()

	// Anything in the post handler list should reject GET
	for path, handler := range info.postHandlers() {
		s := httptest.NewServer(handler)
		defer s.Close()

		resp, err := http.Get(s.URL + "/ct/v1/" + path)
		if err != nil {
			t.Errorf("http.Get(%s)=(_,%q); want (_,nil)", path, err)
			continue
		}
		if got, want := resp.StatusCode, http.StatusMethodNotAllowed; got != want {
			t.Errorf("http.Get(%s)=(%d,nil); want (%d,nil)", path, got, want)
		}

	}
}

func TestGetHandlersRejectPost(t *testing.T) {
	info := setupTest(t, []string{cttestonly.FakeCACertPEM}, nil)
	defer info.mockCtrl.Finish()

	// Anything in the get handler list should reject POST.
	for path, handler := range info.getHandlers() {
		s := httptest.NewServer(handler)
		defer s.Close()

		resp, err := http.Post(s.URL+"/ct/v1/"+path, "application/json", nil)
		if err != nil {
			t.Errorf("http.Post(%s)=(_,%q); want (_,nil)", path, err)
			continue
		}
		if got, want := resp.StatusCode, http.StatusMethodNotAllowed; got != want {
			t.Errorf("http.Post(%s)=(%d,nil); want (%d,nil)", path, got, want)
		}
	}
}

func TestPostHandlersFailure(t *testing.T) {
	var tests = []struct {
		descr string
		body  io.Reader
		want  int
	}{
		{"nil", nil, http.StatusBadRequest},
		{"''", strings.NewReader(""), http.StatusBadRequest},
		{"malformed-json", strings.NewReader("{ !$%^& not valid json "), http.StatusBadRequest},
		{"empty-chain", strings.NewReader(`{ "chain": [] }`), http.StatusBadRequest},
		{"wrong-chain", strings.NewReader(`{ "chain": [ "test" ] }`), http.StatusBadRequest},
	}

	info := setupTest(t, []string{cttestonly.FakeCACertPEM}, nil)
	defer info.mockCtrl.Finish()
	for path, handler := range info.postHandlers() {
		s := httptest.NewServer(handler)
		defer s.Close()

		for _, test := range tests {
			resp, err := http.Post(s.URL+"/ct/v1/"+path, "application/json", test.body)
			if err != nil {
				t.Errorf("http.Post(%s,%s)=(_,%q); want (_,nil)", path, test.descr, err)
				continue
			}
			if resp.StatusCode != test.want {
				t.Errorf("http.Post(%s,%s)=(%d,nil); want (%d,nil)", path, test.descr, resp.StatusCode, test.want)
			}
		}
	}
}

func TestHandlers(t *testing.T) {
	path := "/test-prefix/ct/v1/add-chain"
	var tests = []string{
		"/test-prefix/",
		"test-prefix/",
		"/test-prefix",
		"test-prefix",
	}
	info := setupTest(t, nil, nil)
	defer info.mockCtrl.Finish()
	for _, test := range tests {
		handlers := info.c.Handlers(test)
		if h, ok := handlers[path]; !ok {
			t.Errorf("Handlers(%s)[%q]=%+v; want _", test, path, h)
		} else if h.Name != "AddChain" {
			t.Errorf("Handlers(%s)[%q].Name=%q; want 'AddChain'", test, path, h.Name)
		}
		// Check each entrypoint has a handler
		if got, want := len(handlers), len(Entrypoints); got != want {
			t.Errorf("len(Handlers(%s))=%d; want %d", test, got, want)
		}
	outer:
		for _, ep := range Entrypoints {
			for _, v := range handlers {
				if v.Name == ep {
					continue outer
				}
			}
			t.Errorf("Handlers(%s) missing entry with .Name=%q", test, ep)
		}
	}
}

func TestGetRoots(t *testing.T) {
	info := setupTest(t, []string{caAndIntermediateCertsPEM}, nil)
	defer info.mockCtrl.Finish()
	handler := AppHandler{Context: info.c, Handler: getRoots, Name: "GetRoots", Method: http.MethodGet}

	req, err := http.NewRequest("GET", "http://example.com/ct/v1/get-roots", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if got, want := w.Code, http.StatusOK; got != want {
		t.Fatalf("http.Get(get-roots)=%d; want %d", got, want)
	}

	var parsedJSON map[string][]string
	if err := json.Unmarshal(w.Body.Bytes(), &parsedJSON); err != nil {
		t.Fatalf("json.Unmarshal(%q)=%q; want nil", w.Body.Bytes(), err)
	}
	if got := len(parsedJSON); got != 1 {
		t.Errorf("len(json)=%d; want 1", got)
	}
	certs := parsedJSON[jsonMapKeyCertificates]
	if got := len(certs); got != 2 {
		t.Fatalf("len(%q)=%d; want 2", certs, got)
	}
	if got, want := certs[0], strings.Replace(caCertB64, "\n", "", -1); got != want {
		t.Errorf("certs[0]=%s; want %s", got, want)
	}
	if got, want := certs[1], strings.Replace(intermediateCertB64, "\n", "", -1); got != want {
		t.Errorf("certs[1]=%s; want %s", got, want)
	}
}

func TestAddChain(t *testing.T) {
	var tests = []struct {
		descr  string
		chain  []string
		toSign string // hex-encoded
		want   int
		err    error
	}{
		{
			descr: "leaf-only",
			chain: []string{cttestonly.LeafSignedByFakeIntermediateCertPEM},
			want:  http.StatusBadRequest,
		},
		{
			descr: "wrong-entry-type",
			chain: []string{cttestonly.PrecertPEMValid},
			want:  http.StatusBadRequest,
		},
		{
			descr:  "backend-rpc-fail",
			chain:  []string{cttestonly.LeafSignedByFakeIntermediateCertPEM, cttestonly.FakeIntermediateCertPEM},
			toSign: "1337d72a403b6539f58896decba416d5d4b3603bfa03e1f94bb9b4e898af897d",
			want:   http.StatusInternalServerError,
			err:    status.Errorf(codes.Internal, "error"),
		},
		{
			descr:  "success-without-root",
			chain:  []string{cttestonly.LeafSignedByFakeIntermediateCertPEM, cttestonly.FakeIntermediateCertPEM},
			toSign: "1337d72a403b6539f58896decba416d5d4b3603bfa03e1f94bb9b4e898af897d",
			want:   http.StatusOK,
		},
		{
			descr:  "success",
			chain:  []string{cttestonly.LeafSignedByFakeIntermediateCertPEM, cttestonly.FakeIntermediateCertPEM, cttestonly.FakeCACertPEM},
			toSign: "1337d72a403b6539f58896decba416d5d4b3603bfa03e1f94bb9b4e898af897d",
			want:   http.StatusOK,
		},
	}

	signer, err := setupSigner(fakeSignature)
	if err != nil {
		t.Fatalf("Failed to create test signer: %v", err)
	}

	info := setupTest(t, []string{cttestonly.FakeCACertPEM}, signer)
	defer info.mockCtrl.Finish()

	for _, test := range tests {
		pool := loadCertsIntoPoolOrDie(t, test.chain)
		chain := createJSONChain(t, *pool)
		if len(test.toSign) > 0 {
			root := info.roots.RawCertificates()[0]
			merkleLeaf, err := ct.MerkleTreeLeafFromChain(pool.RawCertificates(), ct.X509LogEntryType, fakeTimeMillis)
			if err != nil {
				t.Errorf("Unexpected error signing SCT: %v", err)
				continue
			}
			leafChain := pool.RawCertificates()
			if !leafChain[len(leafChain)-1].Equal(root) {
				// The submitted chain may not include a root, but the generated LogLeaf will
				fullChain := make([]*x509.Certificate, len(leafChain)+1)
				copy(fullChain, leafChain)
				fullChain[len(leafChain)] = root
				leafChain = fullChain
			}
			leaves := logLeavesForCert(t, leafChain, merkleLeaf, false)
			queuedLeaves := make([]*trillian.QueuedLogLeaf, len(leaves))
			for i, leaf := range leaves {
				queuedLeaves[i] = &trillian.QueuedLogLeaf{
					Leaf:   leaf,
					Status: status.New(codes.OK, "ok").Proto(),
				}
			}
			rsp := trillian.QueueLeavesResponse{QueuedLeaves: queuedLeaves}
			info.client.EXPECT().QueueLeaves(deadlineMatcher(), &trillian.QueueLeavesRequest{LogId: 0x42, Leaves: leaves}).Return(&rsp, test.err)
		}

		recorder := makeAddChainRequest(t, info.c, chain)
		if recorder.Code != test.want {
			t.Errorf("addChain(%s)=%d (body:%v); want %dv", test.descr, recorder.Code, recorder.Body, test.want)
			continue
		}
		if test.want == http.StatusOK {
			var resp ct.AddChainResponse
			if err := json.NewDecoder(recorder.Body).Decode(&resp); err != nil {
				t.Fatalf("json.Decode(%s)=%v; want nil", recorder.Body.Bytes(), err)
			}

			if got, want := ct.Version(resp.SCTVersion), ct.V1; got != want {
				t.Errorf("resp.SCTVersion=%v; want %v", got, want)
			}
			if got, want := resp.ID, demoLogID[:]; !bytes.Equal(got, want) {
				t.Errorf("resp.ID=%v; want %v", got, want)
			}
			if got, want := resp.Timestamp, uint64(1469185273000); got != want {
				t.Errorf("resp.Timestamp=%d; want %d", got, want)
			}
			if got, want := hex.EncodeToString(resp.Signature), "040300067369676e6564"; got != want {
				t.Errorf("resp.Signature=%s; want %s", got, want)
			}
		}
	}
}

func TestAddPrechain(t *testing.T) {
	var tests = []struct {
		descr  string
		chain  []string
		root   string
		toSign string // hex-encoded
		err    error
		want   int
	}{
		{
			descr: "leaf-signed-by-different",
			chain: []string{cttestonly.PrecertPEMValid, cttestonly.FakeIntermediateCertPEM},
			want:  http.StatusBadRequest,
		},
		{
			descr: "wrong-entry-type",
			chain: []string{cttestonly.TestCertPEM},
			want:  http.StatusBadRequest,
		},
		{
			descr:  "backend-rpc-fail",
			chain:  []string{cttestonly.PrecertPEMValid, cttestonly.CACertPEM},
			toSign: "92ecae1a2dc67a6c5f9c96fa5cab4c2faf27c48505b696dad926f161b0ca675a",
			err:    status.Errorf(codes.Internal, "error"),
			want:   http.StatusInternalServerError,
		},
		{
			descr:  "success",
			chain:  []string{cttestonly.PrecertPEMValid, cttestonly.CACertPEM},
			toSign: "92ecae1a2dc67a6c5f9c96fa5cab4c2faf27c48505b696dad926f161b0ca675a",
			want:   http.StatusOK,
		},
		{
			descr:  "success-without-root",
			chain:  []string{cttestonly.PrecertPEMValid},
			toSign: "92ecae1a2dc67a6c5f9c96fa5cab4c2faf27c48505b696dad926f161b0ca675a",
			want:   http.StatusOK,
		},
	}

	signer, err := setupSigner(fakeSignature)
	if err != nil {
		t.Fatalf("Failed to create test signer: %v", err)
	}

	info := setupTest(t, []string{cttestonly.CACertPEM}, signer)
	defer info.mockCtrl.Finish()

	for _, test := range tests {
		pool := loadCertsIntoPoolOrDie(t, test.chain)
		chain := createJSONChain(t, *pool)
		if len(test.toSign) > 0 {
			root := info.roots.RawCertificates()[0]
			merkleLeaf, err := ct.MerkleTreeLeafFromChain([]*x509.Certificate{pool.RawCertificates()[0], root}, ct.PrecertLogEntryType, fakeTimeMillis)
			if err != nil {
				t.Errorf("Unexpected error signing SCT: %v", err)
				continue
			}
			leafChain := pool.RawCertificates()
			if !leafChain[len(leafChain)-1].Equal(root) {
				// The submitted chain may not include a root, but the generated LogLeaf will
				fullChain := make([]*x509.Certificate, len(leafChain)+1)
				copy(fullChain, leafChain)
				fullChain[len(leafChain)] = root
				leafChain = fullChain
			}
			leaves := logLeavesForCert(t, leafChain, merkleLeaf, true)
			queuedLeaves := make([]*trillian.QueuedLogLeaf, len(leaves))
			for i, leaf := range leaves {
				queuedLeaves[i] = &trillian.QueuedLogLeaf{
					Leaf:   leaf,
					Status: status.New(codes.OK, "ok").Proto(),
				}
			}
			rsp := trillian.QueueLeavesResponse{QueuedLeaves: queuedLeaves}
			info.client.EXPECT().QueueLeaves(deadlineMatcher(), &trillian.QueueLeavesRequest{LogId: 0x42, Leaves: leaves}).Return(&rsp, test.err)
		}

		recorder := makeAddPrechainRequest(t, info.c, chain)
		if recorder.Code != test.want {
			t.Errorf("addPrechain(%s)=%d (body:%v); want %d", test.descr, recorder.Code, recorder.Body, test.want)
			continue
		}
		if test.want == http.StatusOK {
			var resp ct.AddChainResponse
			if err := json.NewDecoder(recorder.Body).Decode(&resp); err != nil {
				t.Fatalf("json.Decode(%s)=%v; want nil", recorder.Body.Bytes(), err)
			}

			if got, want := ct.Version(resp.SCTVersion), ct.V1; got != want {
				t.Errorf("resp.SCTVersion=%v; want %v", got, want)
			}
			if got, want := resp.ID, demoLogID[:]; !bytes.Equal(got, want) {
				t.Errorf("resp.ID=%x; want %x", got, want)
			}
			if got, want := resp.Timestamp, uint64(1469185273000); got != want {
				t.Errorf("resp.Timestamp=%d; want %d", got, want)
			}
			if got, want := hex.EncodeToString(resp.Signature), "040300067369676e6564"; got != want {
				t.Errorf("resp.Signature=%s; want %s", got, want)
			}
		}
	}
}

func TestGetSTH(t *testing.T) {
	var tests = []struct {
		descr   string
		rpcRsp  *trillian.GetLatestSignedLogRootResponse
		rpcErr  error
		toSign  string // hex-encoded
		signErr error
		want    int
		errStr  string
	}{
		{
			descr:  "backend-failure",
			rpcErr: errors.New("backendfailure"),
			want:   http.StatusInternalServerError,
			errStr: "request failed",
		},
		{
			descr:  "bad-tree-size",
			rpcRsp: makeGetRootResponseForTest(12345, -50, []byte("abcdabcdabcdabcdabcdabcdabcdabcd")),
			want:   http.StatusInternalServerError,
			errStr: "bad tree size",
		},
		{
			descr:  "bad-hash",
			rpcRsp: makeGetRootResponseForTest(12345, 25, []byte("thisisnot32byteslong")),
			want:   http.StatusInternalServerError,
			errStr: "bad hash size",
		},
		{
			descr:   "signer-fail",
			rpcRsp:  makeGetRootResponseForTest(12345, 25, []byte("abcdabcdabcdabcdabcdabcdabcdabcd")),
			want:    http.StatusInternalServerError,
			signErr: errors.New("signerfails"),
			errStr:  "signerfails",
		},
		{
			descr:  "ok",
			rpcRsp: makeGetRootResponseForTest(12345000000, 25, []byte("abcdabcdabcdabcdabcdabcdabcdabcd")),
			toSign: "1e88546f5157bfaf77ca2454690b602631fedae925bbe7cf708ea275975bfe74",
			want:   http.StatusOK,
		},
	}

	key, err := pem.UnmarshalPublicKey(testdata.DemoPublicKey)
	if err != nil {
		t.Fatalf("Failed to load public key: %v", err)
	}

	for _, test := range tests {
		// Run deferred funcs at the end of each iteration.
		func() {
			var signer *crypto.Signer
			if test.signErr != nil {
				signer = crypto.NewSHA256Signer(testdata.NewSignerWithErr(key, test.signErr))
			} else {
				signer = crypto.NewSHA256Signer(testdata.NewSignerWithFixedSig(key, fakeSignature))
			}

			info := setupTest(t, []string{cttestonly.CACertPEM}, signer)
			defer info.mockCtrl.Finish()

			info.client.EXPECT().GetLatestSignedLogRoot(deadlineMatcher(), &trillian.GetLatestSignedLogRootRequest{LogId: 0x42}).Return(test.rpcRsp, test.rpcErr)
			req, err := http.NewRequest("GET", "http://example.com/ct/v1/get-sth", nil)
			if err != nil {
				t.Errorf("Failed to create request: %v", err)
				return
			}

			handler := AppHandler{Context: info.c, Handler: getSTH, Name: "GetSTH", Method: http.MethodGet}
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			if got := w.Code; got != test.want {
				t.Errorf("GetSTH(%s).Code=%d; want %d", test.descr, got, test.want)
			}
			if test.errStr != "" {
				if body := w.Body.String(); !strings.Contains(body, test.errStr) {
					t.Errorf("GetSTH(%s)=%q; want to find %q", test.descr, body, test.errStr)
				}
				return
			}

			var rsp ct.GetSTHResponse
			if err := json.Unmarshal(w.Body.Bytes(), &rsp); err != nil {
				t.Errorf("Failed to unmarshal json response: %s", w.Body.Bytes())
				return
			}

			if got, want := rsp.TreeSize, uint64(25); got != want {
				t.Errorf("GetSTH(%s).TreeSize=%d; want %d", test.descr, got, want)
			}
			if got, want := rsp.Timestamp, uint64(12345); got != want {
				t.Errorf("GetSTH(%s).Timestamp=%d; want %d", test.descr, got, want)
			}
			if got, want := hex.EncodeToString(rsp.SHA256RootHash), "6162636461626364616263646162636461626364616263646162636461626364"; got != want {
				t.Errorf("GetSTH(%s).SHA256RootHash=%s; want %s", test.descr, got, want)
			}
			if got, want := hex.EncodeToString(rsp.TreeHeadSignature), "040300067369676e6564"; got != want {
				t.Errorf("GetSTH(%s).TreeHeadSignature=%s; want %s", test.descr, got, want)
			}
		}()
	}
}

func runTestGetEntries(t *testing.T) {
	// Create a couple of valid serialized ct.MerkleTreeLeaf objects
	merkleLeaf1 := ct.MerkleTreeLeaf{
		Version:  ct.V1,
		LeafType: ct.TimestampedEntryLeafType,
		TimestampedEntry: &ct.TimestampedEntry{
			Timestamp:  12345,
			EntryType:  ct.X509LogEntryType,
			X509Entry:  &ct.ASN1Cert{Data: []byte("certdatacertdata")},
			Extensions: ct.CTExtensions{},
		},
	}
	merkleLeaf2 := ct.MerkleTreeLeaf{
		Version:  ct.V1,
		LeafType: ct.TimestampedEntryLeafType,
		TimestampedEntry: &ct.TimestampedEntry{
			Timestamp:  67890,
			EntryType:  ct.X509LogEntryType,
			X509Entry:  &ct.ASN1Cert{Data: []byte("certdat2certdat2")},
			Extensions: ct.CTExtensions{},
		},
	}
	merkleBytes1, err1 := tls.Marshal(merkleLeaf1)
	merkleBytes2, err2 := tls.Marshal(merkleLeaf2)
	if err1 != nil || err2 != nil {
		t.Fatalf("failed to tls.Marshal() test data for get-entries: %v %v", err1, err2)
	}

	var tests = []struct {
		descr  string
		req    string
		want   int
		leaves []*trillian.LogLeaf
		rpcErr error
		errStr string
	}{
		{
			descr: "invalid &&s",
			req:   "start=&&&&&&&&&end=wibble",
			want:  http.StatusBadRequest,
		},
		{
			descr: "start non numeric",
			req:   "start=fish&end=3",
			want:  http.StatusBadRequest,
		},
		{
			descr: "end non numeric",
			req:   "start=10&end=wibble",
			want:  http.StatusBadRequest,
		},
		{
			descr: "both non numeric",
			req:   "start=fish&end=wibble",
			want:  http.StatusBadRequest,
		},
		{
			descr: "end missing",
			req:   "start=1",
			want:  http.StatusBadRequest,
		},
		{
			descr: "start missing",
			req:   "end=1",
			want:  http.StatusBadRequest,
		},
		{
			descr: "both missing",
			req:   "",
			want:  http.StatusBadRequest,
		},
		{
			descr:  "backend rpc error",
			req:    "start=1&end=2",
			want:   http.StatusInternalServerError,
			rpcErr: errors.New("bang"),
			errStr: "bang",
		},
		{
			descr:  "backend extra leaves",
			req:    "start=1&end=2",
			want:   http.StatusInternalServerError,
			leaves: []*trillian.LogLeaf{{LeafIndex: 1}, {LeafIndex: 2}, {LeafIndex: 3}},
			errStr: "too many leaves",
		},
		{
			descr:  "backend non-contiguous range",
			req:    "start=1&end=2",
			want:   http.StatusInternalServerError,
			leaves: []*trillian.LogLeaf{{LeafIndex: 1}, {LeafIndex: 3}},
			errStr: "unexpected leaf index",
		},
		{
			descr: "backend leaf corrupt",
			req:   "start=1&end=2",
			want:  http.StatusOK,
			leaves: []*trillian.LogLeaf{
				{LeafIndex: 1, MerkleLeafHash: []byte("hash"), LeafValue: []byte("NOT A MERKLE TREE LEAF")},
				{LeafIndex: 2, MerkleLeafHash: []byte("hash"), LeafValue: []byte("NOT A MERKLE TREE LEAF")},
			},
		},
		{
			descr: "leaves ok",
			req:   "start=1&end=2",
			want:  http.StatusOK,
			leaves: []*trillian.LogLeaf{
				{LeafIndex: 1, MerkleLeafHash: []byte("hash"), LeafValue: merkleBytes1, ExtraData: []byte("extra1")},
				{LeafIndex: 2, MerkleLeafHash: []byte("hash"), LeafValue: merkleBytes2, ExtraData: []byte("extra2")},
			},
		},
	}
	info := setupTest(t, nil, nil)
	defer info.mockCtrl.Finish()
	handler := AppHandler{Context: info.c, Handler: getEntries, Name: "GetEntries", Method: http.MethodGet}

	for _, test := range tests {
		path := fmt.Sprintf("/ct/v1/get-entries?%s", test.req)
		req, err := http.NewRequest("GET", path, nil)
		if err != nil {
			t.Errorf("Failed to create request: %v", err)
			continue
		}
		if len(test.leaves) > 0 || test.rpcErr != nil {
			if *getByRange {
				rsp := trillian.GetLeavesByRangeResponse{Leaves: test.leaves}
				info.client.EXPECT().GetLeavesByRange(deadlineMatcher(), &trillian.GetLeavesByRangeRequest{LogId: 0x42, StartIndex: 1, Count: 2}).Return(&rsp, test.rpcErr)
			} else {
				rsp := trillian.GetLeavesByIndexResponse{Leaves: test.leaves}
				info.client.EXPECT().GetLeavesByIndex(deadlineMatcher(), &trillian.GetLeavesByIndexRequest{LogId: 0x42, LeafIndex: []int64{1, 2}}).Return(&rsp, test.rpcErr)
			}
		}

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if got := w.Code; got != test.want {
			t.Errorf("GetEntries(%q)=%d; want %d (because %s)", test.req, got, test.want, test.descr)
		}
		if test.errStr != "" {
			if body := w.Body.String(); !strings.Contains(body, test.errStr) {
				t.Errorf("GetEntries(%q)=%q; want to find %q (because %s)", test.req, body, test.errStr, test.descr)
			}
			continue
		}
		if test.want != http.StatusOK {
			continue
		}
		// Leaf data should be passed through as-is even if invalid.
		var jsonMap map[string][]ct.LeafEntry
		if err := json.Unmarshal(w.Body.Bytes(), &jsonMap); err != nil {
			t.Errorf("Failed to unmarshal json response %s: %v", w.Body.Bytes(), err)
			continue
		}
		if got := len(jsonMap); got != 1 {
			t.Errorf("len(rspMap)=%d; want 1", got)
		}
		entries := jsonMap["entries"]
		if got, want := len(entries), len(test.leaves); got != want {
			t.Errorf("len(rspMap['entries']=%d; want %d", got, want)
			continue
		}
		for i := 0; i < len(entries); i++ {
			if got, want := string(entries[i].LeafInput), string(test.leaves[i].LeafValue); got != want {
				t.Errorf("rspMap['entries'][%d].LeafInput=%s; want %s", i, got, want)
			}
			if got, want := string(entries[i].ExtraData), string(test.leaves[i].ExtraData); got != want {
				t.Errorf("rspMap['entries'][%d].ExtraData=%s; want %s", i, got, want)
			}
		}
	}
}

func runTestGetEntriesRanges(t *testing.T) {
	var tests = []struct {
		desc   string
		start  int64
		end    int64
		rpcEnd int64 // same as end if zero
		want   int
		rpc    bool
	}{
		{
			desc:  "-ve start value not allowed",
			start: -1,
			end:   0,
			want:  http.StatusBadRequest,
		},
		{
			desc:  "-ve end value not allowed",
			start: 0,
			end:   -1,
			want:  http.StatusBadRequest,
		},
		{
			desc:  "invalid range end>start",
			start: 20,
			end:   10,
			want:  http.StatusBadRequest,
		},
		{
			desc:  "invalid range, -ve end",
			start: 3000,
			end:   -50,
			want:  http.StatusBadRequest,
		},
		{
			desc:  "valid range",
			start: 10,
			end:   20,
			want:  http.StatusInternalServerError,
			rpc:   true,
		},
		{
			desc:  "valid range, one entry",
			start: 10,
			end:   10,
			want:  http.StatusInternalServerError,
			rpc:   true,
		},
		{
			desc:  "invalid range, edge case",
			start: 10,
			end:   9,
			want:  http.StatusBadRequest,
		},
		{
			desc:   "range too large, truncated",
			start:  1000,
			end:    50000,
			rpcEnd: 1000 + MaxGetEntriesAllowed - 1,
			want:   http.StatusInternalServerError,
			rpc:    true,
		},
	}

	info := setupTest(t, nil, nil)
	defer info.mockCtrl.Finish()
	handler := AppHandler{Context: info.c, Handler: getEntries, Name: "GetEntries", Method: http.MethodGet}

	// This tests that only valid ranges make it to the backend for get-entries.
	// We're testing request handling up to the point where we make the RPC so arrange for
	// it to fail with a specific error.
	for _, test := range tests {
		if test.rpc {
			end := test.rpcEnd
			if end == 0 {
				end = test.end
			}
			if *getByRange {
				info.client.EXPECT().GetLeavesByRange(deadlineMatcher(), &trillian.GetLeavesByRangeRequest{LogId: 0x42, StartIndex: test.start, Count: end + 1 - test.start}).Return(nil, errors.New("RPCMADE"))
			} else {
				info.client.EXPECT().GetLeavesByIndex(deadlineMatcher(), &trillian.GetLeavesByIndexRequest{LogId: 0x42, LeafIndex: buildIndicesForRange(test.start, end)}).Return(nil, errors.New("RPCMADE"))
			}
		}

		path := fmt.Sprintf("/ct/v1/get-entries?start=%d&end=%d", test.start, test.end)
		req, err := http.NewRequest("GET", path, nil)
		if err != nil {
			t.Errorf("Failed to create request: %v", err)
			continue
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if got := w.Code; got != test.want {
			t.Errorf("getEntries(%d, %d)=%d; want %d for test %s", test.start, test.end, got, test.want, test.desc)
		}
		if test.rpc && !strings.Contains(w.Body.String(), "RPCMADE") {
			// If an RPC was emitted, it should have received and propagated an error.
			t.Errorf("getEntries(%d, %d)=%q; expect RPCMADE for test %s", test.start, test.end, w.Body, test.desc)
		}
	}
}

func runGetEntriesVariants(t *testing.T, fn func(t *testing.T)) {
	t.Helper()
	defer func(val bool) {
		*getByRange = val
	}(*getByRange)
	for _, val := range []bool{false, true} {
		*getByRange = val
		name := "ByIndex"
		if *getByRange {
			name = "ByName"
		}
		t.Run(name, fn)
	}
}

func TestGetEntriesRanges(t *testing.T) {
	runGetEntriesVariants(t, runTestGetEntriesRanges)
}

func TestGetEntries(t *testing.T) {
	runGetEntriesVariants(t, runTestGetEntries)
}

func TestSortLeafRange(t *testing.T) {
	var tests = []struct {
		start   int64
		end     int64
		entries []int
		errStr  string
	}{
		{1, 2, []int{1, 2}, ""},
		{1, 1, []int{1}, ""},
		{5, 12, []int{5, 6, 7, 8, 9, 10, 11, 12}, ""},
		{5, 12, []int{5, 6, 7, 8, 9, 10}, ""},
		{5, 12, []int{7, 6, 8, 9, 10, 5}, ""},
		{5, 12, []int{5, 5, 6, 7, 8, 9, 10}, "unexpected leaf index"},
		{5, 12, []int{6, 7, 8, 9, 10, 11, 12}, "unexpected leaf index"},
		{5, 12, []int{5, 6, 7, 8, 9, 10, 12}, "unexpected leaf index"},
		{5, 12, []int{5, 6, 7, 8, 9, 10, 11, 12, 13}, "too many leaves"},
		{1, 4, []int{5, 2, 3}, "unexpected leaf index"},
	}
	for _, test := range tests {
		rsp := trillian.GetLeavesByIndexResponse{}
		for _, idx := range test.entries {
			rsp.Leaves = append(rsp.Leaves, &trillian.LogLeaf{LeafIndex: int64(idx)})
		}
		err := sortLeafRange(&rsp, test.start, test.end)
		if test.errStr != "" {
			if err == nil {
				t.Errorf("sortLeafRange(%v, %d, %d)=nil; want substring %q", test.entries, test.start, test.end, test.errStr)
			} else if !strings.Contains(err.Error(), test.errStr) {
				t.Errorf("sortLeafRange(%v, %d, %d)=%v; want substring %q", test.entries, test.start, test.end, err, test.errStr)
			}
			continue
		}
		if err != nil {
			t.Errorf("sortLeafRange(%v, %d, %d)=%v; want nil", test.entries, test.start, test.end, err)
		}
	}
}

func TestGetProofByHash(t *testing.T) {
	auditHashes := [][]byte{
		[]byte("abcdef78901234567890123456789012"),
		[]byte("ghijkl78901234567890123456789012"),
		[]byte("mnopqr78901234567890123456789012"),
	}
	inclusionProof := ct.GetProofByHashResponse{
		LeafIndex: 2,
		AuditPath: auditHashes,
	}

	var tests = []struct {
		req      string
		want     int
		rpcRsp   *trillian.GetInclusionProofByHashResponse
		httpRsp  *ct.GetProofByHashResponse
		httpJSON string
		rpcErr   error
		errStr   string
	}{
		{
			req:  "",
			want: http.StatusBadRequest,
		},
		{
			req:  "hash=&tree_size=1",
			want: http.StatusBadRequest,
		},
		{
			req:  "hash=''&tree_size=1",
			want: http.StatusBadRequest,
		},
		{
			req:  "hash=notbase64data&tree_size=1",
			want: http.StatusBadRequest,
		},
		{
			req:  "tree_size=-1&hash=aGkK",
			want: http.StatusBadRequest,
		},
		{
			req:    "tree_size=6&hash=YWhhc2g=",
			want:   http.StatusInternalServerError,
			rpcErr: errors.New("RPCFAIL"),
			errStr: "RPCFAIL",
		},
		{
			req:  "tree_size=1&hash=YWhhc2g=",
			want: http.StatusOK,
			rpcRsp: &trillian.GetInclusionProofByHashResponse{
				Proof: []*trillian.Proof{
					{
						LeafIndex: 0,
						Hashes:    nil,
					},
				},
			},
			httpRsp: &ct.GetProofByHashResponse{LeafIndex: 0, AuditPath: nil},
			// Check undecoded JSON to confirm use of '[]' not 'null'
			httpJSON: "{\"leaf_index\":0,\"audit_path\":[]}",
		},
		{
			req:  "tree_size=7&hash=YWhhc2g=",
			want: http.StatusOK,
			rpcRsp: &trillian.GetInclusionProofByHashResponse{
				Proof: []*trillian.Proof{
					{
						LeafIndex: 2,
						Hashes:    auditHashes,
					},
					// Second proof ignored.
					{
						LeafIndex: 2,
						Hashes:    [][]byte{[]byte("ghijkl")},
					},
				},
			},
			httpRsp: &inclusionProof,
		},
		{
			req:  "tree_size=9&hash=YWhhc2g=",
			want: http.StatusInternalServerError,
			rpcRsp: &trillian.GetInclusionProofByHashResponse{
				Proof: []*trillian.Proof{
					{
						LeafIndex: 2,
						Hashes: [][]byte{
							auditHashes[0],
							{}, // missing hash
							auditHashes[2],
						},
					},
				},
			},
			errStr: "invalid proof",
		},
		{
			req:  "tree_size=7&hash=YWhhc2g=",
			want: http.StatusOK,
			rpcRsp: &trillian.GetInclusionProofByHashResponse{
				Proof: []*trillian.Proof{
					{
						LeafIndex: 2,
						Hashes:    auditHashes,
					},
				},
			},
			httpRsp: &inclusionProof,
		},
		{
			// Hash with URL-encoded %2B -> '+'.
			req:  "hash=WtfX3Axbm7UwtY7GhHoAHPCtXJVrY5vZsH%2ByaXOD2GI=&tree_size=1",
			want: http.StatusOK,
			rpcRsp: &trillian.GetInclusionProofByHashResponse{
				Proof: []*trillian.Proof{
					{
						LeafIndex: 2,
						Hashes:    auditHashes,
					},
				},
			},
			httpRsp: &inclusionProof,
		},
	}
	info := setupTest(t, nil, nil)
	defer info.mockCtrl.Finish()
	handler := AppHandler{Context: info.c, Handler: getProofByHash, Name: "GetProofByHash", Method: http.MethodGet}

	for _, test := range tests {
		req, err := http.NewRequest("GET", fmt.Sprintf("/ct/v1/proof-by-hash?%s", test.req), nil)
		if err != nil {
			t.Errorf("Failed to create request: %v", err)
			continue
		}
		if test.rpcRsp != nil || test.rpcErr != nil {
			info.client.EXPECT().GetInclusionProofByHash(deadlineMatcher(), gomock.Any()).Return(test.rpcRsp, test.rpcErr)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if got := w.Code; got != test.want {
			t.Errorf("proofByHash(%s)=%d; want %d", test.req, got, test.want)
		}
		if test.errStr != "" {
			if body := w.Body.String(); !strings.Contains(body, test.errStr) {
				t.Errorf("proofByHash(%q)=%q; want to find %q", test.req, body, test.errStr)
			}
			continue
		}
		if test.want != http.StatusOK {
			continue
		}
		jsonData, err := ioutil.ReadAll(w.Body)
		if err != nil {
			t.Errorf("failed to read response body: %v", err)
			continue
		}
		var resp ct.GetProofByHashResponse
		if err = json.Unmarshal(jsonData, &resp); err != nil {
			t.Errorf("Failed to unmarshal json response %s: %v", jsonData, err)
			continue
		}
		if diff := pretty.Compare(resp, test.httpRsp); diff != "" {
			t.Errorf("proofByHash(%q) diff:\n%v", test.req, diff)
		}
		if test.httpJSON != "" {
			// Also check the JSON string is as expected
			if diff := pretty.Compare(string(jsonData), test.httpJSON); diff != "" {
				t.Errorf("proofByHash(%q) diff:\n%v", test.req, diff)
			}
		}
	}
}

func TestGetSTHConsistency(t *testing.T) {
	auditHashes := [][]byte{
		[]byte("abcdef78901234567890123456789012"),
		[]byte("ghijkl78901234567890123456789012"),
		[]byte("mnopqr78901234567890123456789012"),
	}
	var tests = []struct {
		req           string
		want          int
		first, second int64
		rpcRsp        *trillian.GetConsistencyProofResponse
		httpRsp       *ct.GetSTHConsistencyResponse
		httpJSON      string
		rpcErr        error
		errStr        string
	}{
		{
			req:    "",
			want:   http.StatusBadRequest,
			errStr: "parameter 'first' is required",
		},
		{
			req:    "first=apple&second=orange",
			want:   http.StatusBadRequest,
			errStr: "parameter 'first' is malformed",
		},
		{
			req:    "first=1&last=2",
			want:   http.StatusBadRequest,
			errStr: "parameter 'second' is required",
		},
		{
			req:    "first=1&second=a",
			want:   http.StatusBadRequest,
			errStr: "parameter 'second' is malformed",
		},
		{
			req:    "first=a&second=2",
			want:   http.StatusBadRequest,
			errStr: "parameter 'first' is malformed",
		},
		{
			req:    "first=-1&second=10",
			want:   http.StatusBadRequest,
			errStr: "first and second params cannot be <0: -1 10",
		},
		{
			req:    "first=10&second=-11",
			want:   http.StatusBadRequest,
			errStr: "first and second params cannot be <0: 10 -11",
		},
		{
			req:  "first=0&second=1",
			want: http.StatusOK,
			httpRsp: &ct.GetSTHConsistencyResponse{
				Consistency: nil,
			},
			// Check a nil proof is passed through as '[]' not 'null' in raw JSON.
			httpJSON: "{\"consistency\":[]}",
		},
		{
			// Check that unrecognized parameters are ignored.
			req:     "first=0&second=1&third=2&fourth=3",
			want:    http.StatusOK,
			httpRsp: &ct.GetSTHConsistencyResponse{},
		},
		{
			req:    "first=998&second=997",
			want:   http.StatusBadRequest,
			errStr: "invalid first, second params: 998 997",
		},
		{
			req:    "first=1000&second=200",
			want:   http.StatusBadRequest,
			errStr: "invalid first, second params: 1000 200",
		},
		{
			req:    "first=10",
			want:   http.StatusBadRequest,
			errStr: "parameter 'second' is required",
		},
		{
			req:    "second=20",
			want:   http.StatusBadRequest,
			errStr: "parameter 'first' is required",
		},
		{
			req:    "first=10&second=20",
			first:  10,
			second: 20,
			want:   http.StatusInternalServerError,
			rpcErr: errors.New("RPCFAIL"),
			errStr: "RPCFAIL",
		},
		{
			req:    "first=10&second=20",
			first:  10,
			second: 20,
			want:   http.StatusInternalServerError,
			rpcRsp: &trillian.GetConsistencyProofResponse{
				Proof: &trillian.Proof{
					LeafIndex: 2,
					Hashes: [][]byte{
						auditHashes[0],
						{}, // missing hash
						auditHashes[2],
					},
				},
			},
			errStr: "invalid proof",
		},
		{
			req:    "first=10&second=20",
			first:  10,
			second: 20,
			want:   http.StatusInternalServerError,
			rpcRsp: &trillian.GetConsistencyProofResponse{
				Proof: &trillian.Proof{
					LeafIndex: 2,
					Hashes: [][]byte{
						auditHashes[0],
						auditHashes[1][:30], // wrong size hash
						auditHashes[2],
					},
				},
			},
			errStr: "invalid proof",
		},
		{
			req:    "first=10&second=20",
			first:  10,
			second: 20,
			want:   http.StatusOK,
			rpcRsp: &trillian.GetConsistencyProofResponse{
				Proof: &trillian.Proof{
					LeafIndex: 2,
					Hashes:    auditHashes,
				},
			},
			httpRsp: &ct.GetSTHConsistencyResponse{
				Consistency: auditHashes,
			},
		},
		{
			req:    "first=1&second=2",
			first:  1,
			second: 2,
			want:   http.StatusOK,
			rpcRsp: &trillian.GetConsistencyProofResponse{
				Proof: &trillian.Proof{
					LeafIndex: 0,
					Hashes:    nil,
				},
			},
			httpRsp: &ct.GetSTHConsistencyResponse{
				Consistency: nil,
			},
			// Check a nil proof is passed through as '[]' not 'null' in raw JSON.
			httpJSON: "{\"consistency\":[]}",
		},
		{
			req:    "first=332&second=332",
			first:  332,
			second: 332,
			want:   http.StatusOK,
			rpcRsp: &trillian.GetConsistencyProofResponse{
				Proof: &trillian.Proof{
					LeafIndex: 0,
					Hashes:    nil,
				},
			},
			httpRsp: &ct.GetSTHConsistencyResponse{
				Consistency: nil,
			},
			// Check a nil proof is passed through as '[]' not 'null' in raw JSON.
			httpJSON: "{\"consistency\":[]}",
		},
		{
			req:  "first=332&second=331",
			want: http.StatusBadRequest,
		},
	}

	info := setupTest(t, nil, nil)
	defer info.mockCtrl.Finish()
	handler := AppHandler{Context: info.c, Handler: getSTHConsistency, Name: "GetSTHConsistency", Method: http.MethodGet}

	for _, test := range tests {
		req, err := http.NewRequest("GET", fmt.Sprintf("/ct/v1/get-sth-consistency?%s", test.req), nil)
		if err != nil {
			t.Errorf("Failed to create request: %v", err)
			continue
		}
		if test.rpcRsp != nil || test.rpcErr != nil {
			req := trillian.GetConsistencyProofRequest{
				LogId:          0x42,
				FirstTreeSize:  test.first,
				SecondTreeSize: test.second,
			}
			info.client.EXPECT().GetConsistencyProof(deadlineMatcher(), &req).Return(test.rpcRsp, test.rpcErr)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if got := w.Code; got != test.want {
			t.Errorf("getSTHConsistency(%s)=%d; want %d", test.req, got, test.want)
		}
		if test.errStr != "" {
			if body := w.Body.String(); !strings.Contains(body, test.errStr) {
				t.Errorf("getSTHConsistency(%q)=%q; want to find %q", test.req, body, test.errStr)
			}
			continue
		}
		if test.want != http.StatusOK {
			continue
		}
		jsonData, err := ioutil.ReadAll(w.Body)
		if err != nil {
			t.Errorf("failed to read response body: %v", err)
			continue
		}
		var resp ct.GetSTHConsistencyResponse
		if err = json.Unmarshal(jsonData, &resp); err != nil {
			t.Errorf("Failed to unmarshal json response %s: %v", jsonData, err)
			continue
		}
		if diff := pretty.Compare(resp, test.httpRsp); diff != "" {
			t.Errorf("getSTHConsistency(%q) diff:\n%v", test.req, diff)
		}
		if test.httpJSON != "" {
			// Also check the JSON string is as expected
			if diff := pretty.Compare(string(jsonData), test.httpJSON); diff != "" {
				t.Errorf("getSTHConsistency(%q) diff:\n%v", test.req, diff)
			}
		}
	}
}

func TestGetEntryAndProof(t *testing.T) {
	merkleLeaf := ct.MerkleTreeLeaf{
		Version:  ct.V1,
		LeafType: ct.TimestampedEntryLeafType,
		TimestampedEntry: &ct.TimestampedEntry{
			Timestamp:  12345,
			EntryType:  ct.X509LogEntryType,
			X509Entry:  &ct.ASN1Cert{Data: []byte("certdatacertdata")},
			Extensions: ct.CTExtensions{},
		},
	}
	leafBytes, err := tls.Marshal(merkleLeaf)
	if err != nil {
		t.Fatalf("failed to build test Merkle leaf data: %v", err)
	}

	var tests = []struct {
		req    string
		want   int
		rpcRsp *trillian.GetEntryAndProofResponse
		rpcErr error
		errStr string
	}{
		{
			req:  "",
			want: http.StatusBadRequest,
		},
		{
			req:  "leaf_index=b",
			want: http.StatusBadRequest,
		},
		{
			req:  "leaf_index=1&tree_size=-1",
			want: http.StatusBadRequest,
		},
		{
			req:  "leaf_index=-1&tree_size=1",
			want: http.StatusBadRequest,
		},
		{
			req:  "leaf_index=1&tree_size=d",
			want: http.StatusBadRequest,
		},
		{
			req:  "leaf_index=&tree_size=",
			want: http.StatusBadRequest,
		},
		{
			req:  "leaf_index=",
			want: http.StatusBadRequest,
		},
		{
			req:  "leaf_index=1&tree_size=0",
			want: http.StatusBadRequest,
		},
		{
			req:  "leaf_index=10&tree_size=5",
			want: http.StatusBadRequest,
		},
		{
			req:  "leaf_index=tree_size",
			want: http.StatusBadRequest,
		},
		{
			req:    "leaf_index=1&tree_size=3",
			want:   http.StatusInternalServerError,
			rpcErr: errors.New("RPCFAIL"),
			errStr: "RPCFAIL",
		},
		{
			req:  "leaf_index=1&tree_size=3",
			want: http.StatusInternalServerError,
			// No result data in backend response
			rpcRsp: &trillian.GetEntryAndProofResponse{},
		},
		{
			req:  "leaf_index=1&tree_size=3",
			want: http.StatusOK,
			rpcRsp: &trillian.GetEntryAndProofResponse{
				Proof: &trillian.Proof{
					LeafIndex: 2,
					Hashes: [][]byte{
						[]byte("abcdef"),
						[]byte("ghijkl"),
						[]byte("mnopqr"),
					},
				},
				// To match merkleLeaf above.
				Leaf: &trillian.LogLeaf{
					LeafValue:      leafBytes,
					MerkleLeafHash: []byte("ahash"),
					ExtraData:      []byte("extra"),
				},
			},
		},
	}

	info := setupTest(t, nil, nil)
	defer info.mockCtrl.Finish()
	handler := AppHandler{Context: info.c, Handler: getEntryAndProof, Name: "GetEntryAndProof", Method: http.MethodGet}

	for _, test := range tests {
		req, err := http.NewRequest("GET", fmt.Sprintf("/ct/v1/get-entry-and-proof?%s", test.req), nil)
		if err != nil {
			t.Errorf("Failed to create request: %v", err)
			continue
		}

		if test.rpcRsp != nil || test.rpcErr != nil {
			info.client.EXPECT().GetEntryAndProof(deadlineMatcher(), &trillian.GetEntryAndProofRequest{LogId: 0x42, LeafIndex: 1, TreeSize: 3}).Return(test.rpcRsp, test.rpcErr)
		}

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if got := w.Code; got != test.want {
			t.Errorf("getEntryAndProof(%s)=%d; want %d", test.req, got, test.want)
		}
		if test.errStr != "" {
			if body := w.Body.String(); !strings.Contains(body, test.errStr) {
				t.Errorf("getEntryAndProof(%q)=%q; want to find %q", test.req, body, test.errStr)
			}
			continue
		}
		if test.want != http.StatusOK {
			continue
		}

		var resp ct.GetEntryAndProofResponse
		if err = json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Errorf("Failed to unmarshal json response %s: %v", w.Body.Bytes(), err)
			continue
		}
		// The result we expect after a roundtrip in the successful get entry and proof test
		wantRsp := ct.GetEntryAndProofResponse{
			LeafInput: leafBytes,
			ExtraData: []byte("extra"),
			AuditPath: [][]byte{[]byte("abcdef"), []byte("ghijkl"), []byte("mnopqr")},
		}
		if diff := pretty.Compare(resp, wantRsp); diff != "" {
			t.Errorf("getEntryAndProof(%q) diff:\n%v", test.req, diff)
		}
	}
}

func createJSONChain(t *testing.T, p PEMCertPool) io.Reader {
	t.Helper()
	var req ct.AddChainRequest
	for _, rawCert := range p.RawCertificates() {
		req.Chain = append(req.Chain, rawCert.Raw)
	}

	var buffer bytes.Buffer
	// It's tempting to avoid creating and flushing the intermediate writer but it doesn't work
	writer := bufio.NewWriter(&buffer)
	err := json.NewEncoder(writer).Encode(&req)
	writer.Flush()

	if err != nil {
		t.Fatalf("Failed to create test json: %v", err)
	}

	return bufio.NewReader(&buffer)
}

func logLeavesForCert(t *testing.T, certs []*x509.Certificate, merkleLeaf *ct.MerkleTreeLeaf, isPrecert bool) []*trillian.LogLeaf {
	t.Helper()
	leafData, err := tls.Marshal(*merkleLeaf)
	if err != nil {
		t.Fatalf("failed to serialize leaf: %v", err)
	}

	leafIDHash := sha256.Sum256(certs[0].Raw)

	extraData, err := extraDataForChain(certs, isPrecert)
	if err != nil {
		t.Fatalf("failed to serialize extra data: %v", err)
	}

	return []*trillian.LogLeaf{{LeafIdentityHash: leafIDHash[:], LeafValue: leafData, ExtraData: extraData}}
}

type dlMatcher struct {
}

func deadlineMatcher() gomock.Matcher {
	return dlMatcher{}
}

func (d dlMatcher) Matches(x interface{}) bool {
	ctx, ok := x.(context.Context)
	if !ok {
		return false
	}

	deadlineTime, ok := ctx.Deadline()
	if !ok {
		return false // we never make RPC calls without a deadline set
	}

	return deadlineTime == fakeDeadlineTime
}

func (d dlMatcher) String() string {
	return fmt.Sprintf("deadline is %v", fakeDeadlineTime)
}

func makeAddPrechainRequest(t *testing.T, c *LogContext, body io.Reader) *httptest.ResponseRecorder {
	t.Helper()
	handler := AppHandler{Context: c, Handler: addPreChain, Name: "AddPreChain", Method: http.MethodPost}
	return makeAddChainRequestInternal(t, handler, "add-pre-chain", body)
}

func makeAddChainRequest(t *testing.T, c *LogContext, body io.Reader) *httptest.ResponseRecorder {
	t.Helper()
	handler := AppHandler{Context: c, Handler: addChain, Name: "AddChain", Method: http.MethodPost}
	return makeAddChainRequestInternal(t, handler, "add-chain", body)
}

func makeAddChainRequestInternal(t *testing.T, handler AppHandler, path string, body io.Reader) *httptest.ResponseRecorder {
	t.Helper()
	req, err := http.NewRequest("POST", fmt.Sprintf("http://example.com/ct/v1/%s", path), body)
	if err != nil {
		t.Fatalf("Failed to create POST request: %v", err)
	}

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	return w
}

func makeGetRootResponseForTest(stamp, treeSize int64, hash []byte) *trillian.GetLatestSignedLogRootResponse {
	return &trillian.GetLatestSignedLogRootResponse{
		SignedLogRoot: &trillian.SignedLogRoot{
			TimestampNanos: stamp,
			TreeSize:       treeSize,
			RootHash:       hash,
		},
	}
}

func loadCertsIntoPoolOrDie(t *testing.T, certs []string) *PEMCertPool {
	t.Helper()
	pool := NewPEMCertPool()
	for _, cert := range certs {
		if !pool.AppendCertsFromPEM([]byte(cert)) {
			t.Fatalf("couldn't parse test certs: %v", certs)
		}
	}
	return pool
}
