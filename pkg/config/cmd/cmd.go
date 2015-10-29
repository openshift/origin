package cmd

import (
	"fmt"
	"io"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/meta"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/resource"
	"k8s.io/kubernetes/pkg/runtime"
)

// Bulk provides helpers for iterating over a list of items
type Bulk struct {
	Mapper            meta.RESTMapper
	Typer             runtime.ObjectTyper
	RESTClientFactory func(mapping *meta.RESTMapping) (resource.RESTClient, error)
	After             func(*resource.Info, error)
	Retry             func(info *resource.Info, err error) runtime.Object
}

func NewPrintNameOrErrorAfter(mapper meta.RESTMapper, short bool, operation string, out, errs io.Writer) func(*resource.Info, error) {
	return func(info *resource.Info, err error) {
		if err == nil {
			//cmdutil.PrintSuccess(mapper, short, out, info.Mapping.Resource, info.Name, operation)
			cmdutil.PrintSuccess(mapper, short, out, info.Mapping.Kind, info.Name, operation)
		} else {
			fmt.Fprintf(errs, "error: %v\n", err)
		}
	}
}

func NewPrintErrorAfter(mapper meta.RESTMapper, errs io.Writer) func(*resource.Info, error) {
	return func(info *resource.Info, err error) {
		if err != nil {
			fmt.Fprintf(errs, "error: %v\n", err)
		}
	}
}

func encodeAndCreate(info *resource.Info, namespace string, obj runtime.Object) (runtime.Object, error) {
	data, err := info.Mapping.Codec.Encode(obj)
	if err != nil {
		return nil, err
	}
	return resource.NewHelper(info.Client, info.Mapping).Create(namespace, false, data)
}

// Create attempts to create each item generically, gathering all errors in the
// event a failure occurs. The contents of list will be updated to include the
// version from the server.
func (b *Bulk) Create(list *kapi.List, namespace string) []error {
	resourceMapper := &resource.Mapper{ObjectTyper: b.Typer, RESTMapper: b.Mapper, ClientMapper: resource.ClientMapperFunc(b.RESTClientFactory)}
	after := b.After
	if after == nil {
		after = func(*resource.Info, error) {}
	}

	errs := []error{}
	for i, item := range list.Items {
		info, err := resourceMapper.InfoForObject(item)
		if err != nil {
			errs = append(errs, err)
			after(info, err)
			continue
		}
		obj, err := encodeAndCreate(info, namespace, item)
		if err != nil && b.Retry != nil {
			if obj := b.Retry(info, err); obj != nil {
				obj, err = encodeAndCreate(info, namespace, obj)
			}
		}
		if err != nil {
			errs = append(errs, err)
			after(info, err)
			continue
		}
		info.Refresh(obj, true)
		list.Items[i] = obj
		after(info, nil)
	}
	return errs
}
