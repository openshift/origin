package openshift

import (
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kerrorutils "k8s.io/apimachinery/pkg/util/errors"
	kapi "k8s.io/kubernetes/pkg/api"

	"github.com/openshift/origin/pkg/client"
	configcmd "github.com/openshift/origin/pkg/config/cmd"
	genappcmd "github.com/openshift/origin/pkg/generate/app/cmd"
	"github.com/openshift/origin/pkg/oc/bootstrap/docker/errors"
)

func instantiateTemplate(client client.Interface, mapper configcmd.Mapper, templateNamespace, templateName, targetNamespace string, params map[string]string, ignoreExistsErrors bool) error {
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
		filteredErrs := []error{}
		for _, err := range errs {
			if kerrors.IsAlreadyExists(err) && ignoreExistsErrors {
				continue
			}
			filteredErrs = append(filteredErrs, err)
		}
		if len(filteredErrs) == 0 {
			return nil
		}
		err = kerrorutils.NewAggregate(filteredErrs)
		return errors.NewError("cannot create objects from template %s/%s", templateNamespace, templateName).WithCause(err)
	}

	return nil
}
