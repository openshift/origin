// +build integration,!no-etcd

package integration

import (
	"reflect"
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	policy "github.com/openshift/origin/pkg/cmd/experimental/policy"
)

func TestRestrictedAccessForProjectAdmins(t *testing.T) {
	startConfig, err := StartTestMaster()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	openshiftClient, openshiftClientConfig, err := startConfig.GetOpenshiftClient()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// TODO remove once bootstrap authorization rules are tightened
	removeInsecureOptions := &policy.RemoveGroupOptions{
		RoleNamespace:    "master",
		RoleName:         "cluster-admin",
		BindingNamespace: "master",
		Client:           openshiftClient,
		Groups:           []string{"system:authenticated", "system:unauthenticated"},
	}
	if err := removeInsecureOptions.Run(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	haroldClient, err := CreateNewProject(openshiftClient, *openshiftClientConfig, "hammer-project", "harold")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	markClient, err := CreateNewProject(openshiftClient, *openshiftClientConfig, "mallet-project", "mark")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	_, err = haroldClient.Deployments("hammer-project").List(labels.Everything(), labels.Everything())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// TODO make kube and origin authorization failures cause a kapierror.Forbidden
	_, err = markClient.Deployments("hammer-project").List(labels.Everything(), labels.Everything())
	if (err == nil) || (!strings.Contains(err.Error(), "Forbidden")) {
		t.Errorf("expected forbidden error, but didn't get one")
	}

	// projects are a special case where a get of a project actually sets a namespace.  Make sure that
	// the namespace is properly special cased and set for authorization rules
	_, err = haroldClient.Projects().Get("hammer-project")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	// TODO make kube and origin authorization failures cause a kapierror.Forbidden
	_, err = markClient.Projects().Get("hammer-project")
	if (err == nil) || (!strings.Contains(err.Error(), "Forbidden")) {
		t.Errorf("expected forbidden error, but didn't get one")
	}

	// TODO restore this once we have detection for whether the cache is up to date.
	// wait for the project authorization cache to catch the change.  It is on a one second period
	// time.Sleep(5 * time.Second)

	// haroldProjects, err := haroldClient.Projects().List(labels.Everything(), labels.Everything())
	// if err != nil {
	// 	t.Errorf("unexpected error: %v", err)
	// }
	// if !((len(haroldProjects.Items) == 1) && (haroldProjects.Items[0].Name == "hammer-project")) {
	// 	t.Errorf("expected hammer-project, got %#v", haroldProjects.Items)
	// }

	// markProjects, err := markClient.Projects().List(labels.Everything(), labels.Everything())
	// if err != nil {
	// 	t.Errorf("unexpected error: %v", err)
	// }
	// if !((len(markProjects.Items) == 1) && (markProjects.Items[0].Name == "mallet-project")) {
	// 	t.Errorf("expected mallet-project, got %#v", markProjects.Items)
	// }
}

// TODO this list should start collapsing as we continue to tighten access on generated system ids
var globalClusterAdminUsers = util.NewStringSet("system:kube-client", "system:openshift-client", "system:openshift-deployer")
var globalClusterAdminGroups = util.NewStringSet("system:cluster-admins")

func TestResourceAccessReview(t *testing.T) {
	startConfig, err := StartTestMaster()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	openshiftClient, openshiftClientConfig, err := startConfig.GetOpenshiftClient()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// TODO remove once bootstrap authorization rules are tightened
	removeInsecureOptions := &policy.RemoveGroupOptions{
		RoleNamespace:    "master",
		RoleName:         "cluster-admin",
		BindingNamespace: "master",
		Client:           openshiftClient,
		Groups:           []string{"system:authenticated", "system:unauthenticated"},
	}
	if err := removeInsecureOptions.Run(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	haroldClient, err := CreateNewProject(openshiftClient, *openshiftClientConfig, "hammer-project", "harold")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	markClient, err := CreateNewProject(openshiftClient, *openshiftClientConfig, "mallet-project", "mark")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	addValerie := &policy.AddUserOptions{
		RoleNamespace:    "master",
		RoleName:         "view",
		BindingNamespace: "hammer-project",
		Client:           haroldClient,
		Users:            []string{"anypassword:valerie"},
	}
	if err := addValerie.Run(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	addEdgar := &policy.AddUserOptions{
		RoleNamespace:    "master",
		RoleName:         "edit",
		BindingNamespace: "mallet-project",
		Client:           markClient,
		Users:            []string{"anypassword:edgar"},
	}
	if err := addEdgar.Run(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	requestWhoCanViewDeployments := &authorizationapi.ResourceAccessReview{Verb: "get", Resource: "deployments"}

	whoCanViewDeploymentInHammer, err := haroldClient.ResourceAccessReviews("hammer-project").Create(requestWhoCanViewDeployments)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	expectedHammerUsers := util.NewStringSet("anypassword:harold", "anypassword:valerie")
	expectedHammerUsers.Insert(globalClusterAdminUsers.List()...)
	expectedHammerGroups := globalClusterAdminGroups
	actualHammerUsers := util.NewStringSet(whoCanViewDeploymentInHammer.Users...)
	actualHammerGroups := util.NewStringSet(whoCanViewDeploymentInHammer.Groups...)
	if !reflect.DeepEqual(expectedHammerGroups.List(), actualHammerGroups.List()) {
		t.Errorf("expected %v, got %v", expectedHammerGroups.List(), actualHammerGroups.List())
	}
	if !reflect.DeepEqual(expectedHammerUsers.List(), actualHammerUsers.List()) {
		t.Errorf("expected %v, got %v", expectedHammerUsers.List(), actualHammerUsers.List())
	}

	whoCanViewDeploymentInMallet, err := markClient.ResourceAccessReviews("mallet-project").Create(requestWhoCanViewDeployments)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	expectedMalletUsers := util.NewStringSet("anypassword:mark", "anypassword:edgar")
	expectedMalletUsers.Insert(globalClusterAdminUsers.List()...)
	expectedMalletGroups := globalClusterAdminGroups
	actualMalletUsers := util.NewStringSet(whoCanViewDeploymentInMallet.Users...)
	actualMalletGroups := util.NewStringSet(whoCanViewDeploymentInMallet.Groups...)
	if !reflect.DeepEqual(expectedMalletGroups.List(), actualMalletGroups.List()) {
		t.Errorf("expected %v, got %v", expectedMalletGroups.List(), actualMalletGroups.List())
	}
	if !reflect.DeepEqual(expectedMalletUsers.List(), actualMalletUsers.List()) {
		t.Errorf("expected %v, got %v", expectedMalletUsers.List(), actualMalletUsers.List())
	}

	// mark should not be able to make global access review requests
	_, err = markClient.ResourceAccessReviews("").Create(requestWhoCanViewDeployments)
	if (err == nil) || (!strings.Contains(err.Error(), "Forbidden")) {
		t.Errorf("expected forbidden error, but didn't get one")
	}

	// a cluster-admin should be able to make global access review requests
	whoCanViewDeploymentInAnyNamespace, err := openshiftClient.ResourceAccessReviews("").Create(requestWhoCanViewDeployments)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	actualAnyUsers := util.NewStringSet(whoCanViewDeploymentInAnyNamespace.Users...)
	actualAnyGroups := util.NewStringSet(whoCanViewDeploymentInAnyNamespace.Groups...)
	if !reflect.DeepEqual(globalClusterAdminGroups.List(), actualAnyGroups.List()) {
		t.Errorf("expected %v, got %v", globalClusterAdminGroups.List(), actualAnyGroups.List())
	}
	if !reflect.DeepEqual(globalClusterAdminUsers.List(), actualAnyUsers.List()) {
		t.Errorf("expected %v, got %v", globalClusterAdminUsers.List(), actualAnyUsers.List())
	}
}
