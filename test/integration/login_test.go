package integration

import (
	"testing"

	"github.com/spf13/pflag"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	rbacv1client "k8s.io/client-go/kubernetes/typed/rbac/v1"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"

	authorizationclient "github.com/openshift/origin/pkg/authorization/generated/internalclientset"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	newproject "github.com/openshift/origin/pkg/oc/cli/admin/project"
	"github.com/openshift/origin/pkg/oc/cli/login"
	"github.com/openshift/origin/pkg/oc/cli/whoami"
	projectclient "github.com/openshift/origin/pkg/project/generated/internalclientset"
	userclient "github.com/openshift/origin/pkg/user/generated/internalclientset/typed/user/internalversion"
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
	authorizationInterface := authorizationclient.NewForConfigOrDie(clusterAdminClientConfig).Authorization()

	newProjectOptions := &newproject.NewProjectOptions{
		ProjectClient:   projectclient.NewForConfigOrDie(clusterAdminClientConfig).Project(),
		RbacClient:      rbacClient,
		SARClient:       authorizationInterface.SubjectAccessReviews(),
		ProjectName:     project,
		AdminRole:       bootstrappolicy.AdminRoleName,
		AdminUser:       username,
		UseNodeSelector: false,
		IOStreams:       genericclioptions.NewTestIOStreamsDiscard(),
	}
	if err := newProjectOptions.Run(); err != nil {
		t.Fatalf("unexpected error, a project is required to continue: %v", err)
	}

	projectClient, err := projectclient.NewForConfig(loginOptions.Config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	p, err := projectClient.Project().Projects().Get(project, metav1.GetOptions{})
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
	userClient, err := userclient.NewForConfig(loginOptions.Config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	adminUserClient, err := userclient.NewForConfig(clusterAdminClientConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	userWhoamiOptions := whoami.WhoAmIOptions{UserInterface: userClient.Users(), IOStreams: genericclioptions.NewTestIOStreamsDiscard()}
	retrievedUser, err := userWhoamiOptions.WhoAmI()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if retrievedUser.Name != username {
		t.Errorf("expected %v, got %v", retrievedUser.Name, username)
	}

	adminWhoamiOptions := whoami.WhoAmIOptions{UserInterface: adminUserClient.Users(), IOStreams: genericclioptions.NewTestIOStreamsDiscard()}
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
