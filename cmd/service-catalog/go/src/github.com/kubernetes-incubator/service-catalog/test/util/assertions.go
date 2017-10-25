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

// The following assert functions are based on asserts in https://github.com/stretchr/testify.git

package util

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
)

// AssertNoError asserts that a function returned no error (i.e. `nil`).
func AssertNoError(t *testing.T, err error) {
	if err != nil {
		t.Fatalf("Received unexpected error:\n%+v", err)
	}
}

// AssertError asserts that a function returned an error (i.e. not `nil`).
func AssertError(t *testing.T, err error) {
	if err == nil {
		t.Fatal("An error is expected but got nil")
	}
}

// AssertEqualError asserts that a function returned an error (i.e. not `nil`)
// and that it is equal to the provided error.
func AssertEqualError(t *testing.T, theError error, errString string) {
	AssertError(t, theError)
	expected := errString
	actual := theError.Error()
	// don't need to use deep equals here, we know they are both strings
	if expected != actual {
		t.Fatalf("Error message not equal:\n"+
			"expected: %q\n"+
			"actual: %q", expected, actual)
	}
}

// AssertContains asserts that the specified string, list(array, slice...) or map contains the
// specified substring or element.
func AssertContains(t *testing.T, s, contains interface{}) {
	ok, found := includeElement(s, contains)
	if !ok {
		t.Fatalf("\"%s\" could not be applied builtin len()", s)
	}
	if !found {
		t.Fatalf("\"%s\" does not contain \"%s\"", s, contains)
	}
}

// AssertNotContains asserts that the specified string, list(array, slice...) or map does NOT contain the
// specified substring or element.
func AssertNotContains(t *testing.T, s, contains interface{}) {
	ok, found := includeElement(s, contains)
	if !ok {
		t.Fatalf("\"%s\" could not be applied builtin len()", s)
	}
	if found {
		t.Fatalf("\"%s\" should not contain \"%s\"", s, contains)
	}
}

// AssertNil asserts that the specified object is nil.
func AssertNil(t *testing.T, object interface{}) {
	if !isNil(object) {
		t.Fatalf("Expected nil, but got: %#v", object)
	}
}

// isNil checks if a specified object is nil or not, without Failing.
func isNil(object interface{}) bool {
	if object == nil {
		return true
	}

	value := reflect.ValueOf(object)
	kind := value.Kind()
	if kind >= reflect.Chan && kind <= reflect.Slice && value.IsNil() {
		return true
	}

	return false
}

// includeElement try loop over the list check if the list includes the element.
// return (false, false) if impossible.
// return (true, false) if element was not found.
// return (true, true) if element was found.
func includeElement(list interface{}, element interface{}) (ok, found bool) {

	listValue := reflect.ValueOf(list)
	elementValue := reflect.ValueOf(element)
	defer func() {
		if e := recover(); e != nil {
			ok = false
			found = false
		}
	}()

	if reflect.TypeOf(list).Kind() == reflect.String {
		return true, strings.Contains(listValue.String(), elementValue.String())
	}

	if reflect.TypeOf(list).Kind() == reflect.Map {
		mapKeys := listValue.MapKeys()
		for i := 0; i < len(mapKeys); i++ {
			if ObjectsAreEqual(mapKeys[i].Interface(), element) {
				return true, true
			}
		}
		return true, false
	}

	for i := 0; i < listValue.Len(); i++ {
		if ObjectsAreEqual(listValue.Index(i).Interface(), element) {
			return true, true
		}
	}
	return true, false

}

// ObjectsAreEqual determines if two objects are considered equal.
//
// This function does no assertion of any kind.
func ObjectsAreEqual(expected, actual interface{}) bool {

	if expected == nil || actual == nil {
		return expected == actual
	}
	if exp, ok := expected.([]byte); ok {
		act, ok := actual.([]byte)
		if !ok {
			return false
		} else if exp == nil || act == nil {
			return exp == nil && act == nil
		}
		return bytes.Equal(exp, act)
	}
	return reflect.DeepEqual(expected, actual)

}
