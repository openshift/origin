package template

import (
	"fmt"
	"strings"
)

// TemplateReference points to a stored template
type TemplateReference struct {
	Namespace string
	Name      string
}

// ParseTemplateReference parses the reference to a template into a
// TemplateReference.
func ParseTemplateReference(s string) (TemplateReference, error) {
	var ref TemplateReference
	parts := strings.Split(s, "/")
	switch len(parts) {
	case 2:
		// namespace/name
		ref.Namespace = parts[0]
		ref.Name = parts[1]
		break
	case 1:
		// name
		ref.Name = parts[0]
		break
	default:
		return ref, fmt.Errorf("the template reference must be either the template name or namespace and template name separated by slashes")
	}
	return ref, nil
}

func (r TemplateReference) HasNamespace() bool {
	return len(r.Namespace) > 0
}

func (r TemplateReference) String() string {
	if r.HasNamespace() {
		return fmt.Sprintf("%s/%s", r.Namespace, r.Name)
	}
	return r.Name
}
