package openshift

import (
	"encoding/base64"
	"io/ioutil"
	"path/filepath"

	"github.com/golang/glog"

	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/kubectl/resource"

	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/openshift/origin/pkg/cmd/util/variable"
	"github.com/openshift/origin/pkg/oc/bootstrap/docker/errors"
)

const (
	asbNamespace                = "ansible-service-broker"
	asbRBACTemplateName         = "asb-rbac-template"
	asbAPIServerTemplateName    = "asb-template"
	asbRegistrationTemplateName = "ansible-broker-registration"
)

// InstallAnsibleBroker - checks whether the ansible broker is installed and installs it if not already installed
func (h *Helper) InstallAnsibleBroker(f *clientcmd.Factory, imageFormat string, serverLogLevel int) error {
	kubeClient, err := f.ClientSet()
	if err != nil {
		return errors.NewError("cannot obtain API clients").WithCause(err).WithDetails(h.OriginLog())
	}
	templateClient, err := f.OpenshiftInternalTemplateClient()
	if err != nil {
		return err
	}

	// create the namespace if needed.  This is a reserved namespace, so you can't do it with the create project request
	if _, err := kubeClient.Core().Namespaces().Create(&kapi.Namespace{ObjectMeta: metav1.ObjectMeta{Name: asbNamespace}}); err != nil && !kapierrors.IsAlreadyExists(err) {
		return errors.NewError("cannot create ansible broker project").WithCause(err)
	}
	params := map[string]string{
		"NAMESPACE": asbNamespace,
	}

	if err = instantiateTemplate(templateClient.Template(), clientcmd.ResourceMapper(f), nil, OpenshiftInfraNamespace, asbRBACTemplateName, asbNamespace, params, true); err != nil {
		return errors.NewError("cannot instantiate ansible broker permissions").WithCause(err)
	}

	// create the actual resources required
	imageTemplate := variable.NewDefaultImageTemplate()
	imageTemplate.Format = imageFormat
	imageTemplate.Latest = true

	params["BROKER_IMAGE"] = imageTemplate.ExpandOrDie("ansible-service-broker")
	params["LOG_LEVEL"] = getAnsibleBrokerLogLevel(serverLogLevel)

	glog.V(2).Infof("instantiating ansible broker template with parameters %v", params)

	if err = instantiateTemplate(templateClient.Template(), clientcmd.ResourceMapper(f), nil, OpenshiftInfraNamespace, asbAPIServerTemplateName, asbNamespace, params, true); err != nil {
		return errors.NewError("cannot instantiate ansible broker resources").WithCause(err)
	}
	return nil
}

// RegisterAnsibleBroker - registers the ASB with the SC by creating the broker resource
func (h *Helper) RegisterAnsibleBroker(f *clientcmd.Factory, configDir string) error {
	templateClient, err := f.OpenshiftInternalTemplateClient()
	if err != nil {
		return err
	}

	// Register the ansible broker with the service catalog
	glog.V(2).Infof("registering the ansible broker with the service catalog")

	// dynamic mapper is needed to support the broker resource which isn't part of the api.
	dynamicMapper, dynamicTyper, err := f.UnstructuredObject()
	dmapper := &resource.Mapper{
		RESTMapper:   dynamicMapper,
		ObjectTyper:  dynamicTyper,
		ClientMapper: resource.ClientMapperFunc(f.UnstructuredClientForMapping),
	}

	serviceCABytes, err := ioutil.ReadFile(filepath.Join(configDir, "master", "service-signer.crt"))
	serviceCAString := base64.StdEncoding.EncodeToString(serviceCABytes)
	if err != nil {
		return errors.NewError("unable to read service signer cert").WithCause(err)
	}
	if err = instantiateTemplate(templateClient.Template(), clientcmd.ResourceMapper(f), dmapper, OpenshiftInfraNamespace, asbRegistrationTemplateName, asbNamespace, map[string]string{
		"ASB_NAMESPACE": asbNamespace,
		"CA_BUNDLE":     serviceCAString,
	}, true); err != nil {
		return errors.NewError("cannot register the ansible broker").WithCause(err)
	}

	return nil
}

func getAnsibleBrokerLogLevel(serverLogLevel int) string {
	switch serverLogLevel {
	case 0, 1:
		return "critical"
	case 2:
		return "error"
	case 3:
		return "warning"
	case 4:
		return "info"
	case 5:
		return "debug"
	default:
		return "critical"
	}
}
