package componentinstall

import (
	"io/ioutil"
	"path"
	"path/filepath"
	"time"

	"github.com/golang/glog"

	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/openshift/origin/pkg/oc/bootstrap/clusterup/kubeapiserver"
	"github.com/openshift/origin/pkg/oc/bootstrap/docker/dockerhelper"
	"github.com/openshift/origin/pkg/oc/bootstrap/docker/run"
	"github.com/openshift/origin/pkg/oc/errors"
)

type List struct {
	Name string

	Namespace string
	List      []byte

	WaitCondition func() (bool, error)
}

func (t List) MakeReady(image, baseDir string) Component {
	return &installReadyList{
		list:    t,
		image:   image,
		baseDir: baseDir,
	}
}

const listBashScript = `#!/bin/sh
set -e
set -x

ls -alh /

ns=""
if [ -s /namespace-file ]; then
	ns="--namespace=$(cat /namespace-file) "
fi

oc apply ${ns} --config=/kubeconfig.kubeconfig -f /list.yaml 
`

type installReadyList struct {
	list    List
	image   string
	baseDir string
}

func (opt *installReadyList) Name() string {
	return opt.list.Name
}

func (opt *installReadyList) Install(dockerClient dockerhelper.Interface) error {
	imageRunHelper := run.NewRunHelper(dockerhelper.NewHelper(dockerClient)).New()

	clusterAdminConfigBytes, err := ioutil.ReadFile(path.Join(opt.baseDir, kubeapiserver.KubeAPIServerDirName, "admin.kubeconfig"))
	if err != nil {
		return err
	}

	glog.Infof("Installing %q", opt.Name())

	contentToCopy := map[string][]byte{
		"kubeconfig.kubeconfig": clusterAdminConfigBytes,
		"list.yaml":             opt.list.List,
		"apply.sh":              []byte(listBashScript),
		"namespace-file":        []byte(opt.list.Namespace),
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
			Command("-c", "chmod 755 /apply.sh && /apply.sh").Run()
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

	if opt.list.WaitCondition == nil {
		return nil
	}

	if err := wait.PollImmediate(time.Second, 5*time.Minute, opt.list.WaitCondition); err != nil {
		return err
	}

	return nil
}
