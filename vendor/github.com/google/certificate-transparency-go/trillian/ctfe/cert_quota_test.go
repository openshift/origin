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

package ctfe

import (
	"testing"

	"github.com/google/certificate-transparency-go/trillian/ctfe/testonly"
	"github.com/google/certificate-transparency-go/x509"
	"github.com/google/certificate-transparency-go/x509util"
)

func mustDePEM(t *testing.T, pem string) *x509.Certificate {
	t.Helper()
	c, err := x509util.CertificateFromPEM([]byte(pem))
	if x509.IsFatal(err) {
		t.Fatalf("Failed to parse PEM: %v", err)
	}
	return c
}

func TestQuotaUserForCert(t *testing.T) {
	for _, test := range []struct {
		desc string
		cert *x509.Certificate
		want string
	}{
		{
			desc: "cacert",
			cert: mustDePEM(t, testonly.CACertPEM),
			want: "@intermediate O=Certificate Transparency CA,L=Erw Wen,ST=Wales,C=GB 02adddca08",
		},
		{
			desc: "intermediate",
			cert: mustDePEM(t, testonly.FakeIntermediateCertPEM),
			want: "@intermediate CN=FakeIntermediateAuthority,OU=Eng,O=Google,L=London,ST=London,C=GB 6e62e56f67",
		},
	} {
		t.Run(test.desc, func(t *testing.T) {
			if got := QuotaUserForCert(test.cert); got != test.want {
				t.Fatalf("QuotaUserForCert() = %q, want %q", got, test.want)
			}
		})
	}
}
