// Copyright 2016 CoreOS, Inc.
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
	"fmt"
	"path"
	"strings"

	"github.com/coreos/go-systemd/unit"

	"github.com/coreos/ignition/config/shared/errors"
	"github.com/coreos/ignition/config/shared/validations"
	"github.com/coreos/ignition/config/validate/report"
)

func (u Unit) ValidateContents() report.Report {
	r := report.Report{}
	opts, err := validateUnitContent(u.Contents)
	if err != nil {
		r.Add(report.Entry{
			Message: err.Error(),
			Kind:    report.EntryError,
		})
	}

	isEnabled := u.Enable || (u.Enabled != nil && *u.Enabled)
	r.Merge(validations.ValidateInstallSection(u.Name, isEnabled, u.Contents == "", opts))

	return r
}

func (u Unit) ValidateName() report.Report {
	r := report.Report{}
	switch path.Ext(u.Name) {
	case ".service", ".socket", ".device", ".mount", ".automount", ".swap", ".target", ".path", ".timer", ".snapshot", ".slice", ".scope":
	default:
		r.Add(report.Entry{
			Message: errors.ErrInvalidSystemdExt.Error(),
			Kind:    report.EntryError,
		})
	}
	return r
}

func (d Dropin) Validate() report.Report {
	r := report.Report{}

	if _, err := validateUnitContent(d.Contents); err != nil {
		r.Add(report.Entry{
			Message: err.Error(),
			Kind:    report.EntryError,
		})
	}

	switch path.Ext(d.Name) {
	case ".conf":
	default:
		r.Add(report.Entry{
			Message: errors.ErrInvalidSystemdDropinExt.Error(),
			Kind:    report.EntryError,
		})
	}

	return r
}

func (u Networkdunit) Validate() report.Report {
	r := report.Report{}

	if _, err := validateUnitContent(u.Contents); err != nil {
		r.Add(report.Entry{
			Message: err.Error(),
			Kind:    report.EntryError,
		})
	}

	switch path.Ext(u.Name) {
	case ".link", ".netdev", ".network":
	default:
		r.Add(report.Entry{
			Message: errors.ErrInvalidNetworkdExt.Error(),
			Kind:    report.EntryError,
		})
	}

	return r
}

func validateUnitContent(content string) ([]*unit.UnitOption, error) {
	c := strings.NewReader(content)
	opts, err := unit.Deserialize(c)
	if err != nil {
		return nil, fmt.Errorf("invalid unit content: %s", err)
	}
	return opts, nil
}
