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
	"os"

	"github.com/coreos/ignition/config/shared/errors"
	"github.com/coreos/ignition/config/validate/report"
)

type FileMode os.FileMode

type File struct {
	Path     Path     `json:"path,omitempty"`
	Contents string   `json:"contents,omitempty"`
	Mode     FileMode `json:"mode,omitempty"`
	Uid      int      `json:"uid,omitempty"`
	Gid      int      `json:"gid,omitempty"`
}

func (m FileMode) Validate() report.Report {
	if (m &^ 07777) != 0 {
		return report.ReportFromError(errors.ErrFileIllegalMode, report.EntryError)
	}
	return report.Report{}
}
