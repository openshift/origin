package util

import (
	"fmt"
	"io"

	"k8s.io/apimachinery/pkg/api/meta"
)

func PrintSuccess(mapper meta.RESTMapper, shortOutput bool, out io.Writer, resource, name string, dryRun bool, operation string) {
	resource, _ = mapper.ResourceSingularizer(resource)
	dryRunMsg := ""
	if dryRun {
		dryRunMsg = " (dry run)"
	}
	if shortOutput {
		// -o name: prints resource/name
		if len(resource) > 0 {
			fmt.Fprintf(out, "%s/%s\n", resource, name)
		} else {
			fmt.Fprintf(out, "%s\n", name)
		}
	} else {
		// understandable output by default
		if len(resource) > 0 {
			fmt.Fprintf(out, "%s \"%s\" %s%s\n", resource, name, operation, dryRunMsg)
		} else {
			fmt.Fprintf(out, "\"%s\" %s%s\n", name, operation, dryRunMsg)
		}
	}
}