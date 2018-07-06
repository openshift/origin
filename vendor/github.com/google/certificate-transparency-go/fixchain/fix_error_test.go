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

package fixchain

import (
	"errors"
	"fmt"
	"testing"

	"github.com/google/certificate-transparency-go/x509"
)

func TestEqual(t *testing.T) {
	equalTests := []struct {
		e        *FixError
		f        *FixError
		expEqual bool
	}{
		{
			&FixError{},
			&FixError{},
			true,
		},
		{
			&FixError{Type: LogPostFailed},
			&FixError{},
			false,
		},
		{
			&FixError{Type: ParseFailure},
			&FixError{Type: LogPostFailed},
			false,
		},
		{
			&FixError{Cert: GetTestCertificateFromPEM(t, googleLeaf)},
			&FixError{},
			false,
		},
		{
			&FixError{Cert: GetTestCertificateFromPEM(t, googleLeaf)},
			&FixError{Cert: GetTestCertificateFromPEM(t, megaLeaf)},
			false,
		},
		{
			&FixError{
				Chain: []*x509.Certificate{
					GetTestCertificateFromPEM(t, googleLeaf),
					GetTestCertificateFromPEM(t, thawteIntermediate),
					GetTestCertificateFromPEM(t, verisignRoot),
				},
			},
			&FixError{},
			false,
		},
		{ // Chains with only one cert different.
			&FixError{
				Chain: []*x509.Certificate{
					GetTestCertificateFromPEM(t, googleLeaf),
					GetTestCertificateFromPEM(t, thawteIntermediate),
					GetTestCertificateFromPEM(t, verisignRoot),
				},
			},
			&FixError{
				Chain: []*x509.Certificate{
					GetTestCertificateFromPEM(t, googleLeaf),
					GetTestCertificateFromPEM(t, thawteIntermediate),
					GetTestCertificateFromPEM(t, comodoRoot),
				},
			},
			false,
		},
		{ // Completely different chains.
			&FixError{
				Chain: []*x509.Certificate{
					GetTestCertificateFromPEM(t, googleLeaf),
					GetTestCertificateFromPEM(t, thawteIntermediate),
					GetTestCertificateFromPEM(t, verisignRoot),
				},
			},
			&FixError{
				Chain: []*x509.Certificate{
					GetTestCertificateFromPEM(t, megaLeaf),
					GetTestCertificateFromPEM(t, comodoIntermediate),
					GetTestCertificateFromPEM(t, comodoRoot),
				},
			},
			false,
		},
		{
			&FixError{URL: "https://www.test.com"},
			&FixError{},
			false,
		},
		{
			&FixError{URL: "https://www.test.com"},
			&FixError{URL: "https://www.test1.com"},
			false,
		},
		{
			&FixError{Bad: []byte(googleLeaf)},
			&FixError{},
			false,
		},
		{
			&FixError{Bad: []byte(googleLeaf)},
			&FixError{Bad: []byte(megaLeaf)},
			false,
		},
		{
			&FixError{Error: errors.New("Error1")},
			&FixError{},
			false,
		},
		{
			&FixError{Error: errors.New("Error1")},
			&FixError{Error: errors.New("Error2")},
			false,
		},
		{
			&FixError{
				Type: LogPostFailed,
				Cert: GetTestCertificateFromPEM(t, googleLeaf),
				Chain: []*x509.Certificate{
					GetTestCertificateFromPEM(t, googleLeaf),
					GetTestCertificateFromPEM(t, thawteIntermediate),
					GetTestCertificateFromPEM(t, verisignRoot),
				},
				URL:   "https://www.test.com",
				Bad:   GetTestCertificateFromPEM(t, googleLeaf).Raw,
				Error: errors.New("Log Post Failed"),
			},
			&FixError{},
			false,
		},
		{
			&FixError{},
			&FixError{
				Type: LogPostFailed,
				Cert: GetTestCertificateFromPEM(t, googleLeaf),
				Chain: []*x509.Certificate{
					GetTestCertificateFromPEM(t, googleLeaf),
					GetTestCertificateFromPEM(t, thawteIntermediate),
					GetTestCertificateFromPEM(t, verisignRoot),
				},
				URL:   "https://www.test.com",
				Bad:   GetTestCertificateFromPEM(t, googleLeaf).Raw,
				Error: errors.New("Log Post Failed"),
			},
			false,
		},
		{
			&FixError{
				Type: LogPostFailed,
				Cert: GetTestCertificateFromPEM(t, googleLeaf),
				Chain: []*x509.Certificate{
					GetTestCertificateFromPEM(t, googleLeaf),
					GetTestCertificateFromPEM(t, thawteIntermediate),
					GetTestCertificateFromPEM(t, verisignRoot),
				},
				URL:   "https://www.test.com",
				Bad:   GetTestCertificateFromPEM(t, googleLeaf).Raw,
				Error: errors.New("Log Post Failed"),
			},
			&FixError{
				Type: LogPostFailed,
				Cert: GetTestCertificateFromPEM(t, googleLeaf),
				Chain: []*x509.Certificate{
					GetTestCertificateFromPEM(t, googleLeaf),
					GetTestCertificateFromPEM(t, thawteIntermediate),
					GetTestCertificateFromPEM(t, verisignRoot),
				},
				URL:   "https://www.test.com",
				Bad:   GetTestCertificateFromPEM(t, googleLeaf).Raw,
				Error: errors.New("Log Post Failed"),
			},
			true,
		},
		{ // nil test
			&FixError{
				Type: LogPostFailed,
				Cert: GetTestCertificateFromPEM(t, googleLeaf),
				Chain: []*x509.Certificate{
					GetTestCertificateFromPEM(t, googleLeaf),
					GetTestCertificateFromPEM(t, thawteIntermediate),
					GetTestCertificateFromPEM(t, verisignRoot),
				},
				URL:   "https://www.test.com",
				Bad:   GetTestCertificateFromPEM(t, googleLeaf).Raw,
				Error: errors.New("Log Post Failed"),
			},
			nil,
			false,
		},
	}

	for i, test := range equalTests {
		if test.e.Equal(test.f) != test.expEqual {
			t.Errorf("#%d: expected FixError.Equal() to return %t, returned %t", i, test.expEqual, !test.expEqual)
		}
	}
}

func TestTypeString(t *testing.T) {
	typeStringTests := []struct {
		ferr     FixError
		expected string
	}{
		{
			FixError{Type: None},
			"None",
		},
		{
			FixError{Type: ParseFailure},
			"ParseFailure",
		},
		{
			FixError{Type: CannotFetchURL},
			"CannotFetchURL",
		},
		{
			FixError{Type: FixFailed},
			"FixFailed",
		},
		{
			FixError{Type: LogPostFailed},
			"LogPostFailed",
		},
		{
			FixError{Type: VerifyFailed},
			"VerifyFailed",
		},
		{
			FixError{},
			"None",
		},
	}

	for i, test := range typeStringTests {
		if got, want := test.ferr.TypeString(), test.expected; got != want {
			t.Errorf("#%d: TypeString() returned %s, expected %s.", i, got, want)
		}
	}
}

func TestString(t *testing.T) {
	stringTests := []struct {
		ferr *FixError
		str  string
	}{
		{
			&FixError{Type: None},
			"None\n",
		},
		{
			&FixError{
				Type: LogPostFailed,
				Cert: GetTestCertificateFromPEM(t, googleLeaf),
				Chain: []*x509.Certificate{
					GetTestCertificateFromPEM(t, googleLeaf),
					GetTestCertificateFromPEM(t, thawteIntermediate),
					GetTestCertificateFromPEM(t, verisignRoot),
				},
				URL:   "https://www.test.com",
				Error: errors.New("Log Post Failed"),
			},
			"LogPostFailed\n" +
				"Error: Log Post Failed\n" +
				"URL: https://www.test.com\n" +
				"Cert: " + googleLeaf +
				"Chain: " + googleLeaf + thawteIntermediate + verisignRoot,
		},
	}

	for i, test := range stringTests {
		if got, want := test.ferr.String(), test.str; got != want {
			t.Errorf("#%d: String() returned %s, expected %s.", i, got, want)
		}
	}
}

func TestMarshalJSON(t *testing.T) {
	marshalJSONTests := []*FixError{
		{},
		{
			Type: LogPostFailed,
			Cert: GetTestCertificateFromPEM(t, googleLeaf),
			Chain: []*x509.Certificate{
				GetTestCertificateFromPEM(t, googleLeaf),
				GetTestCertificateFromPEM(t, thawteIntermediate),
				GetTestCertificateFromPEM(t, verisignRoot),
			},
			URL:   "https://www.test.com",
			Bad:   GetTestCertificateFromPEM(t, googleLeaf).Raw,
			Error: errors.New("Log Post Failed"),
		},
	}

	for i, test := range marshalJSONTests {
		b, err := test.MarshalJSON()
		if err != nil {
			t.Errorf("#%d: Error marshaling json: %s", i, err.Error())
		}

		ferr, err := UnmarshalJSON(b)
		if err != nil {
			t.Errorf("#%d: Error unmarshaling json: %s", i, err.Error())
		}

		if !test.Equal(ferr) {
			t.Errorf("#%d: Original FixError does not match marshaled-then-unmarshaled FixError", i)
		}
	}
}

func TestDumpPEM(t *testing.T) {
	dumpPEMTests := []string{googleLeaf}

	for i, test := range dumpPEMTests {
		cert := GetTestCertificateFromPEM(t, test)
		p := dumpPEM(cert.Raw)
		certFromPEM := GetTestCertificateFromPEM(t, p)
		if !cert.Equal(certFromPEM) {
			t.Errorf("#%d: cert from output of dumpPEM() does not match original", i)
		}
	}
}

func TestDumpChainPEM(t *testing.T) {
	dumpChainPEMTests := []struct {
		chain    []string
		expected string
	}{
		{
			[]string{googleLeaf, thawteIntermediate},
			fmt.Sprintf("%s%s", googleLeaf, thawteIntermediate),
		},
	}

	for i, test := range dumpChainPEMTests {
		chain := extractTestChain(t, i, test.chain)
		if got := dumpChainPEM(chain); got != test.expected {
			t.Errorf("#%d: dumpChainPEM() returned %s, expected %s", i, got, test.expected)
		}
	}
}
