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
	*allErrs = append(*allErrs, append(i, err).PrefixIndex(index).Prefix("item")...)
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
			})
			result = append(result, ApplyResult{itemErrors.Prefix("Config"), message})
			continue
		}

		client, err := clientFunc(mapping)
		if err != nil {
			reportError(&itemErrors, i, errs.NewFieldNotSupported("client", err))
			result = append(result, ApplyResult{itemErrors.Prefix("Config"), message})
			continue
		}

		jsonResource, err := mapping.Encode(itemBase)
		if err != nil {
			reportError(&itemErrors, i, errs.ValidationError{
				errs.ValidationErrorTypeInvalid,
				"encode",
				err,
			})
			result = append(result, ApplyResult{itemErrors.Prefix("Config"), message})
			continue
		}

		if err := client.Create(namespace, true, jsonResource); err != nil {
			reportError(&itemErrors, i, errs.ValidationError{
				errs.ValidationErrorTypeInvalid,
				"create",
				err,
			})
		} else {
			itemName, _ := mapping.MetadataAccessor.Name(itemBase)
			message = fmt.Sprintf("Creation succeeded for %s with name %s", mapping.Kind, itemName)
		}
		result = append(result, ApplyResult{itemErrors.Prefix("Config"), message})
	}
	return result, nil
}

func addLabelError(kind string, err error) error {
	return fmt.Errorf("Enable to add labels to Template.%s item: %v", kind, err)
}

// AddConfigLabel adds new label(s) to a single Object
// TODO: AddConfigLabel should add labels into all objects that has ObjectMeta
func AddConfigLabel(obj runtime.Object, labels labels.Set) error {
	switch t := obj.(type) {
	case *kapi.Pod:
		if err := util.MergeInto(&t.Labels, labels, util.ErrorOnDifferentDstKeyValue); err != nil {
			return addLabelError("Pod", err)
		}
	case *kapi.Service:
		if err := util.MergeInto(&t.Labels, labels, util.ErrorOnDifferentDstKeyValue); err != nil {
			return addLabelError("Service", err)
		}
	case *kapi.ReplicationController:
		if err := util.MergeInto(&t.Labels, labels, util.ErrorOnDifferentDstKeyValue); err != nil {
			return addLabelError("ReplicationController", err)
		}
		if err := util.MergeInto(&t.DesiredState.PodTemplate.Labels, labels, util.ErrorOnDifferentDstKeyValue); err != nil {
			return addLabelError("ReplicationController.PodTemplate", err)
		}
		if err := util.MergeInto(&t.DesiredState.PodTemplate.Labels, t.DesiredState.ReplicaSelector, util.ErrorOnDifferentDstKeyValue); err != nil {
			return addLabelError("ReplicationController.ReplicaSelector", err)
		}
		// The ReplicaSelector and DesiredState.PodTemplate.Labels need to make
		// create succeed
		if err := util.MergeInto(&t.DesiredState.ReplicaSelector, t.DesiredState.PodTemplate.Labels, util.ErrorOnDifferentDstKeyValue); err != nil {
			return addLabelError("ReplicationController.PodTemplate", err)
		}
	case *deployapi.Deployment:
		if err := util.MergeInto(&t.Labels, labels, util.ErrorOnDifferentDstKeyValue); err != nil {
			return addLabelError("Deployment", err)
		}
		if err := util.MergeInto(&t.ControllerTemplate.PodTemplate.Labels, labels, util.ErrorOnDifferentDstKeyValue); err != nil {
			return addLabelError("Deployment.ControllerTemplate.PodTemplate", err)
		}
	case *deployapi.DeploymentConfig:
		if err := util.MergeInto(&t.Labels, labels, util.ErrorOnDifferentDstKeyValue); err != nil {
			return addLabelError("DeploymentConfig", err)
		}
		if err := util.MergeInto(&t.Template.ControllerTemplate.PodTemplate.Labels, labels, util.ErrorOnDifferentDstKeyValue); err != nil {
			return addLabelError("DeploymentConfig.ControllerTemplate.PodTemplate", err)
		}
	default:
		// TODO: For unknown objects we should rather skip adding Labels as we don't
		//			 know where they are. Lets avoid using reflect for now and fix this
		//			 properly using ObjectMeta/RESTMapper/MetaAccessor
		return nil
	}

	return nil
}

// AddConfigLabels adds new label(s) to all resources defined in the given Config.
func AddConfigLabels(c *api.Config, labels labels.Set) errs.ValidationErrorList {
	itemErrors := errs.ValidationErrorList{}
	for i, in := range c.Items {
		obj, mapping, err := DecodeDataToObject(in.RawJSON)
		if err != nil {
			reportError(&itemErrors, i, errs.NewFieldInvalid("decode", err))
		}
		if err := AddConfigLabel(obj, labels); err != nil {
			reportError(&itemErrors, i, errs.NewFieldInvalid("labels", err))
		}
		item, err := mapping.Encode(obj)
		if err != nil {
			reportError(&itemErrors, i, errs.NewFieldInvalid("encode", err))
		}
		c.Items[i] = runtime.RawExtension{RawJSON: item}
	}
	return itemErrors.Prefix("Config")
}
