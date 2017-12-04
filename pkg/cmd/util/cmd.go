package util

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apimachinery"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
)

var commaSepVarsPattern = regexp.MustCompile(".*=.*,.*=.*")

// ReplaceCommandName recursively processes the examples in a given command to change a hardcoded
// command name (like 'kubectl' to the appropriate target name). It returns c.
func ReplaceCommandName(from, to string, c *cobra.Command) *cobra.Command {
	c.Example = strings.Replace(c.Example, from, to, -1)
	for _, sub := range c.Commands() {
		ReplaceCommandName(from, to, sub)
	}
	return c
}

// GetDisplayFilename returns the absolute path of the filename as long as there was no error, otherwise it returns the filename as-is
func GetDisplayFilename(filename string) string {
	if absName, err := filepath.Abs(filename); err == nil {
		return absName
	}

	return filename
}

// ResolveResource returns the resource type and name of the resourceString.
// If the resource string has no specified type, defaultResource will be returned.
func ResolveResource(defaultResource schema.GroupResource, resourceString string, mapper meta.RESTMapper) (schema.GroupResource, string, error) {
	if mapper == nil {
		return schema.GroupResource{}, "", errors.New("mapper cannot be nil")
	}

	var name string
	parts := strings.Split(resourceString, "/")
	switch len(parts) {
	case 1:
		name = parts[0]
	case 2:
		name = parts[1]

		// Allow specifying the group the same way kubectl does, as "resource.group.name"
		groupResource := schema.ParseGroupResource(parts[0])
		// normalize resource case
		groupResource.Resource = strings.ToLower(groupResource.Resource)

		gvr, err := mapper.ResourceFor(groupResource.WithVersion(""))
		if err != nil {
			return schema.GroupResource{}, "", err
		}
		return gvr.GroupResource(), name, nil
	default:
		return schema.GroupResource{}, "", fmt.Errorf("invalid resource format: %s", resourceString)
	}

	return defaultResource, name, nil
}

// convertItemsForDisplay returns a new list that contains parallel elements that have been converted to the most preferred external version
func convertItemsForDisplay(objs []runtime.Object, preferredVersions ...schema.GroupVersion) ([]runtime.Object, error) {
	ret := []runtime.Object{}

	for i := range objs {
		obj := objs[i]
		kinds, _, err := legacyscheme.Scheme.ObjectKinds(obj)
		if err != nil {
			return nil, err
		}

		// Gather all groups where the object kind is known.
		groups := []*apimachinery.GroupMeta{}
		for _, kind := range kinds {
			groupMeta, err := legacyscheme.Registry.Group(kind.Group)
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
			convertedObject, err := legacyscheme.Scheme.ConvertToVersion(obj, defaultGroupVersioners)
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
			convertedObject, err := legacyscheme.Scheme.ConvertToVersion(obj, preferredVersioner)
			if err != nil {
				return nil, err
			}

			ret = append(ret, convertedObject)
			continue
		}

		convertedObject, err := legacyscheme.Scheme.ConvertToVersion(obj, actualOutputVersion)
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
func convertItemsForDisplayFromDefaultCommand(cmd *cobra.Command, objs []runtime.Object) ([]runtime.Object, error) {
	requested := kcmdutil.GetFlagString(cmd, "output-version")
	versions := []schema.GroupVersion{}
	if len(requested) == 0 {
		return convertItemsForDisplay(objs, versions...)
	}

	for _, v := range strings.Split(requested, ",") {
		version, err := schema.ParseGroupVersion(v)
		if err != nil {
			return nil, err
		}
		versions = append(versions, version)
	}

	return convertItemsForDisplay(objs, versions...)
}

// VersionedPrintObject handles printing an object in the appropriate version by looking at 'output-version'
// on the command
func VersionedPrintObject(fn func(*cobra.Command, bool, meta.RESTMapper, runtime.Object, io.Writer) error, c *cobra.Command, mapper meta.RESTMapper, out io.Writer) func(runtime.Object) error {
	return func(obj runtime.Object) error {
		// TODO: fold into the core printer functionality (preferred output version)
		if list, ok := obj.(*kapi.List); ok {
			var err error
			if list.Items, err = convertItemsForDisplayFromDefaultCommand(c, list.Items); err != nil {
				return err
			}
		} else {
			result, err := convertItemsForDisplayFromDefaultCommand(c, []runtime.Object{obj})
			if err != nil {
				return err
			}
			obj = result[0]
		}
		return fn(c, false, mapper, obj, out)
	}
}

func WarnAboutCommaSeparation(errout io.Writer, values []string, flag string) {
	if errout == nil {
		return
	}
	for _, value := range values {
		if commaSepVarsPattern.MatchString(value) {
			fmt.Fprintf(errout, "warning: %s no longer accepts comma-separated lists of values. %q will be treated as a single key-value pair.\n", flag, value)
		}
	}
}
