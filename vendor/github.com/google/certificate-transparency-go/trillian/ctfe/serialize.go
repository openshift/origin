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
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"sync"

	"github.com/google/certificate-transparency-go/tls"

	ct "github.com/google/certificate-transparency-go"
)

// SignatureCache is a one-entry cache that stores the last generated signature
// for a given bytes input. It helps to reduce the number of signing
// operations, and the number of distinct signatures produced for the same
// input (some signing methods are non-deterministic).
type SignatureCache struct {
	mu    sync.RWMutex
	input []byte
	sig   ct.DigitallySigned
}

// GetSignature returns the latest signature for the given bytes input. If the
// input is not in the cache, it returns (_, false).
func (sc *SignatureCache) GetSignature(input []byte) (ct.DigitallySigned, bool) {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	if !bytes.Equal(input, sc.input) {
		return ct.DigitallySigned{}, false
	}
	return sc.sig, true
}

// SetSignature associates the signature with the given bytes input.
func (sc *SignatureCache) SetSignature(input []byte, sig ct.DigitallySigned) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.input, sc.sig = input, sig
}

// signV1TreeHead signs a tree head for CT. The input STH should have been
// built from a backend response and already checked for validity.
func signV1TreeHead(signer crypto.Signer, sth *ct.SignedTreeHead, cache *SignatureCache) error {
	sthBytes, err := ct.SerializeSTHSignatureInput(*sth)
	if err != nil {
		return err
	}
	if sig, ok := cache.GetSignature(sthBytes); ok {
		sth.TreeHeadSignature = sig
		return nil
	}

	h := sha256.New()
	h.Write(sthBytes)
	signature, err := signer.Sign(rand.Reader, h.Sum(nil), crypto.SHA256)
	if err != nil {
		return err
	}

	sth.TreeHeadSignature = ct.DigitallySigned{
		Algorithm: tls.SignatureAndHashAlgorithm{
			Hash:      tls.SHA256,
			Signature: tls.SignatureAlgorithmFromPubKey(signer.Public()),
		},
		Signature: signature,
	}
	cache.SetSignature(sthBytes, sth.TreeHeadSignature)
	return nil
}

func buildV1SCT(signer crypto.Signer, leaf *ct.MerkleTreeLeaf) (*ct.SignedCertificateTimestamp, error) {
	// Serialize SCT signature input to get the bytes that need to be signed
	sctInput := ct.SignedCertificateTimestamp{
		SCTVersion: ct.V1,
		Timestamp:  leaf.TimestampedEntry.Timestamp,
		Extensions: leaf.TimestampedEntry.Extensions,
	}
	data, err := ct.SerializeSCTSignatureInput(sctInput, ct.LogEntry{Leaf: *leaf})
	if err != nil {
		return nil, fmt.Errorf("failed to serialize SCT data: %v", err)
	}

	h := sha256.Sum256(data)
	signature, err := signer.Sign(rand.Reader, h[:], crypto.SHA256)
	if err != nil {
		return nil, fmt.Errorf("failed to sign SCT data: %v", err)
	}

	digitallySigned := ct.DigitallySigned{
		Algorithm: tls.SignatureAndHashAlgorithm{
			Hash:      tls.SHA256,
			Signature: tls.SignatureAlgorithmFromPubKey(signer.Public()),
		},
		Signature: signature,
	}

	logID, err := GetCTLogID(signer.Public())
	if err != nil {
		return nil, fmt.Errorf("failed to get logID for signing: %v", err)
	}

	return &ct.SignedCertificateTimestamp{
		SCTVersion: ct.V1,
		LogID:      ct.LogID{KeyID: logID},
		Timestamp:  sctInput.Timestamp,
		Extensions: sctInput.Extensions,
		Signature:  digitallySigned,
	}, nil
}
