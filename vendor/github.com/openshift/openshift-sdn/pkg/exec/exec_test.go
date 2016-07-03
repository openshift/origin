package exec

import (
	"fmt"
	"strings"
	"testing"
)

func init() {
	SetTestMode()
	AddTestProgram("/bin/true")
	AddTestProgram("/bin/echo")
	AddTestProgram("/bin/false")
}

func TestLookPath(t *testing.T) {
	path, err := LookPath("true")
	if err != nil || path != "/bin/true" {
		t.Fatalf("Unexpected LookPath failure: %s, %v", path, err)
	}
	path, err = LookPath("echo")
	if err != nil || path != "/bin/echo" {
		t.Fatalf("Unexpected LookPath failure: %s, %v", path, err)
	}
	path, err = LookPath("false")
	if err != nil || path != "/bin/false" {
		t.Fatalf("Unexpected LookPath failure: %s, %v", path, err)
	}

	path, err = LookPath("missing")
	if err == nil {
		t.Fatalf("Unexpected LookPath success: %s, %v", path, err)
	}
}

func TestExecSuccess(t *testing.T) {
	AddTestResult("/bin/true", "", nil)
	AddTestResult("/bin/echo some args", "some args", nil)

	out, err := Exec("/bin/true")
	if err != nil {
		t.Fatalf("Unexpected error from command: %v", err)
	}
	out, err = Exec("/bin/echo", "some", "args")
	if err != nil {
		t.Fatalf("Unexpected error from command: %v", err)
	}
	if out != "some args" {
		t.Fatalf("Unexpected output from command: %s", out)
	}
}

func TestExecFailure(t *testing.T) {
	AddTestResult("/bin/false", "", fmt.Errorf("Exit with status 1"))

	_, err := Exec("/bin/false")
	if err == nil {
		t.Fatalf("Failed to get expected error")
	}
	if err.Error() != "Exit with status 1" {
		t.Fatalf("Failed to get expected error: %v", err)
	}
}

func TestExecNoResults(t *testing.T) {
	defer func() {
		out := recover()
		if !strings.HasPrefix(out.(string), "Ran out of testResults") {
			t.Fatalf("panic()ed for wrong reason")
		}
	}()
	Exec("/bin/true")
	t.Fatalf("Failed to panic due to missing testResults")
}

func TestExecWrongResults(t *testing.T) {
	defer func() {
		out := recover()
		if !strings.HasPrefix(out.(string), "Wrong exec command") {
			t.Fatalf("panic()ed for wrong reason")
		}
	}()
	AddTestResult("/bin/true", "", nil)
	Exec("/bin/false")
	t.Fatalf("Failed to panic due to wrong command")
}
