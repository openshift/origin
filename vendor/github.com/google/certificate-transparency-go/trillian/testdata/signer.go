// Copyright 2017 Google Inc. All Rights Reserved.
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

package testdata

import (
	"crypto"
	"io"
)

// signerStub returns a fixed signature and error, no matter the input.
// It implements crypto.Signer.
type signerStub struct {
	publicKey crypto.PublicKey
	signature []byte
	err       error
}

// Public returns the public key associated with the signer that this stub is based on.
func (s *signerStub) Public() crypto.PublicKey { return s.publicKey }

// Sign will return the signature or error that the signerStub was created to provide.
func (s *signerStub) Sign(rand io.Reader, digest []byte, opts crypto.SignerOpts) ([]byte, error) {
	return s.signature, s.err
}

// NewSignerWithErr creates a signer that always returns err when Sign() is called.
func NewSignerWithErr(pubKey crypto.PublicKey, err error) crypto.Signer {
	return &signerStub{
		publicKey: pubKey,
		err:       err,
	}
}

// NewSignerWithFixedSig creates a signer that always return sig when Sign() is called.
func NewSignerWithFixedSig(pubKey crypto.PublicKey, sig []byte) crypto.Signer {
	return &signerStub{
		publicKey: pubKey,
		signature: sig,
	}
}
