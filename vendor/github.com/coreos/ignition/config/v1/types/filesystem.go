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

type Filesystem struct {
	Device Path              `json:"device,omitempty"`
	Format FilesystemFormat  `json:"format,omitempty"`
	Create *FilesystemCreate `json:"create,omitempty"`
	Files  []File            `json:"files,omitempty"`
}

type FilesystemCreate struct {
	Force   bool        `json:"force,omitempty"`
	Options MkfsOptions `json:"options,omitempty"`
}

type FilesystemFormat string

func (f FilesystemFormat) Validate() report.Report {
	switch f {
	case "ext4", "btrfs", "xfs":
		return report.Report{}
	default:
		return report.ReportFromError(errors.ErrFilesystemInvalidFormat, report.EntryError)
	}
}

type MkfsOptions []string
