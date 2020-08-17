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

package tls

import (
	"crypto"
	"crypto/dsa"
	"crypto/ecdsa"
	"crypto/rsa"
	"testing"
)

func TestHashAlgorithmString(t *testing.T) {
	var tests = []struct {
		algo HashAlgorithm
		want string
	}{
		{None, "None"},
		{MD5, "MD5"},
		{SHA1, "SHA1"},
		{SHA224, "SHA224"},
		{SHA256, "SHA256"},
		{SHA384, "SHA384"},
		{SHA512, "SHA512"},
		{99, "UNKNOWN(99)"},
	}
	for _, test := range tests {
		if got := test.algo.String(); got != test.want {
			t.Errorf("%v.String()=%q; want %q", test.algo, got, test.want)
		}
	}
}

func TestSignatureAlgorithmString(t *testing.T) {
	var tests = []struct {
		algo SignatureAlgorithm
		want string
	}{
		{Anonymous, "Anonymous"},
		{RSA, "RSA"},
		{DSA, "DSA"},
		{ECDSA, "ECDSA"},
		{99, "UNKNOWN(99)"},
	}
	for _, test := range tests {
		if got := test.algo.String(); got != test.want {
			t.Errorf("%v.String()=%q; want %q", test.algo, got, test.want)
		}
	}
}

func TestDigitallySignedString(t *testing.T) {
	var tests = []struct {
		ds   DigitallySigned
		want string
	}{
		{
			ds:   DigitallySigned{Algorithm: SignatureAndHashAlgorithm{Hash: SHA1, Signature: RSA}, Signature: []byte{0x01, 0x02}},
			want: "Signature: HashAlgo=SHA1 SignAlgo=RSA Value=0102",
		},
		{
			ds:   DigitallySigned{Algorithm: SignatureAndHashAlgorithm{Hash: 99, Signature: 99}, Signature: []byte{0x03, 0x04}},
			want: "Signature: HashAlgo=UNKNOWN(99) SignAlgo=UNKNOWN(99) Value=0304",
		},
	}
	for _, test := range tests {
		if got := test.ds.String(); got != test.want {
			t.Errorf("%v.String()=%q; want %q", test.ds, got, test.want)
		}
	}
}

func TestSignatureAlgorithm(t *testing.T) {
	for _, test := range []struct {
		name string
		key  crypto.PublicKey
		want SignatureAlgorithm
	}{
		{name: "ECDSA", key: new(ecdsa.PublicKey), want: ECDSA},
		{name: "RSA", key: new(rsa.PublicKey), want: RSA},
		{name: "DSA", key: new(dsa.PublicKey), want: DSA},
		{name: "Other", key: "foo", want: Anonymous},
	} {
		if got := SignatureAlgorithmFromPubKey(test.key); got != test.want {
			t.Errorf("%v: SignatureAlgorithm() = %v, want %v", test.name, got, test.want)
		}
	}
}
