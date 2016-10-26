package integration

import (
	"fmt"
	"testing"

	"k8s.io/kubernetes/pkg/api"
	kapiv1 "k8s.io/kubernetes/pkg/api/v1"

	v1buildapi "github.com/openshift/origin/pkg/build/api/v1"
	buildclient "github.com/openshift/origin/pkg/build/client/clientset_generated/release_v1_3"
	v1projectapi "github.com/openshift/origin/pkg/project/api/v1"
	projectclient "github.com/openshift/origin/pkg/project/client/clientset_generated/release_v1_3"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

func TestClientSet_v1_3(t *testing.T) {
	const namespace = "test-clientset-v13"
	testutil.RequireEtcd(t)

	_, clusterAdminKubeConfig, err := testserver.StartTestMaster()
	if err != nil {
		t.Fatal(err)
	}
	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatal(err)
	}

	testCreateProject := func() {
		c, err := projectclient.NewForConfig(clusterAdminClientConfig)
		if err != nil {
			t.Fatal(err)
		}

		project := &v1projectapi.Project{}
		project.Name = namespace
		if _, err := c.Projects().Create(project); err != nil {
			t.Fatal(err)
		}
	}

	testBuilds := func() {
		c, err := buildclient.NewForConfig(clusterAdminClientConfig)
		if err != nil {
			t.Fatal(err)
		}

		build := &v1buildapi.Build{}
		build.Name = "test-build"
		build.Spec.Source.Git = &v1buildapi.GitBuildSource{URI: "http://build.uri/build"}
		build.Spec.Strategy.DockerStrategy = &v1buildapi.DockerBuildStrategy{}
		build.Spec.Output.To = &kapiv1.ObjectReference{
			Kind: "DockerImage",
			Name: "namespace/image",
		}
		if _, err := c.Builds(namespace).Create(build); err != nil {
			t.Fatal(err)
		}
		result, err := c.Builds(namespace).List(api.ListOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Items) != 1 {
			t.Fatal(fmt.Errorf("expected to get 1 build, got %d", len(result.Items)))
		}
		if _, err := c.Builds(namespace).Get(build.Name); err != nil {
			t.Fatal(err)
		}
	}

	// try to create the non-namespaced resource
	testCreateProject()
	// try to create the namespace resource
	testBuilds()
}
