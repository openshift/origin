// Copyright 2020 Red Hat, Inc.
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

package types

import (
	"github.com/coreos/go-semver/semver"

	"github.com/coreos/ignition/v2/config/shared/errors"

	"github.com/coreos/vcontext/path"
	"github.com/coreos/vcontext/report"
)

func (v Ignition) Semver() (*semver.Version, error) {
	return semver.NewVersion(v.Version)
}

func (ic IgnitionConfig) Validate(c path.ContextPath) (r report.Report) {
	for i, res := range ic.Merge {
		r.AddOnError(c.Append("merge", i), res.validateRequiredSource())
	}
	return
}

func (v Ignition) Validate(c path.ContextPath) (r report.Report) {
	c = c.Append("version")
	tv, err := v.Semver()
	if err != nil {
		r.AddOnError(c, errors.ErrInvalidVersion)
		return
	}

	if MaxVersion != *tv {
		r.AddOnError(c, errors.ErrUnknownVersion)
	}
	return
}
