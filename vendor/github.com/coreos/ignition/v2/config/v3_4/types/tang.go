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
	"encoding/json"
	"net/url"

	"github.com/coreos/ignition/v2/config/shared/errors"
	"github.com/coreos/ignition/v2/config/util"

	"github.com/coreos/vcontext/path"
	"github.com/coreos/vcontext/report"
)

func (t Tang) Key() string {
	return t.URL
}

func (t Tang) Validate(c path.ContextPath) (r report.Report) {
	r.AddOnError(c.Append("url"), validateTangURL(t.URL))
	if util.NilOrEmpty(t.Thumbprint) {
		r.AddOnError(c.Append("thumbprint"), errors.ErrTangThumbprintRequired)
	}
	r.AddOnError(c.Append("advertisement"), validateTangAdvertisement(t.Advertisement))
	return
}

func validateTangURL(s string) error {
	u, err := url.Parse(s)
	if err != nil {
		return errors.ErrInvalidUrl
	}

	switch u.Scheme {
	case "http", "https":
		return nil
	default:
		return errors.ErrInvalidScheme
	}
}

func validateTangAdvertisement(s *string) error {
	if util.NotEmpty(s) {
		var adv any
		err := json.Unmarshal([]byte(*s), &adv)
		if err != nil {
			return errors.ErrInvalidTangAdvertisement
		}
	}

	return nil
}
