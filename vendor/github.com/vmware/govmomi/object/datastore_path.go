// © Broadcom. All Rights Reserved.
// The term “Broadcom” refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: Apache-2.0

package object

import (
	"fmt"
	"path"
	"strings"
)

// DatastorePath contains the components of a datastore path.
type DatastorePath struct {
	Datastore string
	Path      string
}

// FromString parses a datastore path.
// Returns true if the path could be parsed, false otherwise.
func (p *DatastorePath) FromString(s string) bool {
	if s == "" {
		return false
	}

	s = strings.TrimSpace(s)

	if !strings.HasPrefix(s, "[") {
		return false
	}

	s = s[1:]

	ix := strings.Index(s, "]")
	if ix < 0 {
		return false
	}

	p.Datastore = s[:ix]
	p.Path = strings.TrimSpace(s[ix+1:])

	return true
}

// String formats a datastore path.
func (p *DatastorePath) String() string {
	s := fmt.Sprintf("[%s]", p.Datastore)

	if p.Path == "" {
		return s
	}

	return strings.Join([]string{s, p.Path}, " ")
}

// IsVMDK returns true if Path has a ".vmdk" extension
func (p *DatastorePath) IsVMDK() bool {
	return path.Ext(p.Path) == ".vmdk"
}
