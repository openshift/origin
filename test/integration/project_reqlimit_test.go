// +build integration

package integration

import (
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	apierrors "k8s.io/kubernetes/pkg/api/errors"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/labels"

	"github.com/openshift/origin/pkg/client"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/project/admission/requestlimit"
	projectapi "github.com/openshift/origin/pkg/project/api"
	userapi "github.com/openshift/origin/pkg/user/api"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

func setupProjectRequestLimitTest(t *testing.T, pluginConfig *requestlimit.ProjectRequestLimitConfig) (kclient.Interface, client.Interface, *kclient.Config) {
	masterConfig, err := testserver.DefaultMasterOptions()
	if err != nil {
		t.Fatalf("error creating config: %v", err)
	}
	masterConfig.AdmissionConfig.PluginOrderOverride = []string{"OriginNamespaceLifecycle", "BuildByStrategy", "ProjectRequestLimit"}
	masterConfig.AdmissionConfig.PluginConfig = map[string]configapi.AdmissionPluginConfig{
		"ProjectRequestLimit": {
			Configuration: pluginConfig,
		},
	}
	kubeConfigFile, err := testserver.StartConfiguredMaster(masterConfig)
	if err != nil {
		t.Fatalf("error starting server: %v", err)
	}
	kubeClient, err := testutil.GetClusterAdminKubeClient(kubeConfigFile)
	if err != nil {
		t.Fatalf("error getting client: %v", err)
	}
	openshiftClient, err := testutil.GetClusterAdminClient(kubeConfigFile)
	if err != nil {
		t.Fatalf("error getting openshift client: %v", err)
	}
	clientConfig, err := testutil.GetClusterAdminClientConfig(kubeConfigFile)
	if err != nil {
		t.Fatalf("error getting client config: %v", err)
	}
	return kubeClient, openshiftClient, clientConfig
}

func setupProjectRequestLimitUsers(t *testing.T, client client.Interface, users map[string]labels.Set) {
	for userName, labels := range users {
		user := &userapi.User{}
		user.Name = userName
		user.Labels = map[string]string(labels)
		_, err := client.Users().Create(user)
		if err != nil {
			t.Fatalf("Could not create user %s: %v", userName, err)
		}
	}
}

func setupProjectRequestLimitNamespaces(t *testing.T, kclient kclient.Interface, namespacesByRequester map[string]int) {
	for requester, nsCount := range namespacesByRequester {
		for i := 0; i < nsCount; i++ {
			ns := &kapi.Namespace{}
			ns.GenerateName = "testns"
			ns.Annotations = map[string]string{projectapi.ProjectRequester: requester}
			_, err := kclient.Namespaces().Create(ns)
			if err != nil {
				t.Fatalf("Could not create namespace for requester %s: %v", requester, err)
			}
		}
	}
}

func intPointer(n int) *int {
	return &n
}

func projectRequestLimitEmptyConfig() *requestlimit.ProjectRequestLimitConfig {
	return &requestlimit.ProjectRequestLimitConfig{}
}

func projectRequestLimitMultiLevelConfig() *requestlimit.ProjectRequestLimitConfig {
	return &requestlimit.ProjectRequestLimitConfig{
		Limits: []requestlimit.ProjectLimitBySelector{
			{
				Selector:    map[string]string{"level": "gold"},
				MaxProjects: intPointer(3),
			},
			{
				Selector:    map[string]string{"level": "silver"},
				MaxProjects: intPointer(2),
			},
			{
				Selector:    nil,
				MaxProjects: intPointer(1),
			},
		},
	}
}

func projectRequestLimitSingleDefaultConfig() *requestlimit.ProjectRequestLimitConfig {
	return &requestlimit.ProjectRequestLimitConfig{
		Limits: []requestlimit.ProjectLimitBySelector{
			{
				Selector:    nil,
				MaxProjects: intPointer(1),
			},
		},
	}
}

func projectRequestLimitUsers() map[string]labels.Set {
	return map[string]labels.Set{
		"regular": {"level": "none"},
		"gold":    {"level": "gold"},
		"silver":  {"level": "silver"},
	}
}

func TestProjectRequestLimitMultiLevelConfig(t *testing.T) {
	kclient, oclient, clientConfig := setupProjectRequestLimitTest(t, projectRequestLimitMultiLevelConfig())
	setupProjectRequestLimitUsers(t, oclient, projectRequestLimitUsers())
	setupProjectRequestLimitNamespaces(t, kclient, map[string]int{
		"regular": 0,
		"silver":  2,
		"gold":    2,
	})
	testProjectRequestLimitAdmission(t, "multi-level config", clientConfig, map[string]bool{
		"regular": true,
		"silver":  false,
		"gold":    true,
	})
}

func TestProjectRequestLimitEmptyConfig(t *testing.T) {
	kclient, oclient, clientConfig := setupProjectRequestLimitTest(t, projectRequestLimitEmptyConfig())
	setupProjectRequestLimitUsers(t, oclient, projectRequestLimitUsers())
	setupProjectRequestLimitNamespaces(t, kclient, map[string]int{
		"regular": 5,
		"silver":  2,
		"gold":    2,
	})
	testProjectRequestLimitAdmission(t, "empty config", clientConfig, map[string]bool{
		"regular": true,
		"silver":  true,
		"gold":    true,
	})
}

func TestProjectRequestLimitSingleConfig(t *testing.T) {
	kclient, oclient, clientConfig := setupProjectRequestLimitTest(t, projectRequestLimitSingleDefaultConfig())
	setupProjectRequestLimitUsers(t, oclient, projectRequestLimitUsers())
	setupProjectRequestLimitNamespaces(t, kclient, map[string]int{
		"regular": 0,
		"silver":  1,
		"gold":    0,
	})
	testProjectRequestLimitAdmission(t, "single default config", clientConfig, map[string]bool{
		"regular": true,
		"silver":  false,
		"gold":    true,
	})
}

func testProjectRequestLimitAdmission(t *testing.T, errorPrefix string, clientConfig *kclient.Config, tests map[string]bool) {
	for user, expectSuccess := range tests {
		oclient, _, _, err := testutil.GetClientForUser(*clientConfig, user)
		if err != nil {
			t.Fatalf("Error getting client for user %s: %v", user, err)
		}
		projectRequest := &projectapi.ProjectRequest{}
		projectRequest.Name = kapi.SimpleNameGenerator.GenerateName("test-projectreq")
		_, err = oclient.ProjectRequests().Create(projectRequest)
		if err != nil && expectSuccess {
			t.Errorf("%s: unexpected error for user %s: %v", errorPrefix, user, err)
			continue
		}
		if !expectSuccess {
			if err == nil {
				t.Errorf("%s: did not get expected error for user %s", errorPrefix, user)
				continue
			}
			if !apierrors.IsForbidden(err) {
				t.Errorf("%s: did not get an expected forbidden error for user %s. Got: %v", errorPrefix, user, err)
			}
		}
	}
}
