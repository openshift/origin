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
	"strings"

	"github.com/coreos/ignition/v2/config/shared/errors"
	"github.com/coreos/ignition/v2/config/util"

	"github.com/coreos/vcontext/path"
	"github.com/coreos/vcontext/report"
)

func (l Luks) Key() string {
	return l.Name
}

func (l Luks) IgnoreDuplicates() map[string]struct{} {
	return map[string]struct{}{
		"Options": {},
	}
}

func (l Luks) Validate(c path.ContextPath) (r report.Report) {
	if strings.Contains(l.Name, "/") {
		r.AddOnError(c.Append("name"), errors.ErrLuksNameContainsSlash)
	}
	r.AddOnError(c.Append("label"), l.validateLabel())
	if util.NilOrEmpty(l.Device) {
		r.AddOnError(c.Append("device"), errors.ErrDiskDeviceRequired)
	} else {
		r.AddOnError(c.Append("device"), validatePath(*l.Device))
	}

	if l.Clevis != nil {
		if l.Clevis.Custom != nil && (len(l.Clevis.Tang) > 0 || util.IsTrue(l.Clevis.Tpm2) || (l.Clevis.Threshold != nil && *l.Clevis.Threshold != 0)) {
			r.AddOnError(c.Append("clevis"), errors.ErrClevisCustomWithOthers)
		}
	}

	// fail if a key file is provided and is not valid
	if err := validateURLNilOK(l.KeyFile.Source); err != nil {
		r.AddOnError(c.Append("keys"), errors.ErrInvalidLuksKeyFile)
	}
	return
}

func (l Luks) validateLabel() error {
	if util.NilOrEmpty(l.Label) {
		return nil
	}

	if len(*l.Label) > 47 {
		// LUKS2_LABEL_L has a maximum length of 48 (including the null terminator)
		// https://gitlab.com/cryptsetup/cryptsetup/-/blob/1633f030e89ad2f11ae649ba9600997a41abd3fc/lib/luks2/luks2.h#L86
		return errors.ErrLuksLabelTooLong
	}

	return nil
}
