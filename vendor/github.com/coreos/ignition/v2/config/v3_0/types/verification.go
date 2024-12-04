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
	"crypto"
	"encoding/hex"
	"strings"

	"github.com/coreos/ignition/v2/config/shared/errors"

	"github.com/coreos/vcontext/path"
	"github.com/coreos/vcontext/report"
)

// HashParts will return the sum and function (in that order) of the hash stored
// in this Verification, or an error if there is an issue during parsing.
func (v Verification) HashParts() (string, string, error) {
	if v.Hash == nil {
		// The hash can be nil
		return "", "", nil
	}
	parts := strings.SplitN(*v.Hash, "-", 2)
	if len(parts) != 2 {
		return "", "", errors.ErrHashMalformed
	}

	return parts[0], parts[1], nil
}

func (v Verification) Validate(c path.ContextPath) (r report.Report) {
	c = c.Append("hash")
	if v.Hash == nil {
		// The hash can be nil
		return
	}

	function, sum, err := v.HashParts()
	if err != nil {
		r.AddOnError(c, err)
		return
	}
	var hash crypto.Hash
	switch function {
	case "sha512":
		hash = crypto.SHA512
	default:
		r.AddOnError(c, errors.ErrHashUnrecognized)
		return
	}

	if len(sum) != hex.EncodedLen(hash.Size()) {
		r.AddOnError(c, errors.ErrHashWrongSize)
	}

	return
}
