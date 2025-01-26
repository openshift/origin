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
	"net/http"

	"github.com/coreos/ignition/v2/config/shared/errors"
	"github.com/coreos/vcontext/path"
	"github.com/coreos/vcontext/report"
)

// Parse generates standard net/http headers from the data in HTTPHeaders
func (hs HTTPHeaders) Parse() (http.Header, error) {
	headers := http.Header{}
	for _, header := range hs {
		if header.Name == "" {
			return nil, errors.ErrEmptyHTTPHeaderName
		}
		if header.Value == nil || string(*header.Value) == "" {
			return nil, errors.ErrInvalidHTTPHeader
		}
		headers.Add(header.Name, string(*header.Value))
	}
	return headers, nil
}

func (h HTTPHeader) Validate(c path.ContextPath) (r report.Report) {
	r.AddOnError(c.Append("name"), h.validateName())
	r.AddOnError(c.Append("value"), h.validateValue())
	return
}

func (h HTTPHeader) validateName() error {
	if h.Name == "" {
		return errors.ErrEmptyHTTPHeaderName
	}
	return nil
}

func (h HTTPHeader) validateValue() error {
	if h.Value == nil {
		return nil
	}
	if string(*h.Value) == "" {
		return errors.ErrInvalidHTTPHeader
	}
	return nil
}

func (h HTTPHeader) Key() string {
	return h.Name
}
