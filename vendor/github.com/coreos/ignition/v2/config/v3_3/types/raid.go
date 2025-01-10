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
	"github.com/coreos/ignition/v2/config/shared/errors"
	"github.com/coreos/ignition/v2/config/util"

	"github.com/coreos/vcontext/path"
	"github.com/coreos/vcontext/report"
)

func (r Raid) Key() string {
	return r.Name
}

func (r Raid) IgnoreDuplicates() map[string]struct{} {
	return map[string]struct{}{
		"Options": {},
	}
}

func (ra Raid) Validate(c path.ContextPath) (r report.Report) {
	r.AddOnError(c.Append("level"), ra.validateLevel())
	if len(ra.Devices) == 0 {
		r.AddOnError(c.Append("devices"), errors.ErrRaidDevicesRequired)
	}
	return
}

func (r Raid) validateLevel() error {
	if util.NilOrEmpty(r.Level) {
		return errors.ErrRaidLevelRequired
	}
	switch *r.Level {
	case "linear", "raid0", "0", "stripe":
		if r.Spares != nil && *r.Spares != 0 {
			return errors.ErrSparesUnsupportedForLevel
		}
	case "raid1", "1", "mirror":
	case "raid4", "4":
	case "raid5", "5":
	case "raid6", "6":
	case "raid10", "10":
	default:
		return errors.ErrUnrecognizedRaidLevel
	}

	return nil
}
