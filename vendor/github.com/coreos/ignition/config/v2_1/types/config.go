// Copyright 2015 CoreOS, Inc.
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

	"github.com/coreos/go-semver/semver"

	"github.com/coreos/ignition/config/validate/report"
)

var (
	MaxVersion = semver.Version{
		Major: 2,
		Minor: 1,
	}
)

func (c Config) Validate() report.Report {
	r := report.Report{}
	rules := []rule{
		checkFilesFilesystems,
		checkDuplicateFilesystems,
	}

	for _, rule := range rules {
		rule(c, &r)
	}
	return r
}

type rule func(cfg Config, report *report.Report)

func checkNodeFilesystems(node Node, filesystems map[string]struct{}, nodeType string) report.Report {
	r := report.Report{}
	if node.Filesystem == "" {
		// Filesystem was not specified. This is an error, but its handled in types.File's Validate, not here
		return r
	}
	_, ok := filesystems[node.Filesystem]
	if !ok {
		r.Add(report.Entry{
			Kind: report.EntryWarning,
			Message: fmt.Sprintf("%v %q references nonexistent filesystem %q. (This is ok if it is defined in a referenced config)",
				nodeType, node.Path, node.Filesystem),
		})
	}
	return r
}

func checkFilesFilesystems(cfg Config, r *report.Report) {
	filesystems := map[string]struct{}{"root": {}}
	for _, filesystem := range cfg.Storage.Filesystems {
		filesystems[filesystem.Name] = struct{}{}
	}
	for _, file := range cfg.Storage.Files {
		r.Merge(checkNodeFilesystems(file.Node, filesystems, "File"))
	}
	for _, link := range cfg.Storage.Links {
		r.Merge(checkNodeFilesystems(link.Node, filesystems, "Link"))
	}
	for _, dir := range cfg.Storage.Directories {
		r.Merge(checkNodeFilesystems(dir.Node, filesystems, "Directory"))
	}
}

func checkDuplicateFilesystems(cfg Config, r *report.Report) {
	filesystems := map[string]struct{}{"root": {}}
	for _, filesystem := range cfg.Storage.Filesystems {
		if _, ok := filesystems[filesystem.Name]; ok {
			r.Add(report.Entry{
				Kind:    report.EntryWarning,
				Message: fmt.Sprintf("Filesystem %q shadows exising filesystem definition", filesystem.Name),
			})
		}
		filesystems[filesystem.Name] = struct{}{}
	}
}
