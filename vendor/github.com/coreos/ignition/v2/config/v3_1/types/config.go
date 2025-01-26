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
	"github.com/coreos/ignition/v2/config/shared/errors"
	"github.com/coreos/ignition/v2/config/util"

	"github.com/coreos/go-semver/semver"
	"github.com/coreos/vcontext/path"
	"github.com/coreos/vcontext/report"
)

var (
	MaxVersion = semver.Version{
		Major: 3,
		Minor: 1,
	}
)

func (cfg Config) Validate(c path.ContextPath) (r report.Report) {
	systemdPath := "/etc/systemd/system/"
	unitPaths := map[string]struct{}{}
	for _, unit := range cfg.Systemd.Units {
		if !util.NilOrEmpty(unit.Contents) {
			pathString := systemdPath + unit.Name
			unitPaths[pathString] = struct{}{}
		}
		for _, dropin := range unit.Dropins {
			if !util.NilOrEmpty(dropin.Contents) {
				pathString := systemdPath + unit.Name + ".d/" + dropin.Name
				unitPaths[pathString] = struct{}{}
			}
		}
	}
	for i, f := range cfg.Storage.Files {
		if _, exists := unitPaths[f.Path]; exists {
			r.AddOnError(c.Append("storage", "files", i, "path"), errors.ErrPathConflictsSystemd)
		}
	}
	for i, d := range cfg.Storage.Directories {
		if _, exists := unitPaths[d.Path]; exists {
			r.AddOnError(c.Append("storage", "directories", i, "path"), errors.ErrPathConflictsSystemd)
		}
	}
	for i, l := range cfg.Storage.Links {
		if _, exists := unitPaths[l.Path]; exists {
			r.AddOnError(c.Append("storage", "links", i, "path"), errors.ErrPathConflictsSystemd)
		}
	}
	return
}
