/*
Copyright 2018 The Kubernetes Authors.

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

// Package plugin helps apply kubectl plugin-specific cli configuration.
// See https://kubernetes.io/docs/tasks/extend-kubectl/kubectl-plugins/#accessing-runtime-attributes.
package plugin

import (
	"os"

	"github.com/spf13/viper"
)

const (
	// Name of the plugin binary
	Name = "svcat"

	// EnvPluginCaller contains the path to the parent caller
	// Example: /usr/bin/kubectl.
	EnvPluginCaller = "KUBECTL_PLUGINS_CALLER"

	// EnvPluginLocalFlagPrefix contains the prefix applied to any command flags
	// Example: KUBECTL_PLUGINS_LOCAL_FLAG_FOO
	EnvPluginLocalFlagPrefix = "KUBECTL_PLUGINS_LOCAL_FLAG"

	// EnvPluginNamespace is the final namespace, after taking into account all the
	// kubectl flags and environment variables.
	EnvPluginNamespace = "KUBECTL_PLUGINS_CURRENT_NAMESPACE"

	// EnvPluginPath overrides where plugins should be installed.
	EnvPluginPath = "KUBECTL_PLUGINS_PATH"
)

// IsPlugin determines if the cli is running as a kubectl plugin
func IsPlugin() bool {
	_, ok := os.LookupEnv(EnvPluginCaller)
	return ok
}

// BindEnvironmentVariables binds plugin-specific environment variables to command flags.
func BindEnvironmentVariables(vip *viper.Viper) {
	// KUBECTL_PLUGINS_CURRENT_NAMESPACE provides the final namespace
	// computed by kubectl.
	vip.BindEnv("namespace", EnvPluginNamespace)

	// kubectl intercepts all flags passed to a plugin, and replaces them
	// with prefixed environment variables
	// --foo becomes KUBECTL_PLUGINS_LOCAL_FLAG_FOO
	vip.SetEnvPrefix(EnvPluginLocalFlagPrefix)
}
