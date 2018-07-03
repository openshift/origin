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

package jsonclient

import (
	"context"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/certificate-transparency-go/testdata"
)

func publicKeyPEMToDER(key string) []byte {
	block, _ := pem.Decode([]byte(key))
	if block == nil {
		panic("failed to decode public key PEM")
	}
	if block.Type != "PUBLIC KEY" {
		panic("PEM does not have type 'PUBLIC KEY'")
	}
	return block.Bytes
}

func TestNewJSONClient(t *testing.T) {
	tests := []struct {
		name   string
		opts   Options
		errstr string
	}{
		{
			name:   "invalid PublicKey",
			opts:   Options{PublicKey: "bogus"},
			errstr: "no PEM block",
		},
		{
			name:   "invalid PublicKeyDER",
			opts:   Options{PublicKeyDER: []byte("bogus")},
			errstr: "asn1: structure error",
		},
		{
			name: "RSA PublicKey",
			opts: Options{PublicKey: testdata.RsaPublicKeyPEM},
		},
		{
			name: "RSA PublicKeyDER",
			opts: Options{PublicKeyDER: publicKeyPEMToDER(testdata.RsaPublicKeyPEM)},
		},
		{
			name: "ECDSA PublicKey",
			opts: Options{PublicKey: testdata.EcdsaPublicKeyPEM},
		},
		{
			name: "ECDSA PublicKeyDER",
			opts: Options{PublicKeyDER: publicKeyPEMToDER(testdata.EcdsaPublicKeyPEM)},
		},
		{
			name:   "DSA PublicKey",
			opts:   Options{PublicKey: testdata.DsaPublicKeyPEM},
			errstr: "Unsupported public key type",
		},
		{
			name:   "DSA PublicKeyDER",
			opts:   Options{PublicKeyDER: publicKeyPEMToDER(testdata.DsaPublicKeyPEM)},
			errstr: "Unsupported public key type",
		},
		{
			name:   "PublicKey contains trailing garbage",
			opts:   Options{PublicKey: testdata.RsaPublicKeyPEM + "bogus"},
			errstr: "extra data found",
		},
		{
			name:   "PublicKeyDER contains trailing garbage",
			opts:   Options{PublicKeyDER: append(publicKeyPEMToDER(testdata.RsaPublicKeyPEM), []byte("deadbeef")...)},
			errstr: "trailing data",
		},
	}
	for _, test := range tests {
		client, err := New("http://127.0.0.1", nil, test.opts)
		if test.errstr != "" {
			if err == nil {
				t.Errorf("%v: New()=%p,nil; want error %q", test.name, client, test.errstr)
			} else if !strings.Contains(err.Error(), test.errstr) {
				t.Errorf("%v: New()=nil,%q; want error %q", test.name, err, test.errstr)
			}
			continue
		}
		if err != nil {
			t.Errorf("%v: New()=nil,%q; want no error", test.name, err)
		} else if client == nil {
			t.Errorf("%v: New()=nil,nil; want client", test.name)
		}
	}
}

type TestStruct struct {
	TreeSize  int    `json:"tree_size"`
	Timestamp int    `json:"timestamp"`
	Data      string `json:"data"`
}

type TestParams struct {
	RespCode int `json:"rc"`
}

func MockServer(t *testing.T, failCount int, retryAfter int) *httptest.Server {
	t.Helper()
	mu := sync.Mutex{}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch r.URL.Path {
		case "/struct/path":
			fmt.Fprintf(w, `{"tree_size": 11, "timestamp": 99}`)
		case "/struct/params":
			var s TestStruct
			if r.Method == http.MethodGet {
				s.TreeSize, _ = strconv.Atoi(r.FormValue("tree_size"))
				s.Timestamp, _ = strconv.Atoi(r.FormValue("timestamp"))
				s.Data = r.FormValue("data")
			} else {
				decoder := json.NewDecoder(r.Body)
				err := decoder.Decode(&s)
				if err != nil {
					panic("Failed to decode: " + err.Error())
				}
				defer r.Body.Close()
			}
			fmt.Fprintf(w, `{"tree_size": %d, "timestamp": %d, "data": "%s"}`, s.TreeSize, s.Timestamp, s.Data)
		case "/error":
			var params TestParams
			if r.Method == http.MethodGet {
				params.RespCode, _ = strconv.Atoi(r.FormValue("rc"))
			} else {
				decoder := json.NewDecoder(r.Body)
				err := decoder.Decode(&params)
				if err != nil {
					panic("Failed to decode: " + err.Error())
				}
				defer r.Body.Close()
			}
			http.Error(w, "error page", params.RespCode)
		case "/malformed":
			fmt.Fprintf(w, `{"tree_size": 11, "timestamp": 99`) // no closing }
		case "/retry":
			if failCount > 0 {
				failCount--
				if retryAfter != 0 {
					if retryAfter > 0 {
						w.Header().Add("Retry-After", strconv.Itoa(retryAfter))
					}
					w.WriteHeader(http.StatusServiceUnavailable)
				} else {
					w.WriteHeader(http.StatusRequestTimeout)
				}
			} else {
				fmt.Fprintf(w, `{"tree_size": 11, "timestamp": 99}`)
			}
		case "/retry-rfc1123":
			if failCount > 0 {
				failCount--
				w.Header().Add("Retry-After", time.Now().Add(time.Duration(retryAfter)*time.Second).Format(time.RFC1123))
				w.WriteHeader(http.StatusServiceUnavailable)
			} else {
				fmt.Fprintf(w, `{"tree_size": 11, "timestamp": 99}`)
			}
		default:
			t.Fatalf("Unhandled URL path: %s", r.URL.Path)
		}
	}))
}

func TestGetAndParse(t *testing.T) {
	rc := regexp.MustCompile
	tests := []struct {
		uri      string
		params   map[string]string
		status   int
		result   TestStruct
		errstr   *regexp.Regexp
		wantBody bool
	}{
		{uri: "/short%", errstr: rc("invalid URL escape")},
		{uri: "/malformed", status: http.StatusOK, errstr: rc("unexpected EOF"), wantBody: true},
		{uri: "/error", params: map[string]string{"rc": "404"}, status: http.StatusNotFound, wantBody: true},
		{uri: "/error", params: map[string]string{"rc": "403"}, status: http.StatusForbidden, wantBody: true},
		{uri: "/struct/path", status: http.StatusOK, result: TestStruct{11, 99, ""}, wantBody: true},
		{
			uri:      "/struct/params",
			status:   http.StatusOK,
			params:   map[string]string{"tree_size": "42", "timestamp": "88", "data": "abcd"},
			result:   TestStruct{42, 88, "abcd"},
			wantBody: true,
		},
	}

	ts := MockServer(t, -1, 0)
	defer ts.Close()

	logClient, err := New(ts.URL, &http.Client{}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	for _, test := range tests {
		var result TestStruct
		httpRsp, body, err := logClient.GetAndParse(ctx, test.uri, test.params, &result)
		if gotBody := (body != nil); gotBody != test.wantBody {
			t.Errorf("GetAndParse(%q) got body? %v, want? %v", test.uri, gotBody, test.wantBody)
		}
		if test.errstr != nil {
			if err == nil {
				t.Errorf("GetAndParse(%q)=%+v,_,nil; want error matching %q", test.uri, result, test.errstr)
			} else if !test.errstr.MatchString(err.Error()) {
				t.Errorf("GetAndParse(%q)=nil,_,%q; want error matching %q", test.uri, err.Error(), test.errstr)
			}
			continue
		}
		if httpRsp.StatusCode != test.status {
			t.Errorf("GetAndParse('%s') got status %d; want %d", test.uri, httpRsp.StatusCode, test.status)
		}
		if test.status == http.StatusOK {
			if err != nil {
				t.Errorf("GetAndParse(%q)=nil,_,%q; want %+v", test.uri, err.Error(), result)
			}
			if !reflect.DeepEqual(result, test.result) {
				t.Errorf("GetAndParse(%q)=%+v,_,nil; want %+v", test.uri, result, test.result)
			}
		}
	}
}

func TestPostAndParse(t *testing.T) {
	rc := regexp.MustCompile
	tests := []struct {
		uri      string
		request  interface{}
		status   int
		result   TestStruct
		errstr   *regexp.Regexp
		wantBody bool
	}{
		{uri: "/short%", errstr: rc("invalid URL escape")},
		{uri: "/struct/params", request: json.Number(`invalid`), errstr: rc("invalid number literal")},
		{uri: "/malformed", status: http.StatusOK, errstr: rc("unexpected end of JSON"), wantBody: true},
		{uri: "/error", request: TestParams{RespCode: 404}, status: http.StatusNotFound, wantBody: true},
		{uri: "/error", request: TestParams{RespCode: 403}, status: http.StatusForbidden, wantBody: true},
		{uri: "/struct/path", status: http.StatusOK, result: TestStruct{11, 99, ""}, wantBody: true},
		{
			uri:      "/struct/params",
			status:   http.StatusOK,
			request:  TestStruct{42, 88, "abcd"},
			result:   TestStruct{42, 88, "abcd"},
			wantBody: true,
		},
	}

	ts := MockServer(t, -1, 0)
	defer ts.Close()

	logClient, err := New(ts.URL, &http.Client{}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	for _, test := range tests {
		var result TestStruct
		httpRsp, body, err := logClient.PostAndParse(ctx, test.uri, test.request, &result)
		if gotBody := (body != nil); gotBody != test.wantBody {
			t.Errorf("GetAndParse(%q) returned body %v, wanted %v", test.uri, gotBody, test.wantBody)
		}
		if test.errstr != nil {
			if err == nil {
				t.Errorf("PostAndParse(%q)=%+v,nil; want error matching %q", test.uri, result, test.errstr)
			} else if !test.errstr.MatchString(err.Error()) {
				t.Errorf("PostAndParse(%q)=nil,%q; want error matching %q", test.uri, err.Error(), test.errstr)
			}
			continue
		}
		if httpRsp.StatusCode != test.status {
			t.Errorf("PostAndParse(%q) got status %d; want %d", test.uri, httpRsp.StatusCode, test.status)
		}
		if test.status == http.StatusOK {
			if err != nil {
				t.Errorf("PostAndParse(%q)=nil,%q; want %+v", test.uri, err.Error(), test.result)
			}
			if !reflect.DeepEqual(result, test.result) {
				t.Errorf("PostAndParse(%q)=%+v,nil; want %+v", test.uri, result, test.result)
			}
		}
	}
}

// mockBackoff is not safe for concurrent usage
type mockBackoff struct {
	override time.Duration
}

func (mb *mockBackoff) set(o *time.Duration) time.Duration {
	if o != nil {
		mb.override = *o
	}
	return 0
}
func (mb *mockBackoff) decreaseMultiplier() {}
func (mb *mockBackoff) until() time.Time    { return time.Time{} }

func TestPostAndParseWithRetry(t *testing.T) {
	tests := []struct {
		uri             string
		request         interface{}
		deadlineSecs    int // -1 indicates no deadline
		retryAfter      int // -1 indicates generate 503 with no Retry-After
		failCount       int
		errstr          string
		expectedBackoff time.Duration // 0 indicates no expected backoff override set
	}{
		{
			uri:             "/error",
			request:         TestParams{RespCode: 418},
			deadlineSecs:    -1,
			retryAfter:      0,
			failCount:       0,
			errstr:          "teapot",
			expectedBackoff: 0,
		},
		{
			uri:             "/short%",
			request:         nil,
			deadlineSecs:    0,
			retryAfter:      0,
			failCount:       0,
			errstr:          "deadline exceeded",
			expectedBackoff: 0,
		},
		{
			uri:             "/retry",
			request:         nil,
			deadlineSecs:    -1,
			retryAfter:      0,
			failCount:       1,
			errstr:          "",
			expectedBackoff: 0,
		},
		{
			uri:             "/retry",
			request:         nil,
			deadlineSecs:    -1,
			retryAfter:      5,
			failCount:       1,
			errstr:          "",
			expectedBackoff: 5 * time.Second,
		},
		{
			uri:             "/retry-rfc1123",
			request:         nil,
			deadlineSecs:    -1,
			retryAfter:      5,
			failCount:       1,
			errstr:          "",
			expectedBackoff: 5 * time.Second,
		},
	}
	for _, test := range tests {
		ts := MockServer(t, test.failCount, test.retryAfter)
		defer ts.Close()

		logClient, err := New(ts.URL, &http.Client{}, Options{})
		if err != nil {
			t.Fatal(err)
		}
		mb := mockBackoff{}
		logClient.backoff = &mb
		ctx := context.Background()
		if test.deadlineSecs >= 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithDeadline(context.Background(), time.Now().Add(time.Duration(test.deadlineSecs)*time.Second))
			defer cancel()
		}

		var result TestStruct
		httpRsp, _, err := logClient.PostAndParseWithRetry(ctx, test.uri, test.request, &result)
		if test.errstr != "" {
			if err == nil {
				t.Errorf("PostAndParseWithRetry()=%+v,nil; want error %q", result, test.errstr)
			} else if !strings.Contains(err.Error(), test.errstr) {
				t.Errorf("PostAndParseWithRetry()=nil,%q; want error %q", err.Error(), test.errstr)
			}
			continue
		}
		if err != nil {
			t.Errorf("PostAndParseWithRetry()=nil,%q; want no error", err.Error())
		} else if httpRsp.StatusCode != http.StatusOK {
			t.Errorf("PostAndParseWithRetry() got status %d; want OK(404)", httpRsp.StatusCode)
		}
		if test.expectedBackoff > 0 && !fuzzyDurationEquals(test.expectedBackoff, mb.override, time.Second) {
			t.Errorf("Unexpected backoff override set: got: %s, wanted: %s", mb.override, test.expectedBackoff)
		}
	}
}

func TestContextRequired(t *testing.T) {
	ts := MockServer(t, -1, 0)
	defer ts.Close()

	logClient, err := New(ts.URL, &http.Client{}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	var result TestStruct
	_, _, err = logClient.GetAndParse(nil, "/struct/path", nil, &result)
	if err == nil {
		t.Errorf("GetAndParse() succeeded with empty Context")
	}
	_, _, err = logClient.PostAndParse(nil, "/struct/path", nil, &result)
	if err == nil {
		t.Errorf("PostAndParse() succeeded with empty Context")
	}
	_, _, err = logClient.PostAndParseWithRetry(nil, "/struct/path", nil, &result)
	if err == nil {
		t.Errorf("PostAndParseWithRetry() succeeded with empty Context")
	}
}
