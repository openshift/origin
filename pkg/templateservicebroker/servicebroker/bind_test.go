package servicebroker

import (
	"net/http"
	"reflect"
	"testing"

	templatev1 "github.com/openshift/api/template/v1"
	faketemplatev1 "github.com/openshift/client-go/template/clientset/versioned/typed/template/v1/fake"
	templateapi "github.com/openshift/origin/pkg/template/apis/template"
	"github.com/openshift/origin/pkg/templateservicebroker/openservicebroker/api"

	authorizationv1 "k8s.io/api/authorization/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
	kapi "k8s.io/kubernetes/pkg/apis/core"
)

func TestEvaluateJSONPathExpression(t *testing.T) {
	i := 1
	obj := struct {
		Int            int
		PointerToInt   *int
		InterfaceToInt interface{}
		NullPointer    *int
		NullInterface  interface{}
		IntSlice       []int
		ByteSlice      []byte
		String         string
		Nested         struct {
			NestedInt int
		}
	}{
		Int:            i,
		PointerToInt:   &i,
		InterfaceToInt: i,
		IntSlice:       []int{1, 2, 3},
		ByteSlice:      []byte("testbytes"),
		String:         "teststring",
		Nested: struct {
			NestedInt int
		}{
			NestedInt: 2,
		},
	}

	tests := []struct {
		expression     string
		base64encode   bool
		expectedError  string
		expectedResult string
	}{
		{
			expression:    "{",
			expectedError: "failed to parse annotation a: unclosed action",
		},
		{
			expression:    "{.DoesntExist}",
			expectedError: "FindResults failed on annotation a: DoesntExist is not found",
		},
		{
			expression:    "{.IntSlice[*]}",
			expectedError: "3 JSONPath results found on annotation a",
		},
		{
			expression:    "{.NullPointer}",
			expectedError: "nil kind ptr found in JSONPath result on annotation a",
		},
		{
			expression:    "{.NullInterface}",
			expectedError: "nil kind interface found in JSONPath result on annotation a",
		},
		{
			expression:     "{.Int}",
			expectedResult: "1",
		},
		{
			expression:     "{.PointerToInt}",
			expectedResult: "1",
		},
		{
			expression:     "{.InterfaceToInt}",
			expectedResult: "1",
		},
		{
			expression:     "{.String}",
			expectedResult: "teststring",
		},
		{
			expression:     "{.String}",
			expectedResult: "teststring",
			base64encode:   true,
		},
		{
			expression:     "{.ByteSlice}",
			expectedResult: "testbytes",
		},
		{
			expression:     "{.ByteSlice}",
			expectedResult: "dGVzdGJ5dGVz",
			base64encode:   true,
		},
		{
			expression:    "{.IntSlice}",
			expectedError: "unrepresentable kind slice found in JSONPath result on annotation a",
		},
		{
			expression:    "{.Nested}",
			expectedError: "unrepresentable kind struct found in JSONPath result on annotation a",
		},
		{
			expression:     "{.Nested.NestedInt}",
			expectedResult: "2",
		},
		{
			expression:     "foo/{.ByteSlice}",
			expectedResult: "foo/testbytes",
		},
		{
			expression:     "foo/{.ByteSlice}",
			expectedResult: "foo/dGVzdGJ5dGVz",
			base64encode:   true,
		},
	}

	for i, test := range tests {
		result, err := evaluateJSONPathExpression(obj, "a", test.expression, test.base64encode)
		if err != nil {
			if err.Error() != test.expectedError {
				t.Errorf("%d: error %q", i, err)
			}
			continue
		}
		if result != test.expectedResult {
			t.Errorf("%d: result %q", i, result)
		}
	}
}

func TestDuplicateCredentialKeys(t *testing.T) {
	credentials := map[string]interface{}{}
	err := updateCredentialsForObject(credentials, &kapi.Secret{
		ObjectMeta: v1.ObjectMeta{
			Annotations: map[string]string{
				templateapi.ExposeAnnotationPrefix + "test":       "",
				templateapi.Base64ExposeAnnotationPrefix + "test": "",
			},
		},
	})
	if err.Error() != `credential with key "test" already exists` {
		t.Errorf("unexpected error %q", err)
	}
}

func TestBindConflict(t *testing.T) {
	fakekc := &fake.Clientset{}
	fakekc.AddReactor("create", "subjectaccessreviews", func(action clienttesting.Action) (bool, runtime.Object, error) {
		return true, &authorizationv1.SubjectAccessReview{Status: authorizationv1.SubjectAccessReviewStatus{Allowed: true}}, nil
	})

	faketemplateclient := &faketemplatev1.FakeTemplateV1{Fake: &clienttesting.Fake{}}
	faketemplateclient.AddReactor("get", "brokertemplateinstances", func(action clienttesting.Action) (bool, runtime.Object, error) {
		return true, &templatev1.BrokerTemplateInstance{
			Spec: templatev1.BrokerTemplateInstanceSpec{
				BindingIDs: []string{"alreadyexists"},
			},
		}, nil
	})
	var conflict int
	faketemplateclient.AddReactor("update", "brokertemplateinstances", func(action clienttesting.Action) (bool, runtime.Object, error) {
		if conflict > 0 {
			conflict--
			return true, nil, errors.NewConflict(templatev1.Resource("brokertemplateinstance"), "", nil)
		}
		return true, &templatev1.BrokerTemplateInstance{}, nil
	})

	b := &Broker{
		templateclient: faketemplateclient,
		kc:             fakekc,
	}

	// after 5 conflicts we give up and return ConcurrencyError
	conflict = 5
	resp := b.Bind(&user.DefaultInfo{}, "", "bindingid", &api.BindRequest{})
	if !reflect.DeepEqual(resp, api.NewResponse(http.StatusUnprocessableEntity, &api.ConcurrencyError, nil)) {
		t.Errorf("got response %#v, expected 422/ConcurrencyError", *resp)
	}

	// with fewer conflicts, we should get there in the end
	conflict = 4
	resp = b.Bind(&user.DefaultInfo{}, "", "bindingid", &api.BindRequest{})
	if !reflect.DeepEqual(resp, api.NewResponse(http.StatusCreated, &api.BindResponse{Credentials: map[string]interface{}{}}, nil)) {
		t.Errorf("got response %#v, expected 201", *resp)
	}

	// also check that OK is returned appropriately
	resp = b.Bind(&user.DefaultInfo{}, "", "alreadyexists", &api.BindRequest{})
	if !reflect.DeepEqual(resp, api.NewResponse(http.StatusOK, &api.BindResponse{Credentials: map[string]interface{}{}}, nil)) {
		t.Errorf("got response %#v, expected 200", *resp)
	}
}
