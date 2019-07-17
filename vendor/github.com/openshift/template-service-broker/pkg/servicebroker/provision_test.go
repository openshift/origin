package servicebroker

import (
	"net/http"
	"reflect"
	"testing"

	templateapiv1 "github.com/openshift/api/template/v1"
	templatev1 "github.com/openshift/api/template/v1"
	faketemplatev1 "github.com/openshift/client-go/template/clientset/versioned/typed/template/v1/fake"
	templatelister "github.com/openshift/client-go/template/listers/template/v1"

	"github.com/openshift/template-service-broker/pkg/openservicebroker/api"

	authorizationv1 "k8s.io/api/authorization/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
)

type fakeTemplateLister struct{}

func (fakeTemplateLister) List(selector labels.Selector) ([]*templatev1.Template, error) {
	return nil, nil
}

func (fakeTemplateLister) Templates(namespace string) templatelister.TemplateNamespaceLister {
	return nil
}

func (fakeTemplateLister) GetByUID(uid string) (*templatev1.Template, error) {
	return &templatev1.Template{}, nil
}

var _ templatelister.TemplateLister = &fakeTemplateLister{}

func TestProvisionConflict(t *testing.T) {
	fakekc := &fake.Clientset{}
	fakekc.AddReactor("create", "subjectaccessreviews", func(action clienttesting.Action) (bool, runtime.Object, error) {
		return true, &authorizationv1.SubjectAccessReview{Status: authorizationv1.SubjectAccessReviewStatus{Allowed: true}}, nil
	})

	faketemplateclient := &faketemplatev1.FakeTemplateV1{Fake: &clienttesting.Fake{}}
	var conflict int
	faketemplateclient.AddReactor("update", "brokertemplateinstances", func(action clienttesting.Action) (bool, runtime.Object, error) {
		if conflict > 0 {
			conflict--
			return true, nil, errors.NewConflict(templatev1.Resource("brokertemplateinstance"), "", nil)
		}
		return true, &templatev1.BrokerTemplateInstance{}, nil
	})

	b := &Broker{
		templateclient:     faketemplateclient,
		kc:                 fakekc,
		lister:             &fakeTemplateLister{},
		templateNamespaces: map[string]struct{}{"": {}},
	}

	// after 5 conflicts we give up and return ConcurrencyError
	conflict = 5
	resp := b.Provision(&user.DefaultInfo{}, "", &api.ProvisionRequest{})
	if !reflect.DeepEqual(resp, api.NewResponse(http.StatusUnprocessableEntity, &api.ConcurrencyError, nil)) {
		t.Errorf("got response %#v, expected 422/ConcurrencyError", *resp)
	}

	// with fewer conflicts, we should get there in the end
	conflict = 4
	resp = b.Provision(&user.DefaultInfo{}, "", &api.ProvisionRequest{})
	if !reflect.DeepEqual(resp, api.NewResponse(http.StatusAccepted, api.ProvisionResponse{Operation: api.OperationProvisioning}, nil)) {
		t.Errorf("got response %#v, expected 202", *resp)
	}
}

func TestEnsureSecret(t *testing.T) {
	fakekc := fake.NewSimpleClientset()

	fakekc.PrependReactor("create", "subjectaccessreviews", func(action clienttesting.Action) (bool, runtime.Object, error) {
		return true, &authorizationv1.SubjectAccessReview{Status: authorizationv1.SubjectAccessReviewStatus{Allowed: true}}, nil
	})

	faketemplateclient := &faketemplatev1.FakeTemplateV1{Fake: &clienttesting.Fake{}}

	bti := &templateapiv1.BrokerTemplateInstance{}

	b := &Broker{
		kc:             fakekc,
		templateclient: faketemplateclient,
	}

	didWork := true

	const (
		defaultInstanceID    = "12345"
		defaultSourceRepoUrl = "https://github.com/testorg/testrepo"
		defaultContextDir    = "tools"
		defaultSourceRepoRef = "v1.0.1"
		defaultClusterPasswd = "abcd1234"
	)

	template := &templateapiv1.Template{
		Parameters: []templateapiv1.Parameter{
			{
				Name:        "SOURCE_REPOSITORY_URL",
				DisplayName: "Git source URI for application",
				Description: "Git Repository URL",
				Value:       defaultSourceRepoUrl,
				Required:    true,
			},
			{
				Name:        "CONTEXT_DIR",
				DisplayName: "Context Directory",
				Description: "Path within Git project to build; empty for root project directory.",
				Value:       defaultContextDir,
				Required:    false,
			},
			{
				Name:        "SOURCE_REPOSITORY_REF",
				DisplayName: "Git Reference",
				Description: "Git branch/tag reference",
				Value:       defaultSourceRepoRef,
				Required:    false,
			},
			{
				Name:        "CLUSTER_PASSWORD",
				DisplayName: "Cluster Password",
				Description: "Cluster Password",
				Generate:    "expression",
				From:        "[a-zA-Z0-9]{8}",
				Required:    false,
			},
		},
	}

	testCases := []struct {
		name                  string
		namespace             string
		preq                  *api.ProvisionRequest
		expectedSourceRepoUrl string
		expectedContextDir    string
		expectedSourceRepoDef string
		expectedClusterPasswd string
	}{
		{
			name:      "provision with all default values",
			namespace: "tc1",
			preq: &api.ProvisionRequest{
				Parameters: map[string]string{
					"SOURCE_REPOSITORY_URL": defaultSourceRepoUrl,
					"CONTEXT_DIR":           defaultContextDir,
					"SOURCE_REPOSITORY_REF": defaultSourceRepoRef,
					"CLUSTER_PASSWORD":      defaultClusterPasswd,
				},
			},
			expectedSourceRepoUrl: defaultSourceRepoUrl,
			expectedContextDir:    defaultContextDir,
			expectedSourceRepoDef: defaultSourceRepoRef,
			expectedClusterPasswd: defaultClusterPasswd,
		},
		{
			name:      "provision with different CONTEXT_DIR and SOURCE_REPOSITORY_REF",
			namespace: "tc2",
			preq: &api.ProvisionRequest{
				Parameters: map[string]string{
					"SOURCE_REPOSITORY_URL": defaultSourceRepoUrl,
					"CONTEXT_DIR":           "/master",
					"SOURCE_REPOSITORY_REF": "vbeta.0.1",
					"CLUSTER_PASSWORD":      defaultClusterPasswd,
				},
			},
			expectedSourceRepoUrl: defaultSourceRepoUrl,
			expectedContextDir:    "/master",
			expectedSourceRepoDef: "vbeta.0.1",
			expectedClusterPasswd: defaultClusterPasswd,
		},
		{
			name:      "provision with no CONTEXT_DIR and SOURCE_REPOSITORY_REF",
			namespace: "tc3",
			preq: &api.ProvisionRequest{
				Parameters: map[string]string{
					"SOURCE_REPOSITORY_URL": defaultSourceRepoUrl,
					"CONTEXT_DIR":           "",
					"SOURCE_REPOSITORY_REF": "",
					"CLUSTER_PASSWORD":      defaultClusterPasswd,
				},
			},
			expectedSourceRepoUrl: defaultSourceRepoUrl,
			expectedContextDir:    "",
			expectedSourceRepoDef: "",
			expectedClusterPasswd: defaultClusterPasswd,
		},
		{
			name:      "provision with no CONTEXT_DIR, SOURCE_REPOSITORY_REF, and CLUSTER_PASSWORD",
			namespace: "tc4",
			preq: &api.ProvisionRequest{
				Parameters: map[string]string{
					"SOURCE_REPOSITORY_URL": defaultSourceRepoUrl,
					"CONTEXT_DIR":           "",
					"SOURCE_REPOSITORY_REF": "",
					"CLUSTER_PASSWORD":      "",
				},
			},
			expectedSourceRepoUrl: defaultSourceRepoUrl,
			expectedContextDir:    "",
			expectedSourceRepoDef: "",
			expectedClusterPasswd: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			s, resp := b.ensureSecret(&user.DefaultInfo{}, tc.namespace, bti, defaultInstanceID, tc.preq, template, &didWork)
			if resp != nil {
				t.Errorf("got response %#v, expected nil", *resp)
			}
			if string(s.Data["SOURCE_REPOSITORY_URL"]) != tc.expectedSourceRepoUrl {
				t.Errorf("SOURCE_REPOSITORY_URL param does not match! got '%s', expected '%s'",
					string(s.Data["SOURCE_REPOSITORY_URL"]), tc.expectedSourceRepoUrl)
			}
			if string(s.Data["CONTEXT_DIR"]) != tc.expectedContextDir {
				t.Errorf("CONTEXT_DIR param does not match! got '%s', expected '%s'",
					string(s.Data["CONTEXT_DIR"]), tc.expectedContextDir)
			}
			if string(s.Data["SOURCE_REPOSITORY_REF"]) != tc.expectedSourceRepoDef {
				t.Errorf("SOURCE_REPOSITORY_REF param does not match! got '%s', expected '%s'",
					string(s.Data["SOURCE_REPOSITORY_REF"]), tc.expectedSourceRepoDef)
			}
			if string(s.Data["CLUSTER_PASSWORD"]) != tc.expectedClusterPasswd {
				t.Errorf("CLUSTER_PASSWORD param does not match! got '%s', expected '%s'",
					string(s.Data["CLUSTER_PASSWORD"]), tc.expectedClusterPasswd)
			}
		})
	}
}
