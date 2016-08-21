package integration

import (
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	kapierror "k8s.io/kubernetes/pkg/api/errors"

	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

func TestPodUpdateSCCEnforcement(t *testing.T) {
	testutil.RequireEtcd(t)
	defer testutil.DumpEtcdOnFailure(t)
	_, clusterAdminKubeConfig, err := testserver.StartTestMaster()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	clusterAdminClient, err := testutil.GetClusterAdminClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	clusterAdminKubeClient, err := testutil.GetClusterAdminKubeClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	projectName := "hammer-project"

	if _, err := testserver.CreateNewProject(clusterAdminClient, *clusterAdminClientConfig, projectName, "harold"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, haroldKubeClient, _, err := testutil.GetClientForUser(*clusterAdminClientConfig, "harold")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := testserver.WaitForServiceAccounts(clusterAdminKubeClient, projectName, []string{"default"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// so cluster-admin can create privileged pods, but harold cannot.  This means that harold should not be able
	// to update the privileged pods either, even if he lies about its privileged nature
	privilegedPod := &kapi.Pod{
		ObjectMeta: kapi.ObjectMeta{Name: "unsafe"},
		Spec: kapi.PodSpec{
			Containers: []kapi.Container{
				{Name: "first", Image: "something-innocuous"},
			},
			SecurityContext: &kapi.PodSecurityContext{
				HostPID: true,
			},
		},
	}

	if _, err := haroldKubeClient.Pods(projectName).Create(privilegedPod); !kapierror.IsForbidden(err) {
		t.Fatalf("missing forbidden: %v", err)
	}

	actualPod, err := clusterAdminKubeClient.Pods(projectName).Create(privilegedPod)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	actualPod.Spec.Containers[0].Image = "something-nefarious"
	if _, err := haroldKubeClient.Pods(projectName).Update(actualPod); !kapierror.IsForbidden(err) {
		t.Fatalf("missing forbidden: %v", err)
	}

	// try to lie about the privileged nature
	actualPod.Spec.SecurityContext.HostPID = false
	if _, err := haroldKubeClient.Pods(projectName).Update(actualPod); err == nil {
		t.Fatalf("missing error: %v", err)
	}
}
