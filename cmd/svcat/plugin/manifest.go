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

package plugin

import (
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var reservedFlags = map[string]struct{}{
	"alsologtostderr":       {},
	"as":                    {},
	"as-group":              {},
	"cache-dir":             {},
	"certificate-authority": {},
	"client-certificate":    {},
	"client-key":            {},
	"cluster":               {},
	"context":               {},
	"help":                  {},
	"insecure-skip-tls-verify": {},
	"kubeconfig":               {},
	"kube-context":             {},
	"log-backtrace-at":         {},
	"log-dir":                  {},
	"log-flush-frequency":      {},
	"logtostderr":              {},
	"match-server-version":     {},
	"n":               {},
	"namespace":       {},
	"password":        {},
	"request-timeout": {},
	"s":               {},
	"server":          {},
	"stderrthreshold": {},
	"token":           {},
	"user":            {},
	"username":        {},
	"v":               {},
	"vmodule":         {},
}

// Manifest is the root structure of the kubectl plugin manifest.
type Manifest struct {
	Plugin `yaml:",inline"`
}

// Plugin describes a command exposed by the plugin.
type Plugin struct {
	// Name of the command for the help text. Required.
	Name string `yaml:"name"`

	// ShortDesc is a one-line description of the command. Required.
	ShortDesc string `yaml:"shortDesc"`

	// LongDesc is the optional full description of the command.
	LongDesc string `yaml:"longDesc,omitempty"`

	// Example contains optional examples of how to use the command.
	Example string `yaml:"example,omitempty"`

	// Command that the kubectl plugin runner should execute. Required.
	Command string `yaml:"command"`

	// Flags supported by the command.
	Flags []Flag `yaml:"flags,omitempty"`

	// Tree of child commands.
	Tree []Plugin `yaml:"tree,omitempty"`
}

// Flag describes a flag exposed by a plugin command.
type Flag struct {
	// Name of the flag. Required.
	Name string `yaml:"name"`

	// Shorthand flag, must be a single character.
	Shorthand string `yaml:"shorthand,omitempty"`

	// Desc of the flag for the help text. Required.
	Desc string `yaml:"desc"`

	// DefValue is the default value to use when the flag is not specified.
	DefValue string `yaml:"defValue,omitempty"`
}

// Load a cli command into the plugin manifest structure.
func (m *Manifest) Load(rootCmd *cobra.Command) {
	m.Plugin = m.convertToPlugin(rootCmd)
}

func (m *Manifest) convertToPlugin(cmd *cobra.Command) Plugin {
	p := Plugin{}

	p.Name = strings.Split(cmd.Use, " ")[0]
	p.ShortDesc = cmd.Short
	if p.ShortDesc == "" {
		p.ShortDesc = " " // The plugin won't validate if empty
	}
	p.LongDesc = cmd.Long
	p.Command = "./" + cmd.CommandPath()

	p.Flags = []Flag{}
	cmd.Flags().VisitAll(func(flag *pflag.Flag) {
		result := m.convertToFlag(flag)
		if result != nil {
			p.Flags = append(p.Flags, *result)
		}
	})

	p.Tree = make([]Plugin, len(cmd.Commands()))
	for i, subCmd := range cmd.Commands() {
		p.Tree[i] = m.convertToPlugin(subCmd)
	}
	return p
}

func (m *Manifest) convertToFlag(src *pflag.Flag) *Flag {
	if _, reserved := reservedFlags[src.Name]; reserved {
		return nil
	}

	dest := &Flag{
		Name: src.Name,
		Desc: src.Usage,
	}

	if _, reserved := reservedFlags[src.Shorthand]; !reserved {
		dest.Shorthand = src.Shorthand
	}

	return dest
}
