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

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/storage"
)

var (
	errResourceVersionCannotBeSet = errors.New("resource version cannot be set on objects")
)

type errCouldntGetResourceVersion struct {
	err error
}

func (e errCouldntGetResourceVersion) Error() string {
	return fmt.Sprintf("couldn't get resource version (%s)", e.err)
}

type objState struct {
	obj  runtime.Object
	meta *storage.ResponseMeta
	rev  int64
	data []byte
}

func (t *store) getStateFromObject(obj runtime.Object) (*objState, error) {
	versioner := t.Versioner()
	state := &objState{
		obj:  obj,
		meta: &storage.ResponseMeta{},
	}

	rv, err := versioner.ObjectResourceVersion(obj)
	if err != nil {
		return nil, errCouldntGetResourceVersion{err: err}
	}
	state.rev = int64(rv)
	state.meta.ResourceVersion = uint64(state.rev)

	data, err := runtime.Encode(t.codec, obj)
	if err != nil {
		return nil, err
	}
	state.data = data
	return state, nil
}
