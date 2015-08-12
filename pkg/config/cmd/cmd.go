package cmd

import (
	"fmt"
	"io"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/meta"
	"k8s.io/kubernetes/pkg/kubectl/resource"
	"k8s.io/kubernetes/pkg/runtime"
)

// Bulk provides helpers for iterating over a list of items
type Bulk struct {
	Mapper            meta.RESTMapper
	Typer             runtime.ObjectTyper
	RESTClientFactory func(mapping *meta.RESTMapping) (resource.RESTClient, error)
	After             func(*resource.Info, error)
}

func NewPrintNameOrErrorAfter(out, errs io.Writer) func(*resource.Info, error) {
	return func(info *resource.Info, err error) {
		if err == nil {
			fmt.Fprintf(out, "%s/%s\n", info.Mapping.Resource, info.Name)
		} else {
			fmt.Fprintf(errs, "Error: %v\n", err)
		}
	}
}

// Create attempts to create each item generically, gathering all errors in the
// event a failure occurs. The contents of list will be updated to include the
// version from the server.
func (b *Bulk) Create(list *kapi.List, namespace string) []error {
	resourceMapper := &resource.Mapper{b.Typer, b.Mapper, resource.ClientMapperFunc(b.RESTClientFactory)}
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
		data, err := info.Mapping.Codec.Encode(item)
		if err != nil {
			errs = append(errs, err)
			after(info, err)
			continue
		}
		obj, err := resource.NewHelper(info.Client, info.Mapping).Create(namespace, false, data)
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
