package veneers

import (
	"testing"
	"time"

	"github.com/gonum/graph"
	"github.com/gonum/graph/concrete"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	osgraph "github.com/openshift/origin/pkg/api/graph"
	kubeedges "github.com/openshift/origin/pkg/api/kubegraph"
	kubegraph "github.com/openshift/origin/pkg/api/kubegraph/nodes"
	buildapi "github.com/openshift/origin/pkg/build/api"
	buildedges "github.com/openshift/origin/pkg/build/graph"
	buildgraph "github.com/openshift/origin/pkg/build/graph/nodes"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deployedges "github.com/openshift/origin/pkg/deploy/graph"
	deploygraph "github.com/openshift/origin/pkg/deploy/graph/nodes"
	imageapi "github.com/openshift/origin/pkg/image/api"
	imagegraph "github.com/openshift/origin/pkg/image/graph/nodes"
)

func TestGraph(t *testing.T) {
	g := osgraph.New()
	now := time.Now()
	builds := []buildapi.Build{
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:              "build1-1-abc",
				Labels:            map[string]string{buildapi.BuildConfigLabel: "build1"},
				CreationTimestamp: util.NewTime(now.Add(-10 * time.Second)),
			},
			Status: buildapi.BuildStatusFailed,
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:              "build1-2-abc",
				Labels:            map[string]string{buildapi.BuildConfigLabel: "build1"},
				CreationTimestamp: util.NewTime(now.Add(-5 * time.Second)),
			},
			Status: buildapi.BuildStatusComplete,
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:              "build1-3-abc",
				Labels:            map[string]string{buildapi.BuildConfigLabel: "build1"},
				CreationTimestamp: util.NewTime(now.Add(-15 * time.Second)),
			},
			Status: buildapi.BuildStatusPending,
		},
	}

	bc1Node := buildgraph.EnsureBuildConfigNode(g, &buildapi.BuildConfig{
		ObjectMeta: kapi.ObjectMeta{Namespace: "default", Name: "build1"},
		Triggers: []buildapi.BuildTriggerPolicy{
			{
				ImageChange: &buildapi.ImageChangeTrigger{},
			},
		},
		Parameters: buildapi.BuildParameters{
			Strategy: buildapi.BuildStrategy{
				Type: buildapi.SourceBuildStrategyType,
				SourceStrategy: &buildapi.SourceBuildStrategy{
					From: &kapi.ObjectReference{Kind: "ImageStreamTag", Name: "test:base-image"},
				},
			},
			Output: buildapi.BuildOutput{
				To:  &kapi.ObjectReference{Name: "other"},
				Tag: "tag1",
			},
		},
	})
	buildedges.JoinBuilds(bc1Node, builds)
	bcTestNode := buildgraph.EnsureBuildConfigNode(g, &buildapi.BuildConfig{
		ObjectMeta: kapi.ObjectMeta{Namespace: "default", Name: "test"},
		Parameters: buildapi.BuildParameters{
			Output: buildapi.BuildOutput{
				To:  &kapi.ObjectReference{Name: "other"},
				Tag: "base-image",
			},
		},
	})
	buildgraph.EnsureBuildConfigNode(g, &buildapi.BuildConfig{
		ObjectMeta: kapi.ObjectMeta{Namespace: "default", Name: "build2"},
		Parameters: buildapi.BuildParameters{
			Output: buildapi.BuildOutput{
				DockerImageReference: "mycustom/repo/image",
				Tag:                  "tag2",
			},
		},
	})
	kubegraph.EnsureServiceNode(g, &kapi.Service{
		ObjectMeta: kapi.ObjectMeta{Namespace: "default", Name: "svc-is-ignored"},
		Spec: kapi.ServiceSpec{
			Selector: nil,
		},
	})
	kubegraph.EnsureServiceNode(g, &kapi.Service{
		ObjectMeta: kapi.ObjectMeta{Namespace: "default", Name: "svc1"},
		Spec: kapi.ServiceSpec{
			Selector: map[string]string{
				"deploymentconfig": "deploy1",
			},
		},
	})
	kubegraph.EnsureServiceNode(g, &kapi.Service{
		ObjectMeta: kapi.ObjectMeta{Namespace: "default", Name: "svc2"},
		Spec: kapi.ServiceSpec{
			Selector: map[string]string{
				"deploymentconfig": "deploy1",
				"env":              "prod",
			},
		},
	})
	deploygraph.EnsureDeploymentConfigNode(g, &deployapi.DeploymentConfig{
		ObjectMeta: kapi.ObjectMeta{Namespace: "other", Name: "deploy1"},
		Triggers: []deployapi.DeploymentTriggerPolicy{
			{
				ImageChangeParams: &deployapi.DeploymentTriggerImageChangeParams{
					From:           kapi.ObjectReference{Namespace: "default", Name: "other"},
					ContainerNames: []string{"1", "2"},
					Tag:            "tag1",
				},
			},
		},
		Template: deployapi.DeploymentTemplate{
			ControllerTemplate: kapi.ReplicationControllerSpec{
				Template: &kapi.PodTemplateSpec{
					ObjectMeta: kapi.ObjectMeta{
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
		},
	})
	deploygraph.EnsureDeploymentConfigNode(g, &deployapi.DeploymentConfig{
		ObjectMeta: kapi.ObjectMeta{Namespace: "default", Name: "deploy2"},
		Template: deployapi.DeploymentTemplate{
			ControllerTemplate: kapi.ReplicationControllerSpec{
				Template: &kapi.PodTemplateSpec{
					ObjectMeta: kapi.ObjectMeta{
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
		},
	})

	kubeedges.AddAllExposedPodTemplateSpecEdges(g)
	buildedges.AddAllInputOutputEdges(g)
	deployedges.AddAllTriggerEdges(g)

	t.Log(g)

	ir, dc, bc, other := 0, 0, 0, 0
	for _, node := range g.NodeList() {
		switch g.Object(node).(type) {
		case *deployapi.DeploymentConfig:
			if g.Kind(node) != deploygraph.DeploymentConfigNodeKind {
				t.Fatalf("unexpected kind: %v", g.Kind(node))
			}
			dc++
		case *buildapi.BuildConfig:
			if g.Kind(node) != buildgraph.BuildConfigNodeKind {
				t.Fatalf("unexpected kind: %v", g.Kind(node))
			}
			bc++
		case *imageapi.ImageStream:
			// TODO resolve this check for 2 kinds, since both have the same object type
			if g.Kind(node) != imagegraph.ImageStreamNodeKind && g.Kind(node) != imagegraph.ImageStreamTagNodeKind {
				t.Fatalf("unexpected kind: %v", g.Kind(node))
			}
			ir++
		default:
			other++
		}
	}

	if dc != 2 || bc != 3 || ir != 3 || other != 12 {
		t.Errorf("unexpected nodes: %d %d %d %d", dc, bc, ir, other)
	}
	for _, edge := range g.EdgeList() {
		if g.EdgeKind(edge) == osgraph.UnknownEdgeKind {
			t.Errorf("edge reported unknown kind: %#v", edge)
		}
	}

	// imagestreamtag default/other:base-image
	istID := 0
	for _, node := range g.NodeList() {
		if g.Name(node) == "<imagestreamtag default/other:base-image>" {
			istID = node.ID()
			break
		}
	}

	edge := g.EdgeBetween(concrete.Node(bcTestNode.ID()), concrete.Node(istID))
	if edge == nil {
		t.Fatalf("failed to find edge between %d and %d", bcTestNode.ID(), istID)
	}
	if len(g.SubgraphWithNodes([]graph.Node{edge.Head(), edge.Tail()}, osgraph.ExistingDirectEdge).EdgeList()) != 1 {
		t.Fatalf("expected one edge")
	}
	if len(g.SubgraphWithNodes([]graph.Node{edge.Tail(), edge.Head()}, osgraph.ExistingDirectEdge).EdgeList()) != 1 {
		t.Fatalf("expected one edge")
	}

	if e := g.EdgeBetween(concrete.Node(bcTestNode.ID()), concrete.Node(istID)); e == nil {
		t.Errorf("expected edge for %d-%d", bcTestNode.ID(), istID)
	}
	if e := g.EdgeBetween(concrete.Node(istID), concrete.Node(bcTestNode.ID())); e == nil {
		t.Errorf("expected edge for %d-%d", bcTestNode.ID(), istID)
	}

	pipelines, covered := DeploymentPipelines(g)
	if len(pipelines) != 2 {
		t.Fatalf("unexpected pipelines: %#v", pipelines)
	}
	if len(covered) != 7 {
		t.Fatalf("unexpected covered nodes: %#v", covered)
	}
	for from, images := range pipelines {
		t.Logf("from %s", from.Name)
		for _, path := range images {
			t.Logf("  %v", path)
		}
	}

	serviceGroups := ServiceAndDeploymentGroups(g)
	if len(serviceGroups) != 5 {
		t.Errorf("unexpected service groups: %#v", serviceGroups)
	}
	if len(serviceGroups[3].Builds) != 1 {
		t.Fatalf("unexpected final group: %#v", serviceGroups[2])
	}
	for _, group := range serviceGroups {
		dcs := len(group.Deployments)
		svcs := len(group.Services)
		for _, svc := range group.Services {
			t.Logf("service %s", svc.Service.Name)
		}
		indent := ""
		if svcs > 0 {
			indent = "  "
		}
		for _, deployment := range group.Deployments {
			t.Logf("%sdeployment %s", indent, deployment.Deployment.Name)
			for _, image := range deployment.Images {
				t.Logf("%s  image %s", indent, image.Image.ImageSpec())
				if image.Build != nil {
					if image.Build.LastSuccessfulBuild != nil {
						t.Logf("%s    built at %s", indent, image.Build.LastSuccessfulBuild.CreationTimestamp)
					} else if image.Build.LastUnsuccessfulBuild != nil {
						t.Logf("%s    build %s at %s", indent, image.Build.LastUnsuccessfulBuild.Status, image.Build.LastSuccessfulBuild.CreationTimestamp)
					}
					for _, b := range image.Build.ActiveBuilds {
						t.Logf("%s    build %s %s", indent, b.Name, b.Status)
					}
				}
			}
		}
		if dcs != 0 || svcs != 0 {
			continue
		}
		for _, build := range group.Builds {
			if build.Image != nil {
				if build.Build != nil {
					t.Logf("%s <- build %s (%d)", build.Image.ImageSpec(), build.Build.Name, build.Image.ID())
				} else {
					t.Logf("%s (%d)", build.Image.ImageSpec(), build.Image.ID())
				}
			} else {
				t.Logf("build %s (%d)", build.Build.Name, build.Build.ID())
				t.Errorf("expected build %d to have an image edge", build.Build.ID())
			}
		}
	}
}
