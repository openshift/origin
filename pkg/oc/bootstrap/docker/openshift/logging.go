package openshift

import (
	"bytes"
	"fmt"

	"github.com/blang/semver"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/openshift/origin/pkg/oc/bootstrap/docker/errors"
	"github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
)

const (
	loggingNamespace = "logging"
)

// InstallLoggingViaAnsible checks whether logging is installed and installs it if not already installed
func (h *Helper) InstallLoggingViaAnsible(f *clientcmd.Factory, serverVersion semver.Version, serverIP, publicHostname, loggerHost, imagePrefix, imageVersion, hostConfigDir, imageStreams string) error {
	kubeClient, err := f.ClientSet()
	if err != nil {
		return errors.NewError("cannot obtain API clients").WithCause(err).WithDetails(h.OriginLog())
	}
	securityClient, err := f.OpenshiftInternalSecurityClient()
	if err != nil {
		return errors.NewError("cannot obtain API clients").WithCause(err).WithDetails(h.OriginLog())
	}

	_, err = kubeClient.Core().Namespaces().Get(loggingNamespace, metav1.GetOptions{})
	if err == nil {
		// If there's no error, the logging namespace already exists and we won't initialize it
		return nil
	}

	// Create logging namespace
	out := &bytes.Buffer{}
	err = CreateProject(f, loggingNamespace, "", "", "oc", out)
	if err != nil {
		return errors.NewError("cannot create logging project").WithCause(err).WithDetails(out.String())
	}

	params := newAnsibleInventoryParams()
	params.Template = defaultLoggingInventory
	params.MasterIP = serverIP
	params.MasterPublicURL = fmt.Sprintf("https://%s:8443", publicHostname)
	params.OSERelease = imageVersion
	params.LoggingImagePrefix = fmt.Sprintf("%s-", imagePrefix)
	params.LoggingImageVersion = imageVersion
	params.LoggingNamespace = loggingNamespace
	params.KibanaHostName = loggerHost

	runner := newAnsibleRunner(h, kubeClient, securityClient, loggingNamespace, imageStreams, "logging")

	//run logging playbook

	return runner.RunPlaybook(params, "playbooks/openshift-logging/config.yml", hostConfigDir, imagePrefix, imageVersion)
}

func LoggingHost(routingSuffix string) string {
	return fmt.Sprintf("kibana-logging.%s", routingSuffix)
}
