package integration

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apiserver/pkg/storage/names"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	projectv1 "github.com/openshift/api/project/v1"
	userv1 "github.com/openshift/api/user/v1"
	projectv1client "github.com/openshift/client-go/project/clientset/versioned/typed/project/v1"
	userv1client "github.com/openshift/client-go/user/clientset/versioned/typed/user/v1"
	projectapi "github.com/openshift/openshift-apiserver/pkg/project/apis/project"
	requestlimit "github.com/openshift/openshift-apiserver/pkg/project/apiserver/admission/apis/requestlimit"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
	configapi "github.com/openshift/origin/test/util/server/deprecated_openshift/apis/config"
)

func setupProjectRequestLimitTest(t *testing.T, pluginConfig *requestlimit.ProjectRequestLimitConfig) (kubernetes.Interface, *rest.Config, func()) {
	masterConfig, err := testserver.DefaultMasterOptions()
	if err != nil {
		t.Fatalf("error creating config: %v", err)
	}
	masterConfig.AdmissionConfig.PluginConfig = map[string]*configapi.AdmissionPluginConfig{
		"project.openshift.io/ProjectRequestLimit": {
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
	clientConfig, err := testutil.GetClusterAdminClientConfig(kubeConfigFile)
	if err != nil {
		t.Fatalf("error getting client config: %v", err)
	}
	return kubeClient, clientConfig, func() {
		testserver.CleanupMasterEtcd(t, masterConfig)
	}
}

func setupProjectRequestLimitUsers(t *testing.T, client userv1client.UserV1Interface, users map[string]labels.Set) {
	for userName, labels := range users {
		user := &userv1.User{}
		user.Name = userName
		user.Labels = map[string]string(labels)
		_, err := client.Users().Create(user)
		if err != nil {
			t.Fatalf("Could not create user %s: %v", userName, err)
		}
	}
}

func setupProjectRequestLimitNamespaces(t *testing.T, kclient kubernetes.Interface, namespacesByRequester map[string]int) {
	for requester, nsCount := range namespacesByRequester {
		for i := 0; i < nsCount; i++ {
			ns := &corev1.Namespace{}
			ns.GenerateName = "testns"
			ns.Annotations = map[string]string{projectapi.ProjectRequester: requester}
			_, err := kclient.CoreV1().Namespaces().Create(ns)
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

		MaxProjectsForSystemUsers: intPointer(1),
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
	kclient, clientConfig, fn := setupProjectRequestLimitTest(t, projectRequestLimitMultiLevelConfig())
	defer fn()
	setupProjectRequestLimitUsers(t, userv1client.NewForConfigOrDie(clientConfig), projectRequestLimitUsers())
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
	kclient, clientConfig, fn := setupProjectRequestLimitTest(t, projectRequestLimitEmptyConfig())
	defer fn()
	setupProjectRequestLimitUsers(t, userv1client.NewForConfigOrDie(clientConfig), projectRequestLimitUsers())
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
	kclient, clientConfig, fn := setupProjectRequestLimitTest(t, projectRequestLimitSingleDefaultConfig())
	defer fn()
	setupProjectRequestLimitUsers(t, userv1client.NewForConfigOrDie(clientConfig), projectRequestLimitUsers())
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

// we had a bug where this failed on ` uenxpected error: metadata.name: Invalid value: "system:admin": may not contain ":"`
// make sure we never have that bug again and that project limits for them work
func TestProjectRequestLimitAsSystemAdmin(t *testing.T) {
	_, clientConfig, fn := setupProjectRequestLimitTest(t, projectRequestLimitSingleDefaultConfig())
	defer fn()
	projectClient := projectv1client.NewForConfigOrDie(clientConfig)

	if _, err := projectClient.ProjectRequests().Create(&projectv1.ProjectRequest{
		ObjectMeta: metav1.ObjectMeta{Name: "foo"},
	}); err != nil {
		t.Errorf("uenxpected error: %v", err)
	}
	if _, err := projectClient.ProjectRequests().Create(&projectv1.ProjectRequest{
		ObjectMeta: metav1.ObjectMeta{Name: "bar"},
	}); !apierrors.IsForbidden(err) {
		t.Errorf("missing error: %v", err)
	}
}

func testProjectRequestLimitAdmission(t *testing.T, errorPrefix string, clientConfig *rest.Config, tests map[string]bool) {
	for user, expectSuccess := range tests {
		_, clientConfig, err := testutil.GetClientForUser(clientConfig, user)
		if err != nil {
			t.Fatalf("Error getting client for user %s: %v", user, err)
		}
		projectRequest := &projectv1.ProjectRequest{}
		projectRequest.Name = names.SimpleNameGenerator.GenerateName("test-projectreq")
		_, err = projectv1client.NewForConfigOrDie(clientConfig).ProjectRequests().Create(projectRequest)
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
