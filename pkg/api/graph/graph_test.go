package graph

import (
	"testing"
	"time"

	"github.com/gonum/graph"
	"github.com/gonum/graph/concrete"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	build "github.com/openshift/origin/pkg/build/api"
	deploy "github.com/openshift/origin/pkg/deploy/api"
	image "github.com/openshift/origin/pkg/image/api"
)

func TestGraph(t *testing.T) {
	g := New()
	now := time.Now()
	builds := []build.Build{
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:              "build1-1-abc",
				Labels:            map[string]string{build.BuildConfigLabel: "build1"},
				CreationTimestamp: util.NewTime(now.Add(-10 * time.Second)),
			},
			Status: build.BuildStatusFailed,
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:              "build1-2-abc",
				Labels:            map[string]string{build.BuildConfigLabel: "build1"},
				CreationTimestamp: util.NewTime(now.Add(-5 * time.Second)),
			},
			Status: build.BuildStatusComplete,
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:              "build1-3-abc",
				Labels:            map[string]string{build.BuildConfigLabel: "build1"},
				CreationTimestamp: util.NewTime(now.Add(-15 * time.Second)),
			},
			Status: build.BuildStatusPending,
		},
	}

	n := BuildConfig(g, &build.BuildConfig{
		ObjectMeta: kapi.ObjectMeta{Namespace: "default", Name: "build1"},
		Triggers: []build.BuildTriggerPolicy{
			{
				ImageChange: &build.ImageChangeTrigger{},
			},
		},
		Parameters: build.BuildParameters{
			Strategy: build.BuildStrategy{
				Type: build.SourceBuildStrategyType,
				SourceStrategy: &build.SourceBuildStrategy{
					From: &kapi.ObjectReference{Kind: "ImageStreamTag", Name: "test:base-image"},
				},
			},
			Output: build.BuildOutput{
				To:  &kapi.ObjectReference{Name: "other"},
				Tag: "tag1",
			},
		},
	})
	JoinBuilds(n.(*BuildConfigNode), builds)
	BuildConfig(g, &build.BuildConfig{
		ObjectMeta: kapi.ObjectMeta{Namespace: "default", Name: "test"},
		Parameters: build.BuildParameters{
			Output: build.BuildOutput{
				To:  &kapi.ObjectReference{Name: "other"},
				Tag: "base-image",
			},
		},
	})
	BuildConfig(g, &build.BuildConfig{
		ObjectMeta: kapi.ObjectMeta{Namespace: "default", Name: "build2"},
		Parameters: build.BuildParameters{
			Output: build.BuildOutput{
				DockerImageReference: "mycustom/repo/image",
				Tag:                  "tag2",
			},
		},
	})
	Service(g, &kapi.Service{
		ObjectMeta: kapi.ObjectMeta{Namespace: "default", Name: "svc-is-ignored"},
		Spec: kapi.ServiceSpec{
			Selector: nil,
		},
	})
	Service(g, &kapi.Service{
		ObjectMeta: kapi.ObjectMeta{Namespace: "default", Name: "svc1"},
		Spec: kapi.ServiceSpec{
			Selector: map[string]string{
				"deploymentconfig": "deploy1",
			},
		},
	})
	Service(g, &kapi.Service{
		ObjectMeta: kapi.ObjectMeta{Namespace: "default", Name: "svc2"},
		Spec: kapi.ServiceSpec{
			Selector: map[string]string{
				"deploymentconfig": "deploy1",
				"env":              "prod",
			},
		},
	})
	DeploymentConfig(g, &deploy.DeploymentConfig{
		ObjectMeta: kapi.ObjectMeta{Namespace: "other", Name: "deploy1"},
		Triggers: []deploy.DeploymentTriggerPolicy{
			{
				ImageChangeParams: &deploy.DeploymentTriggerImageChangeParams{
					From:           kapi.ObjectReference{Namespace: "default", Name: "other"},
					ContainerNames: []string{"1", "2"},
					Tag:            "tag1",
				},
			},
		},
		Template: deploy.DeploymentTemplate{
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
	DeploymentConfig(g, &deploy.DeploymentConfig{
		ObjectMeta: kapi.ObjectMeta{Namespace: "default", Name: "deploy2"},
		Template: deploy.DeploymentTemplate{
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

	CoverServices(g)

	ir, dc, bc, other := 0, 0, 0, 0
	for _, node := range g.NodeList() {
		t.Logf("node: %d %v", node.ID(), node)
		switch g.Object(node).(type) {
		case *deploy.DeploymentConfig:
			if g.Kind(node) != DeploymentConfigGraphKind {
				t.Fatalf("unexpected kind: %v", g.Kind(node))
			}
			dc++
		case *build.BuildConfig:
			if g.Kind(node) != BuildConfigGraphKind {
				t.Fatalf("unexpected kind: %v", g.Kind(node))
			}
			bc++
		case *image.ImageStream:
			if g.Kind(node) != ImageStreamGraphKind {
				t.Fatalf("unexpected kind: %v", g.Kind(node))
			}
			ir++
		default:
			other++
		}
	}
	if dc != 2 || bc != 3 || ir != 3 || other != 6 {
		t.Errorf("unexpected nodes: %d %d %d %d", dc, bc, ir, other)
	}
	for _, edge := range g.internal.EdgeList() {
		if g.EdgeKind(edge) == UnknownGraphEdgeKind {
			t.Errorf("edge reported unknown kind: %#v", edge)
		}
		t.Logf("edge: %v", edge)
	}
	reverse := g.EdgeSubgraph(ReverseGraphEdge)
	for _, edge := range reverse.internal.EdgeList() {
		t.Logf("redge: %v", edge)
	}

	edge := g.EdgeBetween(concrete.Node(4), concrete.Node(5))
	if len(g.SubgraphWithNodes([]graph.Node{edge.Head(), edge.Tail()}, ExistingDirectEdge).EdgeList()) != 1 {
		t.Fatalf("expected one edge")
	}
	if len(g.SubgraphWithNodes([]graph.Node{edge.Tail(), edge.Head()}, ExistingDirectEdge).EdgeList()) != 1 {
		t.Fatalf("expected one edge")
	}

	if e := g.EdgeBetween(concrete.Node(4), concrete.Node(5)); e == nil {
		t.Errorf("expected edge for 4-5")
	}
	if e := g.EdgeBetween(concrete.Node(5), concrete.Node(4)); e == nil {
		t.Errorf("expected edge for 4-5")
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
