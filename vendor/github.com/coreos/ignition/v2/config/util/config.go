// Copyright 2021 Red Hat, Inc.
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

package util

import (
	"github.com/coreos/ignition/v2/config/shared/errors"

	"github.com/coreos/go-semver/semver"
	"github.com/coreos/vcontext/report"
)

type versionStub struct {
	Ignition struct {
		Version string
	}
}

// GetConfigVersion parses the version from the given raw config
func GetConfigVersion(raw []byte) (semver.Version, report.Report, error) {
	if len(raw) == 0 {
		return semver.Version{}, report.Report{}, errors.ErrEmpty
	}

	stub := versionStub{}
	if rpt, err := HandleParseErrors(raw, &stub); err != nil {
		return semver.Version{}, rpt, err
	}

	version, err := semver.NewVersion(stub.Ignition.Version)
	if err != nil {
		return semver.Version{}, report.Report{}, errors.ErrInvalidVersion
	}
	return *version, report.Report{}, nil
}
