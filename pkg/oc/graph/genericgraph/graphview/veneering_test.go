package graphview

import (
	"testing"
	"time"

	"github.com/gonum/graph"
	"github.com/gonum/graph/concrete"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	appsedges "github.com/openshift/origin/pkg/oc/graph/appsgraph"
	appsgraph "github.com/openshift/origin/pkg/oc/graph/appsgraph/nodes"
	buildedges "github.com/openshift/origin/pkg/oc/graph/buildgraph"
	buildgraph "github.com/openshift/origin/pkg/oc/graph/buildgraph/nodes"
	osgraph "github.com/openshift/origin/pkg/oc/graph/genericgraph"
	osgraphtest "github.com/openshift/origin/pkg/oc/graph/genericgraph/test"
	imageedges "github.com/openshift/origin/pkg/oc/graph/imagegraph"
	kubeedges "github.com/openshift/origin/pkg/oc/graph/kubegraph"
	kubegraph "github.com/openshift/origin/pkg/oc/graph/kubegraph/nodes"
)

func TestServiceGroup(t *testing.T) {
	g, _, err := osgraphtest.BuildGraph("../../../graph/genericgraph/test/new-app.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	kubeedges.AddAllExposedPodTemplateSpecEdges(g)
	buildedges.AddAllInputOutputEdges(g)
	appsedges.AddAllTriggerDeploymentConfigsEdges(g)

	coveredNodes := IntSet{}

	serviceGroups, coveredByServiceGroups := AllServiceGroups(g, coveredNodes)
	coveredNodes.Insert(coveredByServiceGroups.List()...)

	bareDCPipelines, coveredByDCs := AllDeploymentConfigPipelines(g, coveredNodes)
	coveredNodes.Insert(coveredByDCs.List()...)

	bareBCPipelines, coveredByBCs := AllImagePipelinesFromBuildConfig(g, coveredNodes)
	coveredNodes.Insert(coveredByBCs.List()...)

	t.Log(g)

	if e, a := 2, len(serviceGroups); e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	if e, a := 0, len(bareDCPipelines); e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	if e, a := 1, len(bareBCPipelines); e != a {
		t.Errorf("expected %v, got %v", e, a)
	}

	if e, a := 1, len(serviceGroups[0].DeploymentConfigPipelines); e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	if e, a := 1, len(serviceGroups[0].DeploymentConfigPipelines[0].Images); e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	if e, a := 1, len(serviceGroups[1].DeploymentConfigPipelines); e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	if e, a := 1, len(serviceGroups[1].DeploymentConfigPipelines[0].Images); e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
}

func TestBareRCGroup(t *testing.T) {
	g, _, err := osgraphtest.BuildGraph("../../../graph/genericgraph/test/bare-rc.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	kubeedges.AddAllExposedPodTemplateSpecEdges(g)
	kubeedges.AddAllExposedPodEdges(g)
	kubeedges.AddAllManagedByControllerPodEdges(g)

	coveredNodes := IntSet{}

	serviceGroups, coveredByServiceGroups := AllServiceGroups(g, coveredNodes)
	coveredNodes.Insert(coveredByServiceGroups.List()...)

	bareRCs, coveredByRCs := AllReplicationControllers(g, coveredNodes)
	coveredNodes.Insert(coveredByRCs.List()...)

	if e, a := 1, len(serviceGroups); e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	if e, a := 1, len(bareRCs); e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
}

func TestBareDCGroup(t *testing.T) {
	g, _, err := osgraphtest.BuildGraph("../../../graph/genericgraph/test/bare-dc.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	kubeedges.AddAllExposedPodTemplateSpecEdges(g)
	buildedges.AddAllInputOutputEdges(g)
	appsedges.AddAllTriggerDeploymentConfigsEdges(g)

	coveredNodes := IntSet{}

	serviceGroups, coveredByServiceGroups := AllServiceGroups(g, coveredNodes)
	coveredNodes.Insert(coveredByServiceGroups.List()...)

	bareDCPipelines, coveredByDCs := AllDeploymentConfigPipelines(g, coveredNodes)
	coveredNodes.Insert(coveredByDCs.List()...)

	bareBCPipelines, coveredByBCs := AllImagePipelinesFromBuildConfig(g, coveredNodes)
	coveredNodes.Insert(coveredByBCs.List()...)

	if e, a := 0, len(serviceGroups); e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	if e, a := 1, len(bareDCPipelines); e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	if e, a := 0, len(bareBCPipelines); e != a {
		t.Errorf("expected %v, got %v", e, a)
	}

	if e, a := 1, len(bareDCPipelines[0].Images); e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
}

func TestAllBCImageInputs(t *testing.T) {
	g, _, err := osgraphtest.BuildGraph("../../../graph/genericgraph/test/prereq-image-present.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	buildedges.AddAllInputOutputEdges(g)
	imageedges.AddAllImageStreamRefEdges(g)
	imageedges.AddAllImageStreamImageRefEdges(g)

	coveredNodes := IntSet{}

	bareBCPipelines, coveredByBCs := AllImagePipelinesFromBuildConfig(g, coveredNodes)
	coveredNodes.Insert(coveredByBCs.List()...)

	if e, a := 4, len(bareBCPipelines); e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
}

func TestBareBCGroup(t *testing.T) {
	g, _, err := osgraphtest.BuildGraph("../../../graph/genericgraph/test/bare-bc.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	kubeedges.AddAllExposedPodTemplateSpecEdges(g)
	buildedges.AddAllInputOutputEdges(g)
	appsedges.AddAllTriggerDeploymentConfigsEdges(g)

	coveredNodes := IntSet{}

	serviceGroups, coveredByServiceGroups := AllServiceGroups(g, coveredNodes)
	coveredNodes.Insert(coveredByServiceGroups.List()...)

	bareDCPipelines, coveredByDCs := AllDeploymentConfigPipelines(g, coveredNodes)
	coveredNodes.Insert(coveredByDCs.List()...)

	bareBCPipelines, coveredByBCs := AllImagePipelinesFromBuildConfig(g, coveredNodes)
	coveredNodes.Insert(coveredByBCs.List()...)

	if e, a := 0, len(serviceGroups); e != a {
		t.Errorf("expected %v services, got %v", e, a)
	}
	if e, a := 0, len(bareDCPipelines); e != a {
		t.Errorf("expected %v deploymentConfigs, got %v", e, a)
	}
	if e, a := 1, len(bareBCPipelines); e != a {
		t.Errorf("expected %v buildConfigs, got %v", e, a)
	}
}

func TestGraph(t *testing.T) {
	g := osgraph.New()
	now := time.Now()
	builds := []buildapi.Build{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "build1-1-abc",
				Labels:            map[string]string{buildapi.BuildConfigLabelDeprecated: "build1"},
				CreationTimestamp: metav1.NewTime(now.Add(-10 * time.Second)),
			},
			Status: buildapi.BuildStatus{
				Phase: buildapi.BuildPhaseFailed,
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "build1-2-abc",
				Labels:            map[string]string{buildapi.BuildConfigLabelDeprecated: "build1"},
				CreationTimestamp: metav1.NewTime(now.Add(-5 * time.Second)),
			},
			Status: buildapi.BuildStatus{
				Phase: buildapi.BuildPhaseComplete,
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "build1-3-abc",
				Labels:            map[string]string{buildapi.BuildConfigLabelDeprecated: "build1"},
				CreationTimestamp: metav1.NewTime(now.Add(-15 * time.Second)),
			},
			Status: buildapi.BuildStatus{
				Phase: buildapi.BuildPhasePending,
			},
		},
	}
	for i := range builds {
		buildgraph.EnsureBuildNode(g, &builds[i])
	}

	buildgraph.EnsureBuildConfigNode(g, &buildapi.BuildConfig{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "build1"},
		Spec: buildapi.BuildConfigSpec{
			Triggers: []buildapi.BuildTriggerPolicy{
				{
					ImageChange: &buildapi.ImageChangeTrigger{},
				},
			},
			CommonSpec: buildapi.CommonSpec{
				Strategy: buildapi.BuildStrategy{
					SourceStrategy: &buildapi.SourceBuildStrategy{
						From: kapi.ObjectReference{Kind: "ImageStreamTag", Name: "test:base-image"},
					},
				},
				Output: buildapi.BuildOutput{
					To: &kapi.ObjectReference{Kind: "ImageStreamTag", Name: "other:tag1"},
				},
			},
		},
	})
	bcTestNode := buildgraph.EnsureBuildConfigNode(g, &buildapi.BuildConfig{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "test"},
		Spec: buildapi.BuildConfigSpec{
			CommonSpec: buildapi.CommonSpec{
				Output: buildapi.BuildOutput{
					To: &kapi.ObjectReference{Kind: "ImageStreamTag", Name: "other:base-image"},
				},
			},
		},
	})
	buildgraph.EnsureBuildConfigNode(g, &buildapi.BuildConfig{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "build2"},
		Spec: buildapi.BuildConfigSpec{
			CommonSpec: buildapi.CommonSpec{
				Output: buildapi.BuildOutput{
					To: &kapi.ObjectReference{Kind: "DockerImage", Name: "mycustom/repo/image:tag2"},
				},
			},
		},
	})
	kubegraph.EnsureServiceNode(g, &kapi.Service{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "svc-is-ignored"},
		Spec: kapi.ServiceSpec{
			Selector: nil,
		},
	})
	kubegraph.EnsureServiceNode(g, &kapi.Service{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "svc1"},
		Spec: kapi.ServiceSpec{
			Selector: map[string]string{
				"deploymentconfig": "deploy1",
			},
		},
	})
	kubegraph.EnsureServiceNode(g, &kapi.Service{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "svc2"},
		Spec: kapi.ServiceSpec{
			Selector: map[string]string{
				"deploymentconfig": "deploy1",
				"env":              "prod",
			},
		},
	})
	appsgraph.EnsureDeploymentConfigNode(g, &appsapi.DeploymentConfig{
		ObjectMeta: metav1.ObjectMeta{Namespace: "other", Name: "deploy1"},
		Spec: appsapi.DeploymentConfigSpec{
			Triggers: []appsapi.DeploymentTriggerPolicy{
				{
					ImageChangeParams: &appsapi.DeploymentTriggerImageChangeParams{
						From:           kapi.ObjectReference{Kind: "ImageStreamTag", Namespace: "default", Name: "other:tag1"},
						ContainerNames: []string{"1", "2"},
					},
				},
			},
			Template: &kapi.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"deploymentconfig": "deploy1",
						"env":              "prod",
					},
				},
				Spec: kapi.PodSpec{
					Containers: []kapi.Container{
						{
							Name:  "1",
							Image: "mycustom/repo/image",
						},
						{
							Name:  "2",
							Image: "mycustom/repo/image2",
						},
						{
							Name:  "3",
							Image: "mycustom/repo/image3",
						},
					},
				},
			},
		},
	})
	appsgraph.EnsureDeploymentConfigNode(g, &appsapi.DeploymentConfig{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "deploy2"},
		Spec: appsapi.DeploymentConfigSpec{
			Template: &kapi.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"deploymentconfig": "deploy2",
						"env":              "dev",
					},
				},
				Spec: kapi.PodSpec{
					Containers: []kapi.Container{
						{
							Name:  "1",
							Image: "someother/image:v1",
						},
					},
				},
			},
		},
	})

	kubeedges.AddAllExposedPodTemplateSpecEdges(g)
	buildedges.AddAllInputOutputEdges(g)
	buildedges.AddAllBuildEdges(g)
	appsedges.AddAllTriggerDeploymentConfigsEdges(g)
	appsedges.AddAllDeploymentConfigsDeploymentEdges(g)

	t.Log(g)

	for _, edge := range g.Edges() {
		if g.EdgeKinds(edge).Has(osgraph.UnknownEdgeKind) {
			t.Errorf("edge reported unknown kind: %#v", edge)
		}
	}

	// imagestreamtag default/other:base-image
	istID := 0
	for _, node := range g.Nodes() {
		if g.Name(node) == "ImageStreamTag|default/other:base-image" {
			istID = node.ID()
			break
		}
	}

	edge := g.Edge(concrete.Node(bcTestNode.ID()), concrete.Node(istID))
	if edge == nil {
		t.Fatalf("failed to find edge between %d and %d", bcTestNode.ID(), istID)
	}
	if len(g.SubgraphWithNodes([]graph.Node{edge.From(), edge.To()}, osgraph.ExistingDirectEdge).Edges()) != 1 {
		t.Fatalf("expected one edge")
	}
	if len(g.SubgraphWithNodes([]graph.Node{edge.To(), edge.From()}, osgraph.ExistingDirectEdge).Edges()) != 1 {
		t.Fatalf("expected one edge")
	}

	if e := g.Edge(concrete.Node(bcTestNode.ID()), concrete.Node(istID)); e == nil {
		t.Errorf("expected edge for %d-%d", bcTestNode.ID(), istID)
	}

	coveredNodes := IntSet{}

	serviceGroups, coveredByServiceGroups := AllServiceGroups(g, coveredNodes)
	coveredNodes.Insert(coveredByServiceGroups.List()...)

	bareDCPipelines, coveredByDCs := AllDeploymentConfigPipelines(g, coveredNodes)
	coveredNodes.Insert(coveredByDCs.List()...)

	if len(bareDCPipelines) != 2 {
		t.Fatalf("unexpected pipelines: %#v", bareDCPipelines)
	}
	if len(coveredNodes) != 10 {
		t.Fatalf("unexpected covered nodes: %#v", coveredNodes)
	}

	for _, bareDCPipeline := range bareDCPipelines {
		t.Logf("from %s", bareDCPipeline.DeploymentConfig.DeploymentConfig.Name)
		for _, path := range bareDCPipeline.Images {
			t.Logf("  %v", path)
		}
	}

	if len(serviceGroups) != 3 {
		t.Errorf("unexpected service groups: %#v", serviceGroups)
	}
	for _, serviceGroup := range serviceGroups {
		t.Logf("service %s", serviceGroup.Service.Name)
		indent := "  "

		for _, deployment := range serviceGroup.DeploymentConfigPipelines {
			t.Logf("%sdeployment %s", indent, deployment.DeploymentConfig.DeploymentConfig.Name)
			for _, image := range deployment.Images {
				t.Logf("%s  image %s", indent, image.Image.ImageSpec())
				if image.Build != nil {
					if image.LastSuccessfulBuild != nil {
						t.Logf("%s    built at %s", indent, image.LastSuccessfulBuild.Build.CreationTimestamp)
					} else if image.LastUnsuccessfulBuild != nil {
						t.Logf("%s    build %v at %s", indent, image.LastUnsuccessfulBuild.Build.Status, image.LastUnsuccessfulBuild.Build.CreationTimestamp)
					}
					for _, b := range image.ActiveBuilds {
						t.Logf("%s    build %s %v", indent, b.Build.Name, b.Build.Status)
					}
				}
			}
		}
	}
}
