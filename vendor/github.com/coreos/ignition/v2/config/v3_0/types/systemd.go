// Copyright 2022 Red Hat, Inc.
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
	"regexp"

	"github.com/coreos/ignition/v2/config/shared/errors"
	"github.com/coreos/ignition/v2/config/shared/parse"
	"github.com/coreos/ignition/v2/config/util"

	vpath "github.com/coreos/vcontext/path"
	"github.com/coreos/vcontext/report"
)

func (s Systemd) Validate(c vpath.ContextPath) (r report.Report) {
	units := make(map[string]Unit)
	checkInstanceUnit := regexp.MustCompile(`^(.+?)@(.+?)\.service$`)
	for _, d := range s.Units {
		units[d.Name] = d
	}
	for index, unit := range s.Units {
		if checkInstanceUnit.MatchString(unit.Name) && util.IsTrue(unit.Enabled) {
			instUnitSlice := checkInstanceUnit.FindSubmatch([]byte(unit.Name))
			instantiableUnit := string(instUnitSlice[1]) + "@.service"
			if _, ok := units[instantiableUnit]; ok && util.NotEmpty(units[instantiableUnit].Contents) {
				foundInstallSection := false
				// we're doing a separate validation pass on each unit to identify
				// if an instantiable unit has the install section. So logging an
				// `AddOnError` will produce duplicate errors on bad unit contents
				// because we're already doing that while validating a unit separately.
				opts, err := parse.ParseUnitContents(units[instantiableUnit].Contents)
				if err != nil {
					continue
				}
				for _, section := range opts {
					if section.Section == "Install" {
						foundInstallSection = true
						break
					}
				}
				if !foundInstallSection {
					r.AddOnWarn(c.Append("units", index, "contents"), errors.NewNoInstallSectionForInstantiableUnitError(instantiableUnit, unit.Name))
				}
			}
		}
	}
	return
}
