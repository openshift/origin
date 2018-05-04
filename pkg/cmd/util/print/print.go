package print

import (
	"io"
	"strings"

	"github.com/spf13/cobra"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apimachinery"
	"k8s.io/apimachinery/pkg/apimachinery/registered"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
)

// convertItemsForDisplay returns a new list that contains parallel elements that have been converted to the most preferred external version
func convertItemsForDisplay(scheme *runtime.Scheme, registry *registered.APIRegistrationManager, objs []runtime.Object, preferredVersions ...schema.GroupVersion) ([]runtime.Object, error) {
	ret := []runtime.Object{}

	for i := range objs {
		obj := objs[i]
		kinds, _, err := scheme.ObjectKinds(obj)
		if err != nil {
			return nil, err
		}

		// Gather all groups where the object kind is known.
		groups := []*apimachinery.GroupMeta{}
		for _, kind := range kinds {
			groupMeta, err := registry.Group(kind.Group)
			if err != nil {
				return nil, err
			}
			groups = append(groups, groupMeta)
		}

		// if no preferred versions given, pass all group versions found.
		if len(preferredVersions) == 0 {
			defaultGroupVersions := []runtime.GroupVersioner{}
			for _, group := range groups {
				defaultGroupVersions = append(defaultGroupVersions, group.GroupVersion)
			}

			defaultGroupVersioners := runtime.GroupVersioners(defaultGroupVersions)
			convertedObject, err := scheme.ConvertToVersion(obj, defaultGroupVersioners)
			if err != nil {
				return nil, err
			}
			ret = append(ret, convertedObject)
			continue
		}

		actualOutputVersion := schema.GroupVersion{}
		// Find the first preferred version that contains the object kind group.
		// If there are more groups for the given resource, prefer those that are first in the
		// list of preferred versions.
		for _, version := range preferredVersions {
			for _, group := range groups {
				if version.Group == group.GroupVersion.Group {
					for _, externalVersion := range group.GroupVersions {
						if version == externalVersion {
							actualOutputVersion = externalVersion
							break
						}
						if actualOutputVersion.Empty() {
							actualOutputVersion = externalVersion
						}
					}
				}
				if !actualOutputVersion.Empty() {
					break
				}
			}
			if !actualOutputVersion.Empty() {
				break
			}
		}

		// if no preferred version found in the list of given GroupVersions,
		// attempt to convert to first GroupVersion that satisfies a preferred version
		if len(actualOutputVersion.Version) == 0 {
			preferredVersioners := []runtime.GroupVersioner{}
			for _, gv := range preferredVersions {
				preferredVersions = append(preferredVersions, gv)
			}
			preferredVersioner := runtime.GroupVersioners(preferredVersioners)
			convertedObject, err := scheme.ConvertToVersion(obj, preferredVersioner)
			if err != nil {
				return nil, err
			}

			ret = append(ret, convertedObject)
			continue
		}

		convertedObject, err := scheme.ConvertToVersion(obj, actualOutputVersion)
		if err != nil {
			return nil, err
		}

		ret = append(ret, convertedObject)
	}

	return ret, nil
}

// convertItemsForDisplayFromDefaultCommand returns a new list that contains parallel elements that have been converted to the most preferred external version
// TODO: move this function into the core factory PrintObjects method
// TODO: print-objects should have preferred output versions
func convertItemsForDisplayFromDefaultCommand(scheme *runtime.Scheme, registry *registered.APIRegistrationManager, cmd *cobra.Command, objs []runtime.Object) ([]runtime.Object, error) {
	requested := kcmdutil.GetFlagString(cmd, "output-version")
	versions := []schema.GroupVersion{}
	if len(requested) == 0 {
		return convertItemsForDisplay(scheme, registry, objs, versions...)
	}

	for _, v := range strings.Split(requested, ",") {
		version, err := schema.ParseGroupVersion(v)
		if err != nil {
			return nil, err
		}
		versions = append(versions, version)
	}

	return convertItemsForDisplay(scheme, registry, objs, versions...)
}

// VersionedPrintObject handles printing an object in the appropriate version by looking at 'output-version'
// on the command
func VersionedPrintObject(scheme *runtime.Scheme, registry *registered.APIRegistrationManager, fn func(*cobra.Command, runtime.Object, io.Writer) error, c *cobra.Command, out io.Writer) func(runtime.Object) error {
	return func(obj runtime.Object) error {
		// TODO: fold into the core printer functionality (preferred output version)

		if items, err := meta.ExtractList(obj); err == nil {
			items, err = convertItemsForDisplayFromDefaultCommand(scheme, registry, c, items)
			if err != nil {
				return err
			}
			if err := meta.SetList(obj, items); err != nil {
				return err
			}
		} else {
			result, err := convertItemsForDisplayFromDefaultCommand(scheme, registry, c, []runtime.Object{obj})
			if err != nil {
				return err
			}
			obj = result[0]
		}
		return fn(c, obj, out)
	}
}
