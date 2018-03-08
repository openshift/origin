package componentinstall

import (
	"time"

	"github.com/golang/glog"

	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/openshift/origin/pkg/oc/bootstrap/docker/dockerhelper"
	"github.com/openshift/origin/pkg/oc/bootstrap/docker/run"
	"github.com/openshift/origin/pkg/oc/errors"
)

type List struct {
	ComponentName string

	Image string

	Namespace  string
	KubeConfig []byte
	List       []byte

	WaitCondition func() (bool, error)
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

func (opt List) Name() string {
	return opt.ComponentName
}

func (opt List) Install(dockerClient dockerhelper.Interface, logdir string) error {
	imageRunHelper := run.NewRunHelper(dockerhelper.NewHelper(dockerClient)).New()

	glog.Infof("Installing %q", opt.Name())

	contentToCopy := map[string][]byte{
		"kubeconfig.kubeconfig": opt.KubeConfig,
		"list.yaml":             opt.List,
		"apply.sh":              []byte(listBashScript),
		"namespace-file":        []byte(opt.Namespace),
	}

	var lastErr error
	// do a very simple retry loop on failure. Three times, ten second gaps
	wait.PollImmediate(10*time.Second, 30*time.Second, func() (bool, error) {
		_, stdout, stderr, rc, err := imageRunHelper.Image(opt.Image).
			Privileged().
			DiscardContainer().
			Copy(contentToCopy).
			HostNetwork().
			HostPid().
			Entrypoint("sh").
			Command("-c", "chmod 755 /apply.sh && /apply.sh").Output()

		if err := LogContainer(logdir, opt.ComponentName, stdout, stderr); err != nil {
			glog.Errorf("error logging %q: %v", opt.ComponentName, err)
		}
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

	if opt.WaitCondition == nil {
		return nil
	}

	if err := wait.PollImmediate(time.Second, 5*time.Minute, opt.WaitCondition); err != nil {
		return err
	}

	return nil
}
