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

package integration

import (
	"context"
	"encoding/pem"
	"flag"
	"fmt"
	"io/ioutil"
	"math/rand"
	"testing"
	"time"

	"github.com/golang/protobuf/ptypes"
	"github.com/google/certificate-transparency-go/trillian/ctfe"
	"github.com/google/certificate-transparency-go/trillian/ctfe/configpb"
	"github.com/google/trillian/crypto/keyspb"
	"github.com/google/trillian/storage/testdb"

	// Register PEMKeyFile and PrivateKey ProtoHandlers.
	_ "github.com/google/trillian/crypto/keys/der/proto"
	_ "github.com/google/trillian/crypto/keys/pem/proto"
)

var (
	adminServer    = flag.String("admin_server", "", "Address of log admin RPC server. Required for lifecycle test.")
	httpServers    = flag.String("ct_http_servers", "localhost:8092", "Comma-separated list of (assumed interchangeable) servers, each as address:port")
	metricsServers = flag.String("ct_metrics_servers", "localhost:8093", "Comma-separated list of (assumed interchangeable) metrics servers, each as address:port")
	testDir        = flag.String("testdata_dir", "testdata", "Name of directory with test data")
	seed           = flag.Int64("seed", -1, "Seed for random number generation")
	logConfig      = flag.String("log_config", "", "File holding log config in JSON")
	mmd            = flag.Duration("mmd", 30*time.Second, "MMD for tested logs")
	skipStats      = flag.Bool("skip_stats", false, "Skip checks of expected log statistics")
)

func commonSetup(t *testing.T) []*configpb.LogConfig {
	t.Helper()
	if *logConfig == "" {
		t.Skip("Integration test skipped as no log config provided")
	}
	if *seed == -1 {
		*seed = time.Now().UTC().UnixNano() & 0xFFFFFFFF
	}
	fmt.Printf("Today's test has been brought to you by the letters C and T and the number %#x\n", *seed)
	rand.Seed(*seed)

	cfgs, err := ctfe.LogConfigFromFile(*logConfig)
	if err != nil {
		t.Fatalf("Failed to read log config: %v", err)
	}
	return cfgs
}

func TestLiveCTIntegration(t *testing.T) {
	flag.Parse()
	cfgs := commonSetup(t)
	for _, cfg := range cfgs {
		cfg := cfg // capture config
		t.Run(cfg.Prefix, func(t *testing.T) {
			t.Parallel()
			var stats *logStats
			if !*skipStats {
				stats = newLogStats(cfg.LogId)
			}
			if err := RunCTIntegrationForLog(cfg, *httpServers, *metricsServers, *testDir, *mmd, stats); err != nil {
				t.Errorf("%s: failed: %v", cfg.Prefix, err)
			}
		})
	}
}

func TestLiveLifecycleCTIntegration(t *testing.T) {
	flag.Parse()
	cfgs := commonSetup(t)
	for _, cfg := range cfgs {
		cfg := cfg // capture config
		t.Run(cfg.Prefix, func(t *testing.T) {
			t.Parallel()
			var stats *logStats
			if !*skipStats {
				stats = newLogStats(cfg.LogId)
			}
			if err := RunCTLifecycleForLog(cfg, *httpServers, *metricsServers, *adminServer, *testDir, *mmd, stats); err != nil {
				t.Errorf("%s: failed: %v", cfg.Prefix, err)
			}
		})
	}
}

const (
	rootsPEMFile    = "../testdata/fake-ca.cert"
	pubKeyPEMFile   = "../testdata/ct-http-server.pubkey.pem"
	privKeyPEMFile  = "../testdata/ct-http-server.privkey.pem"
	privKeyPassword = "dirk"
)

func TestInProcessCTIntegration(t *testing.T) {
	testdb.SkipIfNoMySQL(t)

	pubKeyDER, err := loadPublicKey(pubKeyPEMFile)
	if err != nil {
		t.Fatalf("Could not load public key: %v", err)
	}

	pubKey := &keyspb.PublicKey{Der: pubKeyDER}
	privKey, err := ptypes.MarshalAny(&keyspb.PEMKeyFile{Path: privKeyPEMFile, Password: privKeyPassword})
	if err != nil {
		t.Fatalf("Could not marshal private key as protobuf Any: %v", err)
	}

	ctx := context.Background()
	cfgs := []*configpb.LogConfig{
		{
			Prefix:       "athos",
			RootsPemFile: []string{rootsPEMFile},
			PublicKey:    pubKey,
			PrivateKey:   privKey,
		},
		{
			Prefix:       "porthos",
			RootsPemFile: []string{rootsPEMFile},
			PublicKey:    pubKey,
			PrivateKey:   privKey,
		},
		{
			Prefix:       "aramis",
			RootsPemFile: []string{rootsPEMFile},
			PublicKey:    pubKey,
			PrivateKey:   privKey,
		},
	}

	env, err := NewCTLogEnv(ctx, cfgs, 2, "TestInProcessCTIntegration")
	if err != nil {
		t.Fatalf("Failed to launch test environment: %v", err)
	}
	defer env.Close()

	mmd := 120 * time.Second
	// Run a container for the parallel sub-tests, so that we wait until they
	// all complete before terminating the test environment.
	t.Run("container", func(t *testing.T) {
		for _, cfg := range cfgs {
			cfg := cfg // capture config
			t.Run(cfg.Prefix, func(t *testing.T) {
				t.Parallel()
				stats := newLogStats(cfg.LogId)
				if err := RunCTIntegrationForLog(cfg, env.CTAddr, env.CTAddr, "../testdata", mmd, stats); err != nil {
					t.Errorf("%s: failed: %v", cfg.Prefix, err)
				}
			})
		}
	})
}

func loadPublicKey(path string) ([]byte, error) {
	pemKey, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(pemKey)
	if block == nil {
		return nil, fmt.Errorf("could not decode PEM public key: %v", path)
	}
	if block.Type != "PUBLIC KEY" {
		return nil, fmt.Errorf("got %q PEM, want \"PUBLIC KEY\": %v", block.Type, path)
	}

	return block.Bytes, nil
}
