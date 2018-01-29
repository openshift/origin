package apidocs

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/go-openapi/spec"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type operation struct {
	s         *spec.Swagger
	Path      spec.PathItem
	PathName  string
	Operation *spec.Operation
	OpName    string
	gvk       schema.GroupVersionKind
}

func (o operation) bodyParameter() *spec.Parameter {
	for _, parameter := range o.Operation.Parameters {
		if parameter.In == "body" {
			return &parameter
		}
	}
	return nil
}

func (o operation) Description() string {
	return ToUpper(o.Operation.Description)
}

func (o operation) Curl() string {
	s := make([]string, 0, 7)

	s = append(s, "$ curl -k")

	if o.OpName != "Get" {
		s = append(s, " -X "+strings.ToUpper(o.OpName))
	}

	bodyParameter := o.bodyParameter()
	if bodyParameter != nil {
		s = append(s, " -d @-")
	}

	s = append(s,
		` -H "Authorization: Bearer $TOKEN"`,
		" -H 'Accept: application/json'")

	if bodyParameter != nil {
		contentType := "application/json"
		if o.Operation.Consumes[0] != "*/*" {
			contentType = o.Operation.Consumes[0]
		}

		s = append(s, fmt.Sprintf(" -H 'Content-Type: %s'", contentType))
	}

	s = append(s, " https://$ENDPOINT"+EnvStyle(o.PathName))

	var postamble string
	if bodyParameter != nil {
		postamble = " <<'EOF'\n" + o.sampleRequest() + "EOF"
	}

	return strings.Join(s, " \\\n   ") + postamble
}

func (o operation) HTTPReq() string {
	s := make([]string, 0, 7)

	s = append(s,
		strings.ToUpper(o.OpName)+" "+EnvStyle(o.PathName)+" HTTP/1.1",
		"Authorization: Bearer $TOKEN",
		"Accept: application/json",
		"Connection: close")

	if o.bodyParameter() != nil {
		contentType := "application/json"
		if o.Operation.Consumes[0] != "*/*" {
			contentType = o.Operation.Consumes[0]
		}

		s = append(s,
			fmt.Sprintf("Content-Type: %s'", contentType),
			"",
			o.sampleRequest())
	}
	return strings.Join(s, "\n")
}

func getGVK(s spec.Schema, gv schema.GroupVersion) schema.GroupVersionKind {
	for _, gvk := range GroupVersionKinds(s) {
		if gvk.Group == gv.Group && gvk.Version == gv.Version {
			return gvk
		}
	}
	return GroupVersionKinds(s)[0]
}

func (o operation) sampleRequest() string {
	bodyType := RefType(o.bodyParameter().Schema)
	switch bodyType {
	case "io.k8s.apimachinery.pkg.apis.meta.v1.DeleteOptions",
		"io.k8s.apimachinery.pkg.apis.meta.v1.Patch":
		return "{\n  ...\n}\n"
	}

	gvk := getGVK(o.s.Definitions[bodyType], o.gvk.GroupVersion())
	return fmt.Sprintf("{\n  \"kind\": \"%s\",\n  \"apiVersion\": \"%s\",\n  ...\n}\n", gvk.Kind, gvk.GroupVersion().String())
}

func (o operation) Parameters(t string) []spec.Parameter {
	parameters := make([]spec.Parameter, 0, len(o.Path.Parameters)+len(o.Operation.Parameters))
	for _, param := range append(o.Path.Parameters, o.Operation.Parameters...) {
		if param.In == t {
			parameters = append(parameters, param)
		}
	}
	return parameters
}

func (o operation) Verb() string {
	switch o.OpName {
	case "Get":
		if strings.Contains(o.PathName, "/watch/") {
			return "Watch"
		}
	case "Post":
		return "Create"
	case "Put":
		return "Update"
	}
	return o.OpName
}

func (o operation) Plural() bool {
	return o.OpName != "Post" && !strings.Contains(o.PathName, "{name}")
}

func (o operation) Subresource() string {
	if i := strings.LastIndex(o.PathName, "/{name}/"); i != -1 {
		return o.PathName[i+8:]
	}
	return ""
}

func (o operation) Namespaced() bool {
	return strings.Contains(o.PathName, "/namespaces/{namespace}")
}

func (o operation) IsProxy() bool {
	return o.gvk.Group == "" &&
		o.gvk.Version == "v1" &&
		(o.gvk.Kind == "Node" || o.gvk.Kind == "Pod" || o.gvk.Kind == "Service") &&
		strings.Contains(o.PathName, "/proxy")
}

func (o operation) Anchor() string {
	return strings.Map(func(r rune) rune {
		switch r {
		case '/':
			return '-'
		case '{', '}':
			return -1
		default:
			return r
		}
	}, o.OpName+"/"+strings.Trim(o.PathName, "/"))
}

func (o operation) Title() string {
	buf := &bytes.Buffer{}

	if o.IsProxy() {
		fmt.Fprintf(buf, "Proxy %s request to a %s", strings.ToUpper(o.OpName), o.gvk.Kind)
		if o.Namespaced() {
			fmt.Fprintf(buf, " in a namespace")
		}
		if strings.Contains(o.PathName, "/{path}") {
			fmt.Fprintf(buf, " (with path)")
		}
		return buf.String()
	}

	if o.gvk.Group == "" &&
		o.gvk.Version == "v1" &&
		o.gvk.Kind == "Pod" {
		switch {
		case strings.HasSuffix(o.PathName, "/attach"):
			return fmt.Sprintf("Attach to a v1.Pod in a namespace (%s)", strings.ToUpper(o.OpName))
		case strings.HasSuffix(o.PathName, "/exec"):
			return fmt.Sprintf("Exec in a v1.Pod in a namespace (%s)", strings.ToUpper(o.OpName))
		case strings.HasSuffix(o.PathName, "/portforward"):
			return fmt.Sprintf("Port-forward to a v1.Pod in a namespace (%s)", strings.ToUpper(o.OpName))
		}
	}

	fmt.Fprintf(buf, "%s ", o.Verb())

	subresource := o.Subresource()
	if subresource != "" {
		fmt.Fprintf(buf, "%s of ", subresource)
	}

	if o.Plural() {
		fmt.Fprintf(buf, "all %s", Pluralise(o.gvk.Kind))
	} else {
		fmt.Fprintf(buf, "a %s", o.gvk.Kind)
	}

	if o.Namespaced() {
		fmt.Fprintf(buf, " in a namespace")
	}
	return buf.String()
}

func (o operation) GVR() (gvr schema.GroupVersionResource, err error) {
	parts := strings.Split(strings.Trim(o.PathName, "/"), "/")

	if !((parts[0] == "apis" && len(parts) > 3) ||
		(parts[0] == "api" && len(parts) > 2) ||
		(parts[0] == "oapi" && len(parts) > 2)) {
		err = fmt.Errorf("GVR() called on invalid path %s", o.PathName)
		return
	}

	var i int
	switch parts[0] {
	case "apis":
		gvr.Group = parts[1]
		gvr.Version = parts[2]
		i = 3
	case "api", "oapi":
		gvr.Version = parts[1]
		i = 2
	}
	if i < len(parts)-1 && parts[i] == "watch" {
		i++
	}
	if i < len(parts)-2 && parts[i] == "namespaces" && parts[i+1] == "{namespace}" {
		i += 2
	}
	gvr.Resource = parts[i]

	return
}

func (o operation) GVK() (schema.GroupVersionKind, error) {
	parts := strings.Split(strings.Trim(o.PathName, "/"), "/")

	if parts[0] != "api" && parts[0] != "oapi" && parts[0] != "apis" {
		return schema.GroupVersionKind{}, fmt.Errorf("GVK() called on invalid path %s", o.PathName)
	}

	if (parts[0] == "apis" && len(parts) <= 3) || len(parts) <= 2 {
		// API group root.  Categorize by operation return value.
		sch := o.s.Definitions[RefType(o.Operation.Responses.StatusCodeResponses[200].Schema)]
		return GroupVersionKinds(sch)[0], nil
	}

	gvr, err := o.GVR()
	if err != nil {
		return schema.GroupVersionKind{}, err
	}

	if gvr.Group == "extensions" &&
		gvr.Version == "v1beta1" &&
		gvr.Resource == "replicationcontrollers" {
		return schema.GroupVersionKind{
			Version: "v1",
			Kind:    "ReplicationController",
		}, nil
	}

	var kind string
	switch {
	case gvr.Resource == "endpoints",
		gvr.Resource == "securitycontextconstraints":
		// kind is used in the plural form: don't remove 's'
		kind = gvr.Resource
	case gvr.Group == "template.openshift.io" && gvr.Resource == "processedtemplates":
		kind = "template"
	case strings.HasSuffix(gvr.Resource, "ies"):
		kind = gvr.Resource[:len(gvr.Resource)-3] + "y"
	case strings.HasSuffix(gvr.Resource, "ses"):
		kind = gvr.Resource[:len(gvr.Resource)-2]
	case strings.HasSuffix(gvr.Resource, "es"):
		kind = gvr.Resource[:len(gvr.Resource)-1]
	case strings.HasSuffix(gvr.Resource, "s"):
		kind = gvr.Resource[:len(gvr.Resource)-1]
	}

	for _, def := range o.s.Definitions {
		for _, gvk := range GroupVersionKinds(def) {
			if gvk.Group == gvr.Group &&
				gvk.Version == gvr.Version &&
				strings.ToLower(gvk.Kind) == kind {

				return gvk, nil
			}
		}
	}

	return schema.GroupVersionKind{}, fmt.Errorf("GVK for %s not found", gvr)
}
