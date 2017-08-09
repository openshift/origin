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
)

func checkEvents(actual, expected []string) error {
	if len(actual) != len(expected) {
		return fmt.Errorf("expected %d events, got %d", len(expected), len(actual))
	}
	for i, actualEvt := range actual {
		if expectedEvt := expected[i]; actualEvt != expectedEvt {
			return fmt.Errorf("event %d: expected '%s', got '%s'", i, expectedEvt, actualEvt)
		}
	}
	return nil
}
