package jenkinsbootstrapper

import (
	"fmt"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/authentication/user"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/fake"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	templatefake "github.com/openshift/origin/pkg/template/generated/internalclientset/fake"
)

func TestAdmission(t *testing.T) {
	enableBuild := &buildapi.Build{Spec: buildapi.BuildSpec{CommonSpec: buildapi.CommonSpec{Strategy: buildapi.BuildStrategy{JenkinsPipelineStrategy: &buildapi.JenkinsPipelineBuildStrategy{}}}}}
	testCases := []struct {
		name           string
		objects        []runtime.Object
		jenkinsEnabled *bool

		attributes  admission.Attributes
		expectedErr string

		validateClients func(kubeClient *fake.Clientset, templateClient *templatefake.Clientset) string
	}{
		{
			name:            "disabled",
			attributes:      admission.NewAttributesRecord(enableBuild, nil, schema.GroupVersionKind{}, "namespace", "name", buildapi.LegacySchemeGroupVersion.WithResource("builds"), "", admission.Create, &user.DefaultInfo{}),
			jenkinsEnabled:  boolptr(false),
			validateClients: noAction,
		},
		{
			name:            "not a jenkins build",
			attributes:      admission.NewAttributesRecord(&buildapi.Build{Spec: buildapi.BuildSpec{CommonSpec: buildapi.CommonSpec{Strategy: buildapi.BuildStrategy{}}}}, nil, schema.GroupVersionKind{}, "namespace", "name", buildapi.LegacySchemeGroupVersion.WithResource("builds"), "", admission.Create, &user.DefaultInfo{}),
			jenkinsEnabled:  boolptr(true),
			validateClients: noAction,
		},
		{
			name:            "not a build kind",
			attributes:      admission.NewAttributesRecord(&kapi.Service{}, nil, schema.GroupVersionKind{}, "namespace", "name", buildapi.LegacySchemeGroupVersion.WithResource("builds"), "", admission.Create, &user.DefaultInfo{}),
			jenkinsEnabled:  boolptr(true),
			validateClients: noAction,
		},
		{
			name:            "not a build resource",
			attributes:      admission.NewAttributesRecord(enableBuild, nil, schema.GroupVersionKind{}, "namespace", "name", buildapi.LegacySchemeGroupVersion.WithResource("notbuilds"), "", admission.Create, &user.DefaultInfo{}),
			jenkinsEnabled:  boolptr(true),
			validateClients: noAction,
		},
		{
			name:            "subresource",
			attributes:      admission.NewAttributesRecord(enableBuild, nil, schema.GroupVersionKind{}, "namespace", "name", buildapi.LegacySchemeGroupVersion.WithResource("builds"), "subresource", admission.Create, &user.DefaultInfo{}),
			jenkinsEnabled:  boolptr(true),
			validateClients: noAction,
		},
		{
			name:           "service present",
			attributes:     admission.NewAttributesRecord(enableBuild, nil, schema.GroupVersionKind{}, "namespace", "name", buildapi.LegacySchemeGroupVersion.WithResource("builds"), "", admission.Create, &user.DefaultInfo{}),
			jenkinsEnabled: boolptr(true),
			objects: []runtime.Object{
				&kapi.Service{ObjectMeta: metav1.ObjectMeta{Namespace: "namespace", Name: "jenkins"}},
			},
			validateClients: func(kubeClient *fake.Clientset, templateClient *templatefake.Clientset) string {
				if len(kubeClient.Actions()) == 1 && kubeClient.Actions()[0].Matches("get", "services") {
					return ""
				}
				return fmt.Sprintf("missing get service in: %#v", kubeClient.Actions())
			},
		},
		{
			name:       "works on true",
			attributes: admission.NewAttributesRecord(enableBuild, nil, schema.GroupVersionKind{}, "namespace", "name", buildapi.LegacySchemeGroupVersion.WithResource("builds"), "", admission.Create, &user.DefaultInfo{}),
			objects: []runtime.Object{
				&kapi.Service{ObjectMeta: metav1.ObjectMeta{Namespace: "namespace", Name: "jenkins"}},
			},
			jenkinsEnabled: boolptr(true),
			validateClients: func(kubeClient *fake.Clientset, templateClient *templatefake.Clientset) string {
				if len(kubeClient.Actions()) == 1 && kubeClient.Actions()[0].Matches("get", "services") {
					return ""
				}
				return fmt.Sprintf("missing get service in: %v", kubeClient.Actions())
			},
		},
		{
			name:       "enabled default",
			attributes: admission.NewAttributesRecord(enableBuild, nil, schema.GroupVersionKind{}, "namespace", "name", buildapi.LegacySchemeGroupVersion.WithResource("builds"), "", admission.Create, &user.DefaultInfo{}),
			objects: []runtime.Object{
				&kapi.Service{ObjectMeta: metav1.ObjectMeta{Namespace: "namespace", Name: "jenkins"}},
			},
			validateClients: func(kubeClient *fake.Clientset, templateClient *templatefake.Clientset) string {
				if len(kubeClient.Actions()) == 1 && kubeClient.Actions()[0].Matches("get", "services") {
					return ""
				}
				return fmt.Sprintf("missing get service in: %v", kubeClient.Actions())
			},
		},
		{
			name:           "service missing",
			attributes:     admission.NewAttributesRecord(enableBuild, nil, schema.GroupVersionKind{}, "namespace", "name", buildapi.LegacySchemeGroupVersion.WithResource("builds"), "", admission.Create, &user.DefaultInfo{}),
			jenkinsEnabled: boolptr(true),
			objects:        []runtime.Object{},
			validateClients: func(kubeClient *fake.Clientset, templateClient *templatefake.Clientset) string {
				if len(kubeClient.Actions()) == 0 {
					return fmt.Sprintf("missing get service in: %v", kubeClient.Actions())
				}
				if !kubeClient.Actions()[0].Matches("get", "services") {
					return fmt.Sprintf("missing get service in: %v", kubeClient.Actions())
				}
				if len(templateClient.Actions()) == 0 {
					return fmt.Sprintf("missing get template in: %v", templateClient.Actions())
				}
				if !templateClient.Actions()[0].Matches("get", "templates") {
					return fmt.Sprintf("missing get template in: %v", templateClient.Actions())
				}
				return ""
			},
			expectedErr: "Jenkins pipeline template namespace/ not found",
		},
	}

	for _, tc := range testCases {
		kubeClient := fake.NewSimpleClientset(tc.objects...)
		templateClient := templatefake.NewSimpleClientset()

		admission := NewJenkinsBootstrapper().(*jenkinsBootstrapper)
		admission.templateClient = templateClient
		admission.jenkinsConfig = configapi.JenkinsPipelineConfig{
			AutoProvisionEnabled: tc.jenkinsEnabled,
			ServiceName:          "jenkins",
			TemplateNamespace:    "namespace",
		}
		admission.SetInternalKubeClientSet(kubeClient)

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
			if err := tc.validateClients(kubeClient, templateClient); len(err) != 0 {
				t.Errorf("%s: unexpected error: %v", tc.name, err)
			}
		}
	}
}
func noAction(kubeClient *fake.Clientset, templateClient *templatefake.Clientset) string {
	if len(kubeClient.Actions()) != 0 {
		return fmt.Sprintf("unexpected kubernetes client actions: %v", kubeClient.Actions())
	}
	if len(templateClient.Actions()) != 0 {
		return fmt.Sprintf("unexpected openshift template client actions: %v", templateClient.Actions())
	}
	return ""
}

func boolptr(in bool) *bool {
	return &in
}
