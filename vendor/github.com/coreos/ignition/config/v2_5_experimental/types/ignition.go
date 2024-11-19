// Copyright 2015 CoreOS, Inc.
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
	"net/url"

	"github.com/coreos/go-semver/semver"

	"github.com/coreos/ignition/config/shared/errors"
	"github.com/coreos/ignition/config/validate/report"
)

func (c ConfigReference) ValidateSource() report.Report {
	r := report.Report{}
	err := validateURL(c.Source)
	if err != nil {
		r.Add(report.Entry{
			Message: err.Error(),
			Kind:    report.EntryError,
		})
	}
	return r
}

func (c ConfigReference) ValidateHTTPHeaders() report.Report {
	r := report.Report{}

	if len(c.HTTPHeaders) < 1 {
		return r
	}

	u, err := url.Parse(c.Source)
	if err != nil {
		r.Add(report.Entry{
			Message: errors.ErrInvalidUrl.Error(),
			Kind:    report.EntryError,
		})
		return r
	}

	switch u.Scheme {
	case "http", "https":
	default:
		r.Add(report.Entry{
			Message: errors.ErrUnsupportedSchemeForHTTPHeaders.Error(),
			Kind:    report.EntryError,
		})
	}

	return r
}

func (v Ignition) Semver() (*semver.Version, error) {
	return semver.NewVersion(v.Version)
}

func (v Ignition) Validate() report.Report {
	tv, err := v.Semver()
	if err != nil {
		return report.ReportFromError(errors.ErrInvalidVersion, report.EntryError)
	}
	if MaxVersion.Major > tv.Major {
		return report.ReportFromError(errors.ErrOldVersion, report.EntryError)
	}
	if MaxVersion.LessThan(*tv) {
		return report.ReportFromError(errors.ErrNewVersion, report.EntryError)
	}
	return report.Report{}
}
