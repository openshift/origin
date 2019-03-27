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
	"context"
	"strings"
	"testing"
	"time"

	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/timestamp"

	ct "github.com/google/certificate-transparency-go"
	"github.com/google/certificate-transparency-go/trillian/ctfe/configpb"
	"github.com/google/trillian/crypto/keys/der"
	"github.com/google/trillian/crypto/keys/pem"
	_ "github.com/google/trillian/crypto/keys/pem/proto" // Register PEMKeyFile ProtoHandler.
	"github.com/google/trillian/crypto/keyspb"
	"github.com/google/trillian/monitoring"
)

func mustReadPublicKey(t *testing.T) *keyspb.PublicKey {
	t.Helper()
	pubKey, err := pem.ReadPublicKeyFile("../testdata/ct-http-server.pubkey.pem")
	if err != nil {
		t.Fatalf("ReadPublicKeyFile(): %v", err)
	}
	ret, err := der.ToPublicProto(pubKey)
	if err != nil {
		t.Fatalf("ToPublicProto(): %v", err)
	}
	return ret
}

func TestSetUpInstance(t *testing.T) {
	ctx := context.Background()

	privKey := mustMarshalAny(&keyspb.PEMKeyFile{Path: "../testdata/ct-http-server.privkey.pem", Password: "dirk"})
	missingPrivKey := mustMarshalAny(&keyspb.PEMKeyFile{Path: "../testdata/bogus.privkey.pem", Password: "dirk"})
	wrongPassPrivKey := mustMarshalAny(&keyspb.PEMKeyFile{Path: "../testdata/ct-http-server.privkey.pem", Password: "dirkly"})
	pubKey := mustReadPublicKey(t)

	var tests = []struct {
		desc    string
		cfg     configpb.LogConfig
		wantErr string
	}{
		{
			desc: "valid",
			cfg: configpb.LogConfig{
				LogId:        1,
				Prefix:       "log",
				RootsPemFile: []string{"../testdata/fake-ca.cert"},
				PrivateKey:   privKey,
			},
		},
		{
			desc: "valid-mirror",
			cfg: configpb.LogConfig{
				LogId:        1,
				Prefix:       "log",
				RootsPemFile: []string{"../testdata/fake-ca.cert"},
				PublicKey:    pubKey,
				IsMirror:     true,
			},
		},
		{
			desc: "no-roots",
			cfg: configpb.LogConfig{
				LogId:      1,
				Prefix:     "log",
				PrivateKey: privKey,
			},
			wantErr: "specify RootsPemFile",
		},
		{
			desc: "no-roots-mirror",
			cfg: configpb.LogConfig{
				LogId:     1,
				Prefix:    "log",
				PublicKey: pubKey,
				IsMirror:  true,
			},
		},
		{
			desc: "no-priv-key",
			cfg: configpb.LogConfig{
				LogId:        1,
				Prefix:       "log",
				RootsPemFile: []string{"../testdata/fake-ca.cert"},
			},
			wantErr: "empty private key",
		},
		{
			desc: "priv-key-mirror",
			cfg: configpb.LogConfig{
				LogId:      1,
				Prefix:     "log",
				PrivateKey: privKey,
				PublicKey:  pubKey,
				IsMirror:   true,
			},
			wantErr: "unnecessary private key",
		},
		{
			desc: "no-pub-key-mirror",
			cfg: configpb.LogConfig{
				LogId:    1,
				Prefix:   "log",
				IsMirror: true,
			},
			wantErr: "empty public key",
		},
		{
			desc: "missing-root-cert",
			cfg: configpb.LogConfig{
				LogId:        1,
				Prefix:       "log",
				RootsPemFile: []string{"../testdata/bogus.cert"},
				PrivateKey:   privKey,
			},
			wantErr: "failed to read trusted roots",
		},
		{
			desc: "missing-privkey",
			cfg: configpb.LogConfig{
				LogId:        1,
				Prefix:       "log",
				RootsPemFile: []string{"../testdata/fake-ca.cert"},
				PrivateKey:   missingPrivKey,
			},
			wantErr: "failed to load private key",
		},
		{
			desc: "privkey-wrong-password",
			cfg: configpb.LogConfig{
				LogId:        1,
				Prefix:       "log",
				RootsPemFile: []string{"../testdata/fake-ca.cert"},
				PrivateKey:   wrongPassPrivKey,
			},
			wantErr: "failed to load private key",
		},
		{
			desc: "valid-ekus-1",
			cfg: configpb.LogConfig{
				LogId:        1,
				Prefix:       "log",
				RootsPemFile: []string{"../testdata/fake-ca.cert"},
				PrivateKey:   privKey,
				ExtKeyUsages: []string{"Any"},
			},
		},
		{
			desc: "valid-ekus-2",
			cfg: configpb.LogConfig{
				LogId:        1,
				Prefix:       "log",
				RootsPemFile: []string{"../testdata/fake-ca.cert"},
				PrivateKey:   privKey,
				ExtKeyUsages: []string{"Any", "ServerAuth", "TimeStamping"},
			},
		},
		{
			desc: "invalid-ekus-1",
			cfg: configpb.LogConfig{
				LogId:        1,
				Prefix:       "log",
				RootsPemFile: []string{"../testdata/fake-ca.cert"},
				PrivateKey:   privKey,
				ExtKeyUsages: []string{"Any", "ServerAuth", "TimeStomping"},
			},
			wantErr: "unknown extended key usage",
		},
		{
			desc: "invalid-ekus-2",
			cfg: configpb.LogConfig{
				LogId:        1,
				Prefix:       "log",
				RootsPemFile: []string{"../testdata/fake-ca.cert"},
				PrivateKey:   privKey,
				ExtKeyUsages: []string{"Any "},
			},
			wantErr: "unknown extended key usage",
		},
	}

	for _, test := range tests {
		opts := InstanceOptions{Config: &test.cfg, Deadline: time.Second, MetricFactory: monitoring.InertMetricFactory{}}
		t.Run(test.desc, func(t *testing.T) {
			if _, err := SetUpInstance(ctx, opts); err != nil {
				if test.wantErr == "" {
					t.Errorf("SetUpInstance()=_,%v; want _,nil", err)
				} else if !strings.Contains(err.Error(), test.wantErr) {
					t.Errorf("SetUpInstance()=_,%v; want err containing %q", err, test.wantErr)
				}
				return
			}
			if test.wantErr != "" {
				t.Errorf("SetUpInstance()=_,nil; want err containing %q", test.wantErr)
			}
		})
	}
}

func equivalentTimes(a *time.Time, b *timestamp.Timestamp) bool {
	if a == nil && b == nil {
		return true
	}
	tsA, err := ptypes.TimestampProto(*a)
	if err != nil {
		return false
	}
	return ptypes.TimestampString(tsA) == ptypes.TimestampString(b)
}

func TestSetUpInstanceSetsValidationOpts(t *testing.T) {
	ctx := context.Background()

	start := &timestamp.Timestamp{Seconds: 10000}
	limit := &timestamp.Timestamp{Seconds: 12000}

	privKey, err := ptypes.MarshalAny(&keyspb.PEMKeyFile{Path: "../testdata/ct-http-server.privkey.pem", Password: "dirk"})
	if err != nil {
		t.Fatalf("Could not marshal private key proto: %v", err)
	}
	var tests = []struct {
		desc string
		cfg  configpb.LogConfig
	}{
		{
			desc: "no validation opts",
			cfg: configpb.LogConfig{
				LogId:        1,
				Prefix:       "/log",
				RootsPemFile: []string{"../testdata/fake-ca.cert"},
				PrivateKey:   privKey,
			},
		},
		{
			desc: "notAfterStart only",
			cfg: configpb.LogConfig{
				LogId:         1,
				Prefix:        "/log",
				RootsPemFile:  []string{"../testdata/fake-ca.cert"},
				PrivateKey:    privKey,
				NotAfterStart: start,
			},
		},
		{
			desc: "notAfter range",
			cfg: configpb.LogConfig{
				LogId:         1,
				Prefix:        "/log",
				RootsPemFile:  []string{"../testdata/fake-ca.cert"},
				PrivateKey:    privKey,
				NotAfterStart: start,
				NotAfterLimit: limit,
			},
		},
		{
			desc: "caOnly",
			cfg: configpb.LogConfig{
				LogId:        1,
				Prefix:       "/log",
				RootsPemFile: []string{"../testdata/fake-ca.cert"},
				PrivateKey:   privKey,
				AcceptOnlyCa: true,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			opts := InstanceOptions{Config: &test.cfg, Deadline: time.Second, MetricFactory: monitoring.InertMetricFactory{}}
			h, err := SetUpInstance(ctx, opts)
			if err != nil {
				t.Fatalf("%v: SetUpInstance() = %v, want no error", test.desc, err)
			}
			addChainHandler, ok := (*h)[test.cfg.Prefix+ct.AddChainPath]
			if !ok {
				t.Fatal("Couldn't find AddChain handler")
			}
			gotOpts := addChainHandler.Info.validationOpts
			if got, want := gotOpts.notAfterStart, test.cfg.NotAfterStart; want != nil && !equivalentTimes(got, want) {
				t.Errorf("%v: handler notAfterStart %v, want %v", test.desc, got, want)
			}
			if got, want := gotOpts.notAfterLimit, test.cfg.NotAfterLimit; want != nil && !equivalentTimes(got, want) {
				t.Errorf("%v: handler notAfterLimit %v, want %v", test.desc, got, want)
			}
			if got, want := gotOpts.acceptOnlyCA, test.cfg.AcceptOnlyCa; got != want {
				t.Errorf("%v: handler acceptOnlyCA %v, want %v", test.desc, got, want)
			}
		})
	}
}
