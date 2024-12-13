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

func (f File) ValidateMode() report.Report {
	r := report.Report{}
	if err := validateMode(f.Mode); err != nil {
		r.Add(report.Entry{
			Message: err.Error(),
			Kind:    report.EntryError,
		})
	}
	return r
}

func (fc FileContents) ValidateCompression() report.Report {
	r := report.Report{}
	switch fc.Compression {
	case "", "gzip":
	default:
		r.Add(report.Entry{
			Message: errors.ErrCompressionInvalid.Error(),
			Kind:    report.EntryError,
		})
	}
	return r
}

func (fc FileContents) ValidateSource() report.Report {
	r := report.Report{}
	err := validateURL(fc.Source)
	if err != nil {
		r.Add(report.Entry{
			Message: err.Error(),
			Kind:    report.EntryError,
		})
	}
	return r
}
