package integration

import (
	"fmt"
	"testing"
	"time"

	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	authorizationv1 "github.com/openshift/api/authorization/v1"
	"github.com/openshift/api/project"
	projectv1 "github.com/openshift/api/project/v1"
	authorizationv1client "github.com/openshift/client-go/authorization/clientset/versioned"
	projectv1client "github.com/openshift/client-go/project/clientset/versioned/typed/project/v1"
	templatev1client "github.com/openshift/client-go/template/clientset/versioned/typed/template/v1"
	"github.com/openshift/oc/pkg/cli/requestproject"
	"github.com/openshift/oc/pkg/helpers/tokencmd"
	projectapi "github.com/openshift/openshift-apiserver/pkg/project/apis/project"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

func TestUnprivilegedNewProject(t *testing.T) {
	masterConfig, clusterAdminKubeConfig, err := testserver.StartTestMasterAPI()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer testserver.CleanupMasterEtcd(t, masterConfig)

	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	valerieClientConfig := *clusterAdminClientConfig
	valerieClientConfig.Username = ""
	valerieClientConfig.Password = ""
	valerieClientConfig.BearerToken = ""
	valerieClientConfig.CertFile = ""
	valerieClientConfig.KeyFile = ""
	valerieClientConfig.CertData = nil
	valerieClientConfig.KeyData = nil

	accessToken, err := tokencmd.RequestToken(&valerieClientConfig, nil, "valerie", "security!")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	valerieClientConfig.BearerToken = accessToken
	valerieProjectClient := projectv1client.NewForConfigOrDie(&valerieClientConfig)

	// confirm that we have access to request the project

	allowed := &metav1.Status{}
	if err := valerieProjectClient.RESTClient().Get().Resource("projectrequests").Do().Into(allowed); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if allowed.Status != metav1.StatusSuccess {
		t.Fatalf("expected %v, got %v", metav1.StatusSuccess, allowed.Status)
	}

	requestProject := requestproject.RequestProjectOptions{
		ProjectName: "new-project",
		DisplayName: "display name here",
		Description: "the special description",

		Client:    valerieProjectClient,
		IOStreams: genericclioptions.NewTestIOStreamsDiscard(),
	}

	if err := requestProject.Run(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	waitForProject(t, valerieProjectClient, "new-project", 5*time.Second, 10)

	actualProject, err := valerieProjectClient.Projects().Get("new-project", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e, a := "valerie", actualProject.Annotations[projectapi.ProjectRequester]; e != a {
		t.Errorf("incorrect project requester: expected %v, got %v", e, a)
	}

	if err := requestProject.Run(); !kapierrors.IsAlreadyExists(err) {
		t.Fatalf("expected an already exists error, but got %v", err)
	}

}
func TestUnprivilegedNewProjectFromTemplate(t *testing.T) {
	namespace := "foo"
	templateName := "bar"

	masterOptions, err := testserver.DefaultMasterOptions()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer testserver.CleanupMasterEtcd(t, masterOptions)
	masterOptions.ProjectConfig.ProjectRequestTemplate = namespace + "/" + templateName

	clusterAdminKubeConfig, err := testserver.StartConfiguredMaster(masterOptions)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	clusterAdminProjectClient := projectv1client.NewForConfigOrDie(clusterAdminClientConfig)
	clusterAdminTemplateClient := templatev1client.NewForConfigOrDie(clusterAdminClientConfig)

	valerieClientConfig := *clusterAdminClientConfig
	valerieClientConfig.Username = ""
	valerieClientConfig.Password = ""
	valerieClientConfig.BearerToken = ""
	valerieClientConfig.CertFile = ""
	valerieClientConfig.KeyFile = ""
	valerieClientConfig.CertData = nil
	valerieClientConfig.KeyData = nil

	accessToken, err := tokencmd.RequestToken(&valerieClientConfig, nil, "valerie", "security!")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	valerieClientConfig.BearerToken = accessToken
	valerieProjectClient := projectv1client.NewForConfigOrDie(&valerieClientConfig)

	if _, err := clusterAdminProjectClient.Projects().Create(&projectv1.Project{ObjectMeta: metav1.ObjectMeta{Name: namespace}}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	template, err := testutil.GetTemplateFixture("testdata/project-request-template-with-quota.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	template.Name = templateName
	template.Namespace = namespace

	_, err = clusterAdminTemplateClient.Templates(namespace).Create(template)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	requestProject := requestproject.RequestProjectOptions{
		ProjectName: "new-project",
		DisplayName: "display name here",
		Description: "the special description",

		Client:    valerieProjectClient,
		IOStreams: genericclioptions.NewTestIOStreamsDiscard(),
	}

	if err := requestProject.Run(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	waitForProject(t, valerieProjectClient, "new-project", 5*time.Second, 10)
	project, err := valerieProjectClient.Projects().Get("new-project", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if project.Annotations["extra"] != "here" {
		t.Errorf("unexpected project %#v", project)
	}

	if err := clusterAdminTemplateClient.Templates(namespace).Delete(templateName, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	requestProject.ProjectName = "different"
	// This should fail during the template retrieve
	if err := requestProject.Run(); !kapierrors.IsNotFound(err) {
		t.Fatalf("expected a not found error, but got %v", err)
	}

}

func TestUnprivilegedNewProjectDenied(t *testing.T) {
	masterConfig, clusterAdminKubeConfig, err := testserver.StartTestMasterAPI()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer testserver.CleanupMasterEtcd(t, masterConfig)

	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	clusterAdminAuthorizationConfig := authorizationv1client.NewForConfigOrDie(clusterAdminClientConfig).AuthorizationV1()
	role, err := clusterAdminAuthorizationConfig.ClusterRoles().Get("self-provisioner", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	role.Rules = []authorizationv1.PolicyRule{}
	if _, err := clusterAdminAuthorizationConfig.ClusterRoles().Update(role); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	valerieClientConfig := rest.AnonymousClientConfig(clusterAdminClientConfig)

	accessToken, err := tokencmd.RequestToken(valerieClientConfig, nil, "valerie", "security!")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	valerieClientConfig.BearerToken = accessToken

	valerieProjectClient := projectv1client.NewForConfigOrDie(valerieClientConfig)
	valerieKubeClient := kubernetes.NewForConfigOrDie(valerieClientConfig)

	if err := testutil.WaitForClusterPolicyUpdate(valerieKubeClient.AuthorizationV1(), "create", project.Resource("projectrequests"), false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// confirm that we have access to request the project
	err = valerieProjectClient.RESTClient().Get().Resource("projectrequests").Do().Into(&metav1.Status{})
	if err == nil {
		t.Fatalf("expected error: %v", err)
	}
	expectedError := `You may not request a new project via this API.`
	if (err != nil) && (err.Error() != expectedError) {
		t.Fatalf("expected\n\t%v\ngot\n\t%v", expectedError, err.Error())
	}
}

// waitForProject will execute a client list of projects looking for the project with specified name
// if not found, it will retry up to numRetries at the specified delayInterval
func waitForProject(t *testing.T, client projectv1client.ProjectV1Interface, projectName string, delayInterval time.Duration, numRetries int) {
	for i := 0; i <= numRetries; i++ {
		projects, err := client.Projects().List(metav1.ListOptions{})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if (len(projects.Items) == 1) && (projects.Items[0].Name == projectName) {
			fmt.Printf("Waited %v times with interval %v\n", i, delayInterval)
			return
		} else {
			time.Sleep(delayInterval)
		}
	}
	t.Errorf("expected project %v not found", projectName)
}
