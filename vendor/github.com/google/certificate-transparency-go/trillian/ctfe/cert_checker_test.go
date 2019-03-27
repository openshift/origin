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
	"encoding/base64"
	"encoding/pem"
	"testing"
	"time"

	"github.com/google/certificate-transparency-go/trillian/ctfe/testonly"
	"github.com/google/certificate-transparency-go/x509"
	"github.com/google/certificate-transparency-go/x509/pkix"
	"github.com/google/certificate-transparency-go/x509util"
)

func wipeExtensions(cert *x509.Certificate) *x509.Certificate {
	cert.Extensions = cert.Extensions[:0]
	return cert
}

func makePoisonNonCritical(cert *x509.Certificate) *x509.Certificate {
	// Invalid as a pre-cert because poison extension needs to be marked as critical.
	cert.Extensions = []pkix.Extension{{Id: ctPoisonExtensionOID, Critical: false, Value: asn1NullBytes}}
	return cert
}

func makePoisonNonNull(cert *x509.Certificate) *x509.Certificate {
	// Invalid as a pre-cert because poison extension is not ASN.1 NULL value.
	cert.Extensions = []pkix.Extension{{Id: ctPoisonExtensionOID, Critical: false, Value: []byte{0x42, 0x42, 0x42}}}
	return cert
}

func TestIsPrecertificate(t *testing.T) {
	var tests = []struct {
		desc        string
		cert        *x509.Certificate
		wantPrecert bool
		wantErr     bool
	}{
		{
			desc:        "valid-precert",
			cert:        pemToCert(t, testonly.PrecertPEMValid),
			wantPrecert: true,
		},
		{
			desc:        "valid-cert",
			cert:        pemToCert(t, testonly.CACertPEM),
			wantPrecert: false,
		},
		{
			desc:        "remove-exts-from-precert",
			cert:        wipeExtensions(pemToCert(t, testonly.PrecertPEMValid)),
			wantPrecert: false,
		},
		{
			desc:        "poison-non-critical",
			cert:        makePoisonNonCritical(pemToCert(t, testonly.PrecertPEMValid)),
			wantPrecert: false,
			wantErr:     true,
		},
		{
			desc:        "poison-non-null",
			cert:        makePoisonNonNull(pemToCert(t, testonly.PrecertPEMValid)),
			wantPrecert: false,
			wantErr:     true,
		},
	}

	for _, test := range tests {
		gotPrecert, err := IsPrecertificate(test.cert)
		t.Run(test.desc, func(t *testing.T) {
			if err != nil {
				if !test.wantErr {
					t.Errorf("IsPrecertificate()=%v,%v; want %v,nil", gotPrecert, err, test.wantPrecert)
				}
				return
			}
			if test.wantErr {
				t.Errorf("IsPrecertificate()=%v,%v; want _,%v", gotPrecert, err, test.wantErr)
			}
			if gotPrecert != test.wantPrecert {
				t.Errorf("IsPrecertificate()=%v,%v; want %v,nil", gotPrecert, err, test.wantPrecert)
			}
		})
	}
}

func TestValidateChain(t *testing.T) {
	fakeCARoots := NewPEMCertPool()
	if !fakeCARoots.AppendCertsFromPEM([]byte(testonly.FakeCACertPEM)) {
		t.Fatal("failed to load fake root")
	}
	if !fakeCARoots.AppendCertsFromPEM([]byte(testonly.FakeRootCACertPEM)) {
		t.Fatal("failed to load fake root")
	}
	validateOpts := CertValidationOpts{
		trustedRoots: fakeCARoots,
		extKeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
	}

	var tests = []struct {
		desc        string
		chain       [][]byte
		wantErr     bool
		wantPathLen int
	}{
		{
			desc:    "missing-intermediate-cert",
			chain:   pemsToDERChain(t, []string{testonly.LeafSignedByFakeIntermediateCertPEM}),
			wantErr: true,
		},
		{
			desc:    "wrong-cert-order",
			chain:   pemsToDERChain(t, []string{testonly.FakeIntermediateCertPEM, testonly.LeafSignedByFakeIntermediateCertPEM}),
			wantErr: true,
		},
		{
			desc:    "unrelated-cert-in-chain",
			chain:   pemsToDERChain(t, []string{testonly.FakeIntermediateCertPEM, testonly.TestCertPEM}),
			wantErr: true,
		},
		{
			desc:    "unrelated-cert-after-chain",
			chain:   pemsToDERChain(t, []string{testonly.LeafSignedByFakeIntermediateCertPEM, testonly.FakeIntermediateCertPEM, testonly.TestCertPEM}),
			wantErr: true,
		},
		{
			desc:        "valid-chain",
			chain:       pemsToDERChain(t, []string{testonly.LeafSignedByFakeIntermediateCertPEM, testonly.FakeIntermediateCertPEM}),
			wantPathLen: 3,
		},
		{
			desc:        "valid-chain-with-policyconstraints",
			chain:       pemsToDERChain(t, []string{testonly.LeafCertPEM, testonly.FakeIntermediateWithPolicyConstraintsCertPEM}),
			wantPathLen: 3,
		},
		{
			desc:        "valid-chain-with-policyconstraints-inc-root",
			chain:       pemsToDERChain(t, []string{testonly.LeafCertPEM, testonly.FakeIntermediateWithPolicyConstraintsCertPEM, testonly.FakeRootCACertPEM}),
			wantPathLen: 3,
		},
		{
			desc:        "valid-chain-with-nameconstraints",
			chain:       pemsToDERChain(t, []string{testonly.LeafCertPEM, testonly.FakeIntermediateWithNameConstraintsCertPEM}),
			wantPathLen: 3,
		},
		{
			desc:        "chain-with-invalid-nameconstraints",
			chain:       pemsToDERChain(t, []string{testonly.LeafCertPEM, testonly.FakeIntermediateWithInvalidNameConstraintsCertPEM}),
			wantPathLen: 3,
		},
		{
			desc:        "chain-of-len-4",
			chain:       pemFileToDERChain(t, "../testdata/subleaf.chain"),
			wantPathLen: 4,
		},
		{
			desc:    "misordered-chain-of-len-4",
			chain:   pemFileToDERChain(t, "../testdata/subleaf.misordered.chain"),
			wantErr: true,
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			gotPath, err := ValidateChain(test.chain, validateOpts)
			if err != nil {
				if !test.wantErr {
					t.Errorf("ValidateChain()=%v,%v; want _,nil", gotPath, err)
				}
				return
			}
			if test.wantErr {
				t.Errorf("ValidateChain()=%v,%v; want _,non-nil", gotPath, err)
				return
			}
			if len(gotPath) != test.wantPathLen {
				t.Errorf("|ValidateChain()|=%d; want %d", len(gotPath), test.wantPathLen)
				for _, c := range gotPath {
					t.Logf("Subject: %s Issuer: %s", x509util.NameToString(c.Subject), x509util.NameToString(c.Issuer))
				}
			}
		})
	}
}

func TestCA(t *testing.T) {
	fakeCARoots := NewPEMCertPool()
	if !fakeCARoots.AppendCertsFromPEM([]byte(testonly.FakeCACertPEM)) {
		t.Fatal("failed to load fake root")
	}
	validateOpts := CertValidationOpts{
		trustedRoots: fakeCARoots,
		extKeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
	}
	chain := pemsToDERChain(t, []string{testonly.LeafSignedByFakeIntermediateCertPEM, testonly.FakeIntermediateCertPEM})
	leaf, err := x509.ParseCertificate(chain[0])
	if x509.IsFatal(err) {
		t.Fatalf("Failed to parse golden certificate DER: %v", err)
	}
	t.Logf("Cert expiry date: %v", leaf.NotAfter)

	var tests = []struct {
		desc    string
		chain   [][]byte
		caOnly  bool
		wantErr bool
	}{
		{
			desc:  "end-entity, allow non-CA",
			chain: chain,
		},
		{
			desc:    "end-entity, disallow non-CA",
			chain:   chain,
			caOnly:  true,
			wantErr: true,
		},
		{
			desc:  "intermediate, allow non-CA",
			chain: chain[1:],
		},
		{
			desc:   "intermediate, disallow non-CA",
			chain:  chain[1:],
			caOnly: true,
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			validateOpts.acceptOnlyCA = test.caOnly
			gotPath, err := ValidateChain(test.chain, validateOpts)
			if err != nil {
				if !test.wantErr {
					t.Errorf("ValidateChain()=%v,%v; want _,nil", gotPath, err)
				}
				return
			}
			if test.wantErr {
				t.Errorf("ValidateChain()=%v,%v; want _,non-nil", gotPath, err)
			}
		})
	}
}

func TestNotAfterRange(t *testing.T) {
	fakeCARoots := NewPEMCertPool()
	if !fakeCARoots.AppendCertsFromPEM([]byte(testonly.FakeCACertPEM)) {
		t.Fatal("failed to load fake root")
	}
	validateOpts := CertValidationOpts{
		trustedRoots:  fakeCARoots,
		rejectExpired: false,
		extKeyUsages:  []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
	}

	chain := pemsToDERChain(t, []string{testonly.LeafSignedByFakeIntermediateCertPEM, testonly.FakeIntermediateCertPEM})

	var tests = []struct {
		desc          string
		chain         [][]byte
		notAfterStart time.Time
		notAfterLimit time.Time
		wantErr       bool
	}{
		{
			desc:  "valid-chain, no range",
			chain: chain,
		},
		{
			desc:          "valid-chain, valid range",
			chain:         chain,
			notAfterStart: time.Date(2018, 1, 1, 0, 0, 0, 0, time.UTC),
			notAfterLimit: time.Date(2020, 7, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			desc:          "before valid range",
			chain:         chain,
			notAfterStart: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
			wantErr:       true,
		},
		{
			desc:          "after valid range",
			chain:         chain,
			notAfterLimit: time.Date(1999, 1, 1, 0, 0, 0, 0, time.UTC),
			wantErr:       true,
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			if !test.notAfterStart.IsZero() {
				validateOpts.notAfterStart = &test.notAfterStart
			}
			if !test.notAfterLimit.IsZero() {
				validateOpts.notAfterLimit = &test.notAfterLimit
			}
			gotPath, err := ValidateChain(test.chain, validateOpts)
			if err != nil {
				if !test.wantErr {
					t.Errorf("ValidateChain()=%v,%v; want _,nil", gotPath, err)
				}
				return
			}
			if test.wantErr {
				t.Errorf("ValidateChain()=%v,%v; want _,non-nil", gotPath, err)
			}
		})
	}
}

// Builds a chain of DER-encoded certs.
// Note: ordering is important
func pemsToDERChain(t *testing.T, pemCerts []string) [][]byte {
	t.Helper()
	chain := make([][]byte, 0, len(pemCerts))
	for _, pemCert := range pemCerts {
		cert := pemToCert(t, pemCert)
		chain = append(chain, cert.Raw)
	}
	return chain
}

func pemToCert(t *testing.T, pemData string) *x509.Certificate {
	t.Helper()
	bytes, rest := pem.Decode([]byte(pemData))
	if len(rest) > 0 {
		t.Fatalf("Extra data after PEM: %v", rest)
		return nil
	}

	cert, err := x509.ParseCertificate(bytes.Bytes)
	if x509.IsFatal(err) {
		t.Fatal(err)
	}

	return cert
}

func pemFileToDERChain(t *testing.T, filename string) [][]byte {
	t.Helper()
	rawChain, err := x509util.ReadPossiblePEMFile(filename, "CERTIFICATE")
	if err != nil {
		t.Fatalf("failed to load testdata: %v", err)
	}
	return rawChain
}

// Validate a chain including a pre-issuer as produced by Google's Compliance Monitor.
func TestCMPreIssuedCert(t *testing.T) {
	var b64Chain = []string{
		"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",
	}
	rawChain := make([][]byte, len(b64Chain))
	for i, b64Data := range b64Chain {
		var err error
		rawChain[i], err = base64.StdEncoding.DecodeString(b64Data)
		if err != nil {
			t.Fatalf("failed to base64.Decode(chain[%d]): %v", i, err)
		}
	}

	root, err := x509.ParseCertificate(rawChain[len(rawChain)-1])
	if err != nil {
		t.Fatalf("failed to parse root cert: %v", err)
	}
	cmRoot := NewPEMCertPool()
	cmRoot.AddCert(root)
	opts := CertValidationOpts{trustedRoots: cmRoot}
	chain, err := ValidateChain(rawChain, opts)
	if err != nil {
		t.Fatalf("failed to ValidateChain: %v", err)
	}
	for i, c := range chain {
		t.Logf("chain[%d] = \n%s", i, x509util.CertificateToString(c))
	}
}
