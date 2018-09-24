package clusteradd

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/golang/glog"

	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/openshift/origin/pkg/oc/clusterup/componentinstall"
	"github.com/openshift/origin/pkg/oc/clusterup/docker/dockerhelper"
	"github.com/openshift/origin/pkg/oc/clusterup/docker/run"
	"github.com/openshift/origin/pkg/oc/lib/errors"
)

type ManifestInstall struct {
	Name string

	KubeConfigContent []byte
	Image             string

	WaitCondition func() (bool, error)
}

func (t ManifestInstall) MakeReady(image, baseDir string) componentinstall.Component {
	return &installReadyList{
		list:    t,
		image:   image,
		baseDir: baseDir,
	}
}

const listBashScript = `#!/bin/sh
set -e
set -x

mkdir /manifests
oc image extract ${IMAGE} --path=/manifests/:/manifests/
ls -alh /manifests

oc apply --config=/kubeconfig.kubeconfig -f /manifests 
`

type installReadyList struct {
	list    ManifestInstall
	image   string
	baseDir string
}

func (opt *installReadyList) Name() string {
	return opt.list.Name
}

func (opt *installReadyList) Install(dockerClient dockerhelper.Interface) error {
	imageRunHelper := run.NewRunHelper(dockerhelper.NewHelper(dockerClient)).New()

	glog.Infof("Installing %q", opt.Name())

	contentToCopy := map[string][]byte{
		"kubeconfig.kubeconfig": opt.list.KubeConfigContent,
		"apply.sh":              []byte(listBashScript),
	}

	var lastErr error
	// do a very simple retry loop on failure. Six times, ten second gaps
	wait.PollImmediate(10*time.Second, 60*time.Second, func() (bool, error) {
		_, rc, err := imageRunHelper.Image(opt.image).
			Privileged().
			DiscardContainer().
			Copy(contentToCopy).
			Env(fmt.Sprintf("IMAGE=%s", opt.list.Image)).
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
