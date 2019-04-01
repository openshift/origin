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
	"crypto"
	"errors"
	"fmt"
	"io/ioutil"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/google/certificate-transparency-go/trillian/ctfe/configpb"
	"github.com/google/certificate-transparency-go/x509"
	"github.com/google/trillian/crypto/keys/der"
)

// ValidatedLogConfig represents the LogConfig with the information that has
// been successfully parsed as a result of validating it.
type ValidatedLogConfig struct {
	Config        *configpb.LogConfig
	PubKey        crypto.PublicKey
	PrivKey       ptypes.DynamicAny
	KeyUsages     []x509.ExtKeyUsage
	NotAfterStart *time.Time
	NotAfterLimit *time.Time
}

// LogConfigFromFile creates a slice of LogConfig options from the given
// filename, which should contain text-protobuf encoded configuration data.
func LogConfigFromFile(filename string) ([]*configpb.LogConfig, error) {
	cfgText, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var cfg configpb.LogConfigSet
	if err := proto.UnmarshalText(string(cfgText), &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse log config: %v", err)
	}

	if len(cfg.Config) == 0 {
		return nil, errors.New("empty log config found")
	}
	return cfg.Config, nil
}

// ToMultiLogConfig creates a multi backend config proto from the data
// loaded from a single-backend configuration file. All the log configs
// reference a default backend spec as provided.
func ToMultiLogConfig(cfg []*configpb.LogConfig, beSpec string) *configpb.LogMultiConfig {
	defaultBackend := &configpb.LogBackend{Name: "default", BackendSpec: beSpec}
	for _, c := range cfg {
		c.LogBackendName = defaultBackend.Name
	}
	return &configpb.LogMultiConfig{
		LogConfigs: &configpb.LogConfigSet{Config: cfg},
		Backends:   &configpb.LogBackendSet{Backend: []*configpb.LogBackend{defaultBackend}},
	}
}

// MultiLogConfigFromFile creates a LogMultiConfig proto from the given
// filename, which should contain text-protobuf encoded configuration data.
// Does not do full validation of the config but checks that it is non empty.
func MultiLogConfigFromFile(filename string) (*configpb.LogMultiConfig, error) {
	cfgText, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var cfg configpb.LogMultiConfig
	if err := proto.UnmarshalText(string(cfgText), &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse multi-backend log config: %v", err)
	}

	if len(cfg.LogConfigs.GetConfig()) == 0 || len(cfg.Backends.GetBackend()) == 0 {
		return nil, errors.New("config is missing backends and/or log configs")
	}
	return &cfg, nil
}

// ValidateLogConfig checks that a single log config is valid. In particular:
//  - A mirror log has a valid public key and no private key.
//  - A non-mirror log has a private, and optionally a public key (both valid).
//  - Each of NotBeforeStart and NotBeforeLimit, if set, is a valid timestamp
//    proto. If both are set then NotBeforeStart <= NotBeforeLimit.
// Returns the validated structures (useful to avoid double validation).
func ValidateLogConfig(cfg *configpb.LogConfig) (*ValidatedLogConfig, error) {
	if cfg.LogId == 0 {
		return nil, errors.New("empty log ID")
	}

	var err error
	vCfg := ValidatedLogConfig{Config: cfg}

	// Validate the public key.
	if pubKey := cfg.PublicKey; pubKey != nil {
		if vCfg.PubKey, err = der.UnmarshalPublicKey(pubKey.Der); err != nil {
			return nil, fmt.Errorf("invalid public key: %v", err)
		}
	} else if cfg.IsMirror {
		return nil, errors.New("empty public key for mirror")
	}

	// Validate the private key.
	if !cfg.IsMirror {
		if cfg.PrivateKey == nil {
			return nil, errors.New("empty private key")
		}
		if err = ptypes.UnmarshalAny(cfg.PrivateKey, &vCfg.PrivKey); err != nil {
			return nil, fmt.Errorf("invalid private key: %v", err)
		}
	} else if cfg.PrivateKey != nil {
		return nil, errors.New("unnecessary private key for mirror")
	}

	// Validate the extended key usages list.
	if len(cfg.ExtKeyUsages) > 0 {
		for _, kuStr := range cfg.ExtKeyUsages {
			if ku, ok := stringToKeyUsage[kuStr]; ok {
				vCfg.KeyUsages = append(vCfg.KeyUsages, ku)
			} else {
				return nil, fmt.Errorf("unknown extended key usage: %s", kuStr)
			}
		}
	} else {
		vCfg.KeyUsages = []x509.ExtKeyUsage{x509.ExtKeyUsageAny}
	}

	// Validate the time interval.
	start, limit := cfg.NotAfterStart, cfg.NotAfterLimit
	if start != nil {
		vCfg.NotAfterStart = &time.Time{}
		if *vCfg.NotAfterStart, err = ptypes.Timestamp(start); err != nil {
			return nil, fmt.Errorf("invalid start timestamp: %v", err)
		}
	}
	if limit != nil {
		vCfg.NotAfterLimit = &time.Time{}
		if *vCfg.NotAfterLimit, err = ptypes.Timestamp(limit); err != nil {
			return nil, fmt.Errorf("invalid limit timestamp: %v", err)
		}
	}
	if start != nil && limit != nil && (*vCfg.NotAfterLimit).Before(*vCfg.NotAfterStart) {
		return nil, errors.New("limit before start")
	}

	switch {
	case cfg.MaxMergeDelaySec < 0:
		return nil, errors.New("negative maximum merge delay")
	case cfg.ExpectedMergeDelaySec < 0:
		return nil, errors.New("negative expected merge delay")
	case cfg.ExpectedMergeDelaySec > cfg.MaxMergeDelaySec:
		return nil, errors.New("expected merge delay exceeds MMD")
	}

	return &vCfg, nil
}

// LogBackendMap is a map from log backend names to LogBackend objects.
type LogBackendMap = map[string]*configpb.LogBackend

// BuildLogBackendMap returns a map from log backend names to the corresponding
// LogBackend objects. It returns an error unless all backends have unique
// non-empty names and specifications.
func BuildLogBackendMap(lbs *configpb.LogBackendSet) (LogBackendMap, error) {
	lbm := make(LogBackendMap)
	specs := make(map[string]bool)
	for _, be := range lbs.Backend {
		if len(be.Name) == 0 {
			return nil, fmt.Errorf("empty backend name: %v", be)
		}
		if len(be.BackendSpec) == 0 {
			return nil, fmt.Errorf("empty backend spec: %v", be)
		}
		if _, ok := lbm[be.Name]; ok {
			return nil, fmt.Errorf("duplicate backend name: %v", be)
		}
		if ok := specs[be.BackendSpec]; ok {
			return nil, fmt.Errorf("duplicate backend spec: %v", be)
		}
		lbm[be.Name] = be
		specs[be.BackendSpec] = true
	}
	return lbm, nil
}

// ValidateLogMultiConfig checks that a config is valid for use with multiple
// backend log servers. The rules applied are:
//
// 1. The backend set must define a set of log backends with distinct
// (non empty) names and non empty backend specs.
// 2. The backend specs must all be distinct.
// 3. The log configs must all specify a log backend and each must be one of
// those defined in the backend set.
// 4. The prefixes of configured logs must all be distinct and must not be
// empty.
// 5. The set of tree ids for each configured backend must be distinct.
// 6. All log configs must be valid (see ValidateLogConfig).
//
// TODO(pavelkalinnikov): Replace the returned map with a fully fledged
// ValidatedLogMultiConfig that contains a ValidatedLogConfig for each log.
func ValidateLogMultiConfig(cfg *configpb.LogMultiConfig) (LogBackendMap, error) {
	backendMap, err := BuildLogBackendMap(cfg.Backends)
	if err != nil {
		return nil, err
	}

	// Check that logs all reference a defined backend and there are no duplicate
	// or empty prefixes. Apply other LogConfig specific checks.
	logNameMap := make(map[string]bool)
	logIDMap := make(map[string]bool)
	for _, logCfg := range cfg.LogConfigs.Config {
		if _, err := ValidateLogConfig(logCfg); err != nil {
			return nil, fmt.Errorf("log config: %v: %v", err, logCfg)
		}
		if len(logCfg.Prefix) == 0 {
			return nil, fmt.Errorf("log config: empty prefix: %v", logCfg)
		}
		if logNameMap[logCfg.Prefix] {
			return nil, fmt.Errorf("log config: duplicate prefix: %s: %v", logCfg.Prefix, logCfg)
		}
		if _, ok := backendMap[logCfg.LogBackendName]; !ok {
			return nil, fmt.Errorf("log config: references undefined backend: %s: %v", logCfg.LogBackendName, logCfg)
		}
		logNameMap[logCfg.Prefix] = true
		logIDKey := fmt.Sprintf("%s-%d", logCfg.LogBackendName, logCfg.LogId)
		if ok := logIDMap[logIDKey]; ok {
			return nil, fmt.Errorf("log config: dup tree id: %d for: %v", logCfg.LogId, logCfg)
		}
		logIDMap[logIDKey] = true
	}

	return backendMap, nil
}

var stringToKeyUsage = map[string]x509.ExtKeyUsage{
	"Any":                        x509.ExtKeyUsageAny,
	"ServerAuth":                 x509.ExtKeyUsageServerAuth,
	"ClientAuth":                 x509.ExtKeyUsageClientAuth,
	"CodeSigning":                x509.ExtKeyUsageCodeSigning,
	"EmailProtection":            x509.ExtKeyUsageEmailProtection,
	"IPSECEndSystem":             x509.ExtKeyUsageIPSECEndSystem,
	"IPSECTunnel":                x509.ExtKeyUsageIPSECTunnel,
	"IPSECUser":                  x509.ExtKeyUsageIPSECUser,
	"TimeStamping":               x509.ExtKeyUsageTimeStamping,
	"OCSPSigning":                x509.ExtKeyUsageOCSPSigning,
	"MicrosoftServerGatedCrypto": x509.ExtKeyUsageMicrosoftServerGatedCrypto,
	"NetscapeServerGatedCrypto":  x509.ExtKeyUsageNetscapeServerGatedCrypto,
}
