package authorizer

import (
	"testing"

	"k8s.io/apiserver/pkg/authentication/user"
	kauthorizer "k8s.io/apiserver/pkg/authorization/authorizer"
)

func TestDefaultForbiddenMessages(t *testing.T) {
	messageResolver := NewForbiddenMessageResolver("")

	apiForbidden, err := messageResolver.defaultForbiddenMessageMaker.MakeMessage(
		kauthorizer.AttributesRecord{
			ResourceRequest: true,
			User:            &user.DefaultInfo{Name: "chris"},
			Namespace:       "foo",
			Verb:            "get",
			Resource:        "pods",
			Name:            "hammer",
		},
	)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	expectedAPIForbidden := `User "chris" cannot "get" "pods" with name "hammer" in project "foo"`
	if expectedAPIForbidden != apiForbidden {
		t.Errorf("expected %v, got %v", expectedAPIForbidden, apiForbidden)
	}

	messageTest{
		attributes: kauthorizer.AttributesRecord{
			ResourceRequest: false,
			User:            &user.DefaultInfo{Name: "chris"},
			Namespace:       "foo",
			Verb:            "post",
			Path:            "/anything",
		},
		expected: `User "chris" cannot "post" on "/anything"`,
	}.run(t)
}

func TestAttributeRecord(t *testing.T) {
	messageTest{
		attributes: kauthorizer.AttributesRecord{
			ResourceRequest: true,
			User:            &user.DefaultInfo{Name: "chris"},
			Namespace:       "foo",
			Verb:            "get",
			Resource:        "users",
			APIGroup:        "user.openshift.io",
			APIVersion:      "v1",
			Name:            "hammer",
		},
		expected: `User "chris" cannot get users.user.openshift.io in project "foo"`,
	}.run(t)
	messageTest{
		attributes: kauthorizer.AttributesRecord{
			ResourceRequest: false,
			User:            &user.DefaultInfo{Name: "chris"},
			Namespace:       "foo",
			Verb:            "get",
			Resource:        "users",
			APIGroup:        "user.openshift.io",
			APIVersion:      "v1",
			Path:            "/",
		},
		expected: `User "chris" cannot "get" on "/"`,
	}.run(t)
}

func TestProjectRequestForbiddenMessage(t *testing.T) {
	messageTest{
		attributes: kauthorizer.AttributesRecord{
			ResourceRequest: true,
			User:            &user.DefaultInfo{Name: "chris"},
			Verb:            "create",
			Resource:        "projectrequests",
		},
		expected: DefaultProjectRequestForbidden,
	}.run(t)
}

func TestNamespacedForbiddenMessage(t *testing.T) {
	messageTest{
		attributes: kauthorizer.AttributesRecord{
			ResourceRequest: true,
			User:            &user.DefaultInfo{Name: "chris"},
			Namespace:       "foo",
			Verb:            "create",
			Resource:        "builds",
		},
		expected: `User "chris" cannot create builds in project "foo"`,
	}.run(t)

	messageTest{
		attributes: kauthorizer.AttributesRecord{
			ResourceRequest: true,
			User:            &user.DefaultInfo{Name: "chris"},
			Namespace:       "foo",
			Verb:            "get",
			Resource:        "builds",
		},
		expected: `User "chris" cannot get builds in project "foo"`,
	}.run(t)

	messageTest{
		attributes: kauthorizer.AttributesRecord{
			ResourceRequest: true,
			User:            &user.DefaultInfo{Name: "chris"},
			Namespace:       "foo",
			Verb:            "list",
			Resource:        "builds",
		},
		expected: `User "chris" cannot list builds in project "foo"`,
	}.run(t)

	messageTest{
		attributes: kauthorizer.AttributesRecord{
			ResourceRequest: true,
			User:            &user.DefaultInfo{Name: "chris"},
			Namespace:       "foo",
			Verb:            "update",
			Resource:        "builds",
		},
		expected: `User "chris" cannot update builds in project "foo"`,
	}.run(t)

	messageTest{
		attributes: kauthorizer.AttributesRecord{
			ResourceRequest: true,
			User:            &user.DefaultInfo{Name: "chris"},
			Namespace:       "foo",
			Verb:            "delete",
			Resource:        "builds",
		},
		expected: `User "chris" cannot delete builds in project "foo"`,
	}.run(t)

}

func TestRootScopedForbiddenMessage(t *testing.T) {
	messageTest{
		attributes: kauthorizer.AttributesRecord{
			ResourceRequest: true,
			User:            &user.DefaultInfo{Name: "chris"},
			Verb:            "create",
			Resource:        "builds",
		},
		expected: `User "chris" cannot create builds at the cluster scope`,
	}.run(t)

	messageTest{
		attributes: kauthorizer.AttributesRecord{
			ResourceRequest: true,
			User:            &user.DefaultInfo{Name: "chris"},
			Verb:            "get",
			Resource:        "builds",
		},
		expected: `User "chris" cannot get builds at the cluster scope`,
	}.run(t)

	messageTest{
		attributes: kauthorizer.AttributesRecord{
			ResourceRequest: true,
			User:            &user.DefaultInfo{Name: "chris"},
			Verb:            "list",
			Resource:        "builds",
		},
		expected: `User "chris" cannot list all builds in the cluster`,
	}.run(t)

	messageTest{
		attributes: kauthorizer.AttributesRecord{
			ResourceRequest: true,
			User:            &user.DefaultInfo{Name: "chris"},
			Verb:            "update",
			Resource:        "builds",
		},
		expected: `User "chris" cannot update builds at the cluster scope`,
	}.run(t)

	messageTest{
		attributes: kauthorizer.AttributesRecord{
			ResourceRequest: true,
			User:            &user.DefaultInfo{Name: "chris"},
			Verb:            "delete",
			Resource:        "builds",
		},
		expected: `User "chris" cannot delete builds at the cluster scope`,
	}.run(t)

}

type messageTest struct {
	attributes kauthorizer.Attributes
	expected   string
}

func (test messageTest) run(t *testing.T) {
	messageResolver := NewForbiddenMessageResolver("")

	forbidden, err := messageResolver.MakeMessage(test.attributes)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if test.expected != forbidden {
		t.Errorf("expected %v, got %v", test.expected, forbidden)
	}
}
