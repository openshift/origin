package config

import (
	"fmt"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	errs "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	kmeta "github.com/GoogleCloudPlatform/kubernetes/pkg/api/meta"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/config/api"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	"github.com/openshift/origin/pkg/util"
)

// ApplyResult holds the response from the REST server and potential errors
type ApplyResult struct {
	Errors  errs.ValidationErrorList
	Message string
}

// reportError reports the single item validation error and properly set the
// prefix and index to match the Config item JSON index
func reportError(allErrs *errs.ValidationErrorList, index int, err errs.ValidationError) {
	i := errs.ValidationErrorList{}
	*allErrs = append(*allErrs, append(i, &err).PrefixIndex(index).Prefix("item")...)
}

// Apply creates and manages resources defined in the Config. The create process
// won't stop on error, but it will finish the job and then return error and for
// each item in the config an error and status message string.
func Apply(namespace string, data []byte, clientFunc func(*kmeta.RESTMapping) (*kubectl.RESTHelper, error)) ([]ApplyResult, error) {
	confObj, _, err := DecodeDataToObject(data)
	if err != nil {
		return nil, fmt.Errorf("decoding failed, %s", err.Error())
	}

	conf, ok := confObj.(*api.Config)
	if !ok {
		return nil, fmt.Errorf("unable to convert object to Config")
	}

	if len(conf.Items) == 0 {
		return nil, fmt.Errorf("Config items must be not empty")
	}

	result := []ApplyResult{}
	for i, item := range conf.Items {
		itemErrors := errs.ValidationErrorList{}
		message := ""

		itemBase, mapping, err := DecodeDataToObject(item.RawJSON)
		if err != nil {
			reportError(&itemErrors, i, errs.ValidationError{
				errs.ValidationErrorTypeInvalid,
				"decode",
				err,
				fmt.Sprintf("unable to decode: %v", item),
			})
			result = append(result, ApplyResult{itemErrors.Prefix("Config"), message})
			continue
		}

		client, err := clientFunc(mapping)
		if err != nil {
			reportError(&itemErrors, i, *errs.NewFieldNotSupported("client", itemBase))
			result = append(result, ApplyResult{itemErrors.Prefix("Config"), message})
			continue
		}

		jsonResource, err := mapping.Encode(itemBase)
		if err != nil {
			reportError(&itemErrors, i, errs.ValidationError{
				errs.ValidationErrorTypeInvalid,
				"encode",
				err,
				fmt.Sprintf("unable to encode: %v", item),
			})
			result = append(result, ApplyResult{itemErrors.Prefix("Config"), message})
			continue
		}

		if err := client.Create(namespace, true, jsonResource); err != nil {
			reportError(&itemErrors, i, errs.ValidationError{
				errs.ValidationErrorTypeInvalid,
				"create",
				err,
				fmt.Sprintf("unable to create: %v", string(jsonResource)),
			})
		} else {
			itemName, _ := mapping.MetadataAccessor.Name(itemBase)
			message = fmt.Sprintf("Creation succeeded for %s with name %s", mapping.Kind, itemName)
		}
		result = append(result, ApplyResult{itemErrors.Prefix("Config"), message})
	}
	return result, nil
}

// addReplicationControllerNestedLabels adds new label(s) to a nested labels of a single ReplicationController object
func addReplicationControllerNestedLabels(obj *kapi.ReplicationController, labels labels.Set) error {
	if obj.Spec.Template.Labels == nil {
		obj.Spec.Template.Labels = make(map[string]string)
	}
	if err := util.MergeInto(obj.Spec.Template.Labels, labels, util.ErrorOnDifferentDstKeyValue); err != nil {
		return fmt.Errorf("unable to add labels to Template.ReplicationController.Spec.Template: %v", err)
	}
	if err := util.MergeInto(obj.Spec.Template.Labels, obj.Spec.Selector, util.ErrorOnDifferentDstKeyValue); err != nil {
		return fmt.Errorf("unable to add labels to Template.ReplicationController.Spec.Template: %v", err)
	}
	// Selector and Spec.Template.Labels must be equal
	if obj.Spec.Selector == nil {
		obj.Spec.Selector = make(map[string]string)
	}
	if err := util.MergeInto(obj.Spec.Selector, obj.Spec.Template.Labels, util.ErrorOnDifferentDstKeyValue); err != nil {
		return fmt.Errorf("unable to add labels to Template.ReplicationController.Spec.Selector: %v", err)
	}
	return nil
}

// addDeploymentNestedLabels adds new label(s) to a nested labels of a single Deployment object
func addDeploymentNestedLabels(obj *deployapi.Deployment, labels labels.Set) error {
	if obj.ControllerTemplate.Template.Labels == nil {
		obj.ControllerTemplate.Template.Labels = make(map[string]string)
	}
	if err := util.MergeInto(obj.ControllerTemplate.Template.Labels, labels, util.ErrorOnDifferentDstKeyValue); err != nil {
		return fmt.Errorf("unable to add labels to Template.Deployment.ControllerTemplate.Template: %v", err)
	}
	return nil
}

// addDeploymentConfigNestedLabels adds new label(s) to a nested labels of a single DeploymentConfig object
func addDeploymentConfigNestedLabels(obj *deployapi.DeploymentConfig, labels labels.Set) error {
	if obj.Template.ControllerTemplate.Template.Labels == nil {
		obj.Template.ControllerTemplate.Template.Labels = make(map[string]string)
	}
	if err := util.MergeInto(obj.Template.ControllerTemplate.Template.Labels, labels, util.ErrorOnDifferentDstKeyValue); err != nil {
		return fmt.Errorf("unable to add labels to Template.DeploymentConfig.Template.ControllerTemplate.Template: %v", err)
	}
	return nil
}

// AddObjectLabels adds new label(s) to a single runtime.Object
func AddObjectLabels(obj runtime.Object, labels labels.Set) error {
	if labels == nil {
		// Nothing to add
		return nil
	}

	accessor, err := kmeta.Accessor(obj)
	if err != nil {
		return err
	}

	metaLabels := accessor.Labels()
	if metaLabels == nil {
		metaLabels = make(map[string]string)
	}

	if err := util.MergeInto(metaLabels, labels, util.ErrorOnDifferentDstKeyValue); err != nil {
		return fmt.Errorf("unable to add labels to Template.%s: %v", accessor.Kind(), err)
	}
	accessor.SetLabels(metaLabels)

	// Handle nested Labels
	switch objType := obj.(type) {
	case *kapi.ReplicationController:
		return addReplicationControllerNestedLabels(objType, labels)
	case *deployapi.Deployment:
		return addDeploymentNestedLabels(objType, labels)
	case *deployapi.DeploymentConfig:
		return addDeploymentConfigNestedLabels(objType, labels)
	}

	return nil
}

// AddConfigLabels adds new label(s) to all resources defined in the given Config.
func AddConfigLabels(c *api.Config, labels labels.Set) errs.ValidationErrorList {
	itemErrors := errs.ValidationErrorList{}
	for i, in := range c.Items {
		obj, mapping, err := DecodeDataToObject(in.RawJSON)
		if err != nil {
			reportError(&itemErrors, i, *errs.NewFieldInvalid("decode", err, fmt.Sprintf("error decoding %v", in)))
		}
		if err := AddObjectLabels(obj, labels); err != nil {
			reportError(&itemErrors, i, *errs.NewFieldInvalid("labels", err, fmt.Sprintf("error applying labels %v to %v", labels, obj)))
		}
		item, err := mapping.Encode(obj)
		if err != nil {
			reportError(&itemErrors, i, *errs.NewFieldInvalid("encode", err, fmt.Sprintf("error encoding %v", in)))
		}
		c.Items[i] = runtime.RawExtension{RawJSON: item}
	}
	return itemErrors.Prefix("Config")
}
