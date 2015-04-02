package buildchain

import (
	"fmt"
	"strconv"
	"strings"
)

// invalidRepoTagErr is returned when an invalid image repository and tag
// combination has been passed by the user
var invalidRepoTagErr = fmt.Errorf("invalid [imageRepository]:[tag] input")

// parseTag parses the input and returns the repo (namespace+name)
// alongside a tag
func parseTag(input string) (string, string, error) {
	args := strings.Split(input, ":")
	switch len(args) {
	case 1:
		return args[0], "latest", nil
	case 2:
		if strings.TrimSpace(args[1]) == "" {
			return args[0], "latest", nil
		}
		return args[0], args[1], nil
	default:
		return "", "", invalidRepoTagErr
	}
}

// join joins a namespace and a name
func join(namespace, name string) string {
	return namespace + "/" + name
}

var invalidRepoErr = fmt.Errorf("cannot split input to name and namespace")

// split accepts an image repository namespace/name string
// and splits it to namespace (first) and name (second)
func split(repo string) (string, string, error) {
	args := strings.Split(repo, "/")
	if len(args) != 2 {
		return "", "", invalidRepoErr
	}
	return args[0], args[1], nil
}

// validDOT replaces hyphens with undescores so to
// keep the DOT parser happy
func validDOT(input string) string {
	// TODO: The only special character the DOT parser
	// accepts is the underscore (_)
	return strings.Replace(input, "-", "_", -1)
}

// setLabel is a helper function for setting labels
// on any graph object
func setLabel(name, namespace string, attrs map[string]string, tag string) {
	if tag != "" {
		name += ":" + tag
	}
	attrs["label"] = fmt.Sprintf("<%s<BR /><FONT POINT-SIZE=\"10\">%s</FONT>>", name, namespace)
}

// setTag sets tags in nodes as comments
func setTag(tag string, attrs map[string]string) {
	attrs["comment"] = strconv.Quote(tag)
}

// treeSize traverses a tree and returns its size
func treeSize(root *ImageRepo) int {
	if root == nil {
		return 0
	}
	if len(root.Children) == 0 {
		// Leaf node
		return 1
	}

	size := 1
	for _, child := range root.Children {
		size += treeSize(child)
	}

	return size
}
