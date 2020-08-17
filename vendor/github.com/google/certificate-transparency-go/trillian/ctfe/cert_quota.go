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
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/google/certificate-transparency-go/x509"
)

// CertificateQuotaUserPrefix is prepended to all User quota ids association
// with intermediate certificates.
const CertificateQuotaUserPrefix = "@intermediate"

// QuotaUserForCert returns a User quota id string for the passed in
// certificate.
// This is intended to be used for quota limiting by intermediate certificates,
// but the function does not enforce anything about the passed in cert.
//
// Format returned is:
//   "CertificateQuotaUserPrefix Subject hex(SHA256(SubjectPublicKeyInfo)[0:5])"
// See tests for examples.
func QuotaUserForCert(c *x509.Certificate) string {
	spkiHash := sha256.Sum256(c.RawSubjectPublicKeyInfo)
	return fmt.Sprintf("%s %s %s", CertificateQuotaUserPrefix, c.Subject.String(), hex.EncodeToString(spkiHash[0:5]))
}
