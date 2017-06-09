/*
Copyright 2016 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package pkg

import (
	"fmt"
	"os"

	"github.com/spf13/pflag"
)

// VERSION is the version string for built artifacts. It's set by the build system, and should
// not be changed in this codebase
var VERSION = "UNKNOWN"

// Version decides whether we should print the version and leave.
type Version struct {
	print bool
}

// VersionFlag creates the version flag for your application.
func VersionFlag(fs *pflag.FlagSet) *Version {
	v := Version{}
	fs.BoolVar(&v.print, "version", false, "Print version information and quit")
	return &v
}

// PrintAndExitIfRequested will print the version if requested, and exit.
func (v Version) PrintAndExitIfRequested() {
	if v.print {
		fmt.Println(VERSION)
		os.Exit(0)
	}
}
