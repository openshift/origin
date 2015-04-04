// +build integration,!no-etcd

package integration

import (
	"os"
	"testing"

	"github.com/spf13/pflag"

	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd"
	clientcmdapi "github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd/api"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/cli/cmd"
	"github.com/openshift/origin/pkg/cmd/cli/config"
	newproject "github.com/openshift/origin/pkg/cmd/experimental/project"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	"github.com/openshift/origin/pkg/user/api"
	testutil "github.com/openshift/origin/test/util"
)

func init() {
	testutil.RequireEtcd()
}

func TestLogin(t *testing.T) {
	clientcmd.DefaultCluster = clientcmdapi.Cluster{Server: ""}

	_, clusterAdminKubeConfig, err := testutil.StartTestMaster()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	clusterAdminClient, err := testutil.GetClusterAdminClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// empty config, should display message
	loginOptions := newLoginOptions("", "", "", "", false)
	if err := loginOptions.GatherInfo(); err == nil {
		t.Errorf("Raw login should error out")
	}

	username := "joe"
	password := "pass"
	project := "the-singularity-is-near"
	server := clusterAdminClientConfig.Host

	loginOptions = newLoginOptions(server, username, password, "", true)

	if err := loginOptions.GatherInfo(); err != nil {
		t.Fatalf("Error trying to determine server info: %v", err)
	}

	if loginOptions.Username != username {
		t.Fatalf("Unexpected user after authentication: %#v", loginOptions)
	}

	newProjectOptions := &newproject.NewProjectOptions{
		Client:                clusterAdminClient,
		ProjectName:           project,
		AdminRole:             bootstrappolicy.AdminRoleName,
		MasterPolicyNamespace: bootstrappolicy.DefaultMasterAuthorizationNamespace,
		AdminUser:             username,
	}
	if err := newProjectOptions.Run(); err != nil {
		t.Fatalf("unexpected error, a project is required to continue: %v", err)
	}

	oClient, _ := client.New(loginOptions.Config)
	p, err := oClient.Projects().Get(project)
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
}

func newLoginOptions(server string, username string, password string, context string, insecure bool) *cmd.LoginOptions {
	flagset := pflag.NewFlagSet("test-flags", pflag.ContinueOnError)

	flags := []string{}

	if len(server) > 0 {
		flags = append(flags, "--server="+server)
	}
	if len(context) > 0 {
		flags = append(flags, "--context="+context)
	}
	if insecure {
		flags = append(flags, "--insecure-skip-tls-verify")
	}

	loginOptions := &cmd.LoginOptions{
		ClientConfig: defaultClientConfig(flagset),
		Reader:       os.Stdin,
		Username:     username,
		Password:     password,
	}

	flagset.Parse(flags)

	return loginOptions
}

func whoami(clientCfg *kclient.Config) (*api.User, error) {
	oClient, err := client.New(clientCfg)
	if err != nil {
		return nil, err
	}

	me, err := oClient.Users().Get("~")
	if err != nil {
		return nil, err
	}

	return me, nil
}

func defaultClientConfig(flags *pflag.FlagSet) clientcmd.ClientConfig {
	loadingRules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: ""}

	flags.StringVar(&loadingRules.ExplicitPath, config.OpenShiftConfigFlagName, "", "Path to the config file to use for CLI requests.")

	overrides := &clientcmd.ConfigOverrides{}
	overrideFlags := clientcmd.RecommendedConfigOverrideFlags("")
	overrideFlags.ContextOverrideFlags.NamespaceShort = "n"
	clientcmd.BindOverrideFlags(overrides, flags, overrideFlags)

	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides)

	return clientConfig
}
