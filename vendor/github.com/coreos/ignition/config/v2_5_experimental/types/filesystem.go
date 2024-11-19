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

	"github.com/coreos/ignition/config/shared/errors"
	"github.com/coreos/ignition/config/validate/report"
)

func (f Filesystem) Validate() report.Report {
	r := report.Report{}
	if f.Mount == nil && f.Path == nil {
		r.Add(report.Entry{
			Message: errors.ErrFilesystemNoMountPath.Error(),
			Kind:    report.EntryError,
		})
	}
	if f.Mount != nil {
		if f.Path != nil {
			r.Add(report.Entry{
				Message: errors.ErrFilesystemMountAndPath.Error(),
				Kind:    report.EntryError,
			})
		}
		if f.Mount.Create != nil {
			if f.Mount.WipeFilesystem {
				r.Add(report.Entry{
					Message: errors.ErrUsedCreateAndWipeFilesystem.Error(),
					Kind:    report.EntryError,
				})
			}
			if len(f.Mount.Options) > 0 {
				r.Add(report.Entry{
					Message: errors.ErrUsedCreateAndMountOpts.Error(),
					Kind:    report.EntryError,
				})
			}
			r.Add(report.Entry{
				Message: errors.ErrWarningCreateDeprecated.Error(),
				Kind:    report.EntryWarning,
			})
		}
	}
	return r
}

func (f Filesystem) ValidatePath() report.Report {
	r := report.Report{}
	if f.Path != nil && validatePath(*f.Path) != nil {
		r.Add(report.Entry{
			Message: fmt.Sprintf("filesystem %q: path not absolute", f.Name),
			Kind:    report.EntryError,
		})
	}
	return r
}

func (m Mount) Validate() report.Report {
	r := report.Report{}
	switch m.Format {
	case "ext4", "btrfs", "xfs", "swap", "vfat":
	default:
		r.Add(report.Entry{
			Message: errors.ErrFilesystemInvalidFormat.Error(),
			Kind:    report.EntryError,
		})
	}
	return r
}

func (m Mount) ValidateDevice() report.Report {
	r := report.Report{}
	if err := validatePath(m.Device); err != nil {
		r.Add(report.Entry{
			Message: err.Error(),
			Kind:    report.EntryError,
		})
	}
	return r
}

func (m Mount) ValidateLabel() report.Report {
	r := report.Report{}
	if m.Label == nil {
		return r
	}
	switch m.Format {
	case "ext4":
		if len(*m.Label) > 16 {
			// source: man mkfs.ext4
			r.Add(report.Entry{
				Message: errors.ErrExt4LabelTooLong.Error(),
				Kind:    report.EntryError,
			})
		}
	case "btrfs":
		if len(*m.Label) > 256 {
			// source: man mkfs.btrfs
			r.Add(report.Entry{
				Message: errors.ErrBtrfsLabelTooLong.Error(),
				Kind:    report.EntryError,
			})
		}
	case "xfs":
		if len(*m.Label) > 12 {
			// source: man mkfs.xfs
			r.Add(report.Entry{
				Message: errors.ErrXfsLabelTooLong.Error(),
				Kind:    report.EntryError,
			})
		}
	case "swap":
		// mkswap's man page does not state a limit on label size, but through
		// experimentation it appears that mkswap will truncate long labels to
		// 15 characters, so let's enforce that.
		if len(*m.Label) > 15 {
			r.Add(report.Entry{
				Message: errors.ErrSwapLabelTooLong.Error(),
				Kind:    report.EntryError,
			})
		}
	case "vfat":
		if len(*m.Label) > 11 {
			// source: man mkfs.fat
			r.Add(report.Entry{
				Message: errors.ErrVfatLabelTooLong.Error(),
				Kind:    report.EntryError,
			})
		}
	}
	return r
}
