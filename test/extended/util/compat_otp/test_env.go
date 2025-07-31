package compat_otp

import (
	"os"
	"path"
	"sync"
)

const (
	TestEnvProw TestEnvType = 1 << iota
	TestEnvJenkins
	TestEnvLocal

	ArtifactDirEnvProw = "ARTIFACT_DIR"

	PullSecretDirEnvProw       = "CLUSTER_PROFILE_DIR"
	PullSecretFileNameProw     = "pull-secret"
	PullSecretLocationEnvLocal = "PULL_SECRET_LOCATION"
)

type TestEnvType int

type TestEnv struct {
	Type TestEnvType

	ArtifactDir        string
	PullSecretLocation string
}

var (
	globalTestEnv     *TestEnv
	globalTestEnvOnce sync.Once
)

// GetTestEnv gets a initialized *TestEnv in a thread-safe and lazy manner
func GetTestEnv() *TestEnv {
	globalTestEnvOnce.Do(func() {
		globalTestEnv = initTestEnv()
	})
	return globalTestEnv
}

func initTestEnv() *TestEnv {
	env := &TestEnv{}
	env.getEnvType()
	env.getPullSecretLocation()
	env.getArtifactDir()
	return env
}

func (t *TestEnv) IsRunningInProw() bool {
	return t.Type == TestEnvProw
}

func (t *TestEnv) IsRunningInJenkins() bool {
	return t.Type == TestEnvJenkins
}

func (t *TestEnv) IsRunningLocally() bool {
	return t.Type == TestEnvLocal
}

func (t *TestEnv) getEnvType() {
	switch {
	case os.Getenv("JENKINS_HOME") != "":
		t.Type = TestEnvJenkins
	case os.Getenv("OPENSHIFT_CI") != "":
		t.Type = TestEnvProw
	default:
		t.Type = TestEnvLocal
	}
}

func (t *TestEnv) getPullSecretLocation() {
	switch {
	case t.IsRunningInProw():
		t.PullSecretLocation = path.Join(os.Getenv(PullSecretDirEnvProw), PullSecretFileNameProw)
	case t.IsRunningLocally():
		t.PullSecretLocation = os.Getenv(PullSecretLocationEnvLocal)
	}
}

func (t *TestEnv) getArtifactDir() {
	switch {
	case t.IsRunningInProw():
		t.ArtifactDir = os.Getenv(ArtifactDirEnvProw)
	case t.IsRunningLocally():
		t.ArtifactDir = os.TempDir()
	}
}
