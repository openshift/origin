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

func (f File) Validate(c path.ContextPath) (r report.Report) {
	r.Merge(f.Node.Validate(c))
	r.AddOnError(c.Append("mode"), validateMode(f.Mode))
	r.AddOnWarn(c.Append("mode"), validateModeSpecialBits(f.Mode))
	r.AddOnError(c.Append("overwrite"), f.validateOverwrite())
	return
}

func (f File) validateOverwrite() error {
	if util.IsTrue(f.Overwrite) && f.Contents.Source == nil {
		return errors.ErrOverwriteAndNilSource
	}
	return nil
}

func (f FileEmbedded1) IgnoreDuplicates() map[string]struct{} {
	return map[string]struct{}{
		"Append": {},
	}
}
