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
	"errors"
	"fmt"
	"strings"

	"k8s.io/apiserver/pkg/endpoints/request"
)

var (
	errEmptyKey = errors.New("empty key")
)

type errInvalidKey struct {
	k string
}

func (e errInvalidKey) Error() string {
	return fmt.Sprintf("invalid key '%s'", e.k)
}

// Keyer is a containing struct for
type Keyer struct {
	DefaultNamespace string
	ResourceName     string
	Separator        string
}

// KeyRoot is a (k8s.io/kubernetes/pkg/registry/generic/registry).Store compatible function to
// get the key root to be passed to a third party resource based storage Interface.
//
// It is meant to be passed to a Store's KeyRootFunc field, so that a TPR based storage interface
// can parse the namespace and name from fields that it is later given.
//
// The returned string will never be empty is k.DefaultNamespace is not empty
func (k Keyer) KeyRoot(ctx request.Context) string {
	ns, ok := request.NamespaceFrom(ctx)
	if ok && len(ns) > 0 {
		return ns
	}
	return k.DefaultNamespace
}

// Key is a (k8s.io/kubernetes/pkg/registry/generic/registry).Store compatible function to
// get the key to be passed to a third party resource based storage Interface.
//
// It is meant to be passed to a Store's KeyRoot field, so that a TPR based storage interface
// can parse the namespace and name from fields that it is later given
func (k Keyer) Key(ctx request.Context, name string) (string, error) {
	root := k.KeyRoot(ctx)
	return strings.Join([]string{root, name}, k.Separator), nil
}

// NamespaceAndNameFromKey returns the namespace (or an empty string if there wasn't one) and
// name for a given etcd-style key. This function is intended to be used in a TPR based
// storage.Interface that uses k.KeyRoot and k.Key to construct its etcd keys.
//
// The first return value is the namespace, and may be empty if the key represents a
// namespace-less resource. The second return value is the name and will not be empty if the
// error is nil. The error will be non-nil if the key was malformed, in which case all other
// return values will be empty strings.
func (k Keyer) NamespaceAndNameFromKey(key string) (string, string, error) {
	spl := strings.Split(key, k.Separator)
	splLen := len(spl)
	if splLen == 1 {
		// single slice entry is name-less, so return an empty name
		return spl[0], "", nil
	} else if splLen == 2 {
		// two slice entries has a namespace
		return spl[0], spl[1], nil
	}

	return "", "", errInvalidKey{k: key}
}
