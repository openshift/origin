package integration

import (
	"testing"

	"github.com/spf13/pflag"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	rbacv1client "k8s.io/client-go/kubernetes/typed/rbac/v1"
	"k8s.io/client-go/tools/clientcmd"

	authorizationv1typedclient "github.com/openshift/client-go/authorization/clientset/versioned/typed/authorization/v1"
	projectv1typedclient "github.com/openshift/client-go/project/clientset/versioned/typed/project/v1"
	userv1typedclient "github.com/openshift/client-go/user/clientset/versioned/typed/user/v1"
	newproject "github.com/openshift/oc/pkg/cli/admin/project"
	"github.com/openshift/oc/pkg/cli/login"
	"github.com/openshift/oc/pkg/cli/whoami"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

func TestLogin(t *testing.T) {
	masterConfig, clusterAdminKubeConfig, err := testserver.StartTestMasterAPI()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer testserver.CleanupMasterEtcd(t, masterConfig)

	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	username := "joe"
	password := "pass"
	project := "the-singularity-is-near"
	server := clusterAdminClientConfig.Host

	loginOptions := newLoginOptions(server, username, password, true)

	if err := loginOptions.GatherInfo(); err != nil {
		t.Fatalf("Error trying to determine server info: %v", err)
	}

	if loginOptions.Username != username {
		t.Fatalf("Unexpected user after authentication: %#v", loginOptions)
	}
	rbacClient := rbacv1client.NewForConfigOrDie(clusterAdminClientConfig)
	authorizationInterface := authorizationv1typedclient.NewForConfigOrDie(clusterAdminClientConfig)

	newProjectOptions := &newproject.NewProjectOptions{
		ProjectClient:   projectv1typedclient.NewForConfigOrDie(clusterAdminClientConfig),
		RbacClient:      rbacClient,
		SARClient:       authorizationInterface.SubjectAccessReviews(),
		ProjectName:     project,
		AdminRole:       "admin",
		AdminUser:       username,
		UseNodeSelector: false,
		IOStreams:       genericclioptions.NewTestIOStreamsDiscard(),
	}
	if err := newProjectOptions.Run(); err != nil {
		t.Fatalf("unexpected error, a project is required to continue: %v", err)
	}

	projectClient, err := projectv1typedclient.NewForConfig(loginOptions.Config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	p, err := projectClient.Projects().Get(project, metav1.GetOptions{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if p.Name != project {
		t.Fatalf("unexpected project: %#v", p)
	}

	// TODO Commented because of incorrectly hitting cache when listing projects.
	// Should be enabled again when cache eviction is properly fixed.

	// err = loginOptions.GatherProjectInfo()
	// if err != nil {
	// 	t.Fatalf("unexpected error: %v", err)
	// }

	// if loginOptions.Project != project {
	// 	t.Fatalf("Expected project %v but got %v", project, loginOptions.Project)
	// }

	// configFile, err := ioutil.TempFile("", "openshiftconfig")
	// if err != nil {
	// 	t.Fatalf("unexpected error: %v", err)
	// }
	// defer os.Remove(configFile.Name())

	// if _, err = loginOptions.SaveConfig(configFile.Name()); err != nil {
	// 	t.Fatalf("unexpected error: %v", err)
	// }
	userClient, err := userv1typedclient.NewForConfig(loginOptions.Config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	adminUserClient, err := userv1typedclient.NewForConfig(clusterAdminClientConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	userWhoamiOptions := whoami.WhoAmIOptions{UserInterface: userClient, IOStreams: genericclioptions.NewTestIOStreamsDiscard()}
	retrievedUser, err := userWhoamiOptions.WhoAmI()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if retrievedUser.Name != username {
		t.Errorf("expected %v, got %v", retrievedUser.Name, username)
	}

	adminWhoamiOptions := whoami.WhoAmIOptions{UserInterface: adminUserClient, IOStreams: genericclioptions.NewTestIOStreamsDiscard()}
	retrievedAdmin, err := adminWhoamiOptions.WhoAmI()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if retrievedAdmin.Name != "system:admin" {
		t.Errorf("expected %v, got %v", retrievedAdmin.Name, "system:admin")
	}

}

func newLoginOptions(server string, username string, password string, insecure bool) *login.LoginOptions {
	flagset := pflag.NewFlagSet("test-flags", pflag.ContinueOnError)
	flags := []string{}
	clientConfig := defaultClientConfig(flagset)
	flagset.Parse(flags)

	startingConfig, _ := clientConfig.RawConfig()

	loginOptions := &login.LoginOptions{
		Server:             server,
		StartingKubeConfig: &startingConfig,
		Username:           username,
		Password:           password,
		InsecureTLS:        insecure,

		IOStreams: genericclioptions.NewTestIOStreamsDiscard(),
	}

	return loginOptions
}

func defaultClientConfig(flags *pflag.FlagSet) clientcmd.ClientConfig {
	loadingRules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: ""}

	flags.StringVar(&loadingRules.ExplicitPath, genericclioptions.OpenShiftKubeConfigFlagName, "", "Path to the config file to use for CLI requests.")

	overrides := &clientcmd.ConfigOverrides{}
	overrideFlags := clientcmd.RecommendedConfigOverrideFlags("")
	overrideFlags.ContextOverrideFlags.Namespace.ShortName = "n"
	clientcmd.BindOverrideFlags(overrides, flags, overrideFlags)

	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides)

	return clientConfig
}
