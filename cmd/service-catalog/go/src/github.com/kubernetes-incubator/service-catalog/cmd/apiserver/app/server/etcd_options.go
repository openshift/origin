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

package server

import (
	"github.com/spf13/pflag"
	genericserveroptions "k8s.io/apiserver/pkg/server/options"
	"k8s.io/apiserver/pkg/storage/storagebackend"
)

// EtcdOptions contains the complete configuration for an API server that
// communicates with an etcd. This struct is exported so that it can be used by integration
// tests
type EtcdOptions struct {
	// storage with etcd
	*genericserveroptions.EtcdOptions
}

const (
	// DefaultEtcdPathPrefix is the default prefix that is prepended to all
	// resource paths in etcd.  It is intended to allow an operator to
	// differentiate the storage of different API servers from one another in
	// a single etcd.
	DefaultEtcdPathPrefix = "/registry"
)

// NewEtcdOptions creates a new, empty, EtcdOptions instance
func NewEtcdOptions() *EtcdOptions {
	return &EtcdOptions{
		EtcdOptions: genericserveroptions.NewEtcdOptions(storagebackend.NewDefaultConfig(DefaultEtcdPathPrefix, nil)),
	}
}

func (s *EtcdOptions) addFlags(flags *pflag.FlagSet) {
	s.EtcdOptions.AddFlags(flags)
}
