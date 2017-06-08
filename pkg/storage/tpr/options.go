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

package tpr

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	restclient "k8s.io/client-go/rest"
)

// Options is the set of options to create a new TPR storage interface
type Options struct {
	HasNamespace     bool
	RESTOptions      generic.RESTOptions
	DefaultNamespace string
	RESTClient       restclient.Interface
	SingularKind     Kind
	NewSingularFunc  func(string, string) runtime.Object
	ListKind         Kind
	NewListFunc      func() runtime.Object
	CheckObjectFunc  func(runtime.Object) error
	DestroyFunc      func()
	Keyer            Keyer
}
