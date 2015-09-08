package cmd

import (
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/golang/glog"
	"github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/build/api"
	bld "github.com/openshift/origin/pkg/build/builder"
	"github.com/openshift/origin/pkg/build/builder/cmd/scmauth"
	dockerutil "github.com/openshift/origin/pkg/cmd/util/docker"
	"github.com/openshift/origin/pkg/generate/git"
)

type builder interface {
	Build() error
}

type factoryFunc func(
	client bld.DockerClient,
	dockerSocket string,
	build *api.Build) builder

// run is responsible for preparing environment for actual build.
// It accepts factoryFunc and an ordered array of SCMAuths.
func run(builderFactory factoryFunc) {
	client, endpoint, err := dockerutil.NewHelper().GetClient()
	if err != nil {
		glog.Fatalf("Error obtaining docker client: %v", err)
	}
	buildStr := os.Getenv("BUILD")
	glog.V(4).Infof("$BUILD env var is %s \n", buildStr)
	build := api.Build{}
	if err := latest.Codec.DecodeInto([]byte(buildStr), &build); err != nil {
		glog.Fatalf("Unable to parse build: %v", err)
	}
	if build.Spec.Source.SourceSecret != nil {
		sourceURL, err := git.ParseRepository(build.Spec.Source.Git.URI)
		if err != nil {
			glog.Fatalf("Cannot parse build URL: %s", build.Spec.Source.Git.URI)
		}
		scmAuths := auths(sourceURL)
		if err := setupSourceSecret(build.Spec.Source.SourceSecret.Name, scmAuths); err != nil {
			glog.Fatalf("Cannot setup secret file for accessing private repository: %v", err)
		}
	}
	b := builderFactory(client, endpoint, &build)
	if err = b.Build(); err != nil {
		glog.Fatalf("Build error: %v", err)
	}

	if build.Spec.Output.To == nil || len(build.Spec.Output.To.Name) == 0 {
		glog.Warning("Build does not have an Output defined, no output image was pushed to a registry.")
	}

}

// fixSecretPermissions loweres access permissions to very low acceptable level
// TODO: this method should be removed as soon as secrets permissions are fixed upstream
func fixSecretPermissions() error {
	secretTmpDir, err := ioutil.TempDir("", "tmpsecret")
	if err != nil {
		return err
	}
	cmd := exec.Command("cp", "-R", ".", secretTmpDir)
	cmd.Dir = os.Getenv("SOURCE_SECRET_PATH")
	if err := cmd.Run(); err != nil {
		return err
	}
	secretFiles, err := ioutil.ReadDir(secretTmpDir)
	if err != nil {
		return err
	}
	for _, file := range secretFiles {
		if err := os.Chmod(filepath.Join(secretTmpDir, file.Name()), 0600); err != nil {
			return err
		}
	}
	os.Setenv("SOURCE_SECRET_PATH", secretTmpDir)
	return nil
}

func setupSourceSecret(sourceSecretName string, scmAuths []scmauth.SCMAuth) error {
	fixSecretPermissions()
	sourceSecretDir := os.Getenv("SOURCE_SECRET_PATH")
	files, err := ioutil.ReadDir(sourceSecretDir)
	if err != nil {
		return err
	}

	// Filter the list of SCMAuths based on the secret files that are present
	scmAuthsPresent := map[string]scmauth.SCMAuth{}
	for _, file := range files {
		glog.V(3).Infof("Finding auth for %q in secret %q", file.Name(), sourceSecretName)
		for _, scmAuth := range scmAuths {
			if scmAuth.Handles(file.Name()) {
				glog.V(3).Infof("Found SCMAuth %q to handle %q", scmAuth.Name(), file.Name())
				scmAuthsPresent[scmAuth.Name()] = scmAuth
			}
		}
	}

	if len(scmAuthsPresent) == 0 {
		return fmt.Errorf("no auth handler was found for the provided secret %q",
			sourceSecretName)
	}

	for name, auth := range scmAuthsPresent {
		glog.V(3).Infof("Setting up SCMAuth %q", name)
		if err := auth.Setup(sourceSecretDir); err != nil {
			// If an error occurs during setup, fail the build
			return fmt.Errorf("cannot set up source authentication method %q: %v", name, err)
		}
	}

	return nil
}

func auths(sourceURL *url.URL) []scmauth.SCMAuth {
	auths := []scmauth.SCMAuth{
		&scmauth.SSHPrivateKey{},
		&scmauth.UsernamePassword{SourceURL: *sourceURL},
		&scmauth.CACert{SourceURL: *sourceURL},
		&scmauth.GitConfig{},
	}
	return auths
}

// RunDockerBuild creates a docker builder and runs its build
func RunDockerBuild() {
	run(func(client bld.DockerClient, sock string, build *api.Build) builder {
		return bld.NewDockerBuilder(client, build)
	})
}

// RunSTIBuild creates a STI builder and runs its build
func RunSTIBuild() {
	run(func(client bld.DockerClient, sock string, build *api.Build) builder {
		return bld.NewSTIBuilder(client, sock, build)
	})
}
