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
	"errors"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var (
	// ErrNoDeletionTimestamp is the error returned by GetDeletionTimestamp when there is no
	// deletion timestamp set on the object
	ErrNoDeletionTimestamp = errors.New("no deletion timestamp set")
)

// DeletionTimestampExists returns true if a deletion timestamp exists on obj, or a non-nil
// error if that couldn't be reliably determined
func DeletionTimestampExists(obj runtime.Object) (bool, error) {
	_, err := GetDeletionTimestamp(obj)
	if err == ErrNoDeletionTimestamp {
		// if GetDeletionTimestamp reported that no deletion timestamp exists, return false
		// and no error
		return false, nil
	}
	if err != nil {
		// otherwise, if GetDeletionTimestamp returned an unknown error, return the error
		return false, err
	}
	return true, nil
}

// GetDeletionTimestamp returns the deletion timestamp on obj, or a non-nil error if there was
// an error getting it or it isn't set. Returns ErrNoDeletionTimestamp if there was none set
func GetDeletionTimestamp(obj runtime.Object) (*metav1.Time, error) {
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return nil, err
	}
	t := accessor.GetDeletionTimestamp()
	if t == nil {
		return nil, ErrNoDeletionTimestamp
	}
	return t, nil
}

// SetDeletionTimestamp sets the deletion timestamp on obj to t
func SetDeletionTimestamp(obj runtime.Object, t time.Time) error {
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return err
	}
	metaTime := metav1.NewTime(t)
	accessor.SetDeletionTimestamp(&metaTime)
	return nil
}
