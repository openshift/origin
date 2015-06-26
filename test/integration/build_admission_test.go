// +build integration,!no-etcd

package integration

import (
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kapierror "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	buildapi "github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/client"
	policy "github.com/openshift/origin/pkg/cmd/admin/policy"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	testutil "github.com/openshift/origin/test/util"
)

func TestPolicyBasedRestrictionOfBuildStrategies(t *testing.T) {
	const namespace = "hammer"

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

	haroldClient, err := testutil.CreateNewProject(clusterAdminClient, *clusterAdminClientConfig, namespace, "harold")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	joeClient, err := testutil.GetClientForUser(*clusterAdminClientConfig, "joe")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	addJoe := &policy.RoleModificationOptions{
		RoleNamespace:       "",
		RoleName:            bootstrappolicy.EditRoleName,
		RoleBindingAccessor: policy.NewLocalRoleBindingAccessor(namespace, haroldClient),
		Users:               []string{"joe"},
	}
	if err := addJoe.AddRole(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := testutil.WaitForPolicyUpdate(joeClient, namespace, "create", authorizationapi.DockerBuildResource, true); err != nil {
		t.Error(err)
	}

	// by default admins and editors can create all type of builds
	_, err = createDockerBuild(t, haroldClient.Builds(namespace))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	_, err = createDockerBuild(t, joeClient.Builds(namespace))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	_, err = createSourceBuild(t, haroldClient.Builds(namespace))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	_, err = createSourceBuild(t, joeClient.Builds(namespace))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	_, err = createCustomBuild(t, haroldClient.Builds(namespace))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	_, err = createCustomBuild(t, joeClient.Builds(namespace))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// remove resources from role so that certain build strategies are forbidden
	removeBuildStrategyPrivileges(t, clusterAdminClient.ClusterRoles(), bootstrappolicy.EditRoleName)
	if err := testutil.WaitForPolicyUpdate(joeClient, namespace, "create", authorizationapi.DockerBuildResource, false); err != nil {
		t.Error(err)
	}

	removeBuildStrategyPrivileges(t, clusterAdminClient.ClusterRoles(), bootstrappolicy.AdminRoleName)
	if err := testutil.WaitForPolicyUpdate(haroldClient, namespace, "create", authorizationapi.DockerBuildResource, false); err != nil {
		t.Error(err)
	}

	// make sure builds are rejected
	if _, err = createDockerBuild(t, haroldClient.Builds(namespace)); !kapierror.IsForbidden(err) {
		t.Errorf("expected forbidden, got %v", err)
	}
	if _, err = createDockerBuild(t, joeClient.Builds(namespace)); !kapierror.IsForbidden(err) {
		t.Errorf("expected forbidden, got %v", err)
	}
	if _, err = createSourceBuild(t, haroldClient.Builds(namespace)); !kapierror.IsForbidden(err) {
		t.Errorf("expected forbidden, got %v", err)
	}
	if _, err = createSourceBuild(t, joeClient.Builds(namespace)); !kapierror.IsForbidden(err) {
		t.Errorf("expected forbidden, got %v", err)
	}
	if _, err = createCustomBuild(t, haroldClient.Builds(namespace)); !kapierror.IsForbidden(err) {
		t.Errorf("expected forbidden, got %v", err)
	}
	if _, err = createCustomBuild(t, joeClient.Builds(namespace)); !kapierror.IsForbidden(err) {
		t.Errorf("expected forbidden, got %v", err)
	}
}

func removeBuildStrategyPrivileges(t *testing.T, clusterRoleInterface client.ClusterRoleInterface, roleName string) {
	role, err := clusterRoleInterface.Get(roleName)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	for i := range role.Rules {
		role.Rules[i].Resources.Delete(authorizationapi.DockerBuildResource, authorizationapi.SourceBuildResource, authorizationapi.CustomBuildResource)
	}
	if _, err := clusterRoleInterface.Update(role); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

}

func createDockerBuild(t *testing.T, buildInterface client.BuildInterface) (*buildapi.Build, error) {
	dockerBuild := &buildapi.Build{}
	dockerBuild.GenerateName = "docker-build-"
	dockerBuild.Parameters.Strategy.Type = buildapi.DockerBuildStrategyType
	dockerBuild.Parameters.Source.Type = buildapi.BuildSourceGit
	dockerBuild.Parameters.Source.Git = &buildapi.GitBuildSource{URI: "example.org"}

	return buildInterface.Create(dockerBuild)
}

func createSourceBuild(t *testing.T, buildInterface client.BuildInterface) (*buildapi.Build, error) {
	dockerBuild := &buildapi.Build{}
	dockerBuild.GenerateName = "source-build-"
	dockerBuild.Parameters.Strategy.Type = buildapi.SourceBuildStrategyType
	dockerBuild.Parameters.Strategy.SourceStrategy = &buildapi.SourceBuildStrategy{From: kapi.ObjectReference{Name: "name:tag"}}
	dockerBuild.Parameters.Source.Type = buildapi.BuildSourceGit
	dockerBuild.Parameters.Source.Git = &buildapi.GitBuildSource{URI: "example.org"}

	return buildInterface.Create(dockerBuild)
}

func createCustomBuild(t *testing.T, buildInterface client.BuildInterface) (*buildapi.Build, error) {
	dockerBuild := &buildapi.Build{}
	dockerBuild.GenerateName = "custom-build-"
	dockerBuild.Parameters.Strategy.Type = buildapi.CustomBuildStrategyType
	dockerBuild.Parameters.Strategy.CustomStrategy = &buildapi.CustomBuildStrategy{From: kapi.ObjectReference{Name: "name:tag"}}
	dockerBuild.Parameters.Source.Type = buildapi.BuildSourceGit
	dockerBuild.Parameters.Source.Git = &buildapi.GitBuildSource{URI: "example.org"}

	return buildInterface.Create(dockerBuild)
}

func TestPolicyBasedRestrictionOfBuildConfigStrategies(t *testing.T) {
	const namespace = "hammer"

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

	haroldClient, err := testutil.CreateNewProject(clusterAdminClient, *clusterAdminClientConfig, namespace, "harold")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	joeClient, err := testutil.GetClientForUser(*clusterAdminClientConfig, "joe")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	addJoe := &policy.RoleModificationOptions{
		RoleNamespace:       "",
		RoleName:            bootstrappolicy.EditRoleName,
		RoleBindingAccessor: policy.NewLocalRoleBindingAccessor(namespace, haroldClient),
		Users:               []string{"joe"},
	}
	if err := addJoe.AddRole(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := testutil.WaitForPolicyUpdate(joeClient, namespace, "create", authorizationapi.DockerBuildResource, true); err != nil {
		t.Error(err)
	}

	// by default admins and editors can create all type of buildconfigs
	_, err = createDockerBuildConfig(t, haroldClient.BuildConfigs(namespace))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	_, err = createDockerBuildConfig(t, joeClient.BuildConfigs(namespace))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	_, err = createSourceBuildConfig(t, haroldClient.BuildConfigs(namespace))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	_, err = createSourceBuildConfig(t, joeClient.BuildConfigs(namespace))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	_, err = createCustomBuildConfig(t, haroldClient.BuildConfigs(namespace))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	_, err = createCustomBuildConfig(t, joeClient.BuildConfigs(namespace))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// remove resources from role so that certain build strategies are forbidden
	removeBuildStrategyPrivileges(t, clusterAdminClient.ClusterRoles(), bootstrappolicy.EditRoleName)
	if err := testutil.WaitForPolicyUpdate(joeClient, namespace, "create", authorizationapi.DockerBuildResource, false); err != nil {
		t.Error(err)
	}

	removeBuildStrategyPrivileges(t, clusterAdminClient.ClusterRoles(), bootstrappolicy.AdminRoleName)
	if err := testutil.WaitForPolicyUpdate(haroldClient, namespace, "create", authorizationapi.DockerBuildResource, false); err != nil {
		t.Error(err)
	}

	// make sure buildconfigs are rejected
	if _, err = createDockerBuildConfig(t, haroldClient.BuildConfigs(namespace)); !kapierror.IsForbidden(err) {
		t.Errorf("expected forbidden, got %v", err)
	}
	if _, err = createDockerBuildConfig(t, joeClient.BuildConfigs(namespace)); !kapierror.IsForbidden(err) {
		t.Errorf("expected forbidden, got %v", err)
	}
	if _, err = createSourceBuildConfig(t, haroldClient.BuildConfigs(namespace)); !kapierror.IsForbidden(err) {
		t.Errorf("expected forbidden, got %v", err)
	}
	if _, err = createSourceBuildConfig(t, joeClient.BuildConfigs(namespace)); !kapierror.IsForbidden(err) {
		t.Errorf("expected forbidden, got %v", err)
	}
	if _, err = createCustomBuildConfig(t, haroldClient.BuildConfigs(namespace)); !kapierror.IsForbidden(err) {
		t.Errorf("expected forbidden, got %v", err)
	}
	if _, err = createCustomBuildConfig(t, joeClient.BuildConfigs(namespace)); !kapierror.IsForbidden(err) {
		t.Errorf("expected forbidden, got %v", err)
	}
}

func createDockerBuildConfig(t *testing.T, buildConfigInterface client.BuildConfigInterface) (*buildapi.BuildConfig, error) {
	dockerBuild := &buildapi.BuildConfig{}
	dockerBuild.GenerateName = "docker-buildconfig-"
	dockerBuild.Parameters.Strategy.Type = buildapi.DockerBuildStrategyType
	dockerBuild.Parameters.Source.Type = buildapi.BuildSourceGit
	dockerBuild.Parameters.Source.Git = &buildapi.GitBuildSource{URI: "example.org"}

	return buildConfigInterface.Create(dockerBuild)
}

func createSourceBuildConfig(t *testing.T, buildConfigInterface client.BuildConfigInterface) (*buildapi.BuildConfig, error) {
	dockerBuild := &buildapi.BuildConfig{}
	dockerBuild.GenerateName = "source-buildconfig-"
	dockerBuild.Parameters.Strategy.Type = buildapi.SourceBuildStrategyType
	dockerBuild.Parameters.Strategy.SourceStrategy = &buildapi.SourceBuildStrategy{From: kapi.ObjectReference{Name: "name:tag"}}
	dockerBuild.Parameters.Source.Type = buildapi.BuildSourceGit
	dockerBuild.Parameters.Source.Git = &buildapi.GitBuildSource{URI: "example.org"}

	return buildConfigInterface.Create(dockerBuild)
}

func createCustomBuildConfig(t *testing.T, buildConfigInterface client.BuildConfigInterface) (*buildapi.BuildConfig, error) {
	dockerBuild := &buildapi.BuildConfig{}
	dockerBuild.GenerateName = "custom-buildconfig-"
	dockerBuild.Parameters.Strategy.Type = buildapi.CustomBuildStrategyType
	dockerBuild.Parameters.Strategy.CustomStrategy = &buildapi.CustomBuildStrategy{From: kapi.ObjectReference{Name: "name:tag"}}
	dockerBuild.Parameters.Source.Type = buildapi.BuildSourceGit
	dockerBuild.Parameters.Source.Git = &buildapi.GitBuildSource{URI: "example.org"}

	return buildConfigInterface.Create(dockerBuild)
}
