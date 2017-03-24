package validation

import (
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/template/api"
)

const (
	validUUID  = "0153ff2f-77ab-4560-ba59-785931cded5e"
	validUUID2 = "7ee0204f-1ac5-40aa-a976-efcfca1b4b84"
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
		if test.IsValidExpected && len(ValidateParameter(param, nil)) != 0 {
			t.Errorf("Expected zero validation errors on valid parameter name.")
		}
		if !test.IsValidExpected && len(ValidateParameter(param, nil)) == 0 {
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
			t.Errorf("%d: Unexpected non-empty error list: %v", i, errs.ToAggregate())
		}
		if len(errs) == 0 && !test.isValidExpected {
			t.Errorf("%d: Unexpected empty error list: %v", i, errs.ToAggregate())
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
			t.Errorf("%d: Unexpected non-empty error list: %v", i, errs.ToAggregate())
		}
		if len(errs) == 0 && !test.isValidExpected {
			t.Errorf("%d: Unexpected empty error list: %v", i, errs.ToAggregate())
		}
	}
}

func TestValidateTemplateInstance(t *testing.T) {
	var tests = []struct {
		templateInstance api.TemplateInstance
		isValidExpected  bool
	}{
		{
			templateInstance: api.TemplateInstance{},
		},
		{
			templateInstance: api.TemplateInstance{
				ObjectMeta: kapi.ObjectMeta{
					Name:      "test",
					Namespace: "test",
				},
			},
		},
		{
			templateInstance: api.TemplateInstance{
				ObjectMeta: kapi.ObjectMeta{
					Name:      "test",
					Namespace: "test",
				},
				Spec: api.TemplateInstanceSpec{},
			},
		},
		{
			templateInstance: api.TemplateInstance{
				ObjectMeta: kapi.ObjectMeta{
					Name:      "test",
					Namespace: "test",
				},
				Spec: api.TemplateInstanceSpec{
					Template: api.Template{},
				},
			},
		},
		{
			templateInstance: api.TemplateInstance{
				ObjectMeta: kapi.ObjectMeta{
					Name:      "test",
					Namespace: "test",
				},
				Spec: api.TemplateInstanceSpec{
					Template: api.Template{
						ObjectMeta: kapi.ObjectMeta{
							Name:      "test",
							Namespace: "test",
						},
					},
					Requester: &api.TemplateInstanceRequester{
						Username: "test",
					},
				},
			},
			isValidExpected: true,
		},
		{
			templateInstance: api.TemplateInstance{
				ObjectMeta: kapi.ObjectMeta{
					Name:      "test",
					Namespace: "test",
				},
				Spec: api.TemplateInstanceSpec{
					Template: api.Template{
						ObjectMeta: kapi.ObjectMeta{
							Name:      "test",
							Namespace: "test",
						},
						Parameters: []api.Parameter{
							{
								Name: "b@d",
							},
						},
					},
					Requester: &api.TemplateInstanceRequester{
						Username: "test",
					},
				},
			},
		},
		{
			templateInstance: api.TemplateInstance{
				ObjectMeta: kapi.ObjectMeta{
					Name:      "test",
					Namespace: "test",
				},
				Spec: api.TemplateInstanceSpec{
					Template: api.Template{
						ObjectMeta: kapi.ObjectMeta{
							Name:      "test",
							Namespace: "test",
						},
					},
					Secret: kapi.LocalObjectReference{
						Name: "b@d",
					},
					Requester: &api.TemplateInstanceRequester{
						Username: "test",
					},
				},
			},
		},
		{
			templateInstance: api.TemplateInstance{
				ObjectMeta: kapi.ObjectMeta{
					Name:      "test",
					Namespace: "test",
				},
				Spec: api.TemplateInstanceSpec{
					Template: api.Template{
						ObjectMeta: kapi.ObjectMeta{
							Name:      "test",
							Namespace: "test",
						},
					},
					Secret: kapi.LocalObjectReference{
						Name: "test",
					},
					Requester: &api.TemplateInstanceRequester{
						Username: "test",
					},
				},
			},
			isValidExpected: true,
		},
	}

	for i, test := range tests {
		errs := ValidateTemplateInstance(&test.templateInstance)
		if len(errs) != 0 && test.isValidExpected {
			t.Errorf("%d: Unexpected non-empty error list: %v", i, errs.ToAggregate())
		}
		if len(errs) == 0 && !test.isValidExpected {
			t.Errorf("%d: Unexpected empty error list: %v", i, errs.ToAggregate())
		}
	}
}

func TestValidateTemplateInstanceUpdate(t *testing.T) {
	oldTemplateInstance := &api.TemplateInstance{
		ObjectMeta: kapi.ObjectMeta{
			Name:            "test",
			Namespace:       "test",
			ResourceVersion: "1",
		},
		Spec: api.TemplateInstanceSpec{
			Template: api.Template{
				ObjectMeta: kapi.ObjectMeta{
					Name:      "test",
					Namespace: "test",
				},
				Parameters: []api.Parameter{
					{
						Name: "test",
					},
				},
			},
			Secret: kapi.LocalObjectReference{
				Name: "test",
			},
			Requester: &api.TemplateInstanceRequester{
				Username: "test",
			},
		},
	}

	var tests = []struct {
		modifyTemplateInstance func(*api.TemplateInstance)
		isValidExpected        bool
	}{
		{
			modifyTemplateInstance: func(new *api.TemplateInstance) {
			},
			isValidExpected: true,
		},
		{
			modifyTemplateInstance: func(new *api.TemplateInstance) {
				new.Name = "new"
			},
		},
		{
			modifyTemplateInstance: func(new *api.TemplateInstance) {
				new.Namespace = "new"
			},
		},
		{
			modifyTemplateInstance: func(new *api.TemplateInstance) {
				new.Spec.Template.Name = "new"
			},
		},
		{
			modifyTemplateInstance: func(new *api.TemplateInstance) {
				new.Spec.Template.Name = "b@d"
			},
		},
		{
			modifyTemplateInstance: func(new *api.TemplateInstance) {
				new.Spec.Template.Namespace = "new"
			},
		},
		{
			modifyTemplateInstance: func(new *api.TemplateInstance) {
				new.Spec.Template.Namespace = "b@d"
			},
		},
		{
			modifyTemplateInstance: func(new *api.TemplateInstance) {
				new.Spec.Template.Parameters[0].Name = "new"
			},
		},
		{
			modifyTemplateInstance: func(new *api.TemplateInstance) {
				new.Spec.Template.Parameters[0].Name = "b@d"
			},
		},
		{
			modifyTemplateInstance: func(new *api.TemplateInstance) {
				new.Spec.Template.Parameters = nil
			},
		},
		{
			modifyTemplateInstance: func(new *api.TemplateInstance) {
				new.Spec.Secret.Name = "new"
			},
		},
		{
			modifyTemplateInstance: func(new *api.TemplateInstance) {
				new.Spec.Secret.Name = "b@d"
			},
		},
		{
			modifyTemplateInstance: func(new *api.TemplateInstance) {
				new.Spec.Secret.Name = ""
			},
		},
		{
			modifyTemplateInstance: func(new *api.TemplateInstance) {
				new.Spec.Requester.Username = "new"
			},
		},
		{
			modifyTemplateInstance: func(new *api.TemplateInstance) {
				new.Spec.Requester.Username = ""
			},
		},
	}

	for i, test := range tests {
		newTemplateInstance, err := kapi.Scheme.DeepCopy(oldTemplateInstance)
		if err != nil {
			t.Fatal(err)
		}
		test.modifyTemplateInstance(newTemplateInstance.(*api.TemplateInstance))
		errs := ValidateTemplateInstanceUpdate(newTemplateInstance.(*api.TemplateInstance), oldTemplateInstance)
		if len(errs) != 0 && test.isValidExpected {
			t.Errorf("%d: Unexpected non-empty error list: %v", i, errs.ToAggregate())
		}
		if len(errs) == 0 && !test.isValidExpected {
			t.Errorf("%d: Unexpected empty error list: %v", i, errs.ToAggregate())
		}
	}
}

func TestValidateBrokerTemplateInstance(t *testing.T) {
	var tests = []struct {
		brokerTemplateInstance api.BrokerTemplateInstance
		isValidExpected        bool
	}{
		{
			brokerTemplateInstance: api.BrokerTemplateInstance{
				ObjectMeta: kapi.ObjectMeta{
					Name: validUUID,
				},
				Spec: api.BrokerTemplateInstanceSpec{
					TemplateInstance: kapi.ObjectReference{
						Kind:      "TemplateInstance",
						Name:      "test",
						Namespace: "test",
					},
					Secret: kapi.ObjectReference{
						Kind:      "Secret",
						Name:      "test",
						Namespace: "test",
					},
					BindingIDs: []string{
						validUUID,
					},
				},
			},
			isValidExpected: true,
		},
		{
			brokerTemplateInstance: api.BrokerTemplateInstance{
				ObjectMeta: kapi.ObjectMeta{
					Name: validUUID,
				},
				Spec: api.BrokerTemplateInstanceSpec{
					TemplateInstance: kapi.ObjectReference{
						Kind:      "TemplateInstance",
						Name:      "test",
						Namespace: "test",
					},
					Secret: kapi.ObjectReference{
						Kind:      "Secret",
						Name:      "test",
						Namespace: "test",
					},
				},
			},
			isValidExpected: true,
		},
		{
			brokerTemplateInstance: api.BrokerTemplateInstance{
				ObjectMeta: kapi.ObjectMeta{
					Name: validUUID,
				},
				Spec: api.BrokerTemplateInstanceSpec{
					TemplateInstance: kapi.ObjectReference{
						Kind:      "TemplateInstance",
						Name:      "test",
						Namespace: "test",
					},
					Secret: kapi.ObjectReference{
						Kind:      "Secret",
						Name:      "test",
						Namespace: "test",
					},
					BindingIDs: []string{
						"b@d",
					},
				},
			},
		},
		{
			brokerTemplateInstance: api.BrokerTemplateInstance{
				ObjectMeta: kapi.ObjectMeta{
					Name:      validUUID,
					Namespace: "test",
				},
				Spec: api.BrokerTemplateInstanceSpec{
					TemplateInstance: kapi.ObjectReference{
						Kind:      "TemplateInstance",
						Name:      "test",
						Namespace: "test",
					},
					Secret: kapi.ObjectReference{
						Kind:      "Secret",
						Name:      "test",
						Namespace: "test",
					},
				},
			},
		},
		{
			brokerTemplateInstance: api.BrokerTemplateInstance{
				ObjectMeta: kapi.ObjectMeta{
					Name: validUUID,
				},
				Spec: api.BrokerTemplateInstanceSpec{
					TemplateInstance: kapi.ObjectReference{
						Kind:      "TemplateInstance",
						Name:      "b@d",
						Namespace: "test",
					},
					Secret: kapi.ObjectReference{
						Kind:      "Secret",
						Name:      "test",
						Namespace: "test",
					},
				},
			},
		},
		{
			brokerTemplateInstance: api.BrokerTemplateInstance{
				ObjectMeta: kapi.ObjectMeta{
					Name: validUUID,
				},
				Spec: api.BrokerTemplateInstanceSpec{
					TemplateInstance: kapi.ObjectReference{
						Kind:      "TemplateInstance",
						Name:      "test",
						Namespace: "b@d",
					},
					Secret: kapi.ObjectReference{
						Kind:      "Secret",
						Name:      "test",
						Namespace: "test",
					},
				},
			},
		},
		{
			brokerTemplateInstance: api.BrokerTemplateInstance{
				ObjectMeta: kapi.ObjectMeta{
					Name: validUUID,
				},
				Spec: api.BrokerTemplateInstanceSpec{
					TemplateInstance: kapi.ObjectReference{
						Kind:      "test",
						Name:      "test",
						Namespace: "test",
					},
					Secret: kapi.ObjectReference{
						Kind:      "Secret",
						Name:      "test",
						Namespace: "test",
					},
				},
			},
		},
		{
			brokerTemplateInstance: api.BrokerTemplateInstance{
				ObjectMeta: kapi.ObjectMeta{
					Name: validUUID,
				},
				Spec: api.BrokerTemplateInstanceSpec{
					TemplateInstance: kapi.ObjectReference{
						Kind:      "TemplateInstance",
						Name:      "test",
						Namespace: "test",
					},
					Secret: kapi.ObjectReference{
						Kind:      "Secret",
						Name:      "b@d",
						Namespace: "test",
					},
				},
			},
		},
		{
			brokerTemplateInstance: api.BrokerTemplateInstance{
				ObjectMeta: kapi.ObjectMeta{
					Name: validUUID,
				},
				Spec: api.BrokerTemplateInstanceSpec{
					TemplateInstance: kapi.ObjectReference{
						Kind:      "TemplateInstance",
						Name:      "test",
						Namespace: "test",
					},
					Secret: kapi.ObjectReference{
						Kind:      "Secret",
						Name:      "test",
						Namespace: "b@d",
					},
				},
			},
		},
		{
			brokerTemplateInstance: api.BrokerTemplateInstance{
				ObjectMeta: kapi.ObjectMeta{
					Name: validUUID,
				},
				Spec: api.BrokerTemplateInstanceSpec{
					TemplateInstance: kapi.ObjectReference{
						Kind:      "TemplateInstance",
						Name:      "test",
						Namespace: "test",
					},
					Secret: kapi.ObjectReference{
						Kind:      "test",
						Name:      "test",
						Namespace: "test",
					},
				},
			},
		},
		{
			brokerTemplateInstance: api.BrokerTemplateInstance{
				ObjectMeta: kapi.ObjectMeta{
					Name: validUUID,
				},
				Spec: api.BrokerTemplateInstanceSpec{
					TemplateInstance: kapi.ObjectReference{
						Kind:      "TemplateInstance",
						Name:      "test",
						Namespace: "test",
					},
				},
			},
		},
		{
			brokerTemplateInstance: api.BrokerTemplateInstance{
				ObjectMeta: kapi.ObjectMeta{
					Name: validUUID,
				},
				Spec: api.BrokerTemplateInstanceSpec{
					Secret: kapi.ObjectReference{
						Kind:      "Secret",
						Name:      "test",
						Namespace: "test",
					},
				},
			},
		},
	}

	for i, test := range tests {
		errs := ValidateBrokerTemplateInstance(&test.brokerTemplateInstance)
		if len(errs) != 0 && test.isValidExpected {
			t.Errorf("%d: Unexpected non-empty error list: %v", i, errs.ToAggregate())
		}
		if len(errs) == 0 && !test.isValidExpected {
			t.Errorf("%d: Unexpected empty error list: %v", i, errs.ToAggregate())
		}
	}
}

func TestValidateBrokerTemplateInstanceUpdate(t *testing.T) {
	oldBrokerTemplateInstance := &api.BrokerTemplateInstance{
		ObjectMeta: kapi.ObjectMeta{
			Name:            validUUID,
			ResourceVersion: "1",
		},
		Spec: api.BrokerTemplateInstanceSpec{
			TemplateInstance: kapi.ObjectReference{
				Kind:      "TemplateInstance",
				Name:      "test",
				Namespace: "test",
			},
			Secret: kapi.ObjectReference{
				Kind:      "Secret",
				Name:      "test",
				Namespace: "test",
			},
			BindingIDs: []string{
				validUUID,
			},
		},
	}

	var tests = []struct {
		modifyBrokerTemplateInstance func(*api.BrokerTemplateInstance)
		isValidExpected              bool
	}{
		{
			modifyBrokerTemplateInstance: func(new *api.BrokerTemplateInstance) {
			},
			isValidExpected: true,
		},
		{
			modifyBrokerTemplateInstance: func(new *api.BrokerTemplateInstance) {
				new.Name = "new"
			},
		},
		{
			modifyBrokerTemplateInstance: func(new *api.BrokerTemplateInstance) {
				new.Namespace = "new"
			},
		},
		{
			modifyBrokerTemplateInstance: func(new *api.BrokerTemplateInstance) {
				new.Spec.TemplateInstance.Kind = "new"
			},
		},
		{
			modifyBrokerTemplateInstance: func(new *api.BrokerTemplateInstance) {
				new.Spec.TemplateInstance.Kind = ""
			},
		},
		{
			modifyBrokerTemplateInstance: func(new *api.BrokerTemplateInstance) {
				new.Spec.TemplateInstance.Kind = "b@d"
			},
		},
		{
			modifyBrokerTemplateInstance: func(new *api.BrokerTemplateInstance) {
				new.Spec.TemplateInstance.Name = "new"
			},
			isValidExpected: true,
		},
		{
			modifyBrokerTemplateInstance: func(new *api.BrokerTemplateInstance) {
				new.Spec.TemplateInstance.Name = ""
			},
		},
		{
			modifyBrokerTemplateInstance: func(new *api.BrokerTemplateInstance) {
				new.Spec.TemplateInstance.Name = "b@d"
			},
		},
		{
			modifyBrokerTemplateInstance: func(new *api.BrokerTemplateInstance) {
				new.Spec.TemplateInstance.Namespace = "new"
			},
			isValidExpected: true,
		},
		{
			modifyBrokerTemplateInstance: func(new *api.BrokerTemplateInstance) {
				new.Spec.TemplateInstance.Namespace = ""
			},
		},
		{
			modifyBrokerTemplateInstance: func(new *api.BrokerTemplateInstance) {
				new.Spec.TemplateInstance.Namespace = "b@d"
			},
		},
		{
			modifyBrokerTemplateInstance: func(new *api.BrokerTemplateInstance) {
				new.Spec.BindingIDs = []string{validUUID2}
			},
			isValidExpected: true,
		},
		{
			modifyBrokerTemplateInstance: func(new *api.BrokerTemplateInstance) {
				new.Spec.BindingIDs = nil
			},
			isValidExpected: true,
		},
		{
			modifyBrokerTemplateInstance: func(new *api.BrokerTemplateInstance) {
				new.Spec.BindingIDs = []string{"bad"}
			},
		},
	}

	for i, test := range tests {
		newBrokerTemplateInstance, err := kapi.Scheme.DeepCopy(oldBrokerTemplateInstance)
		if err != nil {
			t.Fatal(err)
		}
		test.modifyBrokerTemplateInstance(newBrokerTemplateInstance.(*api.BrokerTemplateInstance))
		errs := ValidateBrokerTemplateInstanceUpdate(newBrokerTemplateInstance.(*api.BrokerTemplateInstance), oldBrokerTemplateInstance)
		if len(errs) != 0 && test.isValidExpected {
			t.Errorf("%d: Unexpected non-empty error list: %v", i, errs.ToAggregate())
		}
		if len(errs) == 0 && !test.isValidExpected {
			t.Errorf("%d: Unexpected empty error list: %v", i, errs.ToAggregate())
		}
	}
}
