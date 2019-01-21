package configflags

import (
	"strings"

	kubecontrolplanev1 "github.com/openshift/api/kubecontrolplane/v1"
)

// ArgsWithPrefix filters arguments by prefix and collects values.
func ArgsWithPrefix(args map[string]kubecontrolplanev1.Arguments, prefix string) map[string][]string {
	filtered := map[string][]string{}
	for key, slice := range args {
		if !strings.HasPrefix(key, prefix) {
			continue
		}
		for _, val := range slice {
			filtered[key] = append(filtered[key], val)
		}
	}
	return filtered
}

func SetIfUnset(cmdLineArgs map[string][]string, key string, value ...string) {
	if _, ok := cmdLineArgs[key]; !ok {
		cmdLineArgs[key] = value
	}
}
