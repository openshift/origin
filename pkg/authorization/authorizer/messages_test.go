package authorizer

import (
	"testing"

	"k8s.io/kubernetes/pkg/auth/user"
)

func TestDefaultForbiddenMessages(t *testing.T) {
	messageResolver := NewForbiddenMessageResolver("")

	apiForbidden, err := messageResolver.defaultForbiddenMessageMaker.MakeMessage(MessageContext{
		User:      &user.DefaultInfo{Name: "chris"},
		Namespace: "foo",
		Attributes: DefaultAuthorizationAttributes{
			Verb:         "get",
			Resource:     "pods",
			ResourceName: "hammer",
		},
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	expectedAPIForbidden := `User "chris" cannot "get" "pods" with name "hammer" in project "foo"`
	if expectedAPIForbidden != apiForbidden {
		t.Errorf("expected %v, got %v", expectedAPIForbidden, apiForbidden)
	}

	messageTest{
		MessageContext{
			User:      &user.DefaultInfo{Name: "chris"},
			Namespace: "foo",
			Attributes: DefaultAuthorizationAttributes{
				Verb:           "post",
				NonResourceURL: true,
				URL:            "/anything",
			},
		},
		`User "chris" cannot "post" on "/anything"`,
	}.run(t)
}

func TestProjectRequestForbiddenMessage(t *testing.T) {
	messageTest{
		MessageContext{
			User: &user.DefaultInfo{Name: "chris"},
			Attributes: DefaultAuthorizationAttributes{
				Verb:     "create",
				Resource: "projectrequests",
			},
		},
		DefaultProjectRequestForbidden,
	}.run(t)
}

func TestNamespacedForbiddenMessage(t *testing.T) {
	messageTest{
		MessageContext{
			User:      &user.DefaultInfo{Name: "chris"},
			Namespace: "foo",
			Attributes: DefaultAuthorizationAttributes{
				Verb:     "create",
				Resource: "builds",
			},
		},
		`User "chris" cannot create builds in project "foo"`,
	}.run(t)

	messageTest{
		MessageContext{
			User:      &user.DefaultInfo{Name: "chris"},
			Namespace: "foo",
			Attributes: DefaultAuthorizationAttributes{
				Verb:     "get",
				Resource: "builds",
			},
		},
		`User "chris" cannot get builds in project "foo"`,
	}.run(t)

	messageTest{
		MessageContext{
			User:      &user.DefaultInfo{Name: "chris"},
			Namespace: "foo",
			Attributes: DefaultAuthorizationAttributes{
				Verb:     "list",
				Resource: "builds",
			},
		},
		`User "chris" cannot list builds in project "foo"`,
	}.run(t)

	messageTest{
		MessageContext{
			User:      &user.DefaultInfo{Name: "chris"},
			Namespace: "foo",
			Attributes: DefaultAuthorizationAttributes{
				Verb:     "update",
				Resource: "builds",
			},
		},
		`User "chris" cannot update builds in project "foo"`,
	}.run(t)

	messageTest{
		MessageContext{
			User:      &user.DefaultInfo{Name: "chris"},
			Namespace: "foo",
			Attributes: DefaultAuthorizationAttributes{
				Verb:     "delete",
				Resource: "builds",
			},
		},
		`User "chris" cannot delete builds in project "foo"`,
	}.run(t)

}

func TestRootScopedForbiddenMessage(t *testing.T) {
	messageTest{
		MessageContext{
			User: &user.DefaultInfo{Name: "chris"},
			Attributes: DefaultAuthorizationAttributes{
				Verb:     "create",
				Resource: "builds",
			},
		},
		`User "chris" cannot create builds at the cluster scope`,
	}.run(t)

	messageTest{
		MessageContext{
			User: &user.DefaultInfo{Name: "chris"},
			Attributes: DefaultAuthorizationAttributes{
				Verb:     "get",
				Resource: "builds",
			},
		},
		`User "chris" cannot get builds at the cluster scope`,
	}.run(t)

	messageTest{
		MessageContext{
			User: &user.DefaultInfo{Name: "chris"},
			Attributes: DefaultAuthorizationAttributes{
				Verb:     "list",
				Resource: "builds",
			},
		},
		`User "chris" cannot list all builds in the cluster`,
	}.run(t)

	messageTest{
		MessageContext{
			User: &user.DefaultInfo{Name: "chris"},
			Attributes: DefaultAuthorizationAttributes{
				Verb:     "update",
				Resource: "builds",
			},
		},
		`User "chris" cannot update builds at the cluster scope`,
	}.run(t)

	messageTest{
		MessageContext{
			User: &user.DefaultInfo{Name: "chris"},
			Attributes: DefaultAuthorizationAttributes{
				Verb:     "delete",
				Resource: "builds",
			},
		},
		`User "chris" cannot delete builds at the cluster scope`,
	}.run(t)

}

type messageTest struct {
	messageContext MessageContext
	expected       string
}

func (test messageTest) run(t *testing.T) {
	messageResolver := NewForbiddenMessageResolver("")

	forbidden, err := messageResolver.MakeMessage(test.messageContext)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if test.expected != forbidden {
		t.Errorf("expected %v, got %v", test.expected, forbidden)
	}
}
