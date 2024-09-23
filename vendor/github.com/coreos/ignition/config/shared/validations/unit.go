// Copyright 2018 CoreOS, Inc.
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

// Package validations contains validations shared between multiple config
// versions.
package validations

import (
	"github.com/coreos/go-systemd/unit"
	"github.com/coreos/ignition/config/shared/errors"
	"github.com/coreos/ignition/config/validate/report"
)

// ValidateInstallSection is a helper to validate a given unit
func ValidateInstallSection(name string, enabled bool, contentsEmpty bool, contentSections []*unit.UnitOption) report.Report {
	if !enabled {
		// install sections don't matter for not-enabled units
		return report.Report{}
	}
	if contentsEmpty {
		// install sections don't matter if it has no contents, e.g. it's being masked or just has dropins or such
		return report.Report{}
	}
	if contentSections == nil {
		// Should only happen if the unit could not be parsed, at which point an
		// error is probably already in the report so we don't need to double-up on
		// errors + warnings.
		return report.Report{}
	}

	for _, section := range contentSections {
		if section.Section == "Install" {
			return report.Report{}
		}
	}

	return report.Report{
		Entries: []report.Entry{{
			Message: errors.NewNoInstallSectionError(name).Error(),
			Kind:    report.EntryWarning,
		}},
	}
}
