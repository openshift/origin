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

func (c Clevis) IsPresent() bool {
	return util.NotEmpty(c.Custom.Pin) ||
		len(c.Tang) > 0 ||
		util.IsTrue(c.Tpm2) ||
		c.Threshold != nil && *c.Threshold != 0
}

func (cu ClevisCustom) Validate(c path.ContextPath) (r report.Report) {
	if util.NilOrEmpty(cu.Pin) && util.NilOrEmpty(cu.Config) && !util.IsTrue(cu.NeedsNetwork) {
		return
	}
	if util.NotEmpty(cu.Pin) {
		switch *cu.Pin {
		case "tpm2", "tang", "sss":
		default:
			r.AddOnError(c.Append("pin"), errors.ErrUnknownClevisPin)
		}
	} else {
		r.AddOnError(c.Append("pin"), errors.ErrClevisPinRequired)
	}
	if util.NilOrEmpty(cu.Config) {
		r.AddOnError(c.Append("config"), errors.ErrClevisConfigRequired)
	}
	return
}
