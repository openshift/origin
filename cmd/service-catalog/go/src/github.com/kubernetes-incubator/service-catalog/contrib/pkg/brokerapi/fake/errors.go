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

package fake

import (
	"errors"
)

var (
	// ErrInstanceNotFound is returned by the fake client when an instance was expected in its
	// internal storage but it wasn't found
	ErrInstanceNotFound = errors.New("instance not found")
	// ErrInstanceAlreadyExists is returned by the fake client when an instance was found in
	// internal storage but it wasn't expected
	ErrInstanceAlreadyExists = errors.New("instance already exists")
	// ErrBindingNotFound is returned by the fake client when a binding was expected in internal
	// storage but it wasn't found
	ErrBindingNotFound = errors.New("binding not found")
	// ErrBindingAlreadyExists is returned by the fake client when a binding was found in internal
	// storage but it wasn't expected
	ErrBindingAlreadyExists = errors.New("binding already exists")
)
