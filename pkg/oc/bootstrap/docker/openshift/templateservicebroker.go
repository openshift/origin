package openshift

import (
	"fmt"
	"time"

	"github.com/golang/glog"

	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	kapi "k8s.io/kubernetes/pkg/api"

	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/openshift/origin/pkg/cmd/util/variable"
	"github.com/openshift/origin/pkg/oc/bootstrap/docker/errors"
)

const (
	tsbNamespace        = "openshift-template-service-broker"
	tsbTemplateName     = "template-service-broker"
	tsbTemplateLocation = "examples/templateservicebroker/templateservicebroker-template.yaml"
)

// InstallServiceCatalog checks whether the template service broker is installed and installs it if not already installed
func (h *Helper) InstallTemplateServiceBroker(f *clientcmd.Factory, imageFormat string, serverLogLevel int) error {
	osClient, kubeClient, err := f.Clients()
	if err != nil {
		return errors.NewError("cannot obtain API clients").WithCause(err).WithDetails(h.OriginLog())
	}

	// create the namespace if needed.  This is a reserved namespace, so you can't do it with the create project request
	if _, err := kubeClient.Core().Namespaces().Create(&kapi.Namespace{ObjectMeta: metav1.ObjectMeta{Name: tsbNamespace}}); err != nil && !kapierrors.IsAlreadyExists(err) {
		return errors.NewError("cannot create template service broker project").WithCause(err)
	}

	// create the template in the tsbNamespace to make it easy to instantiate
	if err := ImportObjects(f, tsbNamespace, tsbTemplateLocation); err != nil {
		return errors.NewError("cannot create template service broker template").WithCause(err)
	}

	// create the actual resources required
	imageTemplate := variable.NewDefaultImageTemplate()
	imageTemplate.Format = imageFormat
	imageTemplate.Latest = false

	if err = instantiateTemplate(osClient, clientcmd.ResourceMapper(f), tsbNamespace, tsbTemplateName, tsbNamespace, map[string]string{
		"IMAGE":    imageTemplate.ExpandOrDie(""),
		"LOGLEVEL": fmt.Sprint(serverLogLevel),
	}, true); err != nil {
		return errors.NewError("cannot instantiate logger accounts").WithCause(err)
	}

	// Wait for the apiserver endpoint to become available
	err = wait.Poll(5*time.Second, 600*time.Second, func() (bool, error) {
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
