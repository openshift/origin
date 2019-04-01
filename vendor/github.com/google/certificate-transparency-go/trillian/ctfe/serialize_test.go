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
	"crypto/sha256"
	"testing"

	"github.com/google/certificate-transparency-go/tls"
	"github.com/google/certificate-transparency-go/trillian/ctfe/testonly"
	"github.com/google/certificate-transparency-go/trillian/testdata"
	"github.com/google/certificate-transparency-go/x509"
	"github.com/google/certificate-transparency-go/x509util"
	"github.com/google/trillian/crypto/keys/pem"
	"github.com/kylelemons/godebug/pretty"

	ct "github.com/google/certificate-transparency-go"
)

func TestBuildV1MerkleTreeLeafForCert(t *testing.T) {
	cert, err := x509util.CertificateFromPEM([]byte(testonly.LeafSignedByFakeIntermediateCertPEM))
	if x509.IsFatal(err) {
		t.Fatalf("failed to set up test cert: %v", err)
	}

	signer, err := setupSigner(fakeSignature)
	if err != nil {
		t.Fatalf("could not create signer: %v", err)
	}

	leaf, err := ct.MerkleTreeLeafFromChain([]*x509.Certificate{cert}, ct.X509LogEntryType, fixedTimeMillis)
	if err != nil {
		t.Fatalf("buildV1MerkleTreeLeafForCert()=nil,%v; want _,nil", err)
	}
	got, err := buildV1SCT(signer, leaf)
	if err != nil {
		t.Fatalf("buildV1SCT()=nil,%v; want _,nil", err)
	}

	expected := ct.SignedCertificateTimestamp{
		SCTVersion: 0,
		LogID:      ct.LogID{KeyID: demoLogID},
		Timestamp:  fixedTimeMillis,
		Extensions: ct.CTExtensions{},
		Signature: ct.DigitallySigned{
			Algorithm: tls.SignatureAndHashAlgorithm{
				Hash:      tls.SHA256,
				Signature: tls.ECDSA},
			Signature: fakeSignature,
		},
	}

	if diff := pretty.Compare(*got, expected); diff != "" {
		t.Fatalf("Mismatched SCT (cert), diff:\n%v", diff)
	}

	// Additional checks that the MerkleTreeLeaf we built is correct
	if got, want := leaf.Version, ct.V1; got != want {
		t.Fatalf("Got a %v leaf, expected a %v leaf", got, want)
	}
	if got, want := leaf.LeafType, ct.TimestampedEntryLeafType; got != want {
		t.Fatalf("Got leaf type %v, expected %v", got, want)
	}
	if got, want := leaf.TimestampedEntry.EntryType, ct.X509LogEntryType; got != want {
		t.Fatalf("Got entry type %v, expected %v", got, want)
	}
	if got, want := leaf.TimestampedEntry.Timestamp, got.Timestamp; got != want {
		t.Fatalf("Entry / sct timestamp mismatch; got %v, expected %v", got, want)
	}
	if got, want := leaf.TimestampedEntry.X509Entry.Data, cert.Raw; !bytes.Equal(got, want) {
		t.Fatalf("Cert bytes mismatch, got %x, expected %x", got, want)
	}
}

func TestSignV1SCTForPrecertificate(t *testing.T) {
	cert, err := x509util.CertificateFromPEM([]byte(testonly.PrecertPEMValid))
	if x509.IsFatal(err) {
		t.Fatalf("failed to set up test precert: %v", err)
	}

	signer, err := setupSigner(fakeSignature)
	if err != nil {
		t.Fatalf("could not create signer: %v", err)
	}

	// Use the same cert as the issuer for convenience.
	leaf, err := ct.MerkleTreeLeafFromChain([]*x509.Certificate{cert, cert}, ct.PrecertLogEntryType, fixedTimeMillis)
	if err != nil {
		t.Fatalf("buildV1MerkleTreeLeafForCert()=nil,%v; want _,nil", err)
	}
	got, err := buildV1SCT(signer, leaf)
	if err != nil {
		t.Fatalf("buildV1SCT()=nil,%v; want _,nil", err)
	}

	expected := ct.SignedCertificateTimestamp{
		SCTVersion: 0,
		LogID:      ct.LogID{KeyID: demoLogID},
		Timestamp:  fixedTimeMillis,
		Extensions: ct.CTExtensions{},
		Signature: ct.DigitallySigned{
			Algorithm: tls.SignatureAndHashAlgorithm{
				Hash:      tls.SHA256,
				Signature: tls.ECDSA},
			Signature: fakeSignature}}

	if diff := pretty.Compare(*got, expected); diff != "" {
		t.Fatalf("Mismatched SCT (precert), diff:\n%v", diff)
	}

	// Additional checks that the MerkleTreeLeaf we built is correct
	if got, want := leaf.Version, ct.V1; got != want {
		t.Fatalf("Got a %v leaf, expected a %v leaf", got, want)
	}
	if got, want := leaf.LeafType, ct.TimestampedEntryLeafType; got != want {
		t.Fatalf("Got leaf type %v, expected %v", got, want)
	}
	if got, want := leaf.TimestampedEntry.EntryType, ct.PrecertLogEntryType; got != want {
		t.Fatalf("Got entry type %v, expected %v", got, want)
	}
	if got, want := got.Timestamp, leaf.TimestampedEntry.Timestamp; got != want {
		t.Fatalf("Entry / sct timestamp mismatch; got %v, expected %v", got, want)
	}
	keyHash := sha256.Sum256(cert.RawSubjectPublicKeyInfo)
	if got, want := keyHash[:], leaf.TimestampedEntry.PrecertEntry.IssuerKeyHash[:]; !bytes.Equal(got, want) {
		t.Fatalf("Issuer key hash bytes mismatch, got %v, expected %v", got, want)
	}
	defangedTBS, _ := x509.RemoveCTPoison(cert.RawTBSCertificate)
	if got, want := leaf.TimestampedEntry.PrecertEntry.TBSCertificate, defangedTBS; !bytes.Equal(got, want) {
		t.Fatalf("TBS cert mismatch, got %v, expected %v", got, want)
	}
}

func TestSignV1TreeHead(t *testing.T) {
	signer, err := pem.UnmarshalPrivateKey(testdata.DemoPrivateKey, testdata.DemoPrivateKeyPass)
	if err != nil {
		t.Fatalf("could not create signer: %v", err)
	}
	var cache SignatureCache

	sth := ct.SignedTreeHead{
		Version:   ct.V1,
		TreeSize:  10,
		Timestamp: 1512993312000,
	}
	if err := signV1TreeHead(signer, &sth, &cache); err != nil {
		t.Fatalf("signV1TreeHead()=%v; want nil", err)
	}
	prevSig := make([]byte, len(sth.TreeHeadSignature.Signature))
	copy(prevSig, sth.TreeHeadSignature.Signature)

	// Signing the same contents should get the same cached signature regardless.
	for i := 0; i < 5; i++ {
		if err := signV1TreeHead(signer, &sth, &cache); err != nil {
			t.Fatalf("signV1TreeHead()=%v; want nil", err)
		}
		sig := make([]byte, len(sth.TreeHeadSignature.Signature))
		copy(sig, sth.TreeHeadSignature.Signature)

		if diff := pretty.Compare(prevSig, sig); diff != "" {
			t.Fatalf("signV1TreeHead().TreeHeadSignature mismatched, diff:\n%v", diff)
		}
	}

	// But changing the contents does change the signature.
	for i := 0; i < 5; i++ {
		sth.TreeSize = uint64(11 + i)
		if err := signV1TreeHead(signer, &sth, &cache); err != nil {
			t.Errorf("signV1TreeHead()=%v; want nil", err)
		}
		sig := make([]byte, len(sth.TreeHeadSignature.Signature))
		copy(sig, sth.TreeHeadSignature.Signature)

		if bytes.Equal(prevSig, sig) {
			t.Fatalf("signV1TreeHead(size=%d).TreeHeadSignature unexpectedly matched", sth.TreeSize)
		}
		prevSig := sig

		// Repeating should again return the cached signature.
		if err := signV1TreeHead(signer, &sth, &cache); err != nil {
			t.Errorf("signV1TreeHead(size=%d)=%v; want nil", sth.TreeSize, err)
		}
		sig = make([]byte, len(sth.TreeHeadSignature.Signature))
		copy(sig, sth.TreeHeadSignature.Signature)
		if diff := pretty.Compare(prevSig, sig); diff != "" {
			t.Fatalf("signV1TreeHead(size=%d).TreeHeadSignature mismatched, diff:\n%v", sth.TreeSize, diff)
		}
	}
}

func TestSignV1TreeHeadDifferentSigners(t *testing.T) {
	signer1, err := pem.UnmarshalPrivateKey(testdata.DemoPrivateKey, testdata.DemoPrivateKeyPass)
	if err != nil {
		t.Fatalf("could not create signer1: %v", err)
	}
	signer2, err := setupSigner(fakeSignature)
	if err != nil {
		t.Fatalf("could not create signer2: %v", err)
	}

	var cache1, cache2 SignatureCache
	sth := ct.SignedTreeHead{
		Version:   ct.V1,
		TreeSize:  10,
		Timestamp: 1512993312000,
	}

	if err := signV1TreeHead(signer1, &sth, &cache1); err != nil {
		t.Fatalf("signV1TreeHead(signer1)=%v; want nil", err)
	}
	sig1 := make([]byte, len(sth.TreeHeadSignature.Signature))
	copy(sig1, sth.TreeHeadSignature.Signature)

	if err := signV1TreeHead(signer2, &sth, &cache2); err != nil {
		t.Fatalf("signV1TreeHead(signer2)=%v; want nil", err)
	}
	sig2 := make([]byte, len(sth.TreeHeadSignature.Signature))
	copy(sig2, sth.TreeHeadSignature.Signature)

	// Check that different signers on the same contents give different results.
	if bytes.Equal(sig1, sig2) {
		t.Fatal("signV1TreeHead().TreeHeadSignature unexpectedly matched")
	}
}
