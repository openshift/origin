package jenkinsbootstrapper

import (
	"fmt"
	"strings"
	"testing"

	"k8s.io/kubernetes/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/auth/user"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/fake"
	"k8s.io/kubernetes/pkg/runtime"

	buildapi "github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/client/testclient"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
)

func TestAdmission(t *testing.T) {
	enableBuild := &buildapi.Build{Spec: buildapi.BuildSpec{CommonSpec: buildapi.CommonSpec{Strategy: buildapi.BuildStrategy{JenkinsPipelineStrategy: &buildapi.JenkinsPipelineBuildStrategy{}}}}}
	testCases := []struct {
		name           string
		objects        []runtime.Object
		jenkinsEnabled *bool

		attributes  admission.Attributes
		expectedErr string

		validateClients func(kubeClient *fake.Clientset, originClient *testclient.Fake) string
	}{
		{
			name:            "disabled",
			attributes:      admission.NewAttributesRecord(enableBuild, nil, unversioned.GroupVersionKind{}, "namespace", "name", buildapi.SchemeGroupVersion.WithResource("builds"), "", admission.Create, &user.DefaultInfo{}),
			jenkinsEnabled:  boolptr(false),
			validateClients: noAction,
		},
		{
			name:            "not a jenkins build",
			attributes:      admission.NewAttributesRecord(&buildapi.Build{Spec: buildapi.BuildSpec{CommonSpec: buildapi.CommonSpec{Strategy: buildapi.BuildStrategy{}}}}, nil, unversioned.GroupVersionKind{}, "namespace", "name", buildapi.SchemeGroupVersion.WithResource("builds"), "", admission.Create, &user.DefaultInfo{}),
			jenkinsEnabled:  boolptr(true),
			validateClients: noAction,
		},
		{
			name:            "not a build kind",
			attributes:      admission.NewAttributesRecord(&kapi.Service{}, nil, unversioned.GroupVersionKind{}, "namespace", "name", buildapi.SchemeGroupVersion.WithResource("builds"), "", admission.Create, &user.DefaultInfo{}),
			jenkinsEnabled:  boolptr(true),
			validateClients: noAction,
		},
		{
			name:            "not a build resource",
			attributes:      admission.NewAttributesRecord(enableBuild, nil, unversioned.GroupVersionKind{}, "namespace", "name", buildapi.SchemeGroupVersion.WithResource("notbuilds"), "", admission.Create, &user.DefaultInfo{}),
			jenkinsEnabled:  boolptr(true),
			validateClients: noAction,
		},
		{
			name:            "subresource",
			attributes:      admission.NewAttributesRecord(enableBuild, nil, unversioned.GroupVersionKind{}, "namespace", "name", buildapi.SchemeGroupVersion.WithResource("builds"), "subresource", admission.Create, &user.DefaultInfo{}),
			jenkinsEnabled:  boolptr(true),
			validateClients: noAction,
		},
		{
			name:           "service present",
			attributes:     admission.NewAttributesRecord(enableBuild, nil, unversioned.GroupVersionKind{}, "namespace", "name", buildapi.SchemeGroupVersion.WithResource("builds"), "", admission.Create, &user.DefaultInfo{}),
			jenkinsEnabled: boolptr(true),
			objects: []runtime.Object{
				&kapi.Service{ObjectMeta: kapi.ObjectMeta{Namespace: "namespace", Name: "jenkins"}},
			},
			validateClients: func(kubeClient *fake.Clientset, originClient *testclient.Fake) string {
				if len(kubeClient.Actions()) == 1 && kubeClient.Actions()[0].Matches("get", "services") {
					return ""
				}
				return fmt.Sprintf("missing get service in: %v", kubeClient.Actions())
			},
		},
		{
			name:       "works on true",
			attributes: admission.NewAttributesRecord(enableBuild, nil, unversioned.GroupVersionKind{}, "namespace", "name", buildapi.SchemeGroupVersion.WithResource("builds"), "", admission.Create, &user.DefaultInfo{}),
			objects: []runtime.Object{
				&kapi.Service{ObjectMeta: kapi.ObjectMeta{Namespace: "namespace", Name: "jenkins"}},
			},
			jenkinsEnabled: boolptr(true),
			validateClients: func(kubeClient *fake.Clientset, originClient *testclient.Fake) string {
				if len(kubeClient.Actions()) == 1 && kubeClient.Actions()[0].Matches("get", "services") {
					return ""
				}
				return fmt.Sprintf("missing get service in: %v", kubeClient.Actions())
			},
		},
		{
			name:       "enabled default",
			attributes: admission.NewAttributesRecord(enableBuild, nil, unversioned.GroupVersionKind{}, "namespace", "name", buildapi.SchemeGroupVersion.WithResource("builds"), "", admission.Create, &user.DefaultInfo{}),
			objects: []runtime.Object{
				&kapi.Service{ObjectMeta: kapi.ObjectMeta{Namespace: "namespace", Name: "jenkins"}},
			},
			validateClients: func(kubeClient *fake.Clientset, originClient *testclient.Fake) string {
				if len(kubeClient.Actions()) == 1 && kubeClient.Actions()[0].Matches("get", "services") {
					return ""
				}
				return fmt.Sprintf("missing get service in: %v", kubeClient.Actions())
			},
		},
		{
			name:           "service missing",
			attributes:     admission.NewAttributesRecord(enableBuild, nil, unversioned.GroupVersionKind{}, "namespace", "name", buildapi.SchemeGroupVersion.WithResource("builds"), "", admission.Create, &user.DefaultInfo{}),
			jenkinsEnabled: boolptr(true),
			objects:        []runtime.Object{},
			validateClients: func(kubeClient *fake.Clientset, originClient *testclient.Fake) string {
				if len(kubeClient.Actions()) == 0 {
					return fmt.Sprintf("missing get service in: %v", kubeClient.Actions())
				}
				if !kubeClient.Actions()[0].Matches("get", "services") {
					return fmt.Sprintf("missing get service in: %v", kubeClient.Actions())
				}
				if len(originClient.Actions()) == 0 {
					return fmt.Sprintf("missing get template in: %v", originClient.Actions())
				}
				if !originClient.Actions()[0].Matches("get", "templates") {
					return fmt.Sprintf("missing get template in: %v", originClient.Actions())
				}
				return ""
			},
			expectedErr: "Jenkins pipeline template / not found",
		},
	}

	for _, tc := range testCases {
		kubeClient := fake.NewSimpleClientset(tc.objects...)
		originClient := testclient.NewSimpleFake(tc.objects...)

		admission := NewJenkinsBootstrapper(kubeClient.Core()).(*jenkinsBootstrapper)
		admission.openshiftClient = originClient
		admission.jenkinsConfig = configapi.JenkinsPipelineConfig{
			AutoProvisionEnabled: tc.jenkinsEnabled,
			ServiceName:          "jenkins",
		}

		err := admission.Admit(tc.attributes)
		switch {
		case len(tc.expectedErr) == 0 && err == nil:
		case len(tc.expectedErr) == 0 && err != nil:
			t.Errorf("%s: unexpected error: %v", tc.name, err)
		case len(tc.expectedErr) != 0 && err == nil:
			t.Errorf("%s: missing error: %v", tc.name, tc.expectedErr)
		case len(tc.expectedErr) != 0 && err != nil && !strings.Contains(err.Error(), tc.expectedErr):
			t.Errorf("%s: missing error: expected %v, got %v", tc.name, tc.expectedErr, err)
		}

		if tc.validateClients != nil {
			if err := tc.validateClients(kubeClient, originClient); len(err) != 0 {
				t.Errorf("%s: unexpected error: %v", tc.name, err)
			}
		}
	}
}
func noAction(kubeClient *fake.Clientset, originClient *testclient.Fake) string {
	if len(kubeClient.Actions()) != 0 {
		return fmt.Sprintf("unexpected actions: %v", kubeClient.Actions())
	}
	if len(originClient.Actions()) != 0 {
		return fmt.Sprintf("unexpected actions: %v", originClient.Actions())
	}
	return ""
}

func boolptr(in bool) *bool {
	return &in
}
