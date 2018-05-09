package componentinstall

import (
	"fmt"
	"io/ioutil"
	"path"
	"path/filepath"
	"time"

	"github.com/golang/glog"
	"github.com/openshift/origin/pkg/oc/bootstrap/clusterup/kubeapiserver"

	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/openshift/origin/pkg/oc/bootstrap/docker/dockerhelper"
	"github.com/openshift/origin/pkg/oc/bootstrap/docker/run"
	"github.com/openshift/origin/pkg/oc/errors"
)

type Template struct {
	Name string

	Namespace         string
	PrivilegedSANames []string
	NamespaceObj      []byte
	RBACTemplate      []byte
	InstallTemplate   []byte

	WaitCondition func() (bool, error)
}

func (t Template) MakeReady(cliImage, baseDir string, params map[string]string) Component {
	return installReadyTemplate{
		template: t,
		image:    cliImage,
		baseDir:  baseDir,
		params:   params,
	}
}

const templateBashScript = `#!/bin/sh
set -e
set -x

ls -alh /

while read p; do
	oc adm policy add-scc-to-user --config=/kubeconfig.kubeconfig privileged ${p}
done </privileged-sa-list.txt

ns=""
if [ -s /namespace-file ]; then
	ns="--namespace=$(cat /namespace-file) "
fi

if [ -s /namespace.yaml ]; then
	oc apply --config=/kubeconfig.kubeconfig -f /namespace.yaml
elif [ -s /namespace-file ]; then
	oc create ns $(cat /namespace-file) --config=/kubeconfig.kubeconfig --dry-run -o yaml | oc apply --config=/kubeconfig.kubeconfig -f -
fi

if [ -s /rbac.yaml ]; then
	oc process --local -o yaml --ignore-unknown-parameters --param-file=/param-file.txt -f /rbac.yaml | oc auth reconcile --config=/kubeconfig.kubeconfig -f - 
fi

oc process --local -o yaml --ignore-unknown-parameters --param-file=/param-file.txt -f /install.yaml | oc apply ${ns} --config=/kubeconfig.kubeconfig -f - 
`

type installReadyTemplate struct {
	template Template
	image    string
	baseDir  string
	params   map[string]string
}

func (opt installReadyTemplate) Name() string {
	return opt.template.Name
}

func (opt installReadyTemplate) Install(dockerClient dockerhelper.Interface) error {
	imageRunHelper := run.NewRunHelper(dockerhelper.NewHelper(dockerClient)).New()

	glog.Infof("Installing %q\n", opt.Name())

	clusterAdminConfigBytes, err := ioutil.ReadFile(path.Join(opt.baseDir, kubeapiserver.KubeAPIServerDirName, "admin.kubeconfig"))
	if err != nil {
		return err
	}

	contentToCopy := map[string][]byte{
		"kubeconfig.kubeconfig":  clusterAdminConfigBytes,
		"namespace.yaml":         opt.template.NamespaceObj,
		"rbac.yaml":              opt.template.RBACTemplate,
		"install.yaml":           opt.template.InstallTemplate,
		"param-file.txt":         toParamFile(opt.params),
		"namespace-file":         []byte(opt.template.Namespace),
		"privileged-sa-list.txt": toPrivilegedSAFile(opt.template.Namespace, opt.template.PrivilegedSANames),
		"install.sh":             []byte(templateBashScript),
	}

	var lastErr error
	// do a very simple retry loop on failure. Three times, ten second gaps
	wait.PollImmediate(10*time.Second, 30*time.Second, func() (bool, error) {
		_, rc, err := imageRunHelper.Image(opt.image).
			Privileged().
			DiscardContainer().
			Copy(contentToCopy).
			SaveContainerLogs(opt.Name(), filepath.Join(opt.baseDir, "logs")).
			HostNetwork().
			HostPid().
			Entrypoint("sh").
			Command("-c", "echo '"+opt.Name()+"' && chmod 755 /install.sh && /install.sh").Run()
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

	if opt.template.WaitCondition == nil {
		return nil
	}

	if err := wait.PollImmediate(time.Second, 5*time.Minute, opt.template.WaitCondition); err != nil {
		return err
	}

	return nil
}

func toParamFile(params map[string]string) []byte {
	output := ""

	for k, v := range params {
		output = output + fmt.Sprintf("%v=%q\n", k, v)
	}
	return []byte(output)
}

func toPrivilegedSAFile(namespace string, privilegedSANames []string) []byte {
	output := ""

	for _, v := range privilegedSANames {
		output = output + fmt.Sprintf("system:serviceaccount:%v:%v\n", namespace, v)
	}
	return []byte(output)
}
