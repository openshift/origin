package internal

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

type testDataSplitReference struct {
	ref   string
	dir   string
	image string
}

type testDataScopeValidation struct {
	scope      string
	errMessage string
}

func TestSplitReferenceIntoDirAndImageWindows(t *testing.T) {
	tests := []testDataSplitReference{
		{`C:\foo\bar:busybox:latest`, `C:\foo\bar`, "busybox:latest"},
		{`C:\foo\bar:busybox`, `C:\foo\bar`, "busybox"},
		{`C:\foo\bar`, `C:\foo\bar`, ""},
	}
	for _, test := range tests {
		dir, image := splitPathAndImageWindows(test.ref)
		assert.Equal(t, test.dir, dir, "Unexpected OCI directory")
		assert.Equal(t, test.image, image, "Unexpected image")
	}
}

func TestSplitReferenceIntoDirAndImageNonWindows(t *testing.T) {
	tests := []testDataSplitReference{
		{"/foo/bar:busybox:latest", "/foo/bar", "busybox:latest"},
		{"/foo/bar:busybox", "/foo/bar", "busybox"},
		{"/foo/bar", "/foo/bar", ""},
	}
	for _, test := range tests {
		dir, image := splitPathAndImageNonWindows(test.ref)
		assert.Equal(t, test.dir, dir, "Unexpected OCI directory")
		assert.Equal(t, test.image, image, "Unexpected image")
	}
}

func TestValidateScopeWindows(t *testing.T) {
	tests := []testDataScopeValidation{
		{`C:\foo`, ""},
		{`D:\`, ""},
		{"C:", "Invalid scope 'C:'. Must be an absolute path"},
		{"E", "Invalid scope 'E'. Must be an absolute path"},
		{"", "Invalid scope ''. Must be an absolute path"},
	}
	for _, test := range tests {
		err := validateScopeWindows(test.scope)
		if test.errMessage == "" {
			assert.NoError(t, err)
		} else {
			assert.EqualError(t, err, test.errMessage, fmt.Sprintf("No error for scope '%s'", test.scope))
		}
	}
}
