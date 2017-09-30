package openshift

import (
	"bytes"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kapi "k8s.io/kubernetes/pkg/api"

	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/openshift/origin/pkg/oc/bootstrap/docker/errors"
)

const (
	loggingNamespace               = "logging"
	svcKibana                      = "kibana-logging"
	loggingDeployerAccountTemplate = "logging-deployer-account-template"
	loggingDeployerTemplate        = "logging-deployer-template"
	loggingPlaybook                = "playbooks/byo/openshift-cluster/openshift-logging.yml"
)

// InstallLoggingViaAnsible checks whether logging is installed and installs it if not already installed
func (h *Helper) InstallLoggingViaAnsible(f *clientcmd.Factory, serverIP, publicHostname, loggerHost, imagePrefix, imageVersion, hostConfigDir, imageStreams string) error {
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
	return runner.RunPlaybook(params, loggingPlaybook, hostConfigDir, imagePrefix, imageVersion)
}

// InstallLogging checks whether logging is installed and installs it if not already installed
func (h *Helper) InstallLogging(f *clientcmd.Factory, publicHostname, loggerHost, imagePrefix, imageVersion string) error {
	kubeClient, err := f.ClientSet()
	if err != nil {
		return errors.NewError("cannot obtain API clients").WithCause(err).WithDetails(h.OriginLog())
	}
	authorizationClient, err := f.OpenshiftInternalAuthorizationClient()
	if err != nil {
		return errors.NewError("cannot obtain API clients").WithCause(err).WithDetails(h.OriginLog())
	}
	templateClient, err := f.OpenshiftInternalTemplateClient()
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

	// Instantiate logging deployer account template
	err = instantiateTemplate(templateClient.Template(), clientcmd.ResourceMapper(f), nil, OpenshiftInfraNamespace, loggingDeployerAccountTemplate, loggingNamespace, nil, false)
	if err != nil {
		return errors.NewError("cannot instantiate logger accounts").WithCause(err)
	}

	// Add oauth-editor cluster role to logging-deployer sa
	if err = AddClusterRole(authorizationClient.Authorization(), "oauth-editor", "system:serviceaccount:logging:logging-deployer"); err != nil {
		return errors.NewError("cannot add oauth editor role to logging deployer service account").WithCause(err).WithDetails(h.OriginLog())
	}

	// Add cluster-reader cluster role to aggregated-logging-fluentd sa
	if err = AddClusterRole(authorizationClient.Authorization(), "cluster-reader", "system:serviceaccount:logging:aggregated-logging-fluentd"); err != nil {
		return errors.NewError("cannot cluster reader role to logging fluentd service account").WithCause(err).WithDetails(h.OriginLog())
	}

	// Add privileged SCC to aggregated-logging-fluentd sa
	if err = AddSCCToServiceAccount(securityClient.Security(), "privileged", "aggregated-logging-fluentd", loggingNamespace, out); err != nil {
		return errors.NewError("cannot add privileged security context constraint to logging fluentd service account").WithCause(err).WithDetails(h.OriginLog())
	}

	// Label all nodes with default fluentd label
	nodeList, err := kubeClient.Core().Nodes().List(metav1.ListOptions{})
	if err != nil {
		return errors.NewError("cannot retrieve nodes").WithCause(err).WithDetails(h.OriginLog())
	}

	// Iterate through all nodes (there should only be one)
	for _, node := range nodeList.Items {
		node.Labels["logging-infra-fluentd"] = "true"
		if _, err = kubeClient.Core().Nodes().Update(&node); err != nil {
			return errors.NewError("cannot update labels on node %s", node.Name).WithCause(err)
		}
	}

	// Create ConfigMap with deployment values
	loggingConfig := &kapi.ConfigMap{}
	loggingConfig.Name = "logging-deployer"
	loggingConfig.Data = map[string]string{
		"kibana-hostname":   loggerHost,
		"public-master-url": fmt.Sprintf("https://%s:8443", publicHostname),
		"es-cluster-size":   "1",
		"es-instance-ram":   "1024M",
		"es-pvc-size":       "100G",
	}
	kubeClient.Core().ConfigMaps(loggingNamespace).Create(loggingConfig)

	// Instantiate logging deployer
	deployerParams := map[string]string{
		"IMAGE_VERSION": imageVersion,
		"IMAGE_PREFIX":  fmt.Sprintf("%s-", imagePrefix),
		"MODE":          "install",
	}
	err = instantiateTemplate(templateClient.Template(), clientcmd.ResourceMapper(f), nil, OpenshiftInfraNamespace, loggingDeployerTemplate, loggingNamespace, deployerParams, false)
	if err != nil {
		return errors.NewError("cannot instantiate logging deployer").WithCause(err)
	}
	return nil
}

func LoggingHost(routingSuffix, serverIP string) string {
	if len(routingSuffix) > 0 {
		return fmt.Sprintf("kibana-logging.%s", routingSuffix)
	}
	return fmt.Sprintf("kibana-logging.%s.nip.io", serverIP)
}
