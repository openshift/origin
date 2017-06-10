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
	"testing"

	"github.com/kubernetes-incubator/service-catalog/pkg/storage/etcd"
	"github.com/kubernetes-incubator/service-catalog/pkg/storage/tpr"
)

func TestNewOptions(t *testing.T) {
	origStorageType := StorageTypeEtcd
	opts := NewOptions(etcd.Options{}, tpr.Options{}, origStorageType)
	retStorageType, err := opts.StorageType()
	if err != nil {
		t.Fatalf("getting storage type (%s)", err)
	}
	if origStorageType != retStorageType {
		t.Fatalf("expected storage type %s, got %s",
			origStorageType,
			retStorageType,
		)
	}

}
