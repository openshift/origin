package cmd

import (
	"fmt"
	"io"
	"strings"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"
)

func selectContainers(containers []kapi.Container, spec string) ([]*kapi.Container, []*kapi.Container) {
	out := []*kapi.Container{}
	skipped := []*kapi.Container{}
	for i, c := range containers {
		if selectString(c.Name, spec) {
			out = append(out, &containers[i])
		} else {
			skipped = append(skipped, &containers[i])
		}
	}
	return out, skipped
}

func handlePodUpdateError(out io.Writer, err error, resource string) {
	if statusError, ok := err.(*errors.StatusError); ok && errors.IsInvalid(err) {
		errorDetails := statusError.Status().Details
		if errorDetails.Kind == "Pod" {
			all, match := true, false
			for _, cause := range errorDetails.Causes {
				if cause.Field == "spec" && strings.Contains(cause.Message, "may not update fields other than") {
					fmt.Fprintf(out, "error: may not update %s in pod %q directly\n", resource, errorDetails.Name)
					match = true
				} else {
					all = false
				}
			}
			if all && match {
				return
			}
		}
	}

	fmt.Fprintf(out, "error: %v\n", err)
}

// selectString returns true if the provided string matches spec, where spec is a string with
// a non-greedy '*' wildcard operator.
// TODO: turn into a regex and handle greedy matches and backtracking.
func selectString(s, spec string) bool {
	if spec == "*" {
		return true
	}
	if !strings.Contains(spec, "*") {
		return s == spec
	}

	pos := 0
	match := true
	parts := strings.Split(spec, "*")
	for i, part := range parts {
		if len(part) == 0 {
			continue
		}
		next := strings.Index(s[pos:], part)
		switch {
		// next part not in string
		case next < pos:
			fallthrough
		// first part does not match start of string
		case i == 0 && pos != 0:
			fallthrough
		// last part does not exactly match remaining part of string
		case i == (len(parts)-1) && len(s) != (len(part)+next):
			match = false
			break
		default:
			pos = next
		}
	}
	return match
}

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
