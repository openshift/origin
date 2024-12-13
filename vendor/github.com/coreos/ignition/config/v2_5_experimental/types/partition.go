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
	"regexp"
	"strings"

	"github.com/coreos/ignition/config/shared/errors"
	"github.com/coreos/ignition/config/validate/report"
)

const (
	guidRegexStr = "^(|[[:xdigit:]]{8}-[[:xdigit:]]{4}-[[:xdigit:]]{4}-[[:xdigit:]]{4}-[[:xdigit:]]{12})$"
)

func (p Partition) Validate() report.Report {
	r := report.Report{}
	if (p.Start != nil || p.Size != nil) && (p.StartMiB != nil || p.SizeMiB != nil) {
		r.Add(report.Entry{
			Message: errors.ErrPartitionsUnitsMismatch.Error(),
			Kind:    report.EntryError,
		})
	}
	if p.ShouldExist != nil && !*p.ShouldExist &&
		(p.Label != nil || p.TypeGUID != "" || p.GUID != "" || p.Start != nil || p.Size != nil) {
		r.Add(report.Entry{
			Message: errors.ErrShouldNotExistWithOthers.Error(),
			Kind:    report.EntryError,
		})
	}
	return r
}

func (p Partition) ValidateSize() report.Report {
	if p.Size != nil {
		return report.ReportFromError(errors.ErrSizeDeprecated, report.EntryDeprecated)
	}
	return report.Report{}
}

func (p Partition) ValidateStart() report.Report {
	if p.Start != nil {
		return report.ReportFromError(errors.ErrStartDeprecated, report.EntryDeprecated)
	}
	return report.Report{}
}

func (p Partition) ValidateLabel() report.Report {
	r := report.Report{}
	if p.Label == nil {
		return r
	}
	// http://en.wikipedia.org/wiki/GUID_Partition_Table#Partition_entries:
	// 56 (0x38) 	72 bytes 	Partition name (36 UTF-16LE code units)

	// XXX(vc): note GPT calls it a name, we're using label for consistency
	// with udev naming /dev/disk/by-partlabel/*.
	if len(*p.Label) > 36 {
		r.Add(report.Entry{
			Message: errors.ErrLabelTooLong.Error(),
			Kind:    report.EntryError,
		})
	}

	// sgdisk uses colons for delimitting compound arguments and does not allow escaping them.
	if strings.Contains(*p.Label, ":") {
		r.Add(report.Entry{
			Message: errors.ErrLabelContainsColon.Error(),
			Kind:    report.EntryWarning,
		})
	}
	return r
}

func (p Partition) ValidateTypeGUID() report.Report {
	return validateGUID(p.TypeGUID)
}

func (p Partition) ValidateGUID() report.Report {
	return validateGUID(p.GUID)
}

func validateGUID(guid string) report.Report {
	r := report.Report{}
	ok, err := regexp.MatchString(guidRegexStr, guid)
	if err != nil {
		r.Add(report.Entry{
			Message: fmt.Sprintf("error matching guid regexp: %v", err),
			Kind:    report.EntryError,
		})
	} else if !ok {
		r.Add(report.Entry{
			Message: errors.ErrDoesntMatchGUIDRegex.Error(),
			Kind:    report.EntryError,
		})
	}
	return r
}
