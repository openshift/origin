package util

import (
	"testing"
)

func TestRejectNonAbsolutePathsThatRequireBacksteps(t *testing.T) {
	path := "../foo"
	paths := []*string{}
	paths = append(paths, &path)

	expectedError := "../foo requires backsteps and is not absolute"

	if err := RelativizePathWithNoBacksteps(paths, "."); err == nil || expectedError != err.Error() {
		t.Errorf("expected %v, got %v", expectedError, err)
	}
}

func TestAcceptAbsolutePath(t *testing.T) {
	path := "/foo"
	paths := []*string{}
	paths = append(paths, &path)

	if err := RelativizePathWithNoBacksteps(paths, "/home/deads"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
