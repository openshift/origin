package authorizer

import (
	"testing"

	kauthorizer "k8s.io/kubernetes/pkg/auth/authorizer"
	"k8s.io/kubernetes/pkg/auth/user"
)

func TestDefaultForbiddenMessages(t *testing.T) {
	messageResolver := NewForbiddenMessageResolver("")

	apiForbidden, err := messageResolver.defaultForbiddenMessageMaker.MakeMessage(MessageContext{
		Attributes: kauthorizer.AttributesRecord{
			ResourceRequest: true,
			User:            &user.DefaultInfo{Name: "chris"},
			Namespace:       "foo",
			Verb:            "get",
			Resource:        "pods",
			Name:            "hammer",
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
			Attributes: kauthorizer.AttributesRecord{
				ResourceRequest: false,
				User:            &user.DefaultInfo{Name: "chris"},
				Namespace:       "foo",
				Verb:            "post",
				Path:            "/anything",
			},
		},
		`User "chris" cannot "post" on "/anything"`,
	}.run(t)
}

func TestProjectRequestForbiddenMessage(t *testing.T) {
	messageTest{
		MessageContext{
			Attributes: kauthorizer.AttributesRecord{
				ResourceRequest: true,
				User:            &user.DefaultInfo{Name: "chris"},
				Verb:            "create",
				Resource:        "projectrequests",
			},
		},
		DefaultProjectRequestForbidden,
	}.run(t)
}

func TestNamespacedForbiddenMessage(t *testing.T) {
	messageTest{
		MessageContext{
			Attributes: kauthorizer.AttributesRecord{
				ResourceRequest: true,
				User:            &user.DefaultInfo{Name: "chris"},
				Namespace:       "foo",
				Verb:            "create",
				Resource:        "builds",
			},
		},
		`User "chris" cannot create builds in project "foo"`,
	}.run(t)

	messageTest{
		MessageContext{
			Attributes: kauthorizer.AttributesRecord{
				ResourceRequest: true,
				User:            &user.DefaultInfo{Name: "chris"},
				Namespace:       "foo",
				Verb:            "get",
				Resource:        "builds",
			},
		},
		`User "chris" cannot get builds in project "foo"`,
	}.run(t)

	messageTest{
		MessageContext{
			Attributes: kauthorizer.AttributesRecord{
				ResourceRequest: true,
				User:            &user.DefaultInfo{Name: "chris"},
				Namespace:       "foo",
				Verb:            "list",
				Resource:        "builds",
			},
		},
		`User "chris" cannot list builds in project "foo"`,
	}.run(t)

	messageTest{
		MessageContext{
			Attributes: kauthorizer.AttributesRecord{
				ResourceRequest: true,
				User:            &user.DefaultInfo{Name: "chris"},
				Namespace:       "foo",
				Verb:            "update",
				Resource:        "builds",
			},
		},
		`User "chris" cannot update builds in project "foo"`,
	}.run(t)

	messageTest{
		MessageContext{
			Attributes: kauthorizer.AttributesRecord{
				ResourceRequest: true,
				User:            &user.DefaultInfo{Name: "chris"},
				Namespace:       "foo",
				Verb:            "delete",
				Resource:        "builds",
			},
		},
		`User "chris" cannot delete builds in project "foo"`,
	}.run(t)

}

func TestRootScopedForbiddenMessage(t *testing.T) {
	messageTest{
		MessageContext{
			Attributes: kauthorizer.AttributesRecord{
				ResourceRequest: true,
				User:            &user.DefaultInfo{Name: "chris"},
				Verb:            "create",
				Resource:        "builds",
			},
		},
		`User "chris" cannot create builds at the cluster scope`,
	}.run(t)

	messageTest{
		MessageContext{
			Attributes: kauthorizer.AttributesRecord{
				ResourceRequest: true,
				User:            &user.DefaultInfo{Name: "chris"},
				Verb:            "get",
				Resource:        "builds",
			},
		},
		`User "chris" cannot get builds at the cluster scope`,
	}.run(t)

	messageTest{
		MessageContext{
			Attributes: kauthorizer.AttributesRecord{
				ResourceRequest: true,
				User:            &user.DefaultInfo{Name: "chris"},
				Verb:            "list",
				Resource:        "builds",
			},
		},
		`User "chris" cannot list all builds in the cluster`,
	}.run(t)

	messageTest{
		MessageContext{
			Attributes: kauthorizer.AttributesRecord{
				ResourceRequest: true,
				User:            &user.DefaultInfo{Name: "chris"},
				Verb:            "update",
				Resource:        "builds",
			},
		},
		`User "chris" cannot update builds at the cluster scope`,
	}.run(t)

	messageTest{
		MessageContext{
			Attributes: kauthorizer.AttributesRecord{
				ResourceRequest: true,
				User:            &user.DefaultInfo{Name: "chris"},
				Verb:            "delete",
				Resource:        "builds",
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
