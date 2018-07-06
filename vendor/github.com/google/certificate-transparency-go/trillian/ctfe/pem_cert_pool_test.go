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
	"testing"

	"github.com/google/certificate-transparency-go/trillian/ctfe/testonly"
)

func TestLoadSingleCertFromPEMs(t *testing.T) {
	for _, pem := range []string{testonly.CACertPEM, testonly.CACertPEMWithOtherStuff, testonly.CACertPEMDuplicated} {
		pool := NewPEMCertPool()

		ok := pool.AppendCertsFromPEM([]byte(pem))
		if !ok {
			t.Fatal("Expected to append a certificate ok")
		}
		if got, want := len(pool.Subjects()), 1; got != want {
			t.Fatalf("Got %d cert(s) in the pool, expected %d", got, want)
		}
	}
}

func TestBadOrEmptyCertificateRejected(t *testing.T) {
	for _, pem := range []string{testonly.UnknownBlockTypePEM, testonly.CACertPEMBad} {
		pool := NewPEMCertPool()

		ok := pool.AppendCertsFromPEM([]byte(pem))
		if ok {
			t.Fatal("Expected appending no certs")
		}
		if got, want := len(pool.Subjects()), 0; got != want {
			t.Fatalf("Got %d cert(s) in pool, expected %d", got, want)
		}
	}
}

func TestLoadMultipleCertsFromPEM(t *testing.T) {
	pool := NewPEMCertPool()

	ok := pool.AppendCertsFromPEM([]byte(testonly.CACertMultiplePEM))
	if !ok {
		t.Fatal("Rejected valid multiple certs")
	}
	if got, want := len(pool.Subjects()), 2; got != want {
		t.Fatalf("Got %d certs in pool, expected %d", got, want)
	}
}
