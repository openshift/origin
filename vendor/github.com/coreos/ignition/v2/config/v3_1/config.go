// Copyright 2019 Red Hat, Inc.
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

package v3_1

import (
	"github.com/coreos/ignition/v2/config/merge"
	"github.com/coreos/ignition/v2/config/shared/errors"
	"github.com/coreos/ignition/v2/config/util"
	prev "github.com/coreos/ignition/v2/config/v3_0"
	"github.com/coreos/ignition/v2/config/v3_1/translate"
	"github.com/coreos/ignition/v2/config/v3_1/types"
	"github.com/coreos/ignition/v2/config/validate"

	"github.com/coreos/go-semver/semver"
	"github.com/coreos/vcontext/report"
)

func Merge(parent, child types.Config) types.Config {
	res, _ := merge.MergeStructTranscribe(parent, child)
	return res.(types.Config)
}

// Parse parses the raw config into a types.Config struct and generates a report of any
// errors, warnings, info, and deprecations it encountered
func Parse(rawConfig []byte) (types.Config, report.Report, error) {
	if len(rawConfig) == 0 {
		return types.Config{}, report.Report{}, errors.ErrEmpty
	}

	var config types.Config
	if rpt, err := util.HandleParseErrors(rawConfig, &config); err != nil {
		return types.Config{}, rpt, err
	}

	version, err := semver.NewVersion(config.Ignition.Version)

	if err != nil || *version != types.MaxVersion {
		return types.Config{}, report.Report{}, errors.ErrUnknownVersion
	}

	rpt := validate.ValidateWithContext(config, rawConfig)
	if rpt.IsFatal() {
		return types.Config{}, rpt, errors.ErrInvalid
	}

	return config, rpt, nil
}

// ParseCompatibleVersion parses the raw config of version 3.1.0 or lesser
// into a 3.1 types.Config struct and generates a report of any errors, warnings,
// info, and deprecations it encountered
func ParseCompatibleVersion(raw []byte) (types.Config, report.Report, error) {
	version, rpt, err := util.GetConfigVersion(raw)
	if err != nil {
		return types.Config{}, rpt, err
	}

	if version == types.MaxVersion {
		return Parse(raw)
	}
	prevCfg, r, err := prev.ParseCompatibleVersion(raw)
	if err != nil {
		return types.Config{}, r, err
	}
	return translate.Translate(prevCfg), r, nil
}
