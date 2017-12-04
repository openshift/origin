package template

import (
	"fmt"
	"strings"

	templateapi "github.com/openshift/origin/pkg/template/apis/template"
	"k8s.io/apiserver/pkg/authentication/user"
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

// ConvertUserToTemplateInstanceRequester copies analogous fields from user.Info to TemplateInstanceRequester
func ConvertUserToTemplateInstanceRequester(u user.Info) templateapi.TemplateInstanceRequester {
	templatereq := templateapi.TemplateInstanceRequester{}

	if u != nil {
		extra := map[string]templateapi.ExtraValue{}
		if u.GetExtra() != nil {
			for k, v := range u.GetExtra() {
				extra[k] = templateapi.ExtraValue(v)
			}
		}

		templatereq.Username = u.GetName()
		templatereq.UID = u.GetUID()
		templatereq.Groups = u.GetGroups()
		templatereq.Extra = extra
	}

	return templatereq
}

// ConvertTemplateInstanceRequesterToUser copies analogous fields from TemplateInstanceRequester to user.Info
func ConvertTemplateInstanceRequesterToUser(templateReq *templateapi.TemplateInstanceRequester) user.Info {
	u := user.DefaultInfo{}
	u.Extra = map[string][]string{}

	if templateReq != nil {
		u.Name = templateReq.Username
		u.UID = templateReq.UID
		u.Groups = templateReq.Groups
		if templateReq.Extra != nil {
			for k, v := range templateReq.Extra {
				u.Extra[k] = []string(v)
			}
		}
	}

	return &u
}
