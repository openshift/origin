package integration

import (
	"testing"

	kapierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

func TestPodUpdateSCCEnforcement(t *testing.T) {
	masterConfig, clusterAdminKubeConfig, err := testserver.StartTestMaster()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer testserver.CleanupMasterEtcd(t, masterConfig)

	clusterAdminKubeClientset, err := testutil.GetClusterAdminKubeClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	projectName := "hammer-project"

	if _, _, err := testserver.CreateNewProject(clusterAdminClientConfig, projectName, "harold"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	haroldKubeClient, _, err := testutil.GetClientForUser(clusterAdminClientConfig, "harold")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := testserver.WaitForServiceAccounts(clusterAdminKubeClientset, projectName, []string{"default"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// so cluster-admin can create privileged pods, but harold cannot.  This means that harold should not be able
	// to update the privileged pods either, even if he lies about its privileged nature
	privilegedPod := &kapi.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "unsafe"},
		Spec: kapi.PodSpec{
			Containers: []kapi.Container{
				{Name: "first", Image: "something-innocuous"},
			},
			SecurityContext: &kapi.PodSecurityContext{
				HostPID: true,
			},
		},
	}

	if _, err := haroldKubeClient.Core().Pods(projectName).Create(privilegedPod); !kapierror.IsForbidden(err) {
		t.Fatalf("missing forbidden: %v", err)
	}

	actualPod, err := clusterAdminKubeClientset.Core().Pods(projectName).Create(privilegedPod)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	actualPod.Spec.Containers[0].Image = "something-nefarious"
	if _, err := haroldKubeClient.Core().Pods(projectName).Update(actualPod); !kapierror.IsForbidden(err) {
		t.Fatalf("missing forbidden: %v", err)
	}

	// try to lie about the privileged nature
	actualPod.Spec.SecurityContext.HostPID = false
	if _, err := haroldKubeClient.Core().Pods(projectName).Update(actualPod); err == nil {
		t.Fatalf("missing error: %v", err)
	}
}
