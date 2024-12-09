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
	"path"

	"github.com/coreos/ignition/v2/config/shared/errors"
	"github.com/coreos/ignition/v2/config/util"

	vpath "github.com/coreos/vcontext/path"
	"github.com/coreos/vcontext/report"
)

func (n Node) Key() string {
	return n.Path
}

func (n Node) Validate(c vpath.ContextPath) (r report.Report) {
	r.AddOnError(c.Append("path"), validatePath(n.Path))
	return
}

func (n Node) Depth() int {
	count := 0
	for p := path.Clean(string(n.Path)); p != "/"; count++ {
		p = path.Dir(p)
	}
	return count
}

func validateIDorName(id *int, name *string) error {
	if id != nil && util.NotEmpty(name) {
		return errors.ErrBothIDAndNameSet
	}
	return nil
}

func (nu NodeUser) Validate(c vpath.ContextPath) (r report.Report) {
	r.AddOnError(c, validateIDorName(nu.ID, nu.Name))
	return
}

func (ng NodeGroup) Validate(c vpath.ContextPath) (r report.Report) {
	r.AddOnError(c, validateIDorName(ng.ID, ng.Name))
	return
}
