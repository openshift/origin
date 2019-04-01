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
	"crypto/rand"
	"fmt"
	"math/big"
	"time"

	// Register PEMKeyFile ProtoHandler
	_ "github.com/google/trillian/crypto/keys/pem/proto"

	"github.com/golang/glog"
	"github.com/google/certificate-transparency-go"
	"github.com/google/certificate-transparency-go/asn1"
	"github.com/google/certificate-transparency-go/gossip/minimal/x509ext"
	"github.com/google/certificate-transparency-go/tls"
	"github.com/google/certificate-transparency-go/x509"
	"github.com/google/certificate-transparency-go/x509/pkix"
	"github.com/google/certificate-transparency-go/x509util"
)

// CertForSTH creates an X.509 certificate with the given STH embedded in
// it.
func (src *sourceLog) CertForSTH(sth *ct.SignedTreeHead, g *Gossiper) (*ct.ASN1Cert, error) {
	// Randomize the subject key ID.
	randData := make([]byte, 128)
	if _, err := rand.Read(randData); err != nil {
		return nil, fmt.Errorf("failed to read random data: %v", err)
	}
	sthInfo := x509ext.LogSTHInfo{
		LogURL:            []byte(src.URL),
		Version:           tls.Enum(sth.Version),
		TreeSize:          sth.TreeSize,
		Timestamp:         sth.Timestamp,
		SHA256RootHash:    sth.SHA256RootHash,
		TreeHeadSignature: sth.TreeHeadSignature,
	}
	sthData, err := tls.Marshal(sthInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to re-marshal STH: %v", err)
	}
	leaf := x509.Certificate{
		SignatureAlgorithm: g.root.SignatureAlgorithm,
		SubjectKeyId:       randData, // TODO(drysdale): use hash of publicKey BIT STRING
		SerialNumber:       big.NewInt(int64(sth.Timestamp)),
		NotBefore:          ctTimestampToTime(sth.Timestamp),
		NotAfter:           ctTimestampToTime(sth.Timestamp).Add(24 * time.Hour),
		Subject: pkix.Name{
			Country:            g.root.Subject.Country,
			Organization:       g.root.Subject.Organization,
			OrganizationalUnit: g.root.Subject.OrganizationalUnit,
			CommonName:         fmt.Sprintf("STH-for-%s <%s> @%d: size=%d hash=%x", src.Name, src.URL, sth.Timestamp, sth.TreeSize, sth.SHA256RootHash),
		},
		ExtraExtensions: []pkix.Extension{
			{Id: x509ext.OIDExtensionCTSTH, Critical: true, Value: sthData},
		},
		UnknownExtKeyUsage: []asn1.ObjectIdentifier{x509ext.OIDExtKeyUsageCTMinimalGossip},
	}

	leafData, err := x509.CreateCertificate(rand.Reader, &leaf, g.root, g.root.PublicKey, g.signer)
	if err != nil {
		return nil, fmt.Errorf("failed to create certificate: %v", err)
	}
	parsed, err := x509.ParseCertificate(leafData)
	if err != nil {
		return nil, fmt.Errorf("failed to re-parse created certificate: %v", err)
	}
	glog.V(2).Infof("created leaf certificate:\n%s", x509util.CertificateToString(parsed))
	return &ct.ASN1Cert{Data: leafData}, nil
}

func ctTimestampToTime(ts uint64) time.Time {
	secs := int64(ts / 1000)
	msecs := int64(ts % 1000)
	return time.Unix(secs, msecs*1000000)
}
