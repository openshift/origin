// Copyright 2014 go-dockerclient authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package docker

import "testing"

func TestChangeString(t *testing.T) {
	t.Parallel()
	tests := []struct {
		change   Change
		expected string
	}{
		{Change{"/etc/passwd", ChangeModify}, "C /etc/passwd"},
		{Change{"/etc/passwd", ChangeAdd}, "A /etc/passwd"},
		{Change{"/etc/passwd", ChangeDelete}, "D /etc/passwd"},
		{Change{"/etc/passwd", 33}, " /etc/passwd"},
	}
	for _, tt := range tests {
		test := tt
		t.Run(test.expected, func(t *testing.T) {
			t.Parallel()
			if got := test.change.String(); got != test.expected {
				t.Errorf("Change.String(): want %q. Got %q.", test.expected, got)
			}
		})
	}
}
