package config

import (
	"fmt"
	"reflect"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	errs "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	kmeta "github.com/GoogleCloudPlatform/kubernetes/pkg/api/meta"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/config/api"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
)

// ApplyResult holds the response from the REST server and potential errors
type ApplyResult struct {
	Errors  errs.ValidationErrorList
	Message string
}

// appendError reports the single item validation error and properly set the
// prefix and index to match the Config item JSON index
func appendError(allErrs *errs.ValidationErrorList, index int, err error) errs.ValidationErrorList {
	i := errs.ValidationErrorList{}
	*allErrs = append(*allErrs, append(i, err).PrefixIndex(index).Prefix("item")...)
	return *allErrs
}

// Apply creates and manages resources defined in the Config. The create process
// won't stop on error, but it will finish the job and then return error and for
// each item in the config an error and status message string.
func Apply(namespace string, data []byte, clientFunc func(*kmeta.RESTMapping) (*kubectl.RESTHelper, error)) ([]ApplyResult, error) {
	obj, err := kapi.Scheme.Decode(data)
	if err != nil {
		return nil, fmt.Errorf("decoding failed, %s", err.Error())
	}

	conf, ok := obj.(*api.Config)
	if !ok {
		return nil, fmt.Errorf("unable to convert object to Config")
	}

	result := []ApplyResult{}
	for i, item := range conf.Items {
		itemErrors := errs.ValidationErrorList{}

		var version, kind string
		var data []byte
		switch t := item.(type) {
		case *runtime.Unknown:
			version, kind = t.APIVersion, t.Kind
			data = t.RawJSON
		default:
			// we ignore the error because we treat objects opaquely
			version, kind, _ = kapi.Scheme.ObjectVersionAndKind(t)
		}

		if len(kind) == 0 {
			appendError(&itemErrors, i, errs.NewFieldRequired("kind", ""))
			result = append(result, ApplyResult{itemErrors.Prefix("Config"), ""})
			continue
		}

		mapping, err := latest.RESTMapper.RESTMapping(version, kind)
		if err != nil {
			appendError(&itemErrors, i, fmt.Errorf("unable to save objects defined for %q and %q: %v", version, kind, err))
			result = append(result, ApplyResult{itemErrors.Prefix("Config"), ""})
			continue
		}

		client, err := clientFunc(mapping)
		if err != nil {
			result = append(result, ApplyResult{appendError(&itemErrors, i, err).Prefix("Config"), ""})
			continue
		}

		if data == nil {
			data, err = mapping.Encode(item)
			if err != nil {
				result = append(result, ApplyResult{appendError(&itemErrors, i, err).Prefix("Config"), ""})
				continue
			}
		}

		if err := client.Create(namespace, true, data); err != nil {
			result = append(result, ApplyResult{appendError(&itemErrors, i, err).Prefix("Config"), ""})
			continue
		}

		name, err := mapping.MetadataAccessor.Name(item)
		if err != nil {
			name = "<unknown>"
		}
		message := fmt.Sprintf("Creation succeeded for %s with name %s", mapping.Kind, name)
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
		if err := mergeMaps(&t.Labels, labels, ErrorOnDifferentDstKeyValue); err != nil {
			return addLabelError("Pod", err)
		}
	case *kapi.Service:
		if err := mergeMaps(&t.Labels, labels, ErrorOnDifferentDstKeyValue); err != nil {
			return addLabelError("Service", err)
		}
	case *kapi.ReplicationController:
		if err := mergeMaps(&t.Labels, labels, ErrorOnDifferentDstKeyValue); err != nil {
			return addLabelError("ReplicationController", err)
		}
		if err := mergeMaps(&t.DesiredState.PodTemplate.Labels, labels, ErrorOnDifferentDstKeyValue); err != nil {
			return addLabelError("ReplicationController.PodTemplate", err)
		}
		if err := mergeMaps(&t.DesiredState.PodTemplate.Labels, t.DesiredState.ReplicaSelector, ErrorOnDifferentDstKeyValue); err != nil {
			return addLabelError("ReplicationController.ReplicaSelector", err)
		}
		// The ReplicaSelector and DesiredState.PodTemplate.Labels need to make
		// create succeed
		if err := mergeMaps(&t.DesiredState.ReplicaSelector, t.DesiredState.PodTemplate.Labels, ErrorOnDifferentDstKeyValue); err != nil {
			return addLabelError("ReplicationController.PodTemplate", err)
		}
	case *deployapi.Deployment:
		if err := mergeMaps(&t.Labels, labels, ErrorOnDifferentDstKeyValue); err != nil {
			return addLabelError("Deployment", err)
		}
		if err := mergeMaps(&t.ControllerTemplate.PodTemplate.Labels, labels, ErrorOnDifferentDstKeyValue); err != nil {
			return addLabelError("Deployment.ControllerTemplate.PodTemplate", err)
		}
	case *deployapi.DeploymentConfig:
		if err := mergeMaps(&t.Labels, labels, ErrorOnDifferentDstKeyValue); err != nil {
			return addLabelError("DeploymentConfig", err)
		}
		if err := mergeMaps(&t.Template.ControllerTemplate.PodTemplate.Labels, labels, ErrorOnDifferentDstKeyValue); err != nil {
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
		if err := AddConfigLabel(in, labels); err != nil {
			appendError(&itemErrors, i, errs.NewFieldInvalid("labels", err))
		}
	}
	return itemErrors.Prefix("Config")
}

// mergeMaps flags
const (
	OverwriteExistingDstKey     = 1 << iota
	ErrorOnExistingDstKey       = 1 << iota
	ErrorOnDifferentDstKeyValue = 1 << iota
)

// mergeMaps merges items from a src map into a dst map.
// Returns an error when the maps are not of the same type.
// Flags:
// - ErrorOnExistingDstKey
//     When set: Return an error if any of the dst keys is already set.
// - ErrorOnDifferentDstKeyValue
//     When set: Return an error if any of the dst keys is already set
//               to a different value than src key.
// - OverwriteDstKey
//     When set: Overwrite existing dst key value with src key value.
func mergeMaps(dst, src interface{}, flags int) error {
	dstVal := reflect.ValueOf(dst)
	srcVal := reflect.ValueOf(src)

	if dstVal.Kind() == reflect.Interface || dstVal.Kind() == reflect.Ptr {
		dstVal = dstVal.Elem()
	}
	if srcVal.Kind() == reflect.Interface || srcVal.Kind() == reflect.Ptr {
		srcVal = srcVal.Elem()
	}

	if !dstVal.IsValid() {
		return fmt.Errorf("Dst is not a valid value")
	}
	if dstVal.Kind() != reflect.Map {
		return fmt.Errorf("Dst is not a map")
	}

	dstTyp := dstVal.Type()
	srcTyp := srcVal.Type()
	if !dstTyp.AssignableTo(srcTyp) {
		return fmt.Errorf("Type mismatch, can't assign '%v' to '%v'", srcTyp, dstTyp)
	}

	if dstVal.IsNil() {
		if !dstVal.CanSet() {
			return fmt.Errorf("Dst value is (not addressable) nil, pass a pointer instead")
		}
		dstVal.Set(reflect.MakeMap(dstTyp))
	}

	for _, k := range srcVal.MapKeys() {
		if dstVal.MapIndex(k).IsValid() {
			if flags&ErrorOnExistingDstKey != 0 {
				return fmt.Errorf("ErrorOnExistingDstKey flag: Dst key already set to a different value, '%v'='%v'", k, dstVal.MapIndex(k))
			}
			if dstVal.MapIndex(k).String() != srcVal.MapIndex(k).String() {
				if flags&ErrorOnDifferentDstKeyValue != 0 {
					return fmt.Errorf("ErrorOnDifferentDstKeyValue flag: Dst key already set to a different value, '%v'='%v'", k, dstVal.MapIndex(k))
				}
				if flags&OverwriteExistingDstKey != 0 {
					dstVal.SetMapIndex(k, srcVal.MapIndex(k))
				}
			}
		} else {
			dstVal.SetMapIndex(k, srcVal.MapIndex(k))
		}
	}

	return nil
}
