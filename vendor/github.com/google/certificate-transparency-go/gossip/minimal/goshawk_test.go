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
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/certificate-transparency-go"
	"github.com/google/certificate-transparency-go/client"
	"github.com/google/certificate-transparency-go/gossip/minimal/x509ext"
	"github.com/google/certificate-transparency-go/jsonclient"
	"github.com/google/certificate-transparency-go/scanner"
	"github.com/google/certificate-transparency-go/tls"
)

func TestNewGoshawkFromFile(t *testing.T) {
	ctx := context.Background()
	var tests = []struct {
		name     string
		filename string
		wantErr  string
	}{
		{name: "OK", filename: "testdata/goshawk.cfg"},
		{name: "EmptyFilename", filename: "", wantErr: "no such file"},
		{name: "MissingFile", filename: "testdata/nofile", wantErr: "no such file"},
		{name: "FailToParse", filename: "testdata/Makefile", wantErr: "failed to parse"},
		{name: "NoSourceLog", filename: "testdata/hawk-no-source-log.cfg", wantErr: "no source log"},
		{name: "NoSourceName", filename: "testdata/hawk-no-source-name.cfg", wantErr: "no log name provided"},
		{name: "DupSourceName", filename: "testdata/hawk-dup-source-name.cfg", wantErr: "duplicate source logs"},
		{name: "NoDestName", filename: "testdata/hawk-no-dest-name.cfg", wantErr: "no log name provided"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := NewGoshawkFromFile(ctx, test.filename, nil, scanner.ScannerOptions{})
			if err != nil {
				if test.wantErr == "" {
					t.Errorf("NewGoshawkFromFile(%v)=nil,%v; want _,nil", test.filename, err)
				} else if !strings.Contains(err.Error(), test.wantErr) {
					t.Errorf("NewGoshawkFromFile(%v)=nil,%v; want nil,err containing %q", test.filename, err, test.wantErr)
				}
				return
			}
			if test.wantErr != "" {
				t.Errorf("NewGoshawkFromFile(%+v)=%+v,nil; want nil,err containing %q", test.filename, got, test.wantErr)
			}
		})
	}
}

// Two real STHs from argon2018
var (
	argonSTH1 = &ct.SignedTreeHead{
		Version:   0,
		TreeSize:  87292240,
		Timestamp: 1519739720195,
		SHA256RootHash: [32]byte{
			0xbb, 0x6a, 0x8b, 0x88, 0x37, 0x7b, 0x84, 0x50, 0x48, 0xf1, 0x77, 0xf8, 0x07, 0x24, 0xef, 0xec,
			0x77, 0x8d, 0xb5, 0xd8, 0x5f, 0xdd, 0x71, 0x4a, 0xd8, 0x3f, 0xf0, 0x2e, 0x6f, 0x01, 0xaa, 0x27,
		},
		TreeHeadSignature: ct.DigitallySigned{
			Algorithm: tls.SignatureAndHashAlgorithm{Hash: tls.SHA256, Signature: tls.ECDSA},
			Signature: dehex("3046022100b855d9aa1d06fe23e2010e1e61d18db494a7b42b84c04d14eed5dd648ad3180c022100d1528aea0f065b87b2ed24f99c1ff9b1102b899df3635cc8f73feebe463d7239"),
		},
	}
	argonSTH2 = &ct.SignedTreeHead{
		Version:   0,
		TreeSize:  87292262,
		Timestamp: 1519739722187,
		SHA256RootHash: [32]byte{
			0x99, 0x74, 0x0d, 0xe7, 0x19, 0x5f, 0x1b, 0xc5, 0x09, 0xeb, 0x79, 0xaf, 0x09, 0xa7, 0x3c, 0x57,
			0x33, 0x22, 0x5c, 0xd6, 0x26, 0xec, 0x0c, 0x0e, 0xcd, 0x09, 0xd7, 0x45, 0xd4, 0x73, 0xcd, 0xae,
		},
		TreeHeadSignature: ct.DigitallySigned{
			Algorithm: tls.SignatureAndHashAlgorithm{Hash: tls.SHA256, Signature: tls.ECDSA},
			Signature: dehex("304602210095748a1576400e640d4915ff65351b688fe9bbeb8dd5c3a0773224d29f9bc4ff022100857d31faa70bc2fa646c5f51be14ef812c127c231fb6ce11c9fbd16ebf069dda"),
		},
	}
	// Consistency proof between them
	argonProof = ct.GetSTHConsistencyResponse{
		Consistency: [][]byte{
			dehex("befb58f2a0a01496c73f222582fab430cd268c2eada874269f1d9a47855b111f"),
			dehex("cdd5bb33fcf18d5e091b36d44a6ad62f4e94c5ade3394e4c92ef7a55e88a48bc"),
			dehex("0c5c4f367f3ca2e28160598c37d9cd977b4477c5063179238ed21dfddf52af7b"),
			dehex("5793259844e9d8124a68fc3a5e88803269a76ee5138338e5852a40f33496e917"),
			dehex("af3d5b57d7e3d6738897bf1d20cdaed61dbc25eb7e181c7b79eaf2a4f1064ce2"),
			dehex("e8ec71a00b0d2153844a10182313986aa75338a64fe19da8141e7bfd89f232d0"),
			dehex("02680067f6eb752c34663b865fdd26767ae975de1a5371b708fab4863b42a1a3"),
			dehex("5fbb1bf706a8055311fc94bdbbbbd84278fdcb234512799bf1ad3bf2b99b950c"),
			dehex("95da606784c607bbbc22a0b69b1333c09e9305d3dd5ffd3f8dc5f3bd668492d2"),
			dehex("0ea94ff3911a799bdab7aaadb88c1273a1ef6f1beb3698f49e303e006d87f389"),
			dehex("7147ee87ab8a4ff71d45d28440893b7d11c82aa0b9786fe8bc0114d8a904008b"),
			dehex("8cc4a32b3822403ba55b9dd3e521b54a59b8af5274f2bc19bddc280935ea8875"),
			dehex("d57beed9c951b6ed5f8aa7df92c6253126fc65868930d302e798b85fc254f01b"),
			dehex("25b33de40a89f359bedd906141dafd418a77c7c4413391d0793d1c0fdbe7beb7"),
			dehex("ce1a9524be4026d92f0a6317e5c1c39a68b9a3f5eab48071ecef34715bff6e80"),
			dehex("7a13109bb1b5ecd24ced9008f41aa30960ab4fb4147538624a2f487499863820"),
		},
	}
	argonKey = `-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE0gBVBa3VR7QZu82V+ynXWD14JM3O
Rp37MtRxTmACJV5ZPtfUA7htQ2hofuigZQs+bnFZkje+qejxoyvk2Q1VaA==
-----END PUBLIC KEY-----`
)

func TestValidateSTH(t *testing.T) {
	ctx := context.Background()
	handlerBroken := false
	proof := &argonProof
	var handler http.HandlerFunc = func(w http.ResponseWriter, _ *http.Request) {
		if handlerBroken {
			return
		}
		jsonData, _ := json.Marshal(proof)
		w.Write(jsonData)
	}
	s := httptest.NewServer(handler)
	defer s.Close()
	client, err := client.New(s.URL+"/ct/v1/get-sth-consistency", nil, jsonclient.Options{PublicKey: argonKey})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	o := originLog{logConfig: logConfig{
		Name: "argon2018",
		URL:  "http://ct.googleapis.com/logs/argon2018",
		Log:  client,
	}}
	sthInfo1 := x509ext.LogSTHInfo{
		LogURL:            []byte(o.URL),
		Version:           tls.Enum(argonSTH1.Version),
		TreeSize:          argonSTH1.TreeSize,
		Timestamp:         argonSTH1.Timestamp,
		SHA256RootHash:    argonSTH1.SHA256RootHash,
		TreeHeadSignature: argonSTH1.TreeHeadSignature,
	}
	sthInfo2 := x509ext.LogSTHInfo{
		LogURL:            []byte(o.URL),
		Version:           tls.Enum(argonSTH2.Version),
		TreeSize:          argonSTH2.TreeSize,
		Timestamp:         argonSTH2.Timestamp,
		SHA256RootHash:    argonSTH2.SHA256RootHash,
		TreeHeadSignature: argonSTH2.TreeHeadSignature,
	}

	// No current STH, validate just checks signature.
	if err := o.validateSTH(ctx, &sthInfo1); err != nil {
		t.Errorf("validateSTH(no-current-sth)=%v; want nil", err)
	}
	// Back-to-front.
	o.updateSTH(argonSTH1)
	if err := o.validateSTH(ctx, &sthInfo2); err != nil {
		t.Errorf("validateSTH(reversed)=%v; want nil", err)
	}
	// Valid proof between STHs.
	o.updateSTH(argonSTH2)
	if err := o.validateSTH(ctx, &sthInfo1); err != nil {
		t.Errorf("validateSTH(valid)=%v; want nil", err)
	}
	// Incorrect signature.
	sthInfo1.Timestamp++
	if err := o.validateSTH(ctx, &sthInfo1); err == nil {
		t.Error("validateSTH(wrong-signature)=nil; want non-nil")
	}
	sthInfo1.Timestamp--
	// Bad response from server.
	handlerBroken = true
	if err := o.validateSTH(ctx, &sthInfo1); err == nil {
		t.Error("validateSTH(bad-rsp)=nil; want non-nil")
	}
	handlerBroken = false
	// Proof doesn't add up.
	proof = &ct.GetSTHConsistencyResponse{
		Consistency: [][]byte{
			dehex("befb58f2a0a01496c73f222582fab430cd268c2eada874269f1d9a47855b111f"),
		},
	}
	if err := o.validateSTH(ctx, &sthInfo1); err == nil {
		t.Error("validateSTH(wrong-proof)=nil; want non-nil")
	}
	proof = &argonProof
}

func TestUpdateAndGetLastSTH(t *testing.T) {
	var nilp *ct.SignedTreeHead
	o := originLog{logConfig: logConfig{Name: "argon2018", URL: "http://ct.googleapis.com/logs/argon2018"}}

	if got, want := o.getLastSTH(), nilp; got != want {
		t.Errorf("o.getLastSTH()=%v; want %v", got, want)
	}
	o.updateSTH(argonSTH1)
	if got, want := o.getLastSTH(), argonSTH1; got != want {
		t.Errorf("o.getLastSTH()=%v; want %v", got, want)
	}
	o.updateSTH(argonSTH2)
	if got, want := o.getLastSTH(), argonSTH2; got != want {
		t.Errorf("o.getLastSTH()=%v; want %v", got, want)
	}
	o.updateSTH(argonSTH1) // Ignored as backwards
	if got, want := o.getLastSTH(), argonSTH2; got != want {
		t.Errorf("o.getLastSTH()=%v; want %v", got, want)
	}
}
