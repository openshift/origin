// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package ecdsa provides an internal version of ecdsa.Verify
// that is used for the Wycheproof tests.
package ecdsa

import (
	"crypto/ecdsa"
	"math/big"

	"golang.org/x/crypto/cryptobyte"
	"golang.org/x/crypto/cryptobyte/asn1"
)

// VerifyASN1 verifies the ASN1 encoded signature, sig, of hash using the
// public key, pub. Its return value records whether the signature is valid.
func VerifyASN1(pub *ecdsa.PublicKey, hash, sig []byte) bool {
	var (
		r, s  = &big.Int{}, &big.Int{}
		inner cryptobyte.String
	)
	input := cryptobyte.String(sig)
	if !input.ReadASN1(&inner, asn1.SEQUENCE) ||
		!input.Empty() ||
		!inner.ReadASN1Integer(r) ||
		!inner.ReadASN1Integer(s) ||
		!inner.Empty() {
		return false
	}
	return ecdsa.Verify(pub, hash, r, s)
}
