/*
Copyright 2017 Google Inc. All Rights Reserved.
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

// Package config provides environment specific configuration elements to unused_deps
package config

var (
	// DefaultBuildTool should be used to build and extract deps
	DefaultBuildTool = "bazel"
	// BuildToolHelp message for above
	BuildToolHelp = "the build executable (like bazel)"
	// ExtraActionFileNameHelp help for extra actions
	ExtraActionFileNameHelp = "When specified, just prints suspected unused deps."
	// DefaultBinDir to look for outputs
	DefaultBinDir = "bazel-bin"
	// DefaultExtraBuildFlags is internal-only
	DefaultExtraBuildFlags = []string{}
)
