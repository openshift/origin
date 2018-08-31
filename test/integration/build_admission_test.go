package integration

import (
	"strings"
	"testing"

	kapierror "k8s.io/apimachinery/pkg/api/errors"
	rbacv1client "k8s.io/client-go/kubernetes/typed/rbac/v1"
	"k8s.io/client-go/rest"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	authorizationtypedclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/authorization/internalversion"

	"github.com/openshift/api/build"
	buildv1 "github.com/openshift/api/build/v1"
	buildv1client "github.com/openshift/client-go/build/clientset/versioned"
	buildv1clienttyped "github.com/openshift/client-go/build/clientset/versioned/typed/build/v1"
	buildutil "github.com/openshift/origin/pkg/build/util"
	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	policy "github.com/openshift/origin/pkg/oc/cli/admin/policy"
	templateclient "github.com/openshift/origin/pkg/template/generated/internalclientset"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

// all build strategy types
func buildStrategyTypes() []string {
	return []string{"source", "docker", "custom", "jenkinspipeline"}
}

// build strategy types that are not granted by default to system:authenticated
func buildStrategyTypesRestricted() []string {
	return []string{"custom"}
}

func TestPolicyBasedRestrictionOfBuildCreateAndCloneByStrategy(t *testing.T) {
	clusterAdminClientConfig, projectAdminKubeClient, projectAdminClient, projectEditorClient, fn := setupBuildStrategyTest(t, false)
	defer fn()

	clients := map[string]buildv1client.Interface{"admin": projectAdminClient, "editor": projectEditorClient}
	builds := map[string]*buildv1.Build{}

	restrictedStrategies := make(map[string]int)
	for key, val := range buildStrategyTypesRestricted() {
		restrictedStrategies[val] = key
	}

	// ensure that restricted strategy types can not be created
	for _, strategy := range buildStrategyTypes() {
		for clientType, client := range clients {
			var err error
			builds[string(strategy)+clientType], err = createBuild(t, client.Build().Builds(testutil.Namespace()), strategy)
			_, restricted := restrictedStrategies[strategy]
			if kapierror.IsForbidden(err) && !restricted {
				t.Errorf("unexpected error for strategy %s and client %s: %v", strategy, clientType, err)
			} else if !kapierror.IsForbidden(err) && restricted {
				t.Errorf("expected forbidden for strategy %s and client %s: Got success instead ", strategy, clientType)
			}
		}
	}

	grantRestrictedBuildStrategyRoleResources(t, rbacv1client.NewForConfigOrDie(clusterAdminClientConfig), projectAdminKubeClient.Authorization())

	// Create builds to setup test
	for _, strategy := range buildStrategyTypes() {
		for clientType, client := range clients {
			var err error
			if builds[string(strategy)+clientType], err = createBuild(t, client.Build().Builds(testutil.Namespace()), strategy); err != nil {
				t.Errorf("unexpected error for strategy %s and client %s: %v", strategy, clientType, err)
			}
		}
	}

	// by default admins and editors can clone builds
	for _, strategy := range buildStrategyTypes() {
		for clientType, client := range clients {
			if _, err := cloneBuild(t, client.Build().Builds(testutil.Namespace()), builds[string(strategy)+clientType]); err != nil {
				t.Errorf("unexpected clone error for strategy %s and client %s: %v", strategy, clientType, err)
			}
		}
	}
	removeBuildStrategyRoleResources(t, rbacv1client.NewForConfigOrDie(clusterAdminClientConfig), projectAdminKubeClient.Authorization())

	// make sure builds are rejected
	for _, strategy := range buildStrategyTypes() {
		for clientType, client := range clients {
			if _, err := createBuild(t, client.Build().Builds(testutil.Namespace()), strategy); !kapierror.IsForbidden(err) {
				t.Errorf("expected forbidden for strategy %s and client %s: got %v", strategy, clientType, err)
			}
		}
	}

	// make sure build updates are rejected
	for _, strategy := range buildStrategyTypes() {
		for clientType, client := range clients {
			if _, err := updateBuild(t, client.Build().Builds(testutil.Namespace()), builds[string(strategy)+clientType]); !kapierror.IsForbidden(err) {
				t.Errorf("expected forbidden for strategy %s and client %s: got %v", strategy, clientType, err)
			}
		}
	}

	// make sure clone is rejected
	for _, strategy := range buildStrategyTypes() {
		for clientType, client := range clients {
			if _, err := cloneBuild(t, client.Build().Builds(testutil.Namespace()), builds[string(strategy)+clientType]); !kapierror.IsForbidden(err) {
				t.Errorf("expected forbidden for strategy %s and client %s: got %v", strategy, clientType, err)
			}
		}
	}
}

func TestPolicyBasedRestrictionOfBuildConfigCreateAndInstantiateByStrategy(t *testing.T) {
	clusterAdminClientConfig, projectAdminKubeClient, projectAdminClient, projectEditorClient, fn := setupBuildStrategyTest(t, true)
	defer fn()

	clients := map[string]buildv1client.Interface{"admin": projectAdminClient, "editor": projectEditorClient}
	buildConfigs := map[string]*buildv1.BuildConfig{}
	restrictedStrategies := make(map[string]int)
	for key, val := range buildStrategyTypesRestricted() {
		restrictedStrategies[val] = key
	}

	// ensure that restricted strategy types can not be created
	for _, strategy := range buildStrategyTypes() {
		for clientType, client := range clients {
			var err error
			buildConfigs[string(strategy)+clientType], err = createBuildConfig(t, client.Build().BuildConfigs(testutil.Namespace()), strategy)
			_, restricted := restrictedStrategies[strategy]
			if kapierror.IsForbidden(err) && !restricted {
				t.Errorf("unexpected error for strategy %s and client %s: %v", strategy, clientType, err)
			} else if !kapierror.IsForbidden(err) && restricted {
				t.Errorf("expected forbidden for strategy %s and client %s: Got success instead ", strategy, clientType)
			}
		}
	}

	grantRestrictedBuildStrategyRoleResources(t, rbacv1client.NewForConfigOrDie(clusterAdminClientConfig), projectAdminKubeClient.Authorization())

	// by default admins and editors can create source, docker, and jenkinspipline buildconfigs
	for _, strategy := range buildStrategyTypes() {
		for clientType, client := range clients {
			var err error
			if buildConfigs[string(strategy)+clientType], err = createBuildConfig(t, client.Build().BuildConfigs(testutil.Namespace()), strategy); err != nil {
				t.Errorf("unexpected error for strategy %s and client %s: %v", strategy, clientType, err)
			}
		}
	}

	// by default admins and editors can instantiate build configs
	for _, strategy := range buildStrategyTypes() {
		for clientType, client := range clients {
			if _, err := instantiateBuildConfig(t, client.Build().BuildConfigs(testutil.Namespace()), buildConfigs[string(strategy)+clientType]); err != nil {
				t.Errorf("unexpected instantiate error for strategy %s and client %s: %v", strategy, clientType, err)
			}
		}
	}

	removeBuildStrategyRoleResources(t, rbacv1client.NewForConfigOrDie(clusterAdminClientConfig), projectAdminKubeClient.Authorization())

	// make sure buildconfigs are rejected
	for _, strategy := range buildStrategyTypes() {
		for clientType, client := range clients {
			if _, err := createBuildConfig(t, client.Build().BuildConfigs(testutil.Namespace()), strategy); !kapierror.IsForbidden(err) {
				t.Errorf("expected forbidden for strategy %s and client %s: got %v", strategy, clientType, err)
			}
		}
	}

	// make sure buildconfig updates are rejected
	for _, strategy := range buildStrategyTypes() {
		for clientType, client := range clients {
			if _, err := updateBuildConfig(t, client.Build().BuildConfigs(testutil.Namespace()), buildConfigs[string(strategy)+clientType]); !kapierror.IsForbidden(err) {
				t.Errorf("expected forbidden for strategy %s and client %s: got %v", strategy, clientType, err)
			}
		}
	}

	// make sure instantiate is rejected
	for _, strategy := range buildStrategyTypes() {
		for clientType, client := range clients {
			if _, err := instantiateBuildConfig(t, client.Build().BuildConfigs(testutil.Namespace()), buildConfigs[string(strategy)+clientType]); !kapierror.IsForbidden(err) {
				t.Errorf("expected forbidden for strategy %s and client %s: got %v", strategy, clientType, err)
			}
		}
	}
}

func setupBuildStrategyTest(t *testing.T, includeControllers bool) (clusterAdminClientConfig *rest.Config, projectAdminKubeClient kclientset.Interface, projectAdminClient, projectEditorClient buildv1client.Interface, cleanup func()) {
	namespace := testutil.Namespace()
	var clusterAdminKubeConfig string
	var masterConfig *configapi.MasterConfig
	var err error

	if includeControllers {
		masterConfig, clusterAdminKubeConfig, err = testserver.StartTestMaster()
	} else {
		masterConfig, clusterAdminKubeConfig, err = testserver.StartTestMasterAPI()
	}
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cleanup = func() {
		testserver.CleanupMasterEtcd(t, masterConfig)
	}

	clusterAdminClientConfig, err = testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var projectAdminConfig *rest.Config
	var projectEditorConfig *rest.Config
	projectAdminKubeClient, projectAdminConfig, err = testserver.CreateNewProject(clusterAdminClientConfig, namespace, "harold")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	projectAdminClient = buildv1client.NewForConfigOrDie(projectAdminConfig)
	_, projectEditorConfig, err = testutil.GetClientForUser(clusterAdminClientConfig, "joe")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	projectEditorClient = buildv1client.NewForConfigOrDie(projectEditorConfig)

	addJoe := &policy.RoleModificationOptions{
		RoleBindingNamespace: namespace,
		RoleName:             bootstrappolicy.EditRoleName,
		RoleKind:             "ClusterRole",
		RbacClient:           rbacv1client.NewForConfigOrDie(projectAdminConfig),
		Users:                []string{"joe"},
	}
	if err := addJoe.AddRole(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := testutil.WaitForPolicyUpdate(projectAdminKubeClient.Authorization(), namespace, "create", build.Resource(bootstrappolicy.DockerBuildResource), true); err != nil {
		t.Fatalf(err.Error())
	}

	if includeControllers {
		if err := testserver.WaitForServiceAccounts(projectAdminKubeClient, namespace, []string{"builder"}); err != nil {
			t.Fatalf(err.Error())
		}
	}

	// we need a template that doesn't create service accounts or rolebindings so editors can create
	// pipeline buildconfig's successfully, so we're not using the standard jenkins template.
	// but we do need a template that creates a service named jenkins.
	template, err := testutil.GetTemplateFixture("../testdata/jenkins-template.json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// pipeline defaults expect to find a template named jenkins-ephemeral
	// in the openshift namespace.
	template.Name = "jenkins-ephemeral"
	template.Namespace = "openshift"

	_, err = templateclient.NewForConfigOrDie(clusterAdminClientConfig).Template().Templates("openshift").Create(template)
	if err != nil {
		t.Fatalf("Couldn't create jenkins template: %v", err)
	}

	if includeControllers {
		clusterAdminKubeClientset, err := testutil.GetClusterAdminKubeInternalClient(clusterAdminKubeConfig)
		if err != nil {
			t.Fatal(err)
		}

		if err := testserver.WaitForServiceAccounts(clusterAdminKubeClientset, testutil.Namespace(), []string{bootstrappolicy.BuilderServiceAccountName, bootstrappolicy.DefaultServiceAccountName}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	return
}

func removeBuildStrategyRoleResources(t *testing.T, clusterAdminAuthorizationClient rbacv1client.RbacV1Interface, selfSarClient authorizationtypedclient.SelfSubjectAccessReviewsGetter) {
	// remove resources from role so that certain build strategies are forbidden
	for _, role := range []string{bootstrappolicy.BuildStrategyCustomRoleName, bootstrappolicy.BuildStrategyDockerRoleName, bootstrappolicy.BuildStrategySourceRoleName, bootstrappolicy.BuildStrategyJenkinsPipelineRoleName} {
		options := &policy.RoleModificationOptions{
			RoleName:   role,
			RoleKind:   "ClusterRole",
			RbacClient: clusterAdminAuthorizationClient,
			Groups:     []string{"system:authenticated"},
		}
		if err := options.RemoveRole(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	if err := testutil.WaitForPolicyUpdate(selfSarClient, testutil.Namespace(), "create", build.Resource(bootstrappolicy.DockerBuildResource), false); err != nil {
		t.Fatal(err)
	}
	if err := testutil.WaitForPolicyUpdate(selfSarClient, testutil.Namespace(), "create", build.Resource(bootstrappolicy.SourceBuildResource), false); err != nil {
		t.Fatal(err)
	}
	if err := testutil.WaitForPolicyUpdate(selfSarClient, testutil.Namespace(), "create", build.Resource(bootstrappolicy.CustomBuildResource), false); err != nil {
		t.Fatal(err)
	}
	if err := testutil.WaitForPolicyUpdate(selfSarClient, testutil.Namespace(), "create", build.Resource(bootstrappolicy.JenkinsPipelineBuildResource), false); err != nil {
		t.Fatal(err)
	}
}

func grantRestrictedBuildStrategyRoleResources(t *testing.T, clusterAdminAuthorizationClient rbacv1client.RbacV1Interface, selfSarClient authorizationtypedclient.SelfSubjectAccessReviewsGetter) {
	// grant resources to role so that restricted build strategies are available
	for _, role := range []string{bootstrappolicy.BuildStrategyCustomRoleName} {
		options := &policy.RoleModificationOptions{
			RoleName:   role,
			RoleKind:   "ClusterRole",
			RbacClient: clusterAdminAuthorizationClient,
			Groups:     []string{"system:authenticated"},
		}
		if err := options.AddRole(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	if err := testutil.WaitForPolicyUpdate(selfSarClient, testutil.Namespace(), "create", build.Resource(bootstrappolicy.CustomBuildResource), true); err != nil {
		t.Fatal(err)
	}
}

func strategyForType(t *testing.T, strategy string) buildv1.BuildStrategy {
	buildStrategy := buildv1.BuildStrategy{}
	switch strategy {
	case "docker":
		buildStrategy.DockerStrategy = &buildv1.DockerBuildStrategy{}
	case "custom":
		buildStrategy.CustomStrategy = &buildv1.CustomBuildStrategy{}
		buildStrategy.CustomStrategy.From.Kind = "DockerImage"
		buildStrategy.CustomStrategy.From.Name = "test/builderimage:latest"
	case "source":
		buildStrategy.SourceStrategy = &buildv1.SourceBuildStrategy{}
		buildStrategy.SourceStrategy.From.Kind = "DockerImage"
		buildStrategy.SourceStrategy.From.Name = "test/builderimage:latest"
	case "jenkinspipeline":
		buildStrategy.JenkinsPipelineStrategy = &buildv1.JenkinsPipelineBuildStrategy{}
	default:
		t.Fatalf("unknown strategy: %#v", strategy)
	}
	return buildStrategy
}

func createBuild(t *testing.T, buildInterface buildv1clienttyped.BuildInterface, strategy string) (*buildv1.Build, error) {
	build := &buildv1.Build{}
	build.ObjectMeta.Labels = map[string]string{
		buildutil.BuildConfigLabel:    "mock-build-config",
		buildutil.BuildRunPolicyLabel: string(buildv1.BuildRunPolicyParallel),
	}
	build.GenerateName = strings.ToLower(string(strategy)) + "-build-"
	build.Spec.Strategy = strategyForType(t, strategy)
	build.Spec.Source.Git = &buildv1.GitBuildSource{URI: "example.org"}

	return buildInterface.Create(build)
}

func updateBuild(t *testing.T, buildInterface buildv1clienttyped.BuildInterface, build *buildv1.Build) (*buildv1.Build, error) {
	build.Labels = map[string]string{"updated": "true"}
	return buildInterface.Update(build)
}

func createBuildConfig(t *testing.T, buildConfigInterface buildv1clienttyped.BuildConfigInterface, strategy string) (*buildv1.BuildConfig, error) {
	buildConfig := &buildv1.BuildConfig{}
	buildConfig.Spec.RunPolicy = buildv1.BuildRunPolicyParallel
	buildConfig.GenerateName = strings.ToLower(string(strategy)) + "-buildconfig-"
	buildConfig.Spec.Strategy = strategyForType(t, strategy)
	buildConfig.Spec.Source.Git = &buildv1.GitBuildSource{URI: "example.org"}

	return buildConfigInterface.Create(buildConfig)
}

func cloneBuild(t *testing.T, buildInterface buildv1clienttyped.BuildInterface, build *buildv1.Build) (*buildv1.Build, error) {
	req := &buildv1.BuildRequest{}
	req.Name = build.Name
	return buildInterface.Clone(build.Name, req)
}

func instantiateBuildConfig(t *testing.T, buildConfigInterface buildv1clienttyped.BuildConfigInterface, buildConfig *buildv1.BuildConfig) (*buildv1.Build, error) {
	req := &buildv1.BuildRequest{}
	req.Name = buildConfig.Name
	return buildConfigInterface.Instantiate(buildConfig.Name, req)
}

func updateBuildConfig(t *testing.T, buildConfigInterface buildv1clienttyped.BuildConfigInterface, buildConfig *buildv1.BuildConfig) (*buildv1.BuildConfig, error) {
	buildConfig.Labels = map[string]string{"updated": "true"}
	return buildConfigInterface.Update(buildConfig)
}
