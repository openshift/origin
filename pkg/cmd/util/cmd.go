package util

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"k8s.io/kubernetes/pkg/api/meta"
	"k8s.io/kubernetes/pkg/api/unversioned"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
)

// ErrExit is a marker interface for cli commands indicating that the response has been processed
var ErrExit = fmt.Errorf("exit directly")

func DefaultSubCommandRun(out io.Writer) func(c *cobra.Command, args []string) {
	return func(c *cobra.Command, args []string) {
		c.SetOutput(out)

		if len(args) > 0 {
			kcmdutil.CheckErr(kcmdutil.UsageError(c, fmt.Sprintf(`unknown command "%s"`, strings.Join(args, " "))))
		}

		c.Help()
	}
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
func ResolveResource(defaultResource, resourceString string, mapper meta.RESTMapper) (string, string, error) {
	if mapper == nil {
		return "", "", errors.New("mapper cannot be nil")
	}

	var name string
	parts := strings.Split(resourceString, "/")
	switch len(parts) {
	case 1:
		name = parts[0]
	case 2:
		gvk, err := mapper.KindFor(unversioned.GroupVersionResource{Resource: parts[0]})
		if err != nil {
			return "", "", err
		}
		name = parts[1]
		resource, _ := meta.KindToResource(gvk, false)
		return resource.Resource, name, nil
	default:
		return "", "", fmt.Errorf("invalid resource format: %s", resourceString)
	}

	return defaultResource, name, nil
}
