package openshift

import (
	"fmt"

	"github.com/blang/semver"
	"k8s.io/kubernetes/pkg/apis/core"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"

	"github.com/openshift/origin/pkg/oc/bootstrap/docker/errors"
	securityclientinternal "github.com/openshift/origin/pkg/security/generated/internalclientset"
)

const (
	loggingNamespace = "logging"
)

// InstallLoggingViaAnsible checks whether logging is installed and installs it if not already installed
func (h *Helper) InstallLoggingViaAnsible(restConfig *rest.Config, serverVersion semver.Version, serverIP, publicHostname, loggerHost, imagePrefix, imageVersion, hostConfigDir, imageStreams string) error {
	kubeClient, err := kclientset.NewForConfig(restConfig)
	if err != nil {
		return errors.NewError("cannot obtain API clients").WithCause(err)
	}
	securityClient, err := securityclientinternal.NewForConfig(restConfig)
	if err != nil {
		return errors.NewError("cannot obtain API clients").WithCause(err)
	}

	_, err = kubeClient.Core().Namespaces().Get(loggingNamespace, metav1.GetOptions{})
	if err == nil {
		// If there's no error, the logging namespace already exists and we won't initialize it
		return nil
	}

	// Create logging namespace
	if _, err := kubeClient.Core().Namespaces().Create(&core.Namespace{ObjectMeta: metav1.ObjectMeta{Name: loggingNamespace}}); err != nil {
		return errors.NewError("cannot create logging namespace").WithCause(err)
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
