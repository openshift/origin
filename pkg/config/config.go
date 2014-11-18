package config

import (
	"encoding/json"
	"fmt"
	"reflect"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/meta"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"

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
	// Unmarshal the Config JSON manually instead of using runtime.Decode()
	conf := struct {
		Items []json.RawMessage `json:"items" yaml:"items"`
	}{}

	if err := json.Unmarshal(data, &conf); err != nil {
		return nil, fmt.Errorf("Unable to parse Config: %v", err)
	}

	if len(conf.Items) == 0 {
		return nil, fmt.Errorf("Config.items is empty")
	}

	for i, item := range conf.Items {
		itemResult := ApplyResult{}

		if item == nil || (len(item) == 4 && string(item) == "null") {
			itemResult.Error = fmt.Errorf("Config.items[%v] is null", i)
			result = append(result, itemResult)
			continue
		}

		itemBase := BaseConfigItem{}

		err = json.Unmarshal(item, &itemBase)
		if err != nil {
			itemResult.Error = fmt.Errorf("Unable to parse Config item: %v", err)
			result = append(result, itemResult)
			continue
		}

		if itemBase.Kind == "" {
			itemResult.Error = fmt.Errorf("Config.items[%v] has an empty 'kind'", i)
			result = append(result, itemResult)
			continue
		}

		if itemBase.Name == "" {
			itemResult.Error = fmt.Errorf("Config.items[%v] has an empty 'name'", i)
			result = append(result, itemResult)
			continue
		}

		client, path, err := getClientAndPath(itemBase.Kind, storage)
		if err != nil {
			itemResult.Error = fmt.Errorf("Config.items[%v]: %v", i, err)
			result = append(result, itemResult)
			continue
		}
		if client == nil {
			itemResult.Error = fmt.Errorf("Config.items[%v]: Unknown client for 'kind=%v'", i, itemBase.Kind)
			result = append(result, itemResult)
			continue
		}

		jsonResource, err := item.MarshalJSON()
		if err != nil {
			itemResult.Error = fmt.Errorf("%v", err)
			continue
		}

		request := client.Verb("POST").Namespace(namespace).Path(path).Body(jsonResource)
		if err = request.Do().Error(); err != nil {
			itemResult.Error = err
		} else {
			itemResult.Message = fmt.Sprintf("Creation succeeded for %v with 'id=%v'", itemBase.Kind, itemBase.Name)
		}
		result = append(result, itemResult)
	}
	return
}

func DecodeConfigItem(in runtime.RawExtension, m meta.RESTMapper, t runtime.ObjectTyper) (runtime.Object, *meta.RESTMapping, error) {
	version, kind, err := t.DataVersionAndKind(in.RawJSON)
	if err != nil {
		// TODO: Make this use ValidationErrorList
		return nil, nil, err
	}
	mapping, err := m.RESTMapping(version, kind)
	if err != nil {
		// TODO: Make this use ValidationErrorList
		return nil, nil, err
	}
	obj, err := mapping.Codec.Decode(in.RawJSON)
	return obj, mapping, err
}

// AddConfigLabels adds new label(s) to all resources defined in the given Config.
func AddConfigLabels(c *api.Config, labels labels.Set, m meta.RESTMapper, t runtime.ObjectTyper) error {
	for i := range c.Items {
		obj, _, err := DecodeConfigItem(c.Items[i], m, t)
		if err != nil {
			return fmt.Errorf("Unable to decode Template.Items[%v]: %v", i, err)
		}
		switch t := obj.(type) {
		case *kapi.Pod:
			if err := mergeMaps(&t.Labels, labels, ErrorOnDifferentDstKeyValue); err != nil {
				return fmt.Errorf("Unable to add labels to Template.Items[%v] Pod.Labels: %v", i, err)
			}
		case *kapi.Service:
			if err := mergeMaps(&t.Labels, labels, ErrorOnDifferentDstKeyValue); err != nil {
				return fmt.Errorf("Unable to add labels to Template.Items[%v] Service.Labels: %v", i, err)
			}
		case *kapi.ReplicationController:
			if err := mergeMaps(&t.Labels, labels, ErrorOnDifferentDstKeyValue); err != nil {
				return fmt.Errorf("Unable to add labels to Template.Items[%v] ReplicationController.Labels: %v", i, err)
			}
			if err := mergeMaps(&t.DesiredState.PodTemplate.Labels, labels, ErrorOnDifferentDstKeyValue); err != nil {
				return fmt.Errorf("Unable to add labels to Template.Items[%v] ReplicationController.DesiredState.PodTemplate.Labels: %v", i, err)
			}
		case *deployapi.Deployment:
			if err := mergeMaps(&t.Labels, labels, ErrorOnDifferentDstKeyValue); err != nil {
				return fmt.Errorf("Unable to add labels to Template.Items[%v] Deployment.Labels: %v", i, err)
			}
			if err := mergeMaps(&t.ControllerTemplate.PodTemplate.Labels, labels, ErrorOnDifferentDstKeyValue); err != nil {
				return fmt.Errorf("Unable to add labels to Template.Items[%v] ControllerTemplate.PodTemplate.Labels: %v", i, err)
			}
		case *deployapi.DeploymentConfig:
			if err := mergeMaps(&t.Labels, labels, ErrorOnDifferentDstKeyValue); err != nil {
				return fmt.Errorf("Unable to add labels to Template.Items[%v] DeploymentConfig.Labels: %v", i, err)
			}
			if err := mergeMaps(&t.Template.ControllerTemplate.PodTemplate.Labels, labels, ErrorOnDifferentDstKeyValue); err != nil {
				return fmt.Errorf("Unable to add labels to Template.Items[%v] Template.ControllerTemplate.PodTemplate.Labels: %v", i, err)
			}
		default:
			// Unknown generic object. Try to find "Labels" field in it.
			obj := reflect.ValueOf(c.Items[i])

			if obj.Kind() == reflect.Interface || obj.Kind() == reflect.Ptr {
				obj = obj.Elem()
			}
			if obj.Kind() != reflect.Struct {
				return fmt.Errorf("Template.Items[%v]: Invalid object kind. Expected: Struct, got:", i, obj.Kind())
			}

			obj = obj.FieldByName("Labels")
			if obj.IsValid() {
				// Merge labels into the Template.Items[i].Labels field.
				if err := mergeMaps(obj.Interface(), labels, ErrorOnDifferentDstKeyValue); err != nil {
					return fmt.Errorf("Unable to add labels to Template.Items[%v] GenericObject.Labels: %v", i, err)
				}
			}
		}
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
