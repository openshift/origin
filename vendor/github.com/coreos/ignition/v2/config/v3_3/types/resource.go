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
	"github.com/coreos/ignition/v2/config/util"

	"github.com/coreos/vcontext/path"
	"github.com/coreos/vcontext/report"
)

func (res Resource) Key() string {
	if res.Source == nil {
		return ""
	}
	return *res.Source
}

func (res Resource) Validate(c path.ContextPath) (r report.Report) {
	r.AddOnError(c.Append("compression"), res.validateCompression())
	r.AddOnError(c.Append("verification", "hash"), res.validateVerification())
	r.AddOnError(c.Append("source"), validateURLNilOK(res.Source))
	r.AddOnError(c.Append("httpHeaders"), res.validateSchemeForHTTPHeaders())
	return
}

func (res Resource) validateCompression() error {
	if res.Compression != nil {
		switch *res.Compression {
		case "", "gzip":
		default:
			return errors.ErrCompressionInvalid
		}
	}
	return nil
}

func (res Resource) validateVerification() error {
	if res.Verification.Hash != nil && res.Source == nil {
		return errors.ErrVerificationAndNilSource
	}
	return nil
}

func (res Resource) validateSchemeForHTTPHeaders() error {
	if len(res.HTTPHeaders) < 1 {
		return nil
	}

	if util.NilOrEmpty(res.Source) {
		return errors.ErrInvalidUrl
	}

	u, err := url.Parse(*res.Source)
	if err != nil {
		return errors.ErrInvalidUrl
	}

	switch u.Scheme {
	case "http", "https":
		return nil
	default:
		return errors.ErrUnsupportedSchemeForHTTPHeaders
	}
}

// Ensure that the Source is specified and valid.  This is not called by
// Resource.Validate() because some structs that embed Resource don't
// require Source to be specified.  Containing structs that require Source
// should call this function from their Validate().
func (res Resource) validateRequiredSource() error {
	if util.NilOrEmpty(res.Source) {
		return errors.ErrSourceRequired
	}
	return validateURL(*res.Source)
}
