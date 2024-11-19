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
	"github.com/coreos/ignition/config/shared/errors"
	"github.com/coreos/ignition/config/validate/report"
)

func (n Raid) ValidateLevel() report.Report {
	r := report.Report{}
	switch n.Level {
	case "linear", "raid0", "0", "stripe":
		if n.Spares != 0 {
			r.Add(report.Entry{
				Message: errors.ErrSparesUnsupportedForLevel.Error(),
				Kind:    report.EntryError,
			})
		}
	case "raid1", "1", "mirror":
	case "raid4", "4":
	case "raid5", "5":
	case "raid6", "6":
	case "raid10", "10":
	default:
		r.Add(report.Entry{
			Message: errors.ErrUnrecognizedRaidLevel.Error(),
			Kind:    report.EntryError,
		})
	}
	return r
}

func (n Raid) ValidateDevices() report.Report {
	r := report.Report{}
	for _, d := range n.Devices {
		if err := validatePath(string(d)); err != nil {
			r.Add(report.Entry{
				Message: errors.ErrPathRelative.Error(),
				Kind:    report.EntryError,
			})
		}
	}
	return r
}
