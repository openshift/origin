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

package controller

import (
	"fmt"
	"strings"
)

func checkEventCounts(actual, expected []string) error {
	if len(actual) != len(expected) {
		return fmt.Errorf("expected %d events, got %d", len(expected), len(actual))
	}
	return nil
}

func checkEvents(actual, expected []string) error {
	if err := checkEventCounts(actual, expected); err != nil {
		return err
	}
	for i, actualEvt := range actual {
		if expectedEvt := expected[i]; actualEvt != expectedEvt {
			return fmt.Errorf("event %d: expected '%s', got '%s'", i, expectedEvt, actualEvt)
		}
	}
	return nil
}

func checkEventPrefixes(actual, expected []string) error {
	if err := checkEventCounts(actual, expected); err != nil {
		return err
	}
	for i, e := range expected {
		a := actual[i]
		if !strings.HasPrefix(a, e) {
			return fmt.Errorf("received unexpected event prefix:\n %s", expectedGot(e, a))
		}
	}
	return nil
}

func checkEventContains(actual, expected string) error {
	if !strings.Contains(actual, expected) {
		return fmt.Errorf("received unexpected event (contains):\n %s", expectedGot(expected, actual))
	}

	return nil
}

func expectedGot(a ...interface{}) string {
	return fmt.Sprintf("\nexpected:\n\t '%v',\ngot:\n\t '%v'", a...)
}
