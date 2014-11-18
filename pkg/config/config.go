package config

import (
	"encoding/json"
	"fmt"
	"reflect"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/meta"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/openshift/origin/pkg/api/latest"

	clientapi "github.com/openshift/origin/pkg/cmd/client/api"
	"github.com/openshift/origin/pkg/config/api"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
)

type ApplyResult struct {
	Error   error
	Message string
}

type BaseConfigItem struct {
	kapi.TypeMeta   `json:",inline" yaml:",inline"`
	kapi.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

// Apply creates and manages resources defined in the Config. The create process wont
// stop on error, but it will finish the job and then return error and for each item
// in the config a error and status message string.
func Apply(namespace string, data []byte, storage clientapi.ClientMappings) (result []ApplyResult, err error) {
	typer := kapi.Scheme
	mapper := latest.RESTMapper

	version, kind, err := typer.DataVersionAndKind(data)
	// TODO: Add proper ValidationErrorList here
	if err != nil {
		return nil, fmt.Errorf("DataVersionAndKind: %v", err)
	}

	mapping, err := mapper.RESTMapping(version, kind)
	// TODO: Add proper ValidationErrorList here
	if err != nil {
		return nil, fmt.Errorf("RESTMapping: %v", err)
	}

	confObj, err := mapping.Codec.Decode(data)
	// TODO: Add proper ValidationErrorList here
	if err != nil {
		return nil, fmt.Errorf("Decode: %v", err)
	}

	conf, ok := confObj.(*api.Config)
	if !ok {
		return nil, fmt.Errorf("Invalid Config")
	}

	if len(conf.Items) == 0 {
		return nil, fmt.Errorf("Config.items is empty")
	}

	for i, item := range conf.Items {
		itemResult := ApplyResult{}

		itemBase, mapping, err := DecodeConfigItem(item)
		if err != nil {
			itemResult.Error = fmt.Errorf("Unable to parse Config item: %v", err)
			result = append(result, itemResult)
			continue
		}

		client, path, err := getClientAndPath(mapping.Kind, storage)
		if err != nil {
			itemResult.Error = fmt.Errorf("Config.items[%v]: %v", i, err)
			result = append(result, itemResult)
			continue
		}
		if client == nil {
			itemResult.Error = fmt.Errorf("Config.items[%v]: Unknown client for 'kind=%v'", i, mapping.Kind)
			result = append(result, itemResult)
			continue
		}

		jsonResource, err := mapping.Encode(itemBase)
		if err != nil {
			itemResult.Error = fmt.Errorf("%v", err)
			continue
		}

		request := client.Verb("POST").Namespace(namespace).Path(path).Body(jsonResource)
		if err = request.Do().Error(); err != nil {
			itemResult.Error = err
		} else {
			itemName, _ := mapping.MetadataAccessor.Name(itemBase)
			itemResult.Message = fmt.Sprintf("Creation succeeded for %v with 'name=%v'", mapping.Kind, itemName)
		}
		result = append(result, itemResult)
	}
	return
}

func DecodeConfigItem(in runtime.RawExtension) (runtime.Object, *meta.RESTMapping, error) {
	typer := kapi.Scheme
	mapper := latest.RESTMapper
	version, kind, err := typer.DataVersionAndKind(in.RawJSON)
	// TODO: Add proper ValidationErrorList here
	if err != nil {
		return nil, nil, fmt.Errorf("DataVersionAndKind: %v", err)
	}

	mapping, err := mapper.RESTMapping(version, kind)
	// TODO: Add proper ValidationErrorList here
	if err != nil {
		return nil, nil, fmt.Errorf("RESTMapping: %v", err)
	}

	obj, err := mapping.Codec.Decode(in.RawJSON)
	// TODO: Add proper ValidationErrorList here
	if err != nil {
		return nil, nil, fmt.Errorf("Decode: %v", err)
	}

	return obj, mapping, err
}

func addLabelError(kind string, err error) error {
	return fmt.Errorf("Enable to add labels to Template.%s item: %v", kind, err)
}

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
			return addLabelError("RepliacationController.PodTemplate", err)
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
		// Unknown generic object. Try to find "Labels" field in it.
		unknownObj := reflect.ValueOf(obj)

		if unknownObj.Kind() == reflect.Interface || unknownObj.Kind() == reflect.Ptr {
			unknownObj = unknownObj.Elem()
		}

		if unknownObj.Kind() != reflect.Struct {
			return fmt.Errorf("Template.Items[%v]: Invalid unknownObject kind. Expected: Struct, got:", unknownObj.Kind())
		}

		unknownObj = unknownObj.FieldByName("Labels")
		if unknownObj.IsValid() {
			// Merge labels into the Template.Items[i].Labels field.
			if err := mergeMaps(unknownObj.Interface(), labels, ErrorOnDifferentDstKeyValue); err != nil {
				return fmt.Errorf("Unable to add labels to Template.Items GenericObject.Labels: %v", err)
			}
		}
	}

	return nil
}

// AddConfigLabels adds new label(s) to all resources defined in the given Config.
func AddConfigLabels(c *api.Config, labels labels.Set) error {
	for _, in := range c.Items {
		obj, _, _ := DecodeConfigItem(in)
		AddConfigLabel(obj, labels)
	}
	return nil
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

// getClientAndPath returns the RESTClient and path defined for a given
// resource kind. Returns an error when no RESTClient is found.
func getClientAndPath(kind string, mappings clientapi.ClientMappings) (clientapi.RESTClient, string, error) {
	for k, m := range mappings {
		if m.Kind == kind {
			return m.Client, k, nil
		}
	}
	return nil, "", fmt.Errorf("No client found for 'kind=%v'", kind)
}

// reportError provides a human-readable error message that include the Config
// item JSON representation.
func reportError(item interface{}, message string) error {
	itemJSON, _ := json.Marshal(item)
	return fmt.Errorf(message+": %s", string(itemJSON))
}
