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
	"encoding/json"

	"github.com/coreos/ignition/v2/config/shared/errors"

	"github.com/coreos/vcontext/path"
	"github.com/coreos/vcontext/report"
	"github.com/coreos/vcontext/tree"
)

// HandleParseErrors will attempt to unmarshal an invalid rawConfig into "to".
// If it fails to unmarsh it will generate a report.Report from the errors.
func HandleParseErrors(rawConfig []byte, to interface{}) (report.Report, error) {
	r := report.Report{}
	err := json.Unmarshal(rawConfig, to)
	if err == nil {
		return report.Report{}, nil
	}

	var node tree.Leaf
	switch t := err.(type) {
	case *json.SyntaxError:
		node.Marker = tree.MarkerFromIndices(t.Offset, -1)
	case *json.UnmarshalTypeError:
		node.Marker = tree.MarkerFromIndices(t.Offset, -1)
	}
	tree.FixLineColumn(node, rawConfig)
	r.AddOnError(path.ContextPath{Tag: "json"}, err)
	r.Correlate(node)

	return r, errors.ErrInvalid
}
