package cmd

import (
	"strings"

	"github.com/golang/glog"
	"github.com/spf13/cobra"
)

// parseNamespaceResourceName parses the value and returns namespace, resource and the
// value (resource name) itself. The valid syntax is:
// oc process mytemplate - implicit namespace (current), implicit resource (template)
// oc process template/mytemplate - implicit namespace (current), explicit resource
// oc process ns/template/mytemplate - explicit namespace, explicit resource
// oc process ns//mytemplate - explicit namespace, implicit resource (template)
func parseNamespaceResourceName(v, defaultNamespace string) (ns, resource, name string, ok bool) {
	parts := strings.Split(strings.TrimSpace(v), "/")
	switch len(parts) {
	case 3:
		return parts[0], parts[1], parts[2], true
	case 2:
		return defaultNamespace, parts[0], parts[1], true
	case 1:
		return defaultNamespace, "", parts[0], true
	}
	return "", "", "", false
}

// TODO: GetFlagStringArray already exists in kubernetes master, remove this
// after rebasing to version that has it
func getFlagStringArray(cmd *cobra.Command, flag string) []string {
	s, err := cmd.Flags().GetStringArray(flag)
	if err != nil {
		glog.Fatalf("error accessing flag %s for command %s: %v", flag, cmd.Name(), err)
	}
	return s
}
