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

package tls_test

import (
	"crypto"
	"encoding/pem"
	mathrand "math/rand"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/certificate-transparency-go/testdata"
	"github.com/google/certificate-transparency-go/tls"
	"github.com/google/certificate-transparency-go/x509"
)

func TestVerifySignature(t *testing.T) {
	var tests = []struct {
		pubKey   crypto.PublicKey
		in       string // hex encoded
		hashAlgo tls.HashAlgorithm
		sigAlgo  tls.SignatureAlgorithm
		errstr   string
		sig      string // hex encoded
	}{
		{PEM2PK(testdata.RsaPublicKeyPEM), "61626364", 99, tls.ECDSA, "unsupported Algorithm.Hash", "1234"},
		{PEM2PK(testdata.RsaPublicKeyPEM), "61626364", tls.SHA256, 99, "unsupported Algorithm.Signature", "1234"},

		{PEM2PK(testdata.RsaPublicKeyPEM), "61626364", tls.SHA256, tls.DSA, "cannot verify DSA", "1234"},
		{PEM2PK(testdata.RsaPublicKeyPEM), "61626364", tls.SHA256, tls.ECDSA, "cannot verify ECDSA", "1234"},
		{PEM2PK(testdata.RsaPublicKeyPEM), "61626364", tls.SHA256, tls.RSA, "verification error", "1234"},
		{PEM2PK(testdata.RsaPublicKeyPEM), "61626364", tls.SHA256, tls.ECDSA, "cannot verify ECDSA", "1234"},

		{PEM2PK(testdata.DsaPublicKeyPEM), "61626364", tls.SHA1, tls.RSA, "cannot verify RSA", "1234"},
		{PEM2PK(testdata.DsaPublicKeyPEM), "61626364", tls.SHA1, tls.ECDSA, "cannot verify ECDSA", "1234"},
		{PEM2PK(testdata.DsaPublicKeyPEM), "61626364", tls.SHA1, tls.DSA, "failed to unmarshal DSA signature", "1234"},
		{PEM2PK(testdata.DsaPublicKeyPEM), "61626364", tls.SHA1, tls.DSA, "failed to verify DSA signature", "3006020101020101eeff"},
		{PEM2PK(testdata.DsaPublicKeyPEM), "61626364", tls.SHA1, tls.DSA, "zero or negative values", "3006020100020181"},

		{PEM2PK(testdata.EcdsaPublicKeyPEM), "61626364", tls.SHA256, tls.RSA, "cannot verify RSA", "1234"},
		{PEM2PK(testdata.EcdsaPublicKeyPEM), "61626364", tls.SHA256, tls.DSA, "cannot verify DSA", "1234"},
		{PEM2PK(testdata.EcdsaPublicKeyPEM), "61626364", tls.SHA256, tls.ECDSA, "failed to unmarshal ECDSA signature", "1234"},
		{PEM2PK(testdata.EcdsaPublicKeyPEM), "61626364", tls.SHA256, tls.ECDSA, "failed to verify ECDSA signature", "3006020101020101eeff"},
		{PEM2PK(testdata.EcdsaPublicKeyPEM), "61626364", tls.SHA256, tls.ECDSA, "zero or negative values", "3006020100020181"},

		{PEM2PK(testdata.RsaPublicKeyPEM), "61626364", tls.SHA256, tls.RSA, "", testdata.RsaSignedAbcdHex},
		{PEM2PK(testdata.DsaPublicKeyPEM), "61626364", tls.SHA1, tls.DSA, "", testdata.DsaSignedAbcdHex},
		{PEM2PK(testdata.EcdsaPublicKeyPEM), "61626364", tls.SHA256, tls.ECDSA, "", testdata.EcdsaSignedAbcdHex},
	}
	for _, test := range tests {
		algo := tls.SignatureAndHashAlgorithm{Hash: test.hashAlgo, Signature: test.sigAlgo}
		signed := tls.DigitallySigned{Algorithm: algo, Signature: testdata.FromHex(test.sig)}

		err := tls.VerifySignature(test.pubKey, testdata.FromHex(test.in), signed)
		if test.errstr != "" {
			if err == nil {
				t.Errorf("VerifySignature(%s)=nil; want %q", test.in, test.errstr)
			} else if !strings.Contains(err.Error(), test.errstr) {
				t.Errorf("VerifySignature(%s)=%q; want %q", test.in, err.Error(), test.errstr)
			}
			continue
		}
		if err != nil {
			t.Errorf("VerifySignature(%s)=%q; want nil", test.in, err)
		}
	}
}

func TestCreateSignatureVerifySignatureRoundTrip(t *testing.T) {
	var tests = []struct {
		privKey  crypto.PrivateKey
		pubKey   crypto.PublicKey
		hashAlgo tls.HashAlgorithm
	}{
		{PEM2PrivKey(testdata.RsaPrivateKeyPEM), PEM2PK(testdata.RsaPublicKeyPEM), tls.SHA256},
		{PEM2PrivKey(testdata.EcdsaPrivateKeyPKCS8PEM), PEM2PK(testdata.EcdsaPublicKeyPEM), tls.SHA256},
	}
	seed := time.Now().UnixNano()
	r := mathrand.New(mathrand.NewSource(seed))
	for _, test := range tests {
		for j := 0; j < 1; j++ {
			dataLen := 10 + r.Intn(100)
			data := make([]byte, dataLen)
			_, _ = r.Read(data)
			sig, err := tls.CreateSignature(test.privKey, test.hashAlgo, data)
			if err != nil {
				t.Errorf("CreateSignature(%T, %v) failed with: %q", test.privKey, test.hashAlgo, err.Error())
				continue
			}

			if err := tls.VerifySignature(test.pubKey, data, sig); err != nil {
				t.Errorf("VerifySignature(%T, %v) failed with: %q", test.pubKey, test.hashAlgo, err)
			}
		}
	}
}

func TestCreateSignatureFailures(t *testing.T) {
	var tests = []struct {
		privKey  crypto.PrivateKey
		hashAlgo tls.HashAlgorithm
		in       string // hex encoded
		errstr   string
	}{
		{PEM2PrivKey(testdata.EcdsaPrivateKeyPKCS8PEM), 99, "abcd", "unsupported Algorithm.Hash"},
		{nil, tls.SHA256, "abcd", "unsupported private key type"},
	}
	for _, test := range tests {
		if sig, err := tls.CreateSignature(test.privKey, test.hashAlgo, testdata.FromHex(test.in)); err == nil {
			t.Errorf("CreateSignature(%T, %v)=%v,nil; want error %q", test.privKey, test.hashAlgo, sig, test.errstr)
		} else if !strings.Contains(err.Error(), test.errstr) {
			t.Errorf("CreateSignature(%T, %v)=nil,%q; want error %q", test.privKey, test.hashAlgo, err.Error(), test.errstr)
		}
	}
}

func PEM2PK(s string) crypto.PublicKey {
	p, _ := pem.Decode([]byte(s))
	if p == nil {
		panic("no PEM block found in " + s)
	}
	pubKey, _ := x509.ParsePKIXPublicKey(p.Bytes)
	if pubKey == nil {
		panic("public key not parsed from " + s)
	}
	return pubKey
}
func PEM2PrivKey(s string) crypto.PrivateKey {
	p, _ := pem.Decode([]byte(s))
	if p == nil {
		panic("no PEM block found in " + s)
	}

	// Try various different private key formats one after another.
	if rsaPrivKey, err := x509.ParsePKCS1PrivateKey(p.Bytes); err == nil {
		return *rsaPrivKey
	}
	if pkcs8Key, err := x509.ParsePKCS8PrivateKey(p.Bytes); err == nil {
		if reflect.TypeOf(pkcs8Key).Kind() == reflect.Ptr {
			pkcs8Key = reflect.ValueOf(pkcs8Key).Elem().Interface()
		}
		return pkcs8Key
	}

	return nil
}
