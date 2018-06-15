package print

import (
	"io"

	"github.com/spf13/cobra"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	"k8s.io/kubernetes/pkg/apis/core"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// convertItemsForDisplay returns a new list that contains parallel elements that have been converted to the most preferred external version
func convertItemsForDisplay(scheme *runtime.Scheme, objs []runtime.Object, preferredVersions ...schema.GroupVersion) ([]runtime.Object, error) {
	ret := []runtime.Object{}

	for i := range objs {
		obj := objs[i]
		kinds, _, err := scheme.ObjectKinds(obj)
		if err != nil {
			return nil, err
		}

		// Gather all groups where the object kind is known.
		groups := []string{}
		for _, kind := range kinds {
			groups = append(groups, kind.Group)
		}

		// if no preferred versions given, pass all group versions found.
		if len(preferredVersions) == 0 {
			defaultGroupVersions := []runtime.GroupVersioner{}
			for _, group := range groups {
				defaultGroupVersions = append(defaultGroupVersions, schema.GroupVersions(scheme.PrioritizedVersionsForGroup(group)))
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
				if version.Group == group {
					for _, externalVersion := range scheme.PrioritizedVersionsForGroup(group) {
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
func ConvertItemsForDisplayFromDefaultCommand(scheme *runtime.Scheme, cmd *cobra.Command, objs []runtime.Object) ([]runtime.Object, error) {
	versions := []schema.GroupVersion{}
	return convertItemsForDisplay(scheme, objs, versions...)
}

// VersionedPrintObject handles printing an object in the appropriate version by looking at 'output-version'
// on the command
func VersionedPrintObject(fn func(*cobra.Command, runtime.Object, io.Writer) error, c *cobra.Command, out io.Writer) func(runtime.Object) error {
	return func(obj runtime.Object) error {
		// TODO: fold into the core printer functionality (preferred output version)

		if items, err := meta.ExtractList(obj); err == nil {
			items, err = ConvertItemsForDisplayFromDefaultCommand(legacyscheme.Scheme, c, items)
			if err != nil {
				return err
			}
			if err := meta.SetList(obj, items); err != nil {
				return err
			}
		} else {
			result, err := ConvertItemsForDisplayFromDefaultCommand(legacyscheme.Scheme, c, []runtime.Object{obj})
			if err != nil {
				return err
			}
			obj = result[0]
		}

		if list, ok := obj.(*core.List); ok {
			listCopy := list.DeepCopy()
			listCopy.Items = []runtime.Object{}
			convertedList, err := legacyscheme.Scheme.ConvertToVersion(listCopy, schema.GroupVersion{Version: "v1"})
			if err != nil {
				return err
			}
			versionedList := convertedList.(*corev1.List)
			for i := range list.Items {
				// these list items have already been converted
				versionedList.Items = append(versionedList.Items, runtime.RawExtension{Object: list.Items[i]})
			}
			return fn(c, convertedList, out)
		}

		return fn(c, obj, out)
	}
}
