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

// AfterFunc takes an info and an error, and returns true if processing should stop.
type AfterFunc func(*resource.Info, error) bool

// Bulk provides helpers for iterating over a list of items
type Bulk struct {
	Mapper            meta.RESTMapper
	Typer             runtime.ObjectTyper
	RESTClientFactory func(mapping *meta.RESTMapping) (resource.RESTClient, error)
	After             AfterFunc
	Retry             func(info *resource.Info, err error) runtime.Object
}

func NewPrintNameOrErrorAfter(mapper meta.RESTMapper, short bool, operation string, out, errs io.Writer) AfterFunc {
	return NewPrintNameOrErrorAfterIndent(mapper, short, operation, out, errs, "")
}

func NewPrintNameOrErrorAfterIndent(mapper meta.RESTMapper, short bool, operation string, out, errs io.Writer, indent string) AfterFunc {
	return func(info *resource.Info, err error) bool {
		if err == nil {
			fmt.Fprintf(out, indent)
			cmdutil.PrintSuccess(mapper, short, out, info.Mapping.Kind, info.Name, operation)
		} else {
			fmt.Fprintf(errs, "%serror: %v\n", indent, err)
		}
		return false
	}
}

func NewPrintErrorAfter(mapper meta.RESTMapper, errs io.Writer) func(*resource.Info, error) bool {
	return func(info *resource.Info, err error) bool {
		if err != nil {
			fmt.Fprintf(errs, "error: %v\n", err)
		}
		return false
	}
}

func HaltOnError(fn AfterFunc) AfterFunc {
	return func(info *resource.Info, err error) bool {
		if fn(info, err) || err != nil {
			return true
		}
		return false
	}
}

func encodeAndCreate(info *resource.Info, namespace string, obj runtime.Object) (runtime.Object, error) {
	return resource.NewHelper(info.Client, info.Mapping).Create(namespace, false, obj)
}

// Create attempts to create each item generically, gathering all errors in the
// event a failure occurs. The contents of list will be updated to include the
// version from the server.
func (b *Bulk) Create(list *kapi.List, namespace string) []error {
	resourceMapper := &resource.Mapper{ObjectTyper: b.Typer, RESTMapper: b.Mapper, ClientMapper: resource.ClientMapperFunc(b.RESTClientFactory)}
	after := b.After
	if after == nil {
		after = func(*resource.Info, error) bool { return false }
	}

	errs := []error{}
	for i, item := range list.Items {
		info, err := resourceMapper.InfoForObject(item)
		if err != nil {
			errs = append(errs, err)
			if after(info, err) {
				break
			}
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
			if after(info, err) {
				break
			}
			continue
		}
		info.Refresh(obj, true)
		list.Items[i] = obj
		if after(info, nil) {
			break
		}
	}
	return errs
}
