package app

import (
	"testing"

	"github.com/openshift/origin/pkg/generate/app/test"
)

func TestFromGitURL(t *testing.T) {
	gen := &SourceRefGenerator{&test.FakeGit{}}
	srcRef, err := gen.FromGitURL("git@github.com:openshift/origin.git#test_branch", "")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if srcRef.Ref != "test_branch" {
		t.Errorf("Unexpected branch: %s", srcRef.Ref)
	}
	if srcRef.URL.String() != "git@github.com:openshift/origin.git" {
		t.Errorf("Unexpected URL: %s", srcRef.URL.String())
	}
}

func TestFromDirectory(t *testing.T) {
	git := &test.FakeGit{
		RootDir: "/tmp/test",
		GitURL:  "https://github.com/openshift/test-project",
		Ref:     "stable",
	}
	gen := &SourceRefGenerator{git}
	srcRef, err := gen.FromDirectory("/test/dir")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if srcRef.Ref != "stable" {
		t.Errorf("Unexpected branch: %s", srcRef.Ref)
	}
	if srcRef.URL.String() != "https://github.com/openshift/test-project" {
		t.Errorf("Unexpected URL: %s", srcRef.URL.String())
	}
}
