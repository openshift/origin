package testclient

import (
	"fmt"
	"io/ioutil"
	"reflect"
	"strings"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/meta"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util/yaml"
)

type ObjectRetriever interface {
	Kind(kind, name string) (runtime.Object, error)
	Add(runtime.Object) error
}

func ObjectReaction(o ObjectRetriever, mapper meta.RESTMapper) ReactionFunc {
	return func(action FakeAction) (runtime.Object, error) {
		segments := strings.SplitN(action.Action, "-", 2)
		if len(segments) == 1 {
			return nil, fmt.Errorf("unrecognized action, need two segments <verb>-<noun>: %s", action.Action)
		}
		verb, resource := segments[0], segments[1]
		_, kind, err := mapper.VersionAndKindForResource(resource)
		if err != nil {
			return nil, fmt.Errorf("unrecognized action %s: %v", resource, err)
		}
		switch verb {
		case "list":
			return o.Kind(kind+"List", "")
		case "get":
			if s, ok := action.Value.(string); ok && action.Value != nil {
				return o.Kind(kind, s)
			}
			return o.Kind(kind, "unknown")
		case "create", "update", "delete":
		default:
			return nil, fmt.Errorf("no reaction implemented for %s", action.Action)
		}
		return nil, nil
	}
}

type objects struct {
	types   map[string][]runtime.Object
	last    map[string]int
	typer   runtime.ObjectTyper
	creater runtime.ObjectCreater
}

func NewObjects(scheme *runtime.Scheme) ObjectRetriever {
	return objects{
		types:   make(map[string][]runtime.Object),
		last:    make(map[string]int),
		typer:   scheme,
		creater: scheme,
	}
}

func AddObjectsFromPath(path string, o ObjectRetriever) error {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	data, err = yaml.ToJSON(data)
	if err != nil {
		return err
	}
	obj, err := api.Codec.Decode(data)
	if err != nil {
		return err
	}
	if err := o.Add(obj); err != nil {
		return err
	}
	return nil
}

func (o objects) Kind(kind, name string) (runtime.Object, error) {
	empty, _ := o.creater.New("", kind)
	nilValue := reflect.Zero(reflect.TypeOf(empty)).Interface().(runtime.Object)

	arr, ok := o.types[kind]
	if !ok {
		if strings.HasSuffix(kind, "List") {
			itemKind := kind[:len(kind)-4]
			arr, ok := o.types[itemKind]
			if !ok {
				return empty, nil
			}
			out, err := o.creater.New("", kind)
			if err != nil {
				return nilValue, err
			}
			if err := runtime.SetList(out, arr); err != nil {
				return nilValue, err
			}
			return out, nil
		}
		return nilValue, errors.NewNotFound(kind, name)
	}

	index := o.last[kind]
	if index >= len(arr) {
		index = len(arr) - 1
	}
	if index < 0 {
		return nilValue, errors.NewNotFound(kind, name)
	}
	out := arr[index]
	o.last[kind] = index + 1

	if status, ok := out.(*api.Status); ok {
		if status.Details != nil {
			status.Details.Kind = kind
		}
		if status.Status != api.StatusSuccess {
			return nilValue, &errors.StatusError{*status}
		}
	}

	return out, nil
}

func (o objects) Add(obj runtime.Object) error {
	_, kind, err := o.typer.ObjectVersionAndKind(obj)
	if err != nil {
		return err
	}

	switch {
	case runtime.IsListType(obj):
		if kind != "List" {
			o.types[kind] = append(o.types[kind], obj)
		}

		list, err := runtime.ExtractList(obj)
		if err != nil {
			return err
		}
		for _, obj := range list {
			if err := o.Add(obj); err != nil {
				return err
			}
		}
	default:
		if status, ok := obj.(*api.Status); ok && status.Details != nil {
			kind = status.Details.Kind
		}
		o.types[kind] = append(o.types[kind], obj)
	}

	return nil
}
