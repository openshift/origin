package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/golang/glog"
	"github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/build/api"
	bld "github.com/openshift/origin/pkg/build/builder"
	"github.com/openshift/origin/pkg/build/builder/cmd/dockercfg"
	"github.com/openshift/origin/pkg/build/builder/cmd/scmauth"
	dockerutil "github.com/openshift/origin/pkg/cmd/util/docker"
)

const DefaultDockerEndpoint = "unix:///var/run/docker.sock"
const DockerCfgFile = ".dockercfg"

type builder interface {
	Build() error
}

type factoryFunc func(
	client bld.DockerClient,
	dockerSocket string,
	authConfig docker.AuthConfiguration,
	authPresent bool,
	build *api.Build) builder

// run is responsible for preparing environment for actual build.
// It accepts factoryFunc and an ordered array of SCMAuths.
func run(builderFactory factoryFunc, scmAuths []scmauth.SCMAuth) {
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
	var (
		authcfg     docker.AuthConfiguration
		authPresent bool
	)
	output := build.Spec.Output.To != nil && len(build.Spec.Output.To.Name) != 0
	if output {
		authcfg, authPresent = dockercfg.NewHelper().GetDockerAuth(
			build.Spec.Output.To.Name,
			dockercfg.PullAuthType,
		)
	}
	if build.Spec.Source.SourceSecret != nil {
		if err := setupSourceSecret(build.Spec.Source.SourceSecret.Name, scmAuths); err != nil {
			glog.Fatalf("Cannot setup secret file for accessing private repository: %v", err)
		}
	}
	b := builderFactory(client, endpoint, authcfg, authPresent, &build)
	if err = b.Build(); err != nil {
		glog.Fatalf("Build error: %v", err)
	}
	if !output {
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
	found := false

SCMAuthLoop:
	for _, scmAuth := range scmAuths {
		glog.V(3).Infof("Checking for '%s' in secret '%s'", scmAuth.Name(), sourceSecretName)
		for _, file := range files {
			if file.Name() == scmAuth.Name() {
				glog.Infof("Using '%s' from secret '%s'", scmAuth.Name(), sourceSecretName)
				if err := scmAuth.Setup(sourceSecretDir); err != nil {
					glog.Warningf("Error setting up '%s': %v", scmAuth.Name(), err)
					continue
				}
				found = true
				break SCMAuthLoop
			}
		}
	}
	if !found {
		return fmt.Errorf("the provided secret '%s' did not have any of the supported keys %v",
			sourceSecretName, getSCMNames(scmAuths))
	}
	return nil
}

func getSCMNames(scmAuths []scmauth.SCMAuth) string {
	var names string
	for _, scmAuth := range scmAuths {
		if len(names) > 0 {
			names += ", "
		}
		names += scmAuth.Name()
	}
	return names
}

// RunDockerBuild creates a docker builder and runs its build
func RunDockerBuild() {
	run(func(client bld.DockerClient, sock string, auth docker.AuthConfiguration, present bool, build *api.Build) builder {
		return bld.NewDockerBuilder(client, auth, present, build)
	}, []scmauth.SCMAuth{&scmauth.SSHPrivateKey{}})
}

// RunSTIBuild creates a STI builder and runs its build
func RunSTIBuild() {
	run(func(client bld.DockerClient, sock string, auth docker.AuthConfiguration, present bool, build *api.Build) builder {
		return bld.NewSTIBuilder(client, sock, auth, present, build)
	}, []scmauth.SCMAuth{&scmauth.SSHPrivateKey{}})
}
