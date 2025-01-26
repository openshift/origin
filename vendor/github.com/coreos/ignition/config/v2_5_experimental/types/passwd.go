// Copyright 2017 CoreOS, Inc.
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
	"github.com/coreos/ignition/config/shared/errors"
	"github.com/coreos/ignition/config/validate/report"
)

func (p PasswdUser) Validate() report.Report {
	r := report.Report{}
	if p.Create != nil {
		r.Add(report.Entry{
			Message: errors.ErrPasswdCreateDeprecated.Error(),
			Kind:    report.EntryWarning,
		})
		addErr := func(err error) {
			r.Add(report.Entry{
				Message: err.Error(),
				Kind:    report.EntryError,
			})
		}
		if p.Gecos != "" {
			addErr(errors.ErrPasswdCreateAndGecos)
		}
		if len(p.Groups) > 0 {
			addErr(errors.ErrPasswdCreateAndGroups)
		}
		if p.HomeDir != "" {
			addErr(errors.ErrPasswdCreateAndHomeDir)
		}
		if p.NoCreateHome {
			addErr(errors.ErrPasswdCreateAndNoCreateHome)
		}
		if p.NoLogInit {
			addErr(errors.ErrPasswdCreateAndNoLogInit)
		}
		if p.NoUserGroup {
			addErr(errors.ErrPasswdCreateAndNoUserGroup)
		}
		if p.PrimaryGroup != "" {
			addErr(errors.ErrPasswdCreateAndPrimaryGroup)
		}
		if p.Shell != "" {
			addErr(errors.ErrPasswdCreateAndShell)
		}
		if p.System {
			addErr(errors.ErrPasswdCreateAndSystem)
		}
		if p.UID != nil {
			addErr(errors.ErrPasswdCreateAndUID)
		}
	}
	return r
}
