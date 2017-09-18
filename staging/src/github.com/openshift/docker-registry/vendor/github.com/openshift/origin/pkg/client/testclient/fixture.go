package testclient

import (
	"io/ioutil"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
	kapi "k8s.io/kubernetes/pkg/api"
)

// ReadObjectsFromPath reads objects from the specified file for testing.
func ReadObjectsFromPath(path, namespace string, decoder runtime.Decoder, typer runtime.ObjectTyper) ([]runtime.Object, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	data, err = yaml.ToJSON(data)
	if err != nil {
		return nil, err
	}
	obj, err := runtime.Decode(decoder, data)
	if err != nil {
		return nil, err
	}
	if !meta.IsListType(obj) {
		if err := setNamespace(typer, obj, namespace); err != nil {
			return nil, err
		}
		return []runtime.Object{obj}, nil
	}
	list, err := meta.ExtractList(obj)
	if err != nil {
		return nil, err
	}
	errs := runtime.DecodeList(list, decoder)
	if len(errs) > 0 {
		return nil, errs[0]
	}
	for _, o := range list {
		if err := setNamespace(typer, o, namespace); err != nil {
			return nil, err
		}
	}
	return list, nil
}

func setNamespace(typer runtime.ObjectTyper, obj runtime.Object, namespace string) error {
	itemMeta, err := meta.Accessor(obj)
	if err != nil {
		return err
	}
	gvks, _, err := typer.ObjectKinds(obj)
	if err != nil {
		return err
	}
	group, err := kapi.Registry.Group(gvks[0].Group)
	if err != nil {
		return err
	}
	mapping, err := group.RESTMapper.RESTMapping(gvks[0].GroupKind(), gvks[0].Version)
	if err != nil {
		return err
	}
	switch mapping.Scope.Name() {
	case meta.RESTScopeNameNamespace:
		if len(itemMeta.GetNamespace()) == 0 {
			itemMeta.SetNamespace(namespace)
		}
	}

	return nil
}
