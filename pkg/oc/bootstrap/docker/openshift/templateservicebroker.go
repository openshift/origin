package openshift

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"time"

	"github.com/golang/glog"

	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/kubectl/resource"

	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/openshift/origin/pkg/cmd/util/variable"
	"github.com/openshift/origin/pkg/oc/bootstrap/docker/errors"
)

const (
	tsbNamespace                = "openshift-template-service-broker"
	tsbRBACTemplateName         = "template-service-broker-rbac"
	tsbAPIServerTemplateName    = "template-service-broker-apiserver"
	tsbRegistrationTemplateName = "template-service-broker-registration"
)

// InstallTemplateServiceBroker checks whether the template service broker is installed and installs it if not already installed
func (h *Helper) InstallTemplateServiceBroker(f *clientcmd.Factory, imageFormat string, serverLogLevel int) error {
	kubeClient, err := f.ClientSet()
	if err != nil {
		return errors.NewError("cannot obtain API clients").WithCause(err).WithDetails(h.OriginLog())
	}
	templateClient, err := f.OpenshiftInternalTemplateClient()
	if err != nil {
		return err
	}

	// create the namespace if needed.  This is a reserved namespace, so you can't do it with the create project request
	if _, err := kubeClient.Core().Namespaces().Create(&kapi.Namespace{ObjectMeta: metav1.ObjectMeta{Name: tsbNamespace}}); err != nil && !kapierrors.IsAlreadyExists(err) {
		return errors.NewError("cannot create template service broker project").WithCause(err)
	}

	if err = instantiateTemplate(templateClient.Template(), clientcmd.ResourceMapper(f), nil, OpenshiftInfraNamespace, tsbRBACTemplateName, tsbNamespace, map[string]string{}, true); err != nil {
		return errors.NewError("cannot instantiate template service broker permissions").WithCause(err)
	}

	// create the actual resources required
	imageTemplate := variable.NewDefaultImageTemplate()
	imageTemplate.Format = imageFormat
	imageTemplate.Latest = false

	params := map[string]string{
		"IMAGE":     imageTemplate.ExpandOrDie(""),
		"LOGLEVEL":  fmt.Sprint(serverLogLevel),
		"NAMESPACE": tsbNamespace,
	}
	glog.V(2).Infof("instantiating template service broker template with parameters %v", params)

	if err = instantiateTemplate(templateClient.Template(), clientcmd.ResourceMapper(f), nil, OpenshiftInfraNamespace, tsbAPIServerTemplateName, tsbNamespace, params, true); err != nil {
		return errors.NewError("cannot instantiate template service broker resources").WithCause(err)
	}

	// Wait for the apiserver endpoint to become available
	err = wait.Poll(1*time.Second, 10*time.Minute, func() (bool, error) {
		glog.V(2).Infof("polling for template service broker api server endpoint availability")
		ds, err := kubeClient.Extensions().DaemonSets(tsbNamespace).Get("apiserver", metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		if ds.Status.NumberReady > 0 {
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		return errors.NewError(fmt.Sprintf("failed to start the template service broker apiserver: %v", err))
	}

	return nil
}

// RegisterTemplateServiceBroker registers the TSB with the SC by creating the broker resource
func (h *Helper) RegisterTemplateServiceBroker(f *clientcmd.Factory, configDir string) error {
	templateClient, err := f.OpenshiftInternalTemplateClient()
	if err != nil {
		return err
	}

	// Register the template broker with the service catalog
	glog.V(2).Infof("registering the template broker with the service catalog")

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
	if err = instantiateTemplate(templateClient.Template(), clientcmd.ResourceMapper(f), dmapper, OpenshiftInfraNamespace, tsbRegistrationTemplateName, tsbNamespace, map[string]string{
		"TSB_NAMESPACE": tsbNamespace,
		"CA_BUNDLE":     serviceCAString,
	}, true); err != nil {
		return errors.NewError("cannot register the template service broker").WithCause(err)
	}

	return nil
}
