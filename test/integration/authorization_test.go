// +build integration,!no-etcd

package integration

import (
	"strings"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"

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

	// wait for the project authorization cache to catch the change.  It is on a one second period
	time.Sleep(2 * time.Second)

	haroldProjects, err := haroldClient.Projects().List(labels.Everything(), labels.Everything())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !((len(haroldProjects.Items) == 1) && (haroldProjects.Items[0].Name == "hammer-project")) {
		t.Errorf("expected hammer-project, got %#v", haroldProjects.Items)
	}

	markProjects, err := markClient.Projects().List(labels.Everything(), labels.Everything())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !((len(markProjects.Items) == 1) && (markProjects.Items[0].Name == "mallet-project")) {
		t.Errorf("expected mallet-project, got %#v", markProjects.Items)
	}
}
