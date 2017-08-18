package cmd

import (
	"fmt"
	"io"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/errors"
	kapi "k8s.io/kubernetes/pkg/api"

	"github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/generate/app"
	"github.com/openshift/origin/pkg/template"
	templateapi "github.com/openshift/origin/pkg/template/apis/template"
)

// TransformTemplate processes a template with the provided parameters, returning an error if transformation fails.
func TransformTemplate(tpl *templateapi.Template, client client.TemplateConfigsNamespacer, namespace string, parameters map[string]string, ignoreUnknownParameters bool) (*templateapi.Template, error) {
	// only set values that match what's expected by the template.
	for k, value := range parameters {
		v := template.GetParameterByName(tpl, k)
		if v != nil {
			v.Value = value
			v.Generate = ""
		} else if !ignoreUnknownParameters {
			return nil, fmt.Errorf("unexpected parameter name %q", k)
		}
	}

	name := localOrRemoteName(tpl.ObjectMeta, namespace)

	// transform the template
	result, err := client.TemplateConfigs(namespace).Create(tpl)
	if err != nil {
		return nil, fmt.Errorf("error processing template %q: %v", name, err)
	}

	// ensure the template objects are decoded
	// TODO: in the future, this should be more automatic
	// TODO: this should use the UniversalDecoder() once we deprecate legacy group. Until
	// then we have to maintain the backward compatibility with old servers so this will
	// force to decode the list as legacy/core API.
	if errs := runtime.DecodeList(result.Objects, kapi.Codecs.LegacyCodec(latest.Version)); len(errs) > 0 {
		err = errors.NewAggregate(errs)
		return nil, fmt.Errorf("error processing template %q: %v", name, err)
	}

	return result, nil
}

func formatString(out io.Writer, tab, s string) {
	labelVals := strings.Split(strings.TrimSuffix(s, "\n"), "\n")

	for _, lval := range labelVals {
		fmt.Fprintf(out, fmt.Sprintf("%s%s\n", tab, lval))
	}
}

// DescribeGeneratedTemplate writes a description of the provided template to out.
func DescribeGeneratedTemplate(out io.Writer, input string, result *templateapi.Template, baseNamespace string) {
	qualifiedName := localOrRemoteName(result.ObjectMeta, baseNamespace)
	if len(input) > 0 && result.ObjectMeta.Name != input {
		fmt.Fprintf(out, "--> Deploying template %q for %q to project %s\n", qualifiedName, input, baseNamespace)
	} else {
		fmt.Fprintf(out, "--> Deploying template %q to project %s\n", qualifiedName, baseNamespace)
	}
	fmt.Fprintln(out)

	name := displayName(result.ObjectMeta)
	message := result.Message
	description := result.Annotations["description"]

	// If there is a message or description
	if len(message) > 0 || len(description) > 0 {
		fmt.Fprintf(out, "     %s\n", name)
		fmt.Fprintf(out, "     ---------\n")
		if len(description) > 0 {
			formatString(out, "     ", description)
			fmt.Fprintln(out)
		}
		if len(message) > 0 {
			formatString(out, "     ", message)
			fmt.Fprintln(out)
		}
	}

	if warnings := result.Annotations[app.GenerationWarningAnnotation]; len(warnings) > 0 {
		delete(result.Annotations, app.GenerationWarningAnnotation)
		fmt.Fprintln(out)
		lines := strings.Split("warning: "+warnings, "\n")
		for _, line := range lines {
			fmt.Fprintf(out, "    %s\n", line)
		}
		fmt.Fprintln(out)
	}

	if len(result.Parameters) > 0 {
		fmt.Fprintf(out, "     * With parameters:\n")
		for _, p := range result.Parameters {
			name := p.DisplayName
			if len(name) == 0 {
				name = p.Name
			}
			var generated string
			if len(p.Generate) > 0 {
				generated = " # generated"
			}
			fmt.Fprintf(out, "        * %s=%s%s\n", name, p.Value, generated)
		}
		fmt.Fprintln(out)
	}
}
