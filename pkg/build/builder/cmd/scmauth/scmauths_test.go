package scmauth

import (
	"io/ioutil"
	"os"
	"testing"
)

type testAuth struct {
	name string
}

func (a *testAuth) Name() string {
	return a.name
}

func (a *testAuth) Handles(name string) bool {
	return name == a.name
}

func (a *testAuth) Setup(baseDir string, context SCMAuthContext) error {
	context.Set(a.name, "test")
	return nil
}

func scmAuths() SCMAuths {
	return SCMAuths{
		&testAuth{name: "one"},
		&testAuth{name: "two"},
		&testAuth{name: "three"},
	}
}

func TestPresent(t *testing.T) {
	secretDir := secretDir(t, "one", "three")
	defer os.RemoveAll(secretDir)
	files, err := ioutil.ReadDir(secretDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	result := scmAuths().present(files)
	if len(result) != 2 {
		t.Errorf("Unexpected result: %#v", result)
	}
	// Ensure that two is not present in result
	for _, a := range result {
		if a.Name() == "two" {
			t.Errorf("Found unexpected auth")
		}
	}
}

func TestSetup(t *testing.T) {
	secretDir := secretDir(t, "one", "two", "three")
	defer os.RemoveAll(secretDir)
	env, _, err := scmAuths().Setup(secretDir)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	expectedVars := map[string]struct{}{"one=test": {}, "two=test": {}, "three=test": {}}
	if len(env) != 3 {
		t.Errorf("unexpected number of vars")
	}
	for _, v := range env {
		if _, exists := expectedVars[v]; !exists {
			t.Errorf("Unexpected variable: %s", v)
		}
	}
}
