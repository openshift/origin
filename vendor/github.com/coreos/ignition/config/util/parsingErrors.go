// Copyright 2018 CoreOS, Inc.
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

package util

import (
	"bytes"
	"errors"

	configErrors "github.com/coreos/ignition/config/shared/errors"
	"github.com/coreos/ignition/config/v2_5_experimental/types"
	"github.com/coreos/ignition/config/validate/report"

	json "github.com/ajeddeloh/go-json"
	"go4.org/errorutil"
)

var (
	ErrValidConfig = errors.New("HandleParseErrors called with a valid config")
)

// HandleParseErrors will attempt to unmarshal an invalid rawConfig into the
// latest config struct, so as to generate a report.Report from the errors. It
// will always return an error. This is called after config/v* parse functions
// chain has failed to parse a config.
func HandleParseErrors(rawConfig []byte) (report.Report, error) {
	config := types.Config{}
	err := json.Unmarshal(rawConfig, &config)
	if err == nil {
		return report.Report{}, ErrValidConfig
	}

	// Handle json syntax and type errors first, since they are fatal but have offset info
	if serr, ok := err.(*json.SyntaxError); ok {
		line, col, highlight := errorutil.HighlightBytePosition(bytes.NewReader(rawConfig), serr.Offset)
		return report.Report{
				Entries: []report.Entry{{
					Kind:      report.EntryError,
					Message:   serr.Error(),
					Line:      line,
					Column:    col,
					Highlight: highlight,
				}},
			},
			configErrors.ErrInvalid
	}

	if terr, ok := err.(*json.UnmarshalTypeError); ok {
		line, col, highlight := errorutil.HighlightBytePosition(bytes.NewReader(rawConfig), terr.Offset)
		return report.Report{
				Entries: []report.Entry{{
					Kind:      report.EntryError,
					Message:   terr.Error(),
					Line:      line,
					Column:    col,
					Highlight: highlight,
				}},
			},
			configErrors.ErrInvalid
	}

	return report.ReportFromError(err, report.EntryError), err
}
