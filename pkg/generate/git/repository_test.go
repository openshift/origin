package git

import (
	"testing"
	"time"
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
		r := &repository{git: makeExecFunc(test.stdout, nil)}
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
	url := "remote.origin.url https://test.com/a/repository/url"
	r := &repository{git: makeExecFunc(url, nil)}
	result, ok, err := r.GetOriginURL("/test/dir")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !ok {
		t.Error("Unexpected not ok")
	}
	if result != "https://test.com/a/repository/url" {
		t.Errorf("Unexpected result: %s. Expected: %s", result, url)
	}
}

func TestGetAlterativeOriginURL(t *testing.T) {
	url := "remote.foo.url https://test.com/a/repository/url\nremote.upstream.url https://test.com/b/repository/url"
	r := &repository{git: makeExecFunc(url, nil)}
	result, ok, err := r.GetOriginURL("/test/dir")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !ok {
		t.Error("Unexpected not ok")
	}
	if result != "https://test.com/b/repository/url" {
		t.Errorf("Unexpected result: %s. Expected: %s", result, url)
	}
}

func TestGetMissingOriginURL(t *testing.T) {
	url := "remote.foo.url https://test.com/a/repository/url\nremote.bar.url https://test.com/b/repository/url"
	r := &repository{git: makeExecFunc(url, nil)}
	result, ok, err := r.GetOriginURL("/test/dir")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if ok {
		t.Error("Unexpected ok")
	}
	if result != "" {
		t.Errorf("Unexpected result: %s. Expected: %s", result, "")
	}
}

func TestGetRef(t *testing.T) {
	ref := "branch1"
	r := &repository{git: makeExecFunc(ref, nil)}
	result := r.GetRef("/test/dir")
	if result != ref {
		t.Errorf("Unexpected result: %s. Expected: %s", result, ref)
	}
}

func TestClone(t *testing.T) {
	r := &repository{git: makeExecFunc("", nil)}
	err := r.Clone("/test/dir", "https://test/url/to/repository")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestCheckout(t *testing.T) {
	r := &repository{git: makeExecFunc("", nil)}
	err := r.Checkout("/test/dir", "branch2")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func makeExecFunc(output string, err error) execGitFunc {
	return func(dir string, args ...string) (out string, errout string, resultErr error) {
		out = output
		resultErr = err
		return
	}
}

// TestTimedCommandTimeout tests that the `oc new-app` machinery that invokes `git ls-remote`
// on uncooperative servers correctly times out
func TestTimedCommandTimeout(t *testing.T) {
	timeout := 1 * time.Millisecond

	outputChannel := make(chan timedCommandOutput)
	go func() {
		stdout, stderr, err := timedCommand(timeout, "yes", "/usr/bin", nil, []string{}...)
		outputChannel <- timedCommandOutput{
			stdout: stdout,
			stderr: stderr,
			err:    err,
		}
	}()

	select {
	case output := <-outputChannel:
		if _, ok := output.err.(*TimeoutError); !ok {
			t.Fatalf("expected command to fail due to timeout, got: %v", output.err)
		}
	case <-time.After(1000 * timeout):
		t.Fatalf("expected command to have timed out, but it didn't")
	}
}

type timedCommandOutput struct {
	stdout string
	stderr string
	err    error
}
