package apihelpers

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/runtime"
)

// Convert_runtime_Object_To_runtime_RawExtension attempts to convert runtime.Objects to the appropriate target, returning an error
// if there is insufficient information on the conversion scope to determine the target version.
func Convert_runtime_Object_To_runtime_RawExtension(c runtime.ObjectConvertor, in *runtime.Object, out *runtime.RawExtension, s conversion.Scope) error {
	if *in == nil {
		return nil
	}
	obj := *in

	switch obj.(type) {
	case *runtime.Unknown, *unstructured.Unstructured:
		out.Raw = nil
		out.Object = obj
		return nil
	}

	switch t := s.Meta().Context.(type) {
	case runtime.GroupVersioner:
		converted, err := c.ConvertToVersion(obj, t)
		if err != nil {
			return err
		}
		out.Raw = nil
		out.Object = converted
	default:
		return fmt.Errorf("unrecognized conversion context for versioning: %#v", t)
	}
	return nil
}

// Convert_runtime_RawExtension_To_runtime_Object attempts to convert an incoming object into the
// appropriate output type.
func Convert_runtime_RawExtension_To_runtime_Object(c runtime.ObjectConvertor, in *runtime.RawExtension, out *runtime.Object, s conversion.Scope) error {
	if in == nil || in.Object == nil {
		return nil
	}

	switch in.Object.(type) {
	case *runtime.Unknown, *unstructured.Unstructured:
		*out = in.Object
		return nil
	}

	switch t := s.Meta().Context.(type) {
	case runtime.GroupVersioner:
		converted, err := c.ConvertToVersion(in.Object, t)
		if err != nil {
			return err
		}
		in.Object = converted
		*out = converted
	default:
		return fmt.Errorf("unrecognized conversion context for conversion to internal: %#v (%T)", t, t)
	}
	return nil
}
