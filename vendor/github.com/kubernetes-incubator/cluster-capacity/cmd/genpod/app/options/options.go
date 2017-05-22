/*
Copyright 2017 The Kubernetes Authors.

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

package options

import (
	"github.com/spf13/pflag"
)

type GenPodOptions struct {
	Kubeconfig string
	Verbose    bool
	Namespace  string
	Format     string
}

func NewGenPodOptions() *GenPodOptions {
	return &GenPodOptions{Namespace: "default"}
}

func (s *GenPodOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&s.Kubeconfig, "kubeconfig", s.Kubeconfig, "Path to kubeconfig file with authorization and master location information.")
	fs.BoolVar(&s.Verbose, "verbose", s.Verbose, "Verbose mode")
	fs.StringVar(&s.Namespace, "namespace", s.Namespace, "Cluster namespace")
	fs.StringVar(&s.Format, "output", s.Format, "Output format")
}
