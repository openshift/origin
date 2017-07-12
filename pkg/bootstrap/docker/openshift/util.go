package openshift

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	kapi "k8s.io/kubernetes/pkg/api"

	"github.com/openshift/origin/pkg/bootstrap/docker/errors"
	"github.com/openshift/origin/pkg/client"
	configcmd "github.com/openshift/origin/pkg/config/cmd"
	genappcmd "github.com/openshift/origin/pkg/generate/app/cmd"
)

func instantiateTemplate(client client.Interface, mapper configcmd.Mapper, templateNamespace, templateName, targetNamespace string, params map[string]string) error {
	template, err := client.Templates(templateNamespace).Get(templateName, metav1.GetOptions{})
	if err != nil {
		return errors.NewError("cannot retrieve template %q from namespace %q", templateName, templateNamespace).WithCause(err)
	}

	// process the template
	result, err := genappcmd.TransformTemplate(template, client, targetNamespace, params, false)
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
