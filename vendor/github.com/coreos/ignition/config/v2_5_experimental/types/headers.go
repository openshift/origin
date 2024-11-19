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

	"github.com/coreos/ignition/config/shared/errors"
	"github.com/coreos/ignition/config/validate/report"
)

func (h HTTPHeaders) Validate() report.Report {
	r := report.Report{}
	found := make(map[string]struct{})
	for _, header := range h {
		// Header name can't be empty
		if header.Name == "" {
			r.Add(report.Entry{
				Message: errors.ErrEmptyHTTPHeaderName.Error(),
				Kind:    report.EntryError,
			})
			continue
		}
		// Header names must be unique
		if _, ok := found[header.Name]; ok {
			r.Add(report.Entry{
				Message: fmt.Sprintf("Found duplicate HTTP header: %q", header.Name),
				Kind:    report.EntryError,
			})
			continue
		}
		found[header.Name] = struct{}{}
	}
	return r
}
