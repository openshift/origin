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
	"net/url"

	"github.com/coreos/ignition/v2/config/shared/errors"

	"github.com/coreos/vcontext/path"
	"github.com/coreos/vcontext/report"
)

func (p Proxy) Validate(c path.ContextPath) (r report.Report) {
	validateProxyURL(p.HTTPProxy, c.Append("httpProxy"), &r, true)
	validateProxyURL(p.HTTPSProxy, c.Append("httpsProxy"), &r, false)
	return
}

func validateProxyURL(s *string, p path.ContextPath, r *report.Report, httpOk bool) {
	if s == nil {
		return
	}
	u, err := url.Parse(*s)
	if err != nil {
		r.AddOnError(p, errors.ErrInvalidUrl)
		return
	}

	if u.Scheme != "https" && u.Scheme != "http" {
		r.AddOnError(p, errors.ErrInvalidProxy)
		return
	}
	if u.Scheme == "http" && !httpOk {
		r.AddOnWarn(p, errors.ErrInsecureProxy)
	}
}
