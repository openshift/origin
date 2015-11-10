package cmd

import (
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/golang/glog"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"

	"github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/build/api"
	bld "github.com/openshift/origin/pkg/build/builder"
	"github.com/openshift/origin/pkg/build/builder/cmd/scmauth"
	"github.com/openshift/origin/pkg/client"
	dockerutil "github.com/openshift/origin/pkg/cmd/util/docker"
	"github.com/openshift/origin/pkg/generate/git"
)

type builder interface {
	Build(dockerClient bld.DockerClient, sock string, buildsClient client.BuildInterface, build *api.Build) error
}

// run is responsible for preparing environment for actual build.
// It accepts factoryFunc and an ordered array of SCMAuths.
func run(b builder) {
	dockerClient, endpoint, err := dockerutil.NewHelper().GetClient()
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
		if build.Spec.Source.Git != nil {
			// TODO: this should be refactored to let each source type manage which secrets
			//   it accepts
			sourceURL, err := git.ParseRepository(build.Spec.Source.Git.URI)
			if err != nil {
				glog.Fatalf("Cannot parse build URL: %s", build.Spec.Source.Git.URI)
			}
			scmAuths := auths(sourceURL)
			sourceURL, err = setupSourceSecret(build.Spec.Source.SourceSecret.Name, scmAuths)
			if err != nil {
				glog.Fatalf("Cannot setup secret file for accessing private repository: %v", err)
			}
			if sourceURL != nil {
				build.Spec.Source.Git.URI = sourceURL.String()
			}
		}
	}
	config, err := kclient.InClusterConfig()
	if err != nil {
		glog.Fatalf("Failed to get client config: %v", err)
	}
	osClient, err := client.New(config)
	if err != nil {
		glog.Fatalf("Error obtaining OpenShift client: %v", err)
	}
	buildsClient := osClient.Builds(build.Namespace)

	if err = b.Build(dockerClient, endpoint, buildsClient, &build); err != nil {
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

func setupSourceSecret(sourceSecretName string, scmAuths []scmauth.SCMAuth) (*url.URL, error) {
	fixSecretPermissions()
	sourceSecretDir := os.Getenv("SOURCE_SECRET_PATH")
	files, err := ioutil.ReadDir(sourceSecretDir)
	if err != nil {
		return nil, err
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
		return nil, fmt.Errorf("no auth handler was found for the provided secret %q",
			sourceSecretName)
	}

	var urlOverride *url.URL = nil
	for name, auth := range scmAuthsPresent {
		glog.V(3).Infof("Setting up SCMAuth %q", name)
		u, err := auth.Setup(sourceSecretDir)
		if err != nil {
			// If an error occurs during setup, fail the build
			return nil, fmt.Errorf("cannot set up source authentication method %q: %v", name, err)
		}

		if u != nil {
			if urlOverride == nil {
				urlOverride = u
			} else if urlOverride.String() != u.String() {
				return nil, fmt.Errorf("secret handler %s set a conflicting override URL %q (conflicts with earlier override URL %q)", auth.Name(), u.String(), urlOverride.String())
			}
		}
	}

	return urlOverride, nil
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

type dockerBuilder struct{}

// Build starts a Docker build.
func (dockerBuilder) Build(dockerClient bld.DockerClient, sock string, buildsClient client.BuildInterface, build *api.Build) error {
	return bld.NewDockerBuilder(dockerClient, buildsClient, build).Build()
}

type s2iBuilder struct{}

// Build starts an S2I build.
func (s2iBuilder) Build(dockerClient bld.DockerClient, sock string, buildsClient client.BuildInterface, build *api.Build) error {
	return bld.NewS2IBuilder(dockerClient, sock, buildsClient, build).Build()
}

// RunDockerBuild creates a docker builder and runs its build
func RunDockerBuild() {
	run(dockerBuilder{})
}

// RunSTIBuild creates a STI builder and runs its build
func RunSTIBuild() {
	run(s2iBuilder{})
}
