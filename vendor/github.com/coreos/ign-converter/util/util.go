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

package util

import (
	"errors"
	"fmt"
	"strings"
)

// Error definitions

// UsesNetworkdError is the error for including networkd configs
var UsesNetworkdError = errors.New("config includes deprecated networkd section - use Files instead")

// NoFilesystemError type for when a filesystem is referenced in a config but there's no mapping to where
// it should be mounted (i.e. `path` in v3+ configs)
type NoFilesystemError string

func (e NoFilesystemError) Error() string {
	return fmt.Sprintf("Config defined filesystem %q but no mapping was defined."+
		"Please specify a path to be used as the filesystem mountpoint.", string(e))
}

// DuplicateInodeError is for when files, directories, or links both specify the same path
type DuplicateInodeError struct {
	Old string // first occurance of the path
	New string // second occurance of the path
}

func (e DuplicateInodeError) Error() string {
	return fmt.Sprintf("Config has conflicting inodes: %q and %q.  All files, directories and links must specify a unique `path`.", e.Old, e.New)
}

// UsesOwnLinkError is for when files, directories, or links use symlinks defined in the config
// in their own path. This is disallowed in v3+ configs.
type UsesOwnLinkError struct {
	LinkPath string
	Name     string
}

func (e UsesOwnLinkError) Error() string {
	return fmt.Sprintf("%s uses link defined in config %q. Please use a link not defined in Storage:Links", e.Name, e.LinkPath)
}

// DuplicateUnitError is for when a unit name is used twice
type DuplicateUnitError struct {
	Name string
}

func (e DuplicateUnitError) Error() string {
	return fmt.Sprintf("Config has duplicate unit name %q.  All units must specify a unique `name`.", e.Name)
}

// DuplicateDropinError is for when a unit has multiple dropins with the same name
type DuplicateDropinError struct {
	Unit string
	Name string
}

func (e DuplicateDropinError) Error() string {
	return fmt.Sprintf("Config has duplicate dropin name %q in unit %q.  All dropins must specify a unique `name`.", e.Name, e.Unit)
}

func CheckPathUsesLink(links []string, path string) string {
	for _, l := range links {
		if strings.HasPrefix(path, l) && path != l {
			return l
		}
	}
	return ""
}

func StrP(in string) *string {
	if in == "" {
		return nil
	}
	return &in
}

func StrPStrict(in string) *string {
	return &in
}

func BoolP(in bool) *bool {
	if !in {
		return nil
	}
	return &in
}

func BoolPStrict(in bool) *bool {
	return &in
}

func IntP(in int) *int {
	if in == 0 {
		return nil
	}
	return &in
}

func StrV(in *string) string {
	if in == nil {
		return ""
	}
	return *in
}

func BoolV(in *bool) bool {
	if in == nil {
		return false
	}
	return *in
}

func IntV(in *int) int {
	if in == nil {
		return 0
	}
	return *in
}
