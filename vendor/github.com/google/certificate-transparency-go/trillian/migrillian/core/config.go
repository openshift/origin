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

package core

import (
	"errors"
	"fmt"
	"io/ioutil"

	"github.com/golang/protobuf/proto"
	"github.com/google/certificate-transparency-go/trillian/ctfe"
	"github.com/google/certificate-transparency-go/trillian/migrillian/configpb"
)

// LoadConfigFromFile reads MigrillianConfig from the given filename, which
// should contain text-protobuf encoded configuration data.
func LoadConfigFromFile(filename string) (*configpb.MigrillianConfig, error) {
	text, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	var cfg configpb.MigrillianConfig
	if err := proto.UnmarshalText(string(text), &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %v", err)
	}
	return &cfg, nil
}

// ValidateMigrationConfig verifies that the migration config is sane.
func ValidateMigrationConfig(cfg *configpb.MigrationConfig) error {
	// TODO(pavelkalinnikov): Also try to parse the public key.
	switch {
	case len(cfg.SourceUri) == 0:
		return errors.New("missing CT log URI")
	case cfg.PublicKey == nil:
		return errors.New("missing public key")
	case len(cfg.LogBackendName) == 0:
		return errors.New("missing log backend name")
	case cfg.LogId <= 0:
		return errors.New("log ID must be positive")
	case cfg.BatchSize <= 0:
		return errors.New("batch size must be positive")
	}
	return nil
}

// ValidateConfig verifies that MigrillianConfig is correct. In particular:
// - The log backends have distinct non-empty names and backend specs.
// - Migration configs are valid (as per ValidateMigrationConfig).
// - Migration configs specify backend names present in the set of backends.
// - Each migration config has a unique (backend, tree ID) pair.
// Returns a map from log backend names to the corresponding LogBackend.
func ValidateConfig(cfg *configpb.MigrillianConfig) (ctfe.LogBackendMap, error) {
	lbm, err := ctfe.BuildLogBackendMap(cfg.Backends)
	if err != nil {
		return nil, err
	}
	// Check that logs all reference a defined backend and there are no duplicate
	// log IDs per backend. Apply other MigrationConfig specific checks.
	logIDs := make(map[string]bool)
	for _, mc := range cfg.MigrationConfigs.Config {
		if err := ValidateMigrationConfig(mc); err != nil {
			return nil, fmt.Errorf("MigrationConfig: %v: %v", err, mc)
		}
		if _, ok := lbm[mc.LogBackendName]; !ok {
			return nil, fmt.Errorf("undefined backend %q: %v", mc.LogBackendName, mc)
		}
		key := fmt.Sprintf("%s-%d", mc.LogBackendName, mc.LogId)
		if ok := logIDs[key]; ok {
			return nil, fmt.Errorf("duplicate tree ID %d: %v", mc.LogId, mc)
		}
		logIDs[key] = true
	}
	return lbm, nil
}
