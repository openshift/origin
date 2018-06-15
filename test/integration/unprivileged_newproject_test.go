package integration

import (
	"io/ioutil"
	"testing"
	"time"

	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	authorizationclient "github.com/openshift/origin/pkg/authorization/generated/internalclientset"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	oc "github.com/openshift/origin/pkg/oc/cli/cmd"
	"github.com/openshift/origin/pkg/oc/util/tokencmd"
	projectapi "github.com/openshift/origin/pkg/project/apis/project"
	projectclient "github.com/openshift/origin/pkg/project/generated/internalclientset"
	templateclient "github.com/openshift/origin/pkg/template/generated/internalclientset"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"

	// make sure all generated clients compile
	// these are only here because it's the spot I chose to use a generated clientset for a test
	_ "github.com/openshift/client-go/apps/clientset/versioned"
	_ "github.com/openshift/client-go/authorization/clientset/versioned"
	_ "github.com/openshift/client-go/build/clientset/versioned"
	_ "github.com/openshift/client-go/image/clientset/versioned"
	_ "github.com/openshift/client-go/network/clientset/versioned"
	_ "github.com/openshift/client-go/project/clientset/versioned"
	_ "github.com/openshift/client-go/quota/clientset/versioned"
	_ "github.com/openshift/client-go/route/clientset/versioned"
	_ "github.com/openshift/client-go/template/clientset/versioned"
	_ "github.com/openshift/client-go/user/clientset/versioned"
	_ "github.com/openshift/origin/pkg/apps/generated/internalclientset"
	_ "github.com/openshift/origin/pkg/authorization/generated/internalclientset"
	_ "github.com/openshift/origin/pkg/build/generated/internalclientset"
	_ "github.com/openshift/origin/pkg/image/generated/internalclientset"
	_ "github.com/openshift/origin/pkg/network/generated/internalclientset"
	_ "github.com/openshift/origin/pkg/oauth/generated/clientset"
	_ "github.com/openshift/origin/pkg/oauth/generated/internalclientset"
	_ "github.com/openshift/origin/pkg/project/generated/internalclientset"
	_ "github.com/openshift/origin/pkg/quota/generated/internalclientset"
	_ "github.com/openshift/origin/pkg/route/generated/internalclientset"
	_ "github.com/openshift/origin/pkg/template/generated/internalclientset"
	_ "github.com/openshift/origin/pkg/user/generated/internalclientset"
	"k8s.io/client-go/rest"
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
	valerieProjectClient := projectclient.NewForConfigOrDie(&valerieClientConfig)

	// confirm that we have access to request the project

	allowed := &metav1.Status{}
	if err := valerieProjectClient.Project().RESTClient().Get().Resource("projectrequests").Do().Into(allowed); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if allowed.Status != metav1.StatusSuccess {
		t.Fatalf("expected %v, got %v", metav1.StatusSuccess, allowed.Status)
	}

	requestProject := oc.NewProjectOptions{
		ProjectName: "new-project",
		DisplayName: "display name here",
		Description: "the special description",

		Client: valerieProjectClient.Project(),
		Out:    ioutil.Discard,
	}

	if err := requestProject.Run(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	waitForProject(t, valerieProjectClient, "new-project", 5*time.Second, 10)

	actualProject, err := valerieProjectClient.Project().Projects().Get("new-project", metav1.GetOptions{})
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
	clusterAdminProjectClient := projectclient.NewForConfigOrDie(clusterAdminClientConfig)
	clusterAdminTemplateClient := templateclient.NewForConfigOrDie(clusterAdminClientConfig)

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
	valerieProjectClient := projectclient.NewForConfigOrDie(&valerieClientConfig)

	if _, err := clusterAdminProjectClient.Project().Projects().Create(&projectapi.Project{ObjectMeta: metav1.ObjectMeta{Name: namespace}}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	template, err := testutil.GetTemplateFixture("testdata/project-request-template-with-quota.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	template.Name = templateName
	template.Namespace = namespace

	_, err = clusterAdminTemplateClient.Template().Templates(namespace).Create(template)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	requestProject := oc.NewProjectOptions{
		ProjectName: "new-project",
		DisplayName: "display name here",
		Description: "the special description",

		Client: valerieProjectClient.Project(),
		Out:    ioutil.Discard,
	}

	if err := requestProject.Run(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	waitForProject(t, valerieProjectClient, "new-project", 5*time.Second, 10)
	project, err := valerieProjectClient.Project().Projects().Get("new-project", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if project.Annotations["extra"] != "here" {
		t.Errorf("unexpected project %#v", project)
	}

	if err := clusterAdminTemplateClient.Template().Templates(namespace).Delete(templateName, nil); err != nil {
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
	clusterAdminAuthorizationConfig := authorizationclient.NewForConfigOrDie(clusterAdminClientConfig).Authorization()
	role, err := clusterAdminAuthorizationConfig.ClusterRoles().Get(bootstrappolicy.SelfProvisionerRoleName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	role.Rules = []authorizationapi.PolicyRule{}
	if _, err := clusterAdminAuthorizationConfig.ClusterRoles().Update(role); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	valerieClientConfig := rest.AnonymousClientConfig(clusterAdminClientConfig)

	accessToken, err := tokencmd.RequestToken(valerieClientConfig, nil, "valerie", "security!")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	valerieClientConfig.BearerToken = accessToken

	valerieProjectClient := projectclient.NewForConfigOrDie(valerieClientConfig)
	valerieKubeClient := kclientset.NewForConfigOrDie(valerieClientConfig)

	if err := testutil.WaitForClusterPolicyUpdate(valerieKubeClient.Authorization(), "create", projectapi.Resource("projectrequests"), false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// confirm that we have access to request the project
	err = valerieProjectClient.Project().RESTClient().Get().Resource("projectrequests").Do().Into(&metav1.Status{})
	if err == nil {
		t.Fatalf("expected error: %v", err)
	}
	expectedError := `You may not request a new project via this API.`
	if (err != nil) && (err.Error() != expectedError) {
		t.Fatalf("expected\n\t%v\ngot\n\t%v", expectedError, err.Error())
	}
}
