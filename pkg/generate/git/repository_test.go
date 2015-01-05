package git

import (
	"testing"
)

func TestGetRootDir(t *testing.T) {
	curDir := "/tests/dir"
	tests := []struct {
		stdout   string
		err      bool
		expected string
	}{
		{"test/result/dir/.git", false, "test/result/dir"}, // The .git directory should be removed
		{".git", false, curDir},                            // When only .git is returned, it is the current dir
		{"", true, ""},                                     // When blank is returned, this is not a git repository
	}
	for _, test := range tests {
		r := &repository{exec: makeExecFunc(test.stdout, nil)}
		result, err := r.GetRootDir(curDir)
		if !test.err && err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if test.err && err == nil {
			t.Errorf("Expected error, but got no error.")
		}
		if !test.err && result != test.expected {
			t.Errorf("Unexpected result: %s. Expected: %s", result, test.expected)
		}
	}
}

func TestGetOriginURL(t *testing.T) {
	url := "https://test.com/a/repository/url"
	r := &repository{exec: makeExecFunc(url, nil)}
	result, err := r.GetOriginURL("/test/dir")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if result != url {
		t.Errorf("Unexpected result: %s. Expected: %s", result, url)
	}
}

func TestGetRef(t *testing.T) {
	ref := "branch1"
	r := &repository{exec: makeExecFunc(ref, nil)}
	result := r.GetRef("/test/dir")
	if result != ref {
		t.Errorf("Unexpected result: %s. Expected: %s", result, ref)
	}
}

func TestClone(t *testing.T) {
	r := &repository{exec: makeExecFunc("", nil)}
	err := r.Clone("/test/dir", "https://test/url/to/repository")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestCheckout(t *testing.T) {
	r := &repository{exec: makeExecFunc("", nil)}
	err := r.Checkout("/test/dir", "branch2")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func makeExecFunc(output string, err error) execCmdFunc {
	return func(dir, name string, args ...string) (out string, errout string, resultErr error) {
		out = output
		resultErr = err
		return
	}
}
