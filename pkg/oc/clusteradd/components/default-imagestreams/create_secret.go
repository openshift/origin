package default_imagestreams

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/golang/glog"

	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/openshift/origin/pkg/oc/clusteradd/componentinstall"
	"github.com/openshift/origin/pkg/oc/clusterup/coreinstall/kubeapiserver"
	"github.com/openshift/origin/pkg/oc/clusterup/docker/dockerhelper"
	"github.com/openshift/origin/pkg/oc/clusterup/docker/run"
	"github.com/openshift/origin/pkg/oc/lib/errors"
)

type DockerConfigSecret struct {
	Name string

	Namespace string
}

func (d DockerConfigSecret) MakeReady(image, baseDir string) componentinstall.Component {
	return &installReadyDockerConfigSecret{
		secret:  d,
		image:   image,
		baseDir: baseDir,
	}
}

const createBashScript = `#!/bin/sh
set -e
set -x

ls -alh /

ns=""
if [ -s /namespace-file ]; then
	ns="--namespace=$(cat /namespace-file) "
fi

dockerconfigjson=$(cat /dockerconfigjson)

oc create secret generic imagestreamsecret --dry-run -o yaml --type=kubernetes.io/dockerconfigjson --from-literal=.dockerconfigjson="${dockerconfigjson}" ${ns} --config=/admin.kubeconfig >& /tmp/secret.yaml
oc apply -f /tmp/secret.yaml ${ns} --config=/admin.kubeconfig
`

type installReadyDockerConfigSecret struct {
	secret  DockerConfigSecret
	image   string
	baseDir string
}

func (opt *installReadyDockerConfigSecret) Name() string {
	return opt.secret.Name
}

func (opt *installReadyDockerConfigSecret) Install(dockerClient dockerhelper.Interface) error {
	imageRunHelper := run.NewRunHelper(dockerhelper.NewHelper(dockerClient)).New()

	clusterAdminConfigBytes, err := ioutil.ReadFile(path.Join(opt.baseDir, kubeapiserver.KubeAPIServerDirName, "admin.kubeconfig"))
	if err != nil {
		return err
	}

	home := os.Getenv("HOME")
	if len(home) == 0 {
		return fmt.Errorf("No $HOME environment variable found")
	}

	dockerConfigJsonBytes, err := ioutil.ReadFile(path.Join(home, ".docker", "config.json"))
	if err != nil {
		glog.Warningf("Error reading $HOME/.docker/config.json: %v, imagestream import credentials will not be setup", err)
		return nil
	}

	glog.Infof("Installing %q", opt.Name())

	contentToCopy := map[string][]byte{
		"admin.kubeconfig": clusterAdminConfigBytes,
		"create.sh":        []byte(createBashScript),
		"namespace-file":   []byte(opt.secret.Namespace),
		"dockerconfigjson": []byte(dockerConfigJsonBytes),
	}

	var lastErr error
	// do a very simple retry loop on failure. Three times, ten second gaps
	wait.PollImmediate(10*time.Second, 30*time.Second, func() (bool, error) {
		_, rc, err := imageRunHelper.Image(opt.image).
			Privileged().
			DiscardContainer().
			Copy(contentToCopy).
			HostNetwork().
			HostPid().
			Entrypoint("sh").
			SaveContainerLogs(opt.Name(), filepath.Join(opt.baseDir, "logs")).
			Command("-c", "chmod 755 /create.sh && /create.sh").Run()
		if err != nil {
			lastErr = errors.NewError("failed to install %q: %v", opt.Name(), err).WithCause(err)
			return false, nil
		}
		if rc != 0 {
			lastErr = errors.NewError("failed to install %q: rc: %d", opt.Name(), rc)
			return false, nil
		}
		lastErr = nil
		return true, nil
	})
	if lastErr != nil {
		return lastErr
	}

	return nil
}
