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

	// Register PEMKeyFile ProtoHandler
	_ "github.com/google/trillian/crypto/keys/pem/proto"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/google/certificate-transparency-go"
	"github.com/google/certificate-transparency-go/trillian/ctfe/configpb"
	"github.com/google/trillian/crypto/keyspb"
	"github.com/google/trillian/monitoring"
)

func TestSetUpInstance(t *testing.T) {
	ctx := context.Background()

	privKey, err := ptypes.MarshalAny(&keyspb.PEMKeyFile{Path: "../testdata/ct-http-server.privkey.pem", Password: "dirk"})
	if err != nil {
		t.Fatalf("Could not marshal private key proto: %v", err)
	}

	missingPrivKey, err := ptypes.MarshalAny(&keyspb.PEMKeyFile{Path: "../testdata/bogus.privkey.pem", Password: "dirk"})
	if err != nil {
		t.Fatalf("Could not marshal private key proto: %v", err)
	}

	wrongPassPrivKey, err := ptypes.MarshalAny(&keyspb.PEMKeyFile{Path: "../testdata/ct-http-server.privkey.pem", Password: "dirkly"})
	if err != nil {
		t.Fatalf("Could not marshal private key proto: %v", err)
	}

	var tests = []struct {
		desc   string
		cfg    configpb.LogConfig
		errStr string
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
			desc: "no-roots",
			cfg: configpb.LogConfig{
				LogId:      1,
				Prefix:     "log",
				PrivateKey: privKey,
			},
			errStr: "specify RootsPemFile",
		},
		{
			desc: "no-priv-key",
			cfg: configpb.LogConfig{
				LogId:        1,
				Prefix:       "log",
				RootsPemFile: []string{"../testdata/fake-ca.cert"},
			},
			errStr: "specify PrivateKey",
		},
		{
			desc: "missing-root-cert",
			cfg: configpb.LogConfig{
				LogId:        1,
				Prefix:       "log",
				RootsPemFile: []string{"../testdata/bogus.cert"},
				PrivateKey:   privKey,
			},
			errStr: "failed to read trusted roots",
		},
		{
			desc: "missing-privkey",
			cfg: configpb.LogConfig{
				LogId:        1,
				Prefix:       "log",
				RootsPemFile: []string{"../testdata/fake-ca.cert"},
				PrivateKey:   missingPrivKey,
			},
			errStr: "failed to load private key",
		},
		{
			desc: "privkey-wrong-password",
			cfg: configpb.LogConfig{
				LogId:        1,
				Prefix:       "log",
				RootsPemFile: []string{"../testdata/fake-ca.cert"},
				PrivateKey:   wrongPassPrivKey,
			},
			errStr: "failed to load private key",
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
			errStr: "unknown extended key usage",
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
			errStr: "unknown extended key usage",
		},
	}

	opts := InstanceOptions{Deadline: time.Second, MetricFactory: monitoring.InertMetricFactory{}}
	for _, test := range tests {
		if _, err := SetUpInstance(ctx, nil, &test.cfg, opts); err != nil {
			if test.errStr == "" {
				t.Errorf("(%v).SetUpInstance()=_,%v; want _,nil", test.desc, err)
			} else if !strings.Contains(err.Error(), test.errStr) {
				t.Errorf("(%v).SetUpInstance()=_,%v; want err containing %q", test.desc, err, test.errStr)
			}
			continue
		}
		if test.errStr != "" {
			t.Errorf("(%v).SetUpInstance()=_,mo;; want err containing %q", test.desc, test.errStr)
		}
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

	opts := InstanceOptions{Deadline: time.Second, MetricFactory: monitoring.InertMetricFactory{}}
	for _, test := range tests {
		h, err := SetUpInstance(ctx, nil, &test.cfg, opts)
		if err != nil {
			t.Errorf("%v: SetUpInstance() = %v, want no error", test.desc, err)
			continue
		}
		addChainHandler, ok := (*h)[test.cfg.Prefix+ct.AddChainPath]
		if !ok {
			t.Error("Couldn't find AddChain handler")
			continue
		}
		gotOpts := addChainHandler.Context.validationOpts
		if got, want := gotOpts.notAfterStart, test.cfg.NotAfterStart; want != nil && !equivalentTimes(got, want) {
			t.Errorf("%v: handler notAfterStart %v, want %v", test.desc, got, want)
		}
		if got, want := gotOpts.notAfterLimit, test.cfg.NotAfterLimit; want != nil && !equivalentTimes(got, want) {
			t.Errorf("%v: handler notAfterLimit %v, want %v", test.desc, got, want)
		}
		if got, want := gotOpts.acceptOnlyCA, test.cfg.AcceptOnlyCa; got != want {
			t.Errorf("%v: handler acceptOnlyCA %v, want %v", test.desc, got, want)
		}
	}
}

func TestValidateLogMultiConfig(t *testing.T) {
	var tests = []struct {
		desc   string
		cfg    configpb.LogMultiConfig
		errStr string
	}{
		{
			desc:   "missing backend name",
			errStr: "empty backend name",
			cfg: configpb.LogMultiConfig{
				Backends: &configpb.LogBackendSet{
					Backend: []*configpb.LogBackend{
						{BackendSpec: "testspec"},
					},
				},
				LogConfigs: &configpb.LogConfigSet{},
			},
		},
		{
			desc:   "missing backend spec",
			errStr: "empty backend_spec",
			cfg: configpb.LogMultiConfig{
				Backends: &configpb.LogBackendSet{
					Backend: []*configpb.LogBackend{
						{Name: "log1"},
					},
				},
				LogConfigs: &configpb.LogConfigSet{},
			},
		},
		{
			desc:   "missing backend name and spec",
			errStr: "empty backend name",
			cfg: configpb.LogMultiConfig{
				Backends: &configpb.LogBackendSet{
					Backend: []*configpb.LogBackend{
						{},
					},
				},
				LogConfigs: &configpb.LogConfigSet{},
			},
		},
		{
			desc:   "dup backend name",
			errStr: "duplicate backend name",
			cfg: configpb.LogMultiConfig{
				Backends: &configpb.LogBackendSet{
					Backend: []*configpb.LogBackend{
						{Name: "dup", BackendSpec: "testspec"},
						{Name: "dup", BackendSpec: "testspec"},
					},
				},
				LogConfigs: &configpb.LogConfigSet{},
			},
		},
		{
			desc:   "dup backend spec",
			errStr: "duplicate backend spec",
			cfg: configpb.LogMultiConfig{
				Backends: &configpb.LogBackendSet{
					Backend: []*configpb.LogBackend{
						{Name: "backend1", BackendSpec: "testspec"},
						{Name: "backend2", BackendSpec: "testspec"},
					},
				},
				LogConfigs: &configpb.LogConfigSet{},
			},
		},
		{
			desc:   "missing backend reference",
			errStr: "empty backend",
			cfg: configpb.LogMultiConfig{
				Backends: &configpb.LogBackendSet{
					Backend: []*configpb.LogBackend{
						{Name: "log1"},
					},
				},
				LogConfigs: &configpb.LogConfigSet{
					Config: []*configpb.LogConfig{
						{LogBackendName: "log2"},
					},
				},
			},
		},
		{
			desc:   "undefined backend reference",
			errStr: "undefined backend",
			cfg: configpb.LogMultiConfig{
				Backends: &configpb.LogBackendSet{
					Backend: []*configpb.LogBackend{
						{Name: "log1", BackendSpec: "testspec"},
					},
				},
				LogConfigs: &configpb.LogConfigSet{
					Config: []*configpb.LogConfig{
						{LogBackendName: "log2", Prefix: "prefix"},
					},
				},
			},
		},
		{
			desc:   "empty log prefix",
			errStr: "empty prefix",
			cfg: configpb.LogMultiConfig{
				Backends: &configpb.LogBackendSet{
					Backend: []*configpb.LogBackend{
						{Name: "log1", BackendSpec: "testspec1"},
						{Name: "log2", BackendSpec: "testspec2"},
						{Name: "log3", BackendSpec: "testspec3"},
					},
				},
				LogConfigs: &configpb.LogConfigSet{
					Config: []*configpb.LogConfig{
						{LogBackendName: "log1", Prefix: "prefix1"},
						{LogBackendName: "log2"},
						{LogBackendName: "log3", Prefix: "prefix3"},
					},
				},
			},
		},
		{
			desc:   "dup log prefix",
			errStr: "duplicate prefix",
			cfg: configpb.LogMultiConfig{
				Backends: &configpb.LogBackendSet{
					Backend: []*configpb.LogBackend{
						{Name: "log1", BackendSpec: "testspec1"},
					},
				},
				LogConfigs: &configpb.LogConfigSet{
					Config: []*configpb.LogConfig{
						{LogBackendName: "log1", Prefix: "prefix1", LogId: 1},
						{LogBackendName: "log1", Prefix: "prefix2", LogId: 2},
						{LogBackendName: "log1", Prefix: "prefix1", LogId: 3},
					},
				},
			},
		},
		{
			desc:   "dup log ids on same backend",
			errStr: "dup tree id",
			cfg: configpb.LogMultiConfig{
				Backends: &configpb.LogBackendSet{
					Backend: []*configpb.LogBackend{
						{Name: "log1", BackendSpec: "testspec1"},
					},
				},
				LogConfigs: &configpb.LogConfigSet{
					Config: []*configpb.LogConfig{
						{LogBackendName: "log1", Prefix: "prefix1", LogId: 1},
						{LogBackendName: "log1", Prefix: "prefix2", LogId: 1},
						{LogBackendName: "log1", Prefix: "prefix1", LogId: 1},
					},
				},
			},
		},
		{
			desc:   "start timestamp invalid",
			errStr: "invalid start",
			cfg: configpb.LogMultiConfig{
				Backends: &configpb.LogBackendSet{
					Backend: []*configpb.LogBackend{
						{Name: "log1", BackendSpec: "testspec1"},
					},
				},
				LogConfigs: &configpb.LogConfigSet{
					Config: []*configpb.LogConfig{
						{
							LogBackendName: "log1",
							Prefix:         "prefix1",
							LogId:          1,
							NotAfterStart:  &timestamp.Timestamp{Seconds: 23, Nanos: -50},
							NotAfterLimit:  &timestamp.Timestamp{Seconds: 23},
						},
					},
				},
			},
		},
		{
			desc:   "limit timestamp invalid",
			errStr: "invalid limit",
			cfg: configpb.LogMultiConfig{
				Backends: &configpb.LogBackendSet{
					Backend: []*configpb.LogBackend{
						{Name: "log1", BackendSpec: "testspec1"},
					},
				},
				LogConfigs: &configpb.LogConfigSet{
					Config: []*configpb.LogConfig{
						{
							LogBackendName: "log1",
							Prefix:         "prefix1",
							LogId:          1,
							NotAfterStart:  &timestamp.Timestamp{Seconds: 23},
							NotAfterLimit:  &timestamp.Timestamp{Seconds: 23, Nanos: -50},
						},
					},
				},
			},
		},
		{
			desc:   "limit before start",
			errStr: "before start",
			cfg: configpb.LogMultiConfig{
				Backends: &configpb.LogBackendSet{
					Backend: []*configpb.LogBackend{
						{Name: "log1", BackendSpec: "testspec1"},
					},
				},
				LogConfigs: &configpb.LogConfigSet{
					Config: []*configpb.LogConfig{
						{
							LogBackendName: "log1",
							Prefix:         "prefix1",
							LogId:          1,
							NotAfterStart:  &timestamp.Timestamp{Seconds: 23},
							NotAfterLimit:  &timestamp.Timestamp{Seconds: 22},
						},
					},
				},
			},
		},
		{
			desc: "valid config",
			cfg: configpb.LogMultiConfig{
				Backends: &configpb.LogBackendSet{
					Backend: []*configpb.LogBackend{
						{Name: "log1", BackendSpec: "testspec1"},
						{Name: "log2", BackendSpec: "testspec2"},
						{Name: "log3", BackendSpec: "testspec3"},
					},
				},
				LogConfigs: &configpb.LogConfigSet{
					Config: []*configpb.LogConfig{
						{LogBackendName: "log1", Prefix: "prefix1", LogId: 1},
						{LogBackendName: "log2", Prefix: "prefix2", LogId: 2},
						{LogBackendName: "log3", Prefix: "prefix3", LogId: 3},
					},
				},
			},
		},
		{
			desc: "valid config dup ids on different backends",
			cfg: configpb.LogMultiConfig{
				Backends: &configpb.LogBackendSet{
					Backend: []*configpb.LogBackend{
						{Name: "log1", BackendSpec: "testspec1"},
						{Name: "log2", BackendSpec: "testspec2"},
						{Name: "log3", BackendSpec: "testspec3"},
					},
				},
				LogConfigs: &configpb.LogConfigSet{
					Config: []*configpb.LogConfig{
						{LogBackendName: "log1", Prefix: "prefix1", LogId: 999},
						{LogBackendName: "log2", Prefix: "prefix2", LogId: 999},
						{LogBackendName: "log3", Prefix: "prefix3", LogId: 999},
					},
				},
			},
		},
		{
			desc: "valid config - only not after start set",
			cfg: configpb.LogMultiConfig{
				Backends: &configpb.LogBackendSet{
					Backend: []*configpb.LogBackend{
						{Name: "log1", BackendSpec: "testspec1"},
					},
				},
				LogConfigs: &configpb.LogConfigSet{
					Config: []*configpb.LogConfig{
						{LogBackendName: "log1", Prefix: "prefix1", LogId: 1, NotAfterStart: &timestamp.Timestamp{Seconds: 23}},
					},
				},
			},
		},
		{
			desc: "valid config - only not after limit set",
			cfg: configpb.LogMultiConfig{
				Backends: &configpb.LogBackendSet{
					Backend: []*configpb.LogBackend{
						{Name: "log1", BackendSpec: "testspec1"},
					},
				},
				LogConfigs: &configpb.LogConfigSet{
					Config: []*configpb.LogConfig{
						{LogBackendName: "log1", Prefix: "prefix1", LogId: 1, NotAfterLimit: &timestamp.Timestamp{Seconds: 23}},
					},
				},
			},
		},
		{
			desc: "valid config with time range",
			cfg: configpb.LogMultiConfig{
				Backends: &configpb.LogBackendSet{
					Backend: []*configpb.LogBackend{
						{Name: "log1", BackendSpec: "testspec1"},
					},
				},
				LogConfigs: &configpb.LogConfigSet{
					Config: []*configpb.LogConfig{
						{
							LogBackendName: "log1",
							Prefix:         "prefix1",
							LogId:          1,
							NotAfterStart:  &timestamp.Timestamp{Seconds: 23},
							NotAfterLimit:  &timestamp.Timestamp{Seconds: 24},
						},
					},
				},
			},
		},
	}

	for _, test := range tests {
		_, err := ValidateLogMultiConfig(&test.cfg)

		if len(test.errStr) == 0 && err != nil {
			t.Errorf("ValidateLogMultiConfig()=%v, want: nil (%v)", err, test.desc)
			continue
		}

		if len(test.errStr) > 0 && (err == nil || !strings.Contains(err.Error(), test.errStr)) {
			t.Errorf("ValidateLogMultiConfig()=%v, want: %v (%v)", err, test.errStr, test.desc)
		}
	}
}

func TestToMultiLogConfig(t *testing.T) {
	var tests = []struct {
		desc string
		cfg  []*configpb.LogConfig
		want *configpb.LogMultiConfig
	}{
		{
			desc: "one valid log config",
			cfg: []*configpb.LogConfig{
				{LogId: 1, Prefix: "test"},
			},
			want: &configpb.LogMultiConfig{
				Backends: &configpb.LogBackendSet{
					Backend: []*configpb.LogBackend{{Name: "default", BackendSpec: "spec"}},
				},
				LogConfigs: &configpb.LogConfigSet{
					Config: []*configpb.LogConfig{{Prefix: "test", LogId: 1, LogBackendName: "default"}},
				},
			},
		},
		{
			desc: "three valid log configs",
			cfg: []*configpb.LogConfig{
				{LogId: 1, Prefix: "test1"},
				{LogId: 2, Prefix: "test2"},
				{LogId: 3, Prefix: "test3"},
			},
			want: &configpb.LogMultiConfig{
				Backends: &configpb.LogBackendSet{
					Backend: []*configpb.LogBackend{{Name: "default", BackendSpec: "spec"}},
				},
				LogConfigs: &configpb.LogConfigSet{
					Config: []*configpb.LogConfig{
						{Prefix: "test1", LogId: 1, LogBackendName: "default"},
						{Prefix: "test2", LogId: 2, LogBackendName: "default"},
						{Prefix: "test3", LogId: 3, LogBackendName: "default"},
					},
				},
			},
		},
	}

	for _, test := range tests {
		got := ToMultiLogConfig(test.cfg, "spec")

		if !proto.Equal(got, test.want) {
			t.Errorf("TestToMultiLogConfig() got: %v, want: %v (%v)", got, test.want, test.desc)
		}
	}
}
