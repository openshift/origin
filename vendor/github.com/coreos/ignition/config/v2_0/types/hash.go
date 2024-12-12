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
	"encoding/json"
	"strings"

	"github.com/coreos/ignition/config/shared/errors"
	"github.com/coreos/ignition/config/validate/report"
)

type Hash struct {
	Function string
	Sum      string
}

func (h *Hash) UnmarshalJSON(data []byte) error {
	var th string
	if err := json.Unmarshal(data, &th); err != nil {
		return err
	}

	parts := strings.SplitN(th, "-", 2)
	if len(parts) != 2 {
		return errors.ErrHashMalformed
	}

	h.Function = parts[0]
	h.Sum = parts[1]

	return nil
}

func (h Hash) MarshalJSON() ([]byte, error) {
	return []byte(`"` + h.Function + "-" + h.Sum + `"`), nil
}

func (h Hash) String() string {
	bytes, _ := h.MarshalJSON()
	return string(bytes)
}

func (h Hash) Validate() report.Report {
	var hash crypto.Hash
	switch h.Function {
	case "sha512":
		hash = crypto.SHA512
	default:
		return report.ReportFromError(errors.ErrHashUnrecognized, report.EntryError)
	}

	if len(h.Sum) != hex.EncodedLen(hash.Size()) {
		return report.ReportFromError(errors.ErrHashWrongSize, report.EntryError)
	}

	return report.Report{}
}
