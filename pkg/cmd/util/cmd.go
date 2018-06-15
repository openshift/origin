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
	"k8s.io/apimachinery/pkg/runtime/schema"
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
