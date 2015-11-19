package validation

import (
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/errors"

	"github.com/openshift/origin/pkg/template/api"
)

func makeParameter(name, value string) *api.Parameter {
	return &api.Parameter{
		Name:  name,
		Value: value,
	}
}

func TestValidateParameter(t *testing.T) {
	var tests = []struct {
		ParameterName   string
		IsValidExpected bool
	}{
		{"VALname_NAME", true},
		{"_valid_name_99", true},
		{"10gen_valid_name", true},
		{"", false},
		{"INVALname NAME", false},
		{"IVALname-NAME", false},
		{">INVALname_NAME", false},
		{"$INVALname_NAME", false},
		{"${INVALname_NAME}", false},
	}

	for _, test := range tests {
		param := makeParameter(test.ParameterName, "1")
		if test.IsValidExpected && len(ValidateParameter(param)) != 0 {
			t.Errorf("Expected zero validation errors on valid parameter name.")
		}
		if !test.IsValidExpected && len(ValidateParameter(param)) == 0 {
			t.Errorf("Expected some validation errors on invalid parameter name.")
		}
	}
}

func TestValidateProcessTemplate(t *testing.T) {
	var tests = []struct {
		template        *api.Template
		isValidExpected bool
	}{
		{ // Empty Template, should pass
			&api.Template{},
			true,
		},
		{ // Template with name, should pass
			&api.Template{
				ObjectMeta: kapi.ObjectMeta{Name: "templateId"},
			},
			true,
		},
		{ // Template with invalid Parameter, should fail on Parameter name
			&api.Template{
				ObjectMeta: kapi.ObjectMeta{Name: "templateId"},
				Parameters: []api.Parameter{
					*(makeParameter("", "1")),
				},
			},
			false,
		},
		{ // Template with valid Parameter, should pass
			&api.Template{
				ObjectMeta: kapi.ObjectMeta{Name: "templateId"},
				Parameters: []api.Parameter{
					*(makeParameter("VALname_NAME", "1")),
				},
			},
			true,
		},
		{ // Template with Item of unknown Kind, should pass
			&api.Template{
				ObjectMeta: kapi.ObjectMeta{Name: "templateId"},
				Parameters: []api.Parameter{
					*(makeParameter("VALname_NAME", "1")),
				},
				Objects: []runtime.Object{},
			},
			true,
		},
	}

	for i, test := range tests {
		errs := ValidateProcessedTemplate(test.template)
		if len(errs) != 0 && test.isValidExpected {
			t.Errorf("%d: Unexpected non-empty error list: %v", i, errors.NewAggregate(errs))
		}
		if len(errs) == 0 && !test.isValidExpected {
			t.Errorf("%d: Unexpected empty error list: %#v", i, errs)
		}
	}
}

func TestValidateTemplate(t *testing.T) {
	var tests = []struct {
		template        *api.Template
		isValidExpected bool
	}{
		{ // Empty Template, should fail on empty name
			&api.Template{},
			false,
		},
		{ // Template with name, should pass
			&api.Template{
				ObjectMeta: kapi.ObjectMeta{
					Name:      "template",
					Namespace: kapi.NamespaceDefault,
				},
			},
			true,
		},
		{ // Template without namespace, should fail
			&api.Template{
				ObjectMeta: kapi.ObjectMeta{
					Name: "template",
				},
			},
			false,
		},
		{ // Template with invalid name characters, should fail
			&api.Template{
				ObjectMeta: kapi.ObjectMeta{
					Name:      "templateId",
					Namespace: kapi.NamespaceDefault,
				},
			},
			false,
		},
		{ // Template with invalid Parameter, should fail on Parameter name
			&api.Template{
				ObjectMeta: kapi.ObjectMeta{Name: "template", Namespace: kapi.NamespaceDefault},
				Parameters: []api.Parameter{
					*(makeParameter("", "1")),
				},
			},
			false,
		},
		{ // Template with valid Parameter, should pass
			&api.Template{
				ObjectMeta: kapi.ObjectMeta{Name: "template", Namespace: kapi.NamespaceDefault},
				Parameters: []api.Parameter{
					*(makeParameter("VALname_NAME", "1")),
				},
			},
			true,
		},
		{ // Template with empty items, should pass
			&api.Template{
				ObjectMeta: kapi.ObjectMeta{Name: "template", Namespace: kapi.NamespaceDefault},
				Parameters: []api.Parameter{},
				Objects:    []runtime.Object{},
			},
			true,
		},
		{ // Template with an item that is invalid, should pass
			&api.Template{
				ObjectMeta: kapi.ObjectMeta{Name: "template", Namespace: kapi.NamespaceDefault},
				Parameters: []api.Parameter{},
				Objects: []runtime.Object{
					&kapi.Service{
						ObjectMeta: kapi.ObjectMeta{
							GenerateName: "test",
						},
						Spec: kapi.ServiceSpec{
							Ports: []kapi.ServicePort{{Port: 8080}},
						},
					},
				},
			},
			true,
		},
	}

	for i, test := range tests {
		errs := ValidateTemplate(test.template)
		if len(errs) != 0 && test.isValidExpected {
			t.Errorf("%d: Unexpected non-empty error list: %v", i, errors.NewAggregate(errs))
		}
		if len(errs) == 0 && !test.isValidExpected {
			t.Errorf("%d: Unexpected empty error list: %v", i, errs)
		}
	}
}
