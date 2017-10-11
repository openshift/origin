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

package meta

import (
	"testing"
	"time"

	sc "github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestDeletionTimestampExists(t *testing.T) {
	obj := &sc.ServiceInstance{
		ObjectMeta: metav1.ObjectMeta{},
	}
	exists, err := DeletionTimestampExists(obj)
	if err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Fatalf("deletion timestamp reported as exists when it didn't")
	}
	tme := metav1.NewTime(time.Now())
	obj.DeletionTimestamp = &tme
	exists, err = DeletionTimestampExists(obj)
	if err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Fatal("deletion timestamp reported as missing when it isn't")
	}
}

func TestRoundTripDeletionTimestamp(t *testing.T) {
	t1 := metav1.NewTime(time.Now())
	t2 := metav1.NewTime(time.Now().Add(1 * time.Hour))
	obj := &sc.ServiceInstance{
		ObjectMeta: metav1.ObjectMeta{
			DeletionTimestamp: &t1,
		},
	}
	t1Ret, err := GetDeletionTimestamp(obj)
	if err != nil {
		t.Fatalf("error getting 1st deletion timestamp (%s)", err)
	}
	if !t1.Equal(t1Ret) {
		t.Fatalf("expected deletion timestamp %s, got %s", t1, *t1Ret)
	}
	if err := SetDeletionTimestamp(obj, t2.Time); err != nil {
		t.Fatalf("error setting deletion timestamp (%s)", err)
	}
	t2Ret, err := GetDeletionTimestamp(obj)
	if err != nil {
		t.Fatalf("error getting 2nd deletion timestamp (%s)", err)
	}
	if !t2.Equal(t2Ret) {
		t.Fatalf("expected deletion timestamp %s, got %s", t2, *t2Ret)
	}
}
