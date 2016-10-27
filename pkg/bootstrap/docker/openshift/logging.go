package openshift

import (
	"bytes"
	"fmt"

	kapi "k8s.io/kubernetes/pkg/api"
	kerrors "k8s.io/kubernetes/pkg/util/errors"

	"github.com/openshift/origin/pkg/bootstrap/docker/errors"
	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	configcmd "github.com/openshift/origin/pkg/config/cmd"
	genappcmd "github.com/openshift/origin/pkg/generate/app/cmd"
)

const (
	loggingNamespace               = "logging"
	svcKibana                      = "kibana-logging"
	loggingDeployerAccountTemplate = "logging-deployer-account-template"
	loggingDeployerTemplate        = "logging-deployer-template"
)

func instantiateTemplate(client client.Interface, mapper configcmd.Mapper, templateNamespace, templateName, targetNamespace string, params map[string]string) error {
	template, err := client.Templates(templateNamespace).Get(templateName)
	if err != nil {
		return errors.NewError("cannot retrieve template %q from namespace %q", templateName, templateNamespace).WithCause(err)
	}

	// process the template
	result, err := genappcmd.TransformTemplate(template, client, targetNamespace, params)
	if err != nil {
		return errors.NewError("cannot process template %s/%s", templateNamespace, templateName).WithCause(err)
	}

	// Create objects
	bulk := &configcmd.Bulk{
		Mapper: mapper,
		Op:     configcmd.Create,
	}
	itemsToCreate := &kapi.List{
		Items: result.Objects,
	}
	if errs := bulk.Run(itemsToCreate, targetNamespace); len(errs) > 0 {
		err = kerrors.NewAggregate(errs)
		return errors.NewError("cannot create objects from template %s/%s", templateNamespace, templateName).WithCause(err)
	}

	return nil
}

// InstallLogging checks whether logging is installed and installs it if not already installed
func (h *Helper) InstallLogging(f *clientcmd.Factory, publicHostname, loggerHost, imagePrefix, imageVersion string) error {
	osClient, kubeClient, err := f.Clients()
	if err != nil {
		return errors.NewError("cannot obtain API clients").WithCause(err).WithDetails(h.OriginLog())
	}

	_, err = kubeClient.Namespaces().Get(loggingNamespace)
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
	err = instantiateTemplate(osClient, clientcmd.ResourceMapper(f), "openshift", loggingDeployerAccountTemplate, loggingNamespace, nil)
	if err != nil {
		return errors.NewError("cannot instantiate logger accounts").WithCause(err)
	}

	// Add oauth-editor cluster role to logging-deployer sa
	if err = AddClusterRole(osClient, "oauth-editor", "system:serviceaccount:logging:logging-deployer"); err != nil {
		return errors.NewError("cannot add oauth editor role to logging deployer service account").WithCause(err).WithDetails(h.OriginLog())
	}

	// Add cluster-reader cluster role to aggregated-logging-fluentd sa
	if err = AddClusterRole(osClient, "cluster-reader", "system:serviceaccount:logging:aggregated-logging-fluentd"); err != nil {
		return errors.NewError("cannot cluster reader role to logging fluentd service account").WithCause(err).WithDetails(h.OriginLog())
	}

	// Add privileged SCC to aggregated-logging-fluentd sa
	if err = AddSCCToServiceAccount(kubeClient, "privileged", "aggregated-logging-fluentd", loggingNamespace); err != nil {
		return errors.NewError("cannot add privileged security context constraint to logging fluentd service account").WithCause(err).WithDetails(h.OriginLog())
	}

	// Label all nodes with default fluentd label
	nodeList, err := kubeClient.Nodes().List(kapi.ListOptions{})
	if err != nil {
		return errors.NewError("cannot retrieve nodes").WithCause(err).WithDetails(h.OriginLog())
	}

	// Iterate through all nodes (there should only be one)
	for _, node := range nodeList.Items {
		node.Labels["logging-infra-fluentd"] = "true"
		if _, err = kubeClient.Nodes().Update(&node); err != nil {
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
	}
	kubeClient.ConfigMaps(loggingNamespace).Create(loggingConfig)

	// Instantiate logging deployer
	deployerParams := map[string]string{
		"IMAGE_VERSION": imageVersion,
		"IMAGE_PREFIX":  fmt.Sprintf("%s-", imagePrefix),
		"MODE":          "install",
	}
	err = instantiateTemplate(osClient, clientcmd.ResourceMapper(f), "openshift", loggingDeployerTemplate, loggingNamespace, deployerParams)
	if err != nil {
		return errors.NewError("cannot instantiate logging deployer").WithCause(err)
	}
	return nil
}

func LoggingHost(routingSuffix, serverIP string) string {
	if len(routingSuffix) > 0 {
		return fmt.Sprintf("kibana-logging.%s", routingSuffix)
	}
	return fmt.Sprintf("kibana-logging.%s.xip.io", serverIP)
}
