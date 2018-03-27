package openshift

import (
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/blang/semver"
	"github.com/openshift/origin/pkg/oc/bootstrap/docker/errors"
	"github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
)

const (
	infraNamespace = "openshift-infra"
	svcMetrics     = "hawkular-metrics"
)

// InstallMetricsViaAnsible checks whether metrics is installed and installs it if not already installed
func (h *Helper) InstallMetricsViaAnsible(f *clientcmd.Factory, serverVersion semver.Version, serverIP, publicHostname, hostName, imagePrefix, imageVersion, hostConfigDir, imageStreams string) error {
	kubeClient, err := f.ClientSet()
	if err != nil {
		return errors.NewError("cannot obtain API clients").WithCause(err).WithDetails(h.OriginLog())
	}
	securityClient, err := f.OpenshiftInternalSecurityClient()
	if err != nil {
		return errors.NewError("cannot obtain API clients").WithCause(err).WithDetails(h.OriginLog())
	}

	_, err = kubeClient.Core().Services(infraNamespace).Get(svcMetrics, metav1.GetOptions{})
	if err == nil {
		// If there's no error, the metrics service already exists
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return errors.NewError("error retrieving metrics service").WithCause(err).WithDetails(h.OriginLog())
	}

	params := newAnsibleInventoryParams()
	params.Template = defaultMetricsInventory
	params.MasterIP = serverIP
	params.MasterPublicURL = fmt.Sprintf("https://%s:8443", publicHostname)
	params.OSERelease = imageVersion
	params.MetricsImagePrefix = fmt.Sprintf("%s-", imagePrefix)
	params.MetricsImageVersion = imageVersion
	params.HawkularHostName = hostName
	params.MetricsResolution = "10s"

	runner := newAnsibleRunner(h, kubeClient, securityClient, infraNamespace, imageStreams, "metrics")

	//run playbook
	return runner.RunPlaybook(params, "playbooks/openshift-metrics/config.yml", hostConfigDir, imagePrefix, imageVersion)
}

func MetricsHost(routingSuffix string) string {
	return fmt.Sprintf("hawkular-metrics-openshift-infra.%s", routingSuffix)
}
