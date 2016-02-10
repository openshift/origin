package api

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/conversion"
	"k8s.io/kubernetes/pkg/runtime"
)

// Convert_runtime_Object_To_runtime_RawExtension is conversion function that assumes that the runtime.Object you've embedded is in
// the same GroupVersion that your containing type is in.  This is signficantly better than simply breaking.
// Given an ordered list of preferred external versions for a given encode or conversion call, the behavior of this function could be
// made generic, predictable, and controllable.
func Convert_runtime_Object_To_runtime_RawExtension(in runtime.Object, out *runtime.RawExtension, s conversion.Scope) error {
	if in == nil {
		return nil
	}

	externalObject, err := kapi.Scheme.ConvertToVersion(in, s.Meta().DestVersion)
	if err != nil {
		return err
	}

	targetVersion, err := unversioned.ParseGroupVersion(s.Meta().DestVersion)
	if err != nil {
		return err
	}
	bytes, err := runtime.Encode(kapi.Codecs.LegacyCodec(targetVersion), externalObject)
	if err != nil {
		return err
	}

	out.RawJSON = bytes
	out.Object = externalObject

	return nil
}

// Convert_runtime_RawExtension_To_runtime_Object well, this is the reason why there was runtime.Embedded.  The `out` here is hopeless.
// The caller doesn't know the type ahead of time and that means this method can't communicate the return value.  This sucks really badly.
// I'm going to set the `in.Object` field can have callers to this function do magic to pull it back out.  I'm also going to bitch about it.
func Convert_runtime_RawExtension_To_runtime_Object(in *runtime.RawExtension, out runtime.Object, s conversion.Scope) error {
	if in == nil || len(in.RawJSON) == 0 || in.Object != nil {
		return nil
	}

	// the scheme knows all available group versions, so its possible to build the decoder properly, but it would require some refactoring

	srcVersion, err := unversioned.ParseGroupVersion(s.Meta().SrcVersion)
	if err != nil {
		return err
	}
	decodedObject, err := runtime.Decode(kapi.Codecs.UniversalDecoder(srcVersion), in.RawJSON)
	if err != nil {
		return err
	}

	internalObject, err := kapi.Scheme.ConvertToVersion(decodedObject, s.Meta().DestVersion)
	if err != nil {
		return err
	}

	in.Object = internalObject

	return nil
}
