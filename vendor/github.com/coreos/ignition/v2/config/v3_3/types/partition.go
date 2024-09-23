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
	"fmt"
	"regexp"
	"strings"

	"github.com/coreos/ignition/v2/config/shared/errors"
	"github.com/coreos/ignition/v2/config/util"

	"github.com/coreos/vcontext/path"
	"github.com/coreos/vcontext/report"
)

const (
	guidRegexStr = "^(|[[:xdigit:]]{8}-[[:xdigit:]]{4}-[[:xdigit:]]{4}-[[:xdigit:]]{4}-[[:xdigit:]]{12})$"
)

var (
	guidRegex = regexp.MustCompile(guidRegexStr)
)

func (p Partition) Key() string {
	if p.Number != 0 {
		return fmt.Sprintf("number:%d", p.Number)
	} else if p.Label != nil {
		return fmt.Sprintf("label:%s", *p.Label)
	} else {
		return ""
	}
}

func (p Partition) Validate(c path.ContextPath) (r report.Report) {
	if util.IsFalse(p.ShouldExist) &&
		(p.Label != nil || util.NotEmpty(p.TypeGUID) || util.NotEmpty(p.GUID) || p.StartMiB != nil || p.SizeMiB != nil) {
		r.AddOnError(c, errors.ErrShouldNotExistWithOthers)
	}
	if p.Number == 0 && p.Label == nil {
		r.AddOnError(c, errors.ErrNeedLabelOrNumber)
	}

	r.AddOnError(c.Append("label"), p.validateLabel())
	r.AddOnError(c.Append("guid"), validateGUID(p.GUID))
	r.AddOnError(c.Append("typeGuid"), validateGUID(p.TypeGUID))
	return
}

func (p Partition) validateLabel() error {
	if p.Label == nil {
		return nil
	}
	// http://en.wikipedia.org/wiki/GUID_Partition_Table#Partition_entries:
	// 56 (0x38) 	72 bytes 	Partition name (36 UTF-16LE code units)

	// XXX(vc): note GPT calls it a name, we're using label for consistency
	// with udev naming /dev/disk/by-partlabel/*.
	if len(*p.Label) > 36 {
		return errors.ErrLabelTooLong
	}

	// sgdisk uses colons for delimitting compound arguments and does not allow escaping them.
	if strings.Contains(*p.Label, ":") {
		return errors.ErrLabelContainsColon
	}
	return nil
}

func validateGUID(guidPointer *string) error {
	if guidPointer == nil {
		return nil
	}
	guid := *guidPointer
	if ok := guidRegex.MatchString(guid); !ok {
		return errors.ErrDoesntMatchGUIDRegex
	}
	return nil
}
