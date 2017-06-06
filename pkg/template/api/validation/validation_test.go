package validation

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	kapi "k8s.io/kubernetes/pkg/api"

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
				ObjectMeta: metav1.ObjectMeta{Name: "templateId"},
			},
			true,
		},
		{ // Template with invalid Parameter, should fail on Parameter name
			&api.Template{
				ObjectMeta: metav1.ObjectMeta{Name: "templateId"},
				Parameters: []api.Parameter{
					*(makeParameter("", "1")),
				},
			},
			false,
		},
		{ // Template with valid Parameter, should pass
			&api.Template{
				ObjectMeta: metav1.ObjectMeta{Name: "templateId"},
				Parameters: []api.Parameter{
					*(makeParameter("VALname_NAME", "1")),
				},
			},
			true,
		},
		{ // Template with Item of unknown Kind, should pass
			&api.Template{
				ObjectMeta: metav1.ObjectMeta{Name: "templateId"},
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
				ObjectMeta: metav1.ObjectMeta{
					Name:      "template",
					Namespace: metav1.NamespaceDefault,
				},
			},
			true,
		},
		{ // Template without namespace, should fail
			&api.Template{
				ObjectMeta: metav1.ObjectMeta{
					Name: "template",
				},
			},
			false,
		},
		{ // Template with invalid name characters, should fail
			&api.Template{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "templateId",
					Namespace: metav1.NamespaceDefault,
				},
			},
			false,
		},
		{ // Template with invalid Parameter, should fail on Parameter name
			&api.Template{
				ObjectMeta: metav1.ObjectMeta{Name: "template", Namespace: metav1.NamespaceDefault},
				Parameters: []api.Parameter{
					*(makeParameter("", "1")),
				},
			},
			false,
		},
		{ // Template with valid Parameter, should pass
			&api.Template{
				ObjectMeta: metav1.ObjectMeta{Name: "template", Namespace: metav1.NamespaceDefault},
				Parameters: []api.Parameter{
					*(makeParameter("VALname_NAME", "1")),
				},
			},
			true,
		},
		{ // Template with empty items, should pass
			&api.Template{
				ObjectMeta: metav1.ObjectMeta{Name: "template", Namespace: metav1.NamespaceDefault},
				Parameters: []api.Parameter{},
				Objects:    []runtime.Object{},
			},
			true,
		},
		{ // Template with an item that is invalid, should pass
			&api.Template{
				ObjectMeta: metav1.ObjectMeta{Name: "template", Namespace: metav1.NamespaceDefault},
				Parameters: []api.Parameter{},
				Objects: []runtime.Object{
					&kapi.Service{
						ObjectMeta: metav1.ObjectMeta{
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
		templateInstance  api.TemplateInstance
		expectedErrorType field.ErrorType
	}{
		{
			templateInstance:  api.TemplateInstance{},
			expectedErrorType: field.ErrorTypeRequired,
		},
		{
			templateInstance: api.TemplateInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
				},
			},
			expectedErrorType: field.ErrorTypeRequired,
		},
		{
			templateInstance: api.TemplateInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
				},
				Spec: api.TemplateInstanceSpec{},
			},
			expectedErrorType: field.ErrorTypeRequired,
		},
		{
			templateInstance: api.TemplateInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
				},
				Spec: api.TemplateInstanceSpec{
					Template: api.Template{},
				},
			},
			expectedErrorType: field.ErrorTypeRequired,
		},
		{
			templateInstance: api.TemplateInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
				},
				Spec: api.TemplateInstanceSpec{
					Template: api.Template{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test",
							Namespace: "test",
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
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
				},
				Spec: api.TemplateInstanceSpec{
					Template: api.Template{
						ObjectMeta: metav1.ObjectMeta{
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
			expectedErrorType: field.ErrorTypeInvalid,
		},
		{
			templateInstance: api.TemplateInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
				},
				Spec: api.TemplateInstanceSpec{
					Template: api.Template{
						ObjectMeta: metav1.ObjectMeta{
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
			expectedErrorType: field.ErrorTypeInvalid,
		},
		{
			templateInstance: api.TemplateInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
				},
				Spec: api.TemplateInstanceSpec{
					Template: api.Template{
						ObjectMeta: metav1.ObjectMeta{
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
		},
	}

	for i, test := range tests {
		errs := ValidateTemplateInstance(&test.templateInstance)
		if test.expectedErrorType == "" {
			if len(errs) != 0 {
				t.Errorf("%d: Unexpected non-empty error list", i)
			}
		} else {
			if len(errs) == 0 {
				t.Errorf("%d: Unexpected length error list: %v", i, errs.ToAggregate())
			} else {
				for _, err := range errs {
					if err.Type != test.expectedErrorType {
						t.Errorf("%d: Unexpected error type: %v", i, errs.ToAggregate())
					}
				}
			}
		}
	}
}

func TestValidateTemplateInstanceUpdate(t *testing.T) {
	oldTemplateInstance := &api.TemplateInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "test",
			Namespace:       "test",
			ResourceVersion: "1",
		},
		Spec: api.TemplateInstanceSpec{
			Template: api.Template{
				ObjectMeta: metav1.ObjectMeta{
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
		expectedErrorType      field.ErrorType
	}{
		{
			modifyTemplateInstance: func(new *api.TemplateInstance) {
			},
		},
		{
			modifyTemplateInstance: func(new *api.TemplateInstance) {
				new.Name = "new"
			},
			expectedErrorType: field.ErrorTypeInvalid,
		},
		{
			modifyTemplateInstance: func(new *api.TemplateInstance) {
				new.Namespace = "new"
			},
			expectedErrorType: field.ErrorTypeInvalid,
		},
		{
			modifyTemplateInstance: func(new *api.TemplateInstance) {
				new.Spec.Template.Name = "new"
			},
			expectedErrorType: field.ErrorTypeForbidden,
		},
		{
			modifyTemplateInstance: func(new *api.TemplateInstance) {
				new.Spec.Template.Name = "b@d"
			},
			expectedErrorType: field.ErrorTypeForbidden,
		},
		{
			modifyTemplateInstance: func(new *api.TemplateInstance) {
				new.Spec.Template.Namespace = "new"
			},
			expectedErrorType: field.ErrorTypeForbidden,
		},
		{
			modifyTemplateInstance: func(new *api.TemplateInstance) {
				new.Spec.Template.Namespace = "b@d"
			},
			expectedErrorType: field.ErrorTypeForbidden,
		},
		{
			modifyTemplateInstance: func(new *api.TemplateInstance) {
				new.Spec.Template.Parameters[0].Name = "new"
			},
			expectedErrorType: field.ErrorTypeForbidden,
		},
		{
			modifyTemplateInstance: func(new *api.TemplateInstance) {
				new.Spec.Template.Parameters[0].Name = "b@d"
			},
			expectedErrorType: field.ErrorTypeForbidden,
		},
		{
			modifyTemplateInstance: func(new *api.TemplateInstance) {
				new.Spec.Template.Parameters = nil
			},
			expectedErrorType: field.ErrorTypeForbidden,
		},
		{
			modifyTemplateInstance: func(new *api.TemplateInstance) {
				new.Spec.Secret.Name = "new"
			},
			expectedErrorType: field.ErrorTypeForbidden,
		},
		{
			modifyTemplateInstance: func(new *api.TemplateInstance) {
				new.Spec.Secret.Name = "b@d"
			},
			expectedErrorType: field.ErrorTypeForbidden,
		},
		{
			modifyTemplateInstance: func(new *api.TemplateInstance) {
				new.Spec.Secret.Name = ""
			},
			expectedErrorType: field.ErrorTypeForbidden,
		},
		{
			modifyTemplateInstance: func(new *api.TemplateInstance) {
				new.Spec.Requester.Username = "new"
			},
			expectedErrorType: field.ErrorTypeForbidden,
		},
		{
			modifyTemplateInstance: func(new *api.TemplateInstance) {
				new.Spec.Requester.Username = ""
			},
			expectedErrorType: field.ErrorTypeForbidden,
		},
	}

	for i, test := range tests {
		newTemplateInstance, err := kapi.Scheme.DeepCopy(oldTemplateInstance)
		if err != nil {
			t.Fatal(err)
		}
		test.modifyTemplateInstance(newTemplateInstance.(*api.TemplateInstance))
		errs := ValidateTemplateInstanceUpdate(newTemplateInstance.(*api.TemplateInstance), oldTemplateInstance)
		if test.expectedErrorType == "" {
			if len(errs) != 0 {
				t.Errorf("%d: Unexpected non-empty error list", i)
			}
		} else {
			if len(errs) == 0 {
				t.Errorf("%d: Unexpected length error list: %v", i, errs.ToAggregate())
			} else {
				for _, err := range errs {
					if err.Type != test.expectedErrorType {
						t.Errorf("%d: Unexpected error type: %v", i, errs.ToAggregate())
					}
				}
			}
		}
	}
}

func TestValidateBrokerTemplateInstance(t *testing.T) {
	var tests = []struct {
		brokerTemplateInstance api.BrokerTemplateInstance
		expectedErrorType      field.ErrorType
	}{
		{
			brokerTemplateInstance: api.BrokerTemplateInstance{
				ObjectMeta: metav1.ObjectMeta{
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
		},
		{
			brokerTemplateInstance: api.BrokerTemplateInstance{
				ObjectMeta: metav1.ObjectMeta{
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
		},
		{
			brokerTemplateInstance: api.BrokerTemplateInstance{
				ObjectMeta: metav1.ObjectMeta{
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
			expectedErrorType: field.ErrorTypeInvalid,
		},
		{
			brokerTemplateInstance: api.BrokerTemplateInstance{
				ObjectMeta: metav1.ObjectMeta{
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
			expectedErrorType: field.ErrorTypeForbidden,
		},
		{
			brokerTemplateInstance: api.BrokerTemplateInstance{
				ObjectMeta: metav1.ObjectMeta{
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
			expectedErrorType: field.ErrorTypeInvalid,
		},
		{
			brokerTemplateInstance: api.BrokerTemplateInstance{
				ObjectMeta: metav1.ObjectMeta{
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
			expectedErrorType: field.ErrorTypeInvalid,
		},
		{
			brokerTemplateInstance: api.BrokerTemplateInstance{
				ObjectMeta: metav1.ObjectMeta{
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
			expectedErrorType: field.ErrorTypeInvalid,
		},
		{
			brokerTemplateInstance: api.BrokerTemplateInstance{
				ObjectMeta: metav1.ObjectMeta{
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
			expectedErrorType: field.ErrorTypeInvalid,
		},
		{
			brokerTemplateInstance: api.BrokerTemplateInstance{
				ObjectMeta: metav1.ObjectMeta{
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
			expectedErrorType: field.ErrorTypeInvalid,
		},
		{
			brokerTemplateInstance: api.BrokerTemplateInstance{
				ObjectMeta: metav1.ObjectMeta{
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
			expectedErrorType: field.ErrorTypeInvalid,
		},
		{
			brokerTemplateInstance: api.BrokerTemplateInstance{
				ObjectMeta: metav1.ObjectMeta{
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
			expectedErrorType: field.ErrorTypeRequired,
		},
		{
			brokerTemplateInstance: api.BrokerTemplateInstance{
				ObjectMeta: metav1.ObjectMeta{
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
			expectedErrorType: field.ErrorTypeRequired,
		},
	}

	for i, test := range tests {
		errs := ValidateBrokerTemplateInstance(&test.brokerTemplateInstance)
		if test.expectedErrorType == "" {
			if len(errs) != 0 {
				t.Errorf("%d: Unexpected non-empty error list", i)
			}
		} else {
			if len(errs) == 0 {
				t.Errorf("%d: Unexpected length error list: %v", i, errs.ToAggregate())
			} else {
				for _, err := range errs {
					if err.Type != test.expectedErrorType {
						t.Errorf("%d: Unexpected error type: %v", i, errs.ToAggregate())
					}
				}
			}
		}
	}
}

func TestValidateBrokerTemplateInstanceUpdate(t *testing.T) {
	oldBrokerTemplateInstance := &api.BrokerTemplateInstance{
		ObjectMeta: metav1.ObjectMeta{
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
		expectedErrorType            field.ErrorType
	}{
		{
			modifyBrokerTemplateInstance: func(new *api.BrokerTemplateInstance) {
			},
		},
		{
			modifyBrokerTemplateInstance: func(new *api.BrokerTemplateInstance) {
				new.Name = "new"
			},
			expectedErrorType: field.ErrorTypeInvalid,
		},
		{
			modifyBrokerTemplateInstance: func(new *api.BrokerTemplateInstance) {
				new.Namespace = "new"
			},
			expectedErrorType: field.ErrorTypeInvalid,
		},
		{
			modifyBrokerTemplateInstance: func(new *api.BrokerTemplateInstance) {
				new.Spec.TemplateInstance.Kind = "new"
			},
			expectedErrorType: field.ErrorTypeInvalid,
		},
		{
			modifyBrokerTemplateInstance: func(new *api.BrokerTemplateInstance) {
				new.Spec.TemplateInstance.Kind = ""
			},
			expectedErrorType: field.ErrorTypeRequired,
		},
		{
			modifyBrokerTemplateInstance: func(new *api.BrokerTemplateInstance) {
				new.Spec.TemplateInstance.Kind = "b@d"
			},
			expectedErrorType: field.ErrorTypeInvalid,
		},
		{
			modifyBrokerTemplateInstance: func(new *api.BrokerTemplateInstance) {
				new.Spec.TemplateInstance.Name = "new"
			},
		},
		{
			modifyBrokerTemplateInstance: func(new *api.BrokerTemplateInstance) {
				new.Spec.TemplateInstance.Name = ""
			},
			expectedErrorType: field.ErrorTypeRequired,
		},
		{
			modifyBrokerTemplateInstance: func(new *api.BrokerTemplateInstance) {
				new.Spec.TemplateInstance.Name = "b@d"
			},
			expectedErrorType: field.ErrorTypeInvalid,
		},
		{
			modifyBrokerTemplateInstance: func(new *api.BrokerTemplateInstance) {
				new.Spec.TemplateInstance.Namespace = "new"
			},
		},
		{
			modifyBrokerTemplateInstance: func(new *api.BrokerTemplateInstance) {
				new.Spec.TemplateInstance.Namespace = ""
			},
			expectedErrorType: field.ErrorTypeRequired,
		},
		{
			modifyBrokerTemplateInstance: func(new *api.BrokerTemplateInstance) {
				new.Spec.TemplateInstance.Namespace = "b@d"
			},
			expectedErrorType: field.ErrorTypeInvalid,
		},
		{
			modifyBrokerTemplateInstance: func(new *api.BrokerTemplateInstance) {
				new.Spec.BindingIDs = []string{validUUID2}
			},
		},
		{
			modifyBrokerTemplateInstance: func(new *api.BrokerTemplateInstance) {
				new.Spec.BindingIDs = nil
			},
		},
		{
			modifyBrokerTemplateInstance: func(new *api.BrokerTemplateInstance) {
				new.Spec.BindingIDs = []string{"bad"}
			},
			expectedErrorType: field.ErrorTypeInvalid,
		},
	}

	for i, test := range tests {
		newBrokerTemplateInstance, err := kapi.Scheme.DeepCopy(oldBrokerTemplateInstance)
		if err != nil {
			t.Fatal(err)
		}
		test.modifyBrokerTemplateInstance(newBrokerTemplateInstance.(*api.BrokerTemplateInstance))
		errs := ValidateBrokerTemplateInstanceUpdate(newBrokerTemplateInstance.(*api.BrokerTemplateInstance), oldBrokerTemplateInstance)
		if test.expectedErrorType == "" {
			if len(errs) != 0 {
				t.Errorf("%d: Unexpected non-empty error list", i)
			}
		} else {
			if len(errs) == 0 {
				t.Errorf("%d: Unexpected length error list: %v", i, errs.ToAggregate())
			} else {
				for _, err := range errs {
					if err.Type != test.expectedErrorType {
						t.Errorf("%d: Unexpected error type: %v", i, errs.ToAggregate())
					}
				}
			}
		}
	}
}
