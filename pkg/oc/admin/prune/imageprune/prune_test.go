package imageprune

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"reflect"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/rest/fake"
	clientgotesting "k8s.io/client-go/testing"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kapisext "k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/staging/src/k8s.io/apimachinery/pkg/util/diff"

	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	imageclient "github.com/openshift/origin/pkg/image/generated/internalclientset/fake"
	"github.com/openshift/origin/pkg/oc/admin/prune/imageprune/testutil"
	"github.com/openshift/origin/pkg/oc/graph/genericgraph"
	imagegraph "github.com/openshift/origin/pkg/oc/graph/imagegraph/nodes"

	// these are needed to make kapiref.GetReference work in the prune.go file
	_ "github.com/openshift/origin/pkg/apps/apis/apps/install"
	_ "github.com/openshift/origin/pkg/build/apis/build/install"
	_ "github.com/openshift/origin/pkg/image/apis/image/install"
	_ "k8s.io/kubernetes/pkg/apis/core/install"
	_ "k8s.io/kubernetes/pkg/apis/extensions/install"
)

var logLevel = flag.Int("loglevel", 0, "")

func TestImagePruning(t *testing.T) {
	flag.Lookup("v").Value.Set(fmt.Sprint(*logLevel))
	registryHost := "registry.io"
	registryURL := "https://" + registryHost

	tests := []struct {
		name                       string
		pruneOverSizeLimit         *bool
		allImages                  *bool
		pruneRegistry              *bool
		keepTagRevisions           *int
		namespace                  string
		images                     imageapi.ImageList
		pods                       kapi.PodList
		streams                    imageapi.ImageStreamList
		rcs                        kapi.ReplicationControllerList
		bcs                        buildapi.BuildConfigList
		builds                     buildapi.BuildList
		dss                        kapisext.DaemonSetList
		deployments                kapisext.DeploymentList
		dcs                        appsapi.DeploymentConfigList
		rss                        kapisext.ReplicaSetList
		limits                     map[string][]*kapi.LimitRange
		expectedImageDeletions     []string
		expectedStreamUpdates      []string
		expectedLayerLinkDeletions []string
		expectedBlobDeletions      []string
		expectedErrorString        string
	}{
		{
			name:   "1 pod - phase pending - don't prune",
			images: testutil.ImageList(testutil.Image("sha256:0000000000000000000000000000000000000000000000000000000000000000", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000")),
			pods:   testutil.PodList(testutil.Pod("foo", "pod1", kapi.PodPending, registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000")),
			expectedImageDeletions: []string{},
		},

		{
			name:   "3 pods - last phase pending - don't prune",
			images: testutil.ImageList(testutil.Image("sha256:0000000000000000000000000000000000000000000000000000000000000000", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000")),
			pods: testutil.PodList(
				testutil.Pod("foo", "pod1", kapi.PodSucceeded, registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000"),
				testutil.Pod("foo", "pod2", kapi.PodSucceeded, registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000"),
				testutil.Pod("foo", "pod3", kapi.PodPending, registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000"),
			),
			expectedImageDeletions: []string{},
		},

		{
			name:   "1 pod - phase running - don't prune",
			images: testutil.ImageList(testutil.Image("sha256:0000000000000000000000000000000000000000000000000000000000000000", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000")),
			pods:   testutil.PodList(testutil.Pod("foo", "pod1", kapi.PodRunning, registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000")),
			expectedImageDeletions: []string{},
		},

		{
			name:   "3 pods - last phase running - don't prune",
			images: testutil.ImageList(testutil.Image("sha256:0000000000000000000000000000000000000000000000000000000000000000", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000")),
			pods: testutil.PodList(
				testutil.Pod("foo", "pod1", kapi.PodSucceeded, registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000"),
				testutil.Pod("foo", "pod2", kapi.PodSucceeded, registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000"),
				testutil.Pod("foo", "pod3", kapi.PodRunning, registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000"),
			),
			expectedImageDeletions: []string{},
		},

		{
			name:   "pod phase succeeded - prune",
			images: testutil.ImageList(testutil.Image("sha256:0000000000000000000000000000000000000000000000000000000000000000", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000")),
			pods:   testutil.PodList(testutil.Pod("foo", "pod1", kapi.PodSucceeded, registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000")),
			expectedImageDeletions: []string{"sha256:0000000000000000000000000000000000000000000000000000000000000000"},
			expectedBlobDeletions: []string{
				registryURL + "|" + testutil.Layer1,
				registryURL + "|" + testutil.Layer2,
				registryURL + "|" + testutil.Layer3,
				registryURL + "|" + testutil.Layer4,
				registryURL + "|" + testutil.Layer5,
			},
		},

		{
			name:          "pod phase succeeded - prune leave registry alone",
			pruneRegistry: newBool(false),
			images:        testutil.ImageList(testutil.Image("sha256:0000000000000000000000000000000000000000000000000000000000000000", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000")),
			pods:          testutil.PodList(testutil.Pod("foo", "pod1", kapi.PodSucceeded, registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000")),
			expectedImageDeletions: []string{"sha256:0000000000000000000000000000000000000000000000000000000000000000"},
			expectedBlobDeletions:  []string{},
		},

		{
			name:   "pod phase succeeded, pod less than min pruning age - don't prune",
			images: testutil.ImageList(testutil.Image("sha256:0000000000000000000000000000000000000000000000000000000000000000", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000")),
			pods:   testutil.PodList(testutil.AgedPod("foo", "pod1", kapi.PodSucceeded, 5, registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000")),
			expectedImageDeletions: []string{},
		},

		{
			name:   "pod phase succeeded, image less than min pruning age - don't prune",
			images: testutil.ImageList(testutil.AgedImage("sha256:0000000000000000000000000000000000000000000000000000000000000000", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000", 5)),
			pods:   testutil.PodList(testutil.Pod("foo", "pod1", kapi.PodSucceeded, registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000")),
			expectedImageDeletions: []string{},
		},

		{
			name:   "pod phase failed - prune",
			images: testutil.ImageList(testutil.Image("sha256:0000000000000000000000000000000000000000000000000000000000000000", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000")),
			pods: testutil.PodList(
				testutil.Pod("foo", "pod1", kapi.PodFailed, registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000"),
				testutil.Pod("foo", "pod2", kapi.PodFailed, registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000"),
				testutil.Pod("foo", "pod3", kapi.PodFailed, registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000"),
			),
			expectedImageDeletions: []string{"sha256:0000000000000000000000000000000000000000000000000000000000000000"},
			expectedBlobDeletions: []string{
				registryURL + "|" + testutil.Layer1,
				registryURL + "|" + testutil.Layer2,
				registryURL + "|" + testutil.Layer3,
				registryURL + "|" + testutil.Layer4,
				registryURL + "|" + testutil.Layer5,
			},
		},

		{
			name:   "pod phase unknown - prune",
			images: testutil.ImageList(testutil.Image("sha256:0000000000000000000000000000000000000000000000000000000000000000", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000")),
			pods: testutil.PodList(
				testutil.Pod("foo", "pod1", kapi.PodUnknown, registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000"),
				testutil.Pod("foo", "pod2", kapi.PodUnknown, registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000"),
				testutil.Pod("foo", "pod3", kapi.PodUnknown, registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000"),
			),
			expectedImageDeletions: []string{"sha256:0000000000000000000000000000000000000000000000000000000000000000"},
			expectedBlobDeletions: []string{
				registryURL + "|" + testutil.Layer1,
				registryURL + "|" + testutil.Layer2,
				registryURL + "|" + testutil.Layer3,
				registryURL + "|" + testutil.Layer4,
				registryURL + "|" + testutil.Layer5,
			},
		},

		{
			name:   "pod container image not parsable",
			images: testutil.ImageList(testutil.Image("sha256:0000000000000000000000000000000000000000000000000000000000000000", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000")),
			pods: testutil.PodList(
				testutil.Pod("foo", "pod1", kapi.PodRunning, "a/b/c/d/e"),
			),
			expectedImageDeletions: []string{"sha256:0000000000000000000000000000000000000000000000000000000000000000"},
			expectedBlobDeletions: []string{
				registryURL + "|" + testutil.Layer1,
				registryURL + "|" + testutil.Layer2,
				registryURL + "|" + testutil.Layer3,
				registryURL + "|" + testutil.Layer4,
				registryURL + "|" + testutil.Layer5,
			},
		},

		{
			name:   "pod container image doesn't have an id",
			images: testutil.ImageList(testutil.Image("sha256:0000000000000000000000000000000000000000000000000000000000000000", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000")),
			pods: testutil.PodList(
				testutil.Pod("foo", "pod1", kapi.PodRunning, "foo/bar:latest"),
			),
			expectedImageDeletions: []string{"sha256:0000000000000000000000000000000000000000000000000000000000000000"},
			expectedBlobDeletions: []string{
				registryURL + "|" + testutil.Layer1,
				registryURL + "|" + testutil.Layer2,
				registryURL + "|" + testutil.Layer3,
				registryURL + "|" + testutil.Layer4,
				registryURL + "|" + testutil.Layer5,
			},
		},

		{
			name:   "pod refers to image not in graph",
			images: testutil.ImageList(testutil.Image("sha256:0000000000000000000000000000000000000000000000000000000000000000", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000")),
			pods: testutil.PodList(
				testutil.Pod("foo", "pod1", kapi.PodRunning, registryHost+"/foo/bar@sha256:ABC0000000000000000000000000000000000000000000000000000000000002"),
			),
			expectedImageDeletions: []string{"sha256:0000000000000000000000000000000000000000000000000000000000000000"},
			expectedBlobDeletions: []string{
				registryURL + "|" + testutil.Layer1,
				registryURL + "|" + testutil.Layer2,
				registryURL + "|" + testutil.Layer3,
				registryURL + "|" + testutil.Layer4,
				registryURL + "|" + testutil.Layer5,
			},
		},

		{
			name:   "referenced by rc - don't prune",
			images: testutil.ImageList(testutil.Image("sha256:0000000000000000000000000000000000000000000000000000000000000000", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000")),
			rcs:    testutil.RCList(testutil.RC("foo", "rc1", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000")),
			expectedImageDeletions: []string{},
		},

		{
			name:   "referenced by dc - don't prune",
			images: testutil.ImageList(testutil.Image("sha256:0000000000000000000000000000000000000000000000000000000000000000", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000")),
			dcs:    testutil.DCList(testutil.DC("foo", "rc1", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000")),
			expectedImageDeletions: []string{},
		},

		{
			name: "referenced by daemonset - don't prune",
			images: testutil.ImageList(
				testutil.Image("sha256:0000000000000000000000000000000000000000000000000000000000000000", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000"),
				testutil.Image("sha256:0000000000000000000000000000000000000000000000000000000000000001", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000001"),
			),
			dss: testutil.DSList(testutil.DS("foo", "rc1", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000")),
			expectedImageDeletions: []string{"sha256:0000000000000000000000000000000000000000000000000000000000000001"},
		},

		{
			name: "referenced by replicaset - don't prune",
			images: testutil.ImageList(
				testutil.Image("sha256:0000000000000000000000000000000000000000000000000000000000000000", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000"),
				testutil.Image("sha256:0000000000000000000000000000000000000000000000000000000000000001", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000001"),
			),
			rss: testutil.RSList(testutil.RS("foo", "rc1", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000")),
			expectedImageDeletions: []string{"sha256:0000000000000000000000000000000000000000000000000000000000000001"},
		},

		{
			name: "referenced by upstream deployment - don't prune",
			images: testutil.ImageList(
				testutil.Image("sha256:0000000000000000000000000000000000000000000000000000000000000000", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000"),
				testutil.Image("sha256:0000000000000000000000000000000000000000000000000000000000000001", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000001"),
			),
			deployments:            testutil.DeploymentList(testutil.Deployment("foo", "rc1", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000")),
			expectedImageDeletions: []string{"sha256:0000000000000000000000000000000000000000000000000000000000000001"},
		},

		{
			name:   "referenced by bc - sti - ImageStreamImage - don't prune",
			images: testutil.ImageList(testutil.Image("sha256:0000000000000000000000000000000000000000000000000000000000000000", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000")),
			bcs:    testutil.BCList(testutil.BC("foo", "bc1", "source", "ImageStreamImage", "foo", "bar@sha256:0000000000000000000000000000000000000000000000000000000000000000")),
			expectedImageDeletions: []string{},
		},

		{
			name:   "referenced by bc - docker - ImageStreamImage - don't prune",
			images: testutil.ImageList(testutil.Image("sha256:0000000000000000000000000000000000000000000000000000000000000000", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000")),
			bcs:    testutil.BCList(testutil.BC("foo", "bc1", "docker", "ImageStreamImage", "foo", "bar@sha256:0000000000000000000000000000000000000000000000000000000000000000")),
			expectedImageDeletions: []string{},
		},

		{
			name:   "referenced by bc - custom - ImageStreamImage - don't prune",
			images: testutil.ImageList(testutil.Image("sha256:0000000000000000000000000000000000000000000000000000000000000000", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000")),
			bcs:    testutil.BCList(testutil.BC("foo", "bc1", "custom", "ImageStreamImage", "foo", "bar@sha256:0000000000000000000000000000000000000000000000000000000000000000")),
			expectedImageDeletions: []string{},
		},

		{
			name:   "referenced by bc - sti - DockerImage - don't prune",
			images: testutil.ImageList(testutil.Image("sha256:0000000000000000000000000000000000000000000000000000000000000000", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000")),
			bcs:    testutil.BCList(testutil.BC("foo", "bc1", "source", "DockerImage", "foo", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000")),
			expectedImageDeletions: []string{},
		},

		{
			name:   "referenced by bc - docker - DockerImage - don't prune",
			images: testutil.ImageList(testutil.Image("sha256:0000000000000000000000000000000000000000000000000000000000000000", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000")),
			bcs:    testutil.BCList(testutil.BC("foo", "bc1", "docker", "DockerImage", "foo", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000")),
			expectedImageDeletions: []string{},
		},

		{
			name:   "referenced by bc - custom - DockerImage - don't prune",
			images: testutil.ImageList(testutil.Image("sha256:0000000000000000000000000000000000000000000000000000000000000000", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000")),
			bcs:    testutil.BCList(testutil.BC("foo", "bc1", "custom", "DockerImage", "foo", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000")),
			expectedImageDeletions: []string{},
		},

		{
			name:                   "referenced by build - sti - ImageStreamImage - don't prune",
			images:                 testutil.ImageList(testutil.Image("sha256:0000000000000000000000000000000000000000000000000000000000000000", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000")),
			builds:                 testutil.BuildList(testutil.Build("foo", "build1", "source", "ImageStreamImage", "foo", "bar@sha256:0000000000000000000000000000000000000000000000000000000000000000")),
			expectedImageDeletions: []string{},
		},

		{
			name:                   "referenced by build - docker - ImageStreamImage - don't prune",
			images:                 testutil.ImageList(testutil.Image("sha256:0000000000000000000000000000000000000000000000000000000000000000", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000")),
			builds:                 testutil.BuildList(testutil.Build("foo", "build1", "docker", "ImageStreamImage", "foo", "bar@sha256:0000000000000000000000000000000000000000000000000000000000000000")),
			expectedImageDeletions: []string{},
		},

		{
			name:                   "referenced by build - custom - ImageStreamImage - don't prune",
			images:                 testutil.ImageList(testutil.Image("sha256:0000000000000000000000000000000000000000000000000000000000000000", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000")),
			builds:                 testutil.BuildList(testutil.Build("foo", "build1", "custom", "ImageStreamImage", "foo", "bar@sha256:0000000000000000000000000000000000000000000000000000000000000000")),
			expectedImageDeletions: []string{},
		},

		{
			name:                   "referenced by build - sti - DockerImage - don't prune",
			images:                 testutil.ImageList(testutil.Image("sha256:0000000000000000000000000000000000000000000000000000000000000000", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000")),
			builds:                 testutil.BuildList(testutil.Build("foo", "build1", "source", "DockerImage", "foo", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000")),
			expectedImageDeletions: []string{},
		},

		{
			name:                   "referenced by build - docker - DockerImage - don't prune",
			images:                 testutil.ImageList(testutil.Image("sha256:0000000000000000000000000000000000000000000000000000000000000000", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000")),
			builds:                 testutil.BuildList(testutil.Build("foo", "build1", "docker", "DockerImage", "foo", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000")),
			expectedImageDeletions: []string{},
		},

		{
			name:                   "referenced by build - custom - DockerImage - don't prune",
			images:                 testutil.ImageList(testutil.Image("sha256:0000000000000000000000000000000000000000000000000000000000000000", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000")),
			builds:                 testutil.BuildList(testutil.Build("foo", "build1", "custom", "DockerImage", "foo", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000")),
			expectedImageDeletions: []string{},
		},

		{
			name: "image stream - keep most recent n images",
			images: testutil.ImageList(
				testutil.UnmanagedImage("sha256:0000000000000000000000000000000000000000000000000000000000000000", "otherregistry/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000", false, "", ""),
				testutil.Image("sha256:0000000000000000000000000000000000000000000000000000000000000002", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000002"),
				testutil.Image("sha256:0000000000000000000000000000000000000000000000000000000000000003", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000003"),
				testutil.Image("sha256:0000000000000000000000000000000000000000000000000000000000000004", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000004"),
			),
			streams: testutil.StreamList(
				testutil.Stream(registryHost, "foo", "bar", testutil.Tags(
					testutil.Tag("latest",
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000000", "otherregistry/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000"),
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000002", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000002"),
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000003", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000003"),
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000004", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000004"),
					),
				)),
			),
			expectedImageDeletions: []string{"sha256:0000000000000000000000000000000000000000000000000000000000000004"},
			expectedStreamUpdates:  []string{"foo/bar|sha256:0000000000000000000000000000000000000000000000000000000000000004"},
		},

		{
			name: "image stream - same manifest listed multiple times in tag history",
			images: testutil.ImageList(
				testutil.Image("sha256:0000000000000000000000000000000000000000000000000000000000000001", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000001"),
				testutil.Image("sha256:0000000000000000000000000000000000000000000000000000000000000002", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000002"),
			),
			streams: testutil.StreamList(
				testutil.Stream(registryHost, "foo", "bar", testutil.Tags(
					testutil.Tag("latest",
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000001", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000001"),
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000002", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000002"),
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000001", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000001"),
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000002", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000002"),
					),
				)),
			),
		},

		{
			name: "image stream age less than min pruning age - don't prune",
			images: testutil.ImageList(
				testutil.Image("sha256:0000000000000000000000000000000000000000000000000000000000000000", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000"),
				testutil.Image("sha256:0000000000000000000000000000000000000000000000000000000000000002", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000002"),
				testutil.Image("sha256:0000000000000000000000000000000000000000000000000000000000000003", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000003"),
				testutil.Image("sha256:0000000000000000000000000000000000000000000000000000000000000004", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000004"),
			),
			streams: testutil.StreamList(
				testutil.AgedStream(registryHost, "foo", "bar", 5, testutil.Tags(
					testutil.Tag("latest",
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000000", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000"),
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000002", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000002"),
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000003", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000003"),
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000004", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000004"),
					),
				)),
			),
			expectedImageDeletions: []string{},
			expectedStreamUpdates:  []string{},
		},

		{
			name: "image stream - unreference absent image",
			images: testutil.ImageList(
				testutil.Image("sha256:0000000000000000000000000000000000000000000000000000000000000001", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000001"),
			),
			streams: testutil.StreamList(
				testutil.Stream(registryHost, "foo", "bar", testutil.Tags(
					testutil.Tag("latest",
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000000", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000"),
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000001", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000001"),
					),
				)),
			),
			expectedStreamUpdates: []string{"foo/bar|sha256:0000000000000000000000000000000000000000000000000000000000000000"},
		},

		{
			name: "image stream with dangling references - delete tags",
			images: testutil.ImageList(
				testutil.ImageWithLayers("sha256:0000000000000000000000000000000000000000000000000000000000000001", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000001", nil, "layer1"),
			),
			streams: testutil.StreamList(
				testutil.Stream(registryHost, "foo", "bar", testutil.Tags(
					testutil.Tag("latest",
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000000", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000"),
					),
					testutil.Tag("tag",
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000002", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000002"),
					),
				)),
			),
			expectedImageDeletions: []string{"sha256:0000000000000000000000000000000000000000000000000000000000000001"},
			expectedStreamUpdates: []string{
				"foo/bar:latest",
				"foo/bar:tag",
				"foo/bar|sha256:0000000000000000000000000000000000000000000000000000000000000000",
				"foo/bar|sha256:0000000000000000000000000000000000000000000000000000000000000002",
			},
			expectedBlobDeletions: []string{registryURL + "|layer1"},
		},

		{
			name: "image stream - keep reference to a young absent image",
			images: testutil.ImageList(
				testutil.Image("sha256:0000000000000000000000000000000000000000000000000000000000000001", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000001"),
				testutil.ImageWithLayers("sha256:0000000000000000000000000000000000000000000000000000000000000002", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000002", nil),
			),
			streams: testutil.StreamList(
				testutil.Stream(registryHost, "foo", "bar", testutil.Tags(
					testutil.Tag("latest",
						testutil.YoungTagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000000", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000", metav1.Now()),
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000001", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000001"),
					),
				)),
			),
			expectedImageDeletions: []string{"sha256:0000000000000000000000000000000000000000000000000000000000000002"},
		},

		{
			name:             "images referenced by istag - keep",
			keepTagRevisions: keepTagRevisions(0),
			images: testutil.ImageList(
				testutil.Image("sha256:0000000000000000000000000000000000000000000000000000000000000001", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000001"),
				testutil.Image("sha256:0000000000000000000000000000000000000000000000000000000000000002", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000002"),
				testutil.Image("sha256:0000000000000000000000000000000000000000000000000000000000000003", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000003"),
				testutil.Image("sha256:0000000000000000000000000000000000000000000000000000000000000004", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000004"),
				testutil.Image("sha256:0000000000000000000000000000000000000000000000000000000000000005", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000005"),
				testutil.Image("sha256:0000000000000000000000000000000000000000000000000000000000000006", registryHost+"/foo/baz@sha256:0000000000000000000000000000000000000000000000000000000000000006"),
			),
			streams: testutil.StreamList(
				testutil.Stream(registryHost, "foo", "bar", testutil.Tags(
					testutil.Tag("latest",
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000000", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000"),
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000001", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000001"),
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000002", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000002"),
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000003", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000003"),
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000004", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000004"),
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000005", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000005"),
					),
					testutil.Tag("dummy", // removed because no object references the image (the nm/dcfoo has mismatched repository name)
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000005", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000005"),
					),
				)),
				testutil.Stream(registryHost, "foo", "baz", testutil.Tags(
					testutil.Tag("late", // kept because replicaset references the tagged image
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000002", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000002"),
					),
					testutil.Tag("keepme", // kept because a deployment references the tagged image
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000006", registryHost+"/foo/baz@sha256:0000000000000000000000000000000000000000000000000000000000000006"),
					),
				)),
			),
			dss: testutil.DSList(testutil.DS("nm", "dsfoo", fmt.Sprintf("%s/%s/%s:%s", registryHost, "foo", "bar", "latest"))),
			dcs: testutil.DCList(testutil.DC("nm", "dcfoo", fmt.Sprintf("%s/%s/%s:%s", registryHost, "foo", "repo", "dummy"))),
			rss: testutil.RSList(testutil.RS("nm", "rsfoo", fmt.Sprintf("%s/%s/%s:%s", registryHost, "foo", "baz", "late"))),
			// ignore different registry hostname
			deployments: testutil.DeploymentList(testutil.Deployment("nm", "depfoo", fmt.Sprintf("%s/%s/%s:%s", "external.registry:5000", "foo", "baz", "keepme"))),
			expectedImageDeletions: []string{
				"sha256:0000000000000000000000000000000000000000000000000000000000000001",
				"sha256:0000000000000000000000000000000000000000000000000000000000000003",
				"sha256:0000000000000000000000000000000000000000000000000000000000000004",
				"sha256:0000000000000000000000000000000000000000000000000000000000000005",
			},
			expectedStreamUpdates: []string{
				"foo/bar:dummy",
				"foo/bar|sha256:0000000000000000000000000000000000000000000000000000000000000000",
				"foo/bar|sha256:0000000000000000000000000000000000000000000000000000000000000001",
				"foo/bar|sha256:0000000000000000000000000000000000000000000000000000000000000003",
				"foo/bar|sha256:0000000000000000000000000000000000000000000000000000000000000004",
				"foo/bar|sha256:0000000000000000000000000000000000000000000000000000000000000005",
			},
		},

		{
			name: "multiple resources pointing to image - don't prune",
			images: testutil.ImageList(
				testutil.Image("sha256:0000000000000000000000000000000000000000000000000000000000000000", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000"),
				testutil.Image("sha256:0000000000000000000000000000000000000000000000000000000000000002", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000002"),
			),
			streams: testutil.StreamList(
				testutil.Stream(registryHost, "foo", "bar", testutil.Tags(
					testutil.Tag("latest",
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000000", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000"),
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000002", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000002"),
					),
				)),
			),
			rcs:                    testutil.RCList(testutil.RC("foo", "rc1", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000002")),
			pods:                   testutil.PodList(testutil.Pod("foo", "pod1", kapi.PodRunning, registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000002")),
			dcs:                    testutil.DCList(testutil.DC("foo", "rc1", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000")),
			bcs:                    testutil.BCList(testutil.BC("foo", "bc1", "source", "DockerImage", "foo", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000")),
			builds:                 testutil.BuildList(testutil.Build("foo", "build1", "custom", "ImageStreamImage", "foo", "bar@sha256:0000000000000000000000000000000000000000000000000000000000000000")),
			expectedImageDeletions: []string{},
			expectedStreamUpdates:  []string{},
		},

		{
			name: "image with nil annotations",
			images: testutil.ImageList(
				testutil.UnmanagedImage("sha256:0000000000000000000000000000000000000000000000000000000000000000", "someregistry/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000", false, "", ""),
			),
			expectedImageDeletions: []string{"sha256:0000000000000000000000000000000000000000000000000000000000000000"},
			expectedStreamUpdates:  []string{},
		},

		{
			name:      "prune all-images=true image with nil annotations",
			allImages: newBool(true),
			images: testutil.ImageList(
				testutil.UnmanagedImage("sha256:0000000000000000000000000000000000000000000000000000000000000000", "someregistry/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000", false, "", ""),
			),
			expectedImageDeletions: []string{"sha256:0000000000000000000000000000000000000000000000000000000000000000"},
			expectedStreamUpdates:  []string{},
		},

		{
			name:      "prune all-images=false image with nil annotations",
			allImages: newBool(false),
			images: testutil.ImageList(
				testutil.UnmanagedImage("sha256:0000000000000000000000000000000000000000000000000000000000000000", "someregistry/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000", false, "", ""),
			),
			expectedImageDeletions: []string{},
			expectedStreamUpdates:  []string{},
		},

		{
			name: "image missing managed annotation",
			images: testutil.ImageList(
				testutil.UnmanagedImage("sha256:0000000000000000000000000000000000000000000000000000000000000000", "someregistry/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000", true, "foo", "bar"),
			),
			expectedImageDeletions: []string{"sha256:0000000000000000000000000000000000000000000000000000000000000000"},
			expectedStreamUpdates:  []string{},
		},

		{
			name: "image with managed annotation != true",
			images: testutil.ImageList(
				testutil.UnmanagedImage("sha256:0000000000000000000000000000000000000000000000000000000000000000", "someregistry/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000", true, imageapi.ManagedByOpenShiftAnnotation, "false"),
				testutil.UnmanagedImage("sha256:0000000000000000000000000000000000000000000000000000000000000001", "someregistry/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000", true, imageapi.ManagedByOpenShiftAnnotation, "0"),
				testutil.UnmanagedImage("sha256:0000000000000000000000000000000000000000000000000000000000000002", "someregistry/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000", true, imageapi.ManagedByOpenShiftAnnotation, "1"),
				testutil.UnmanagedImage("sha256:0000000000000000000000000000000000000000000000000000000000000003", "someregistry/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000", true, imageapi.ManagedByOpenShiftAnnotation, "True"),
				testutil.UnmanagedImage("sha256:0000000000000000000000000000000000000000000000000000000000000004", "someregistry/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000", true, imageapi.ManagedByOpenShiftAnnotation, "yes"),
				testutil.UnmanagedImage("sha256:0000000000000000000000000000000000000000000000000000000000000005", "someregistry/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000", true, imageapi.ManagedByOpenShiftAnnotation, "Yes"),
			),
			expectedImageDeletions: []string{
				"sha256:0000000000000000000000000000000000000000000000000000000000000000",
				"sha256:0000000000000000000000000000000000000000000000000000000000000001",
				"sha256:0000000000000000000000000000000000000000000000000000000000000002",
				"sha256:0000000000000000000000000000000000000000000000000000000000000003",
				"sha256:0000000000000000000000000000000000000000000000000000000000000004",
				"sha256:0000000000000000000000000000000000000000000000000000000000000005",
			},
			expectedStreamUpdates: []string{},
		},

		{
			name:      "prune all-images=true with image missing managed annotation",
			allImages: newBool(true),
			images: testutil.ImageList(
				testutil.UnmanagedImage("sha256:0000000000000000000000000000000000000000000000000000000000000000", "someregistry/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000", true, "foo", "bar"),
			),
			expectedImageDeletions: []string{"sha256:0000000000000000000000000000000000000000000000000000000000000000"},
			expectedStreamUpdates:  []string{},
		},

		{
			name:      "prune all-images=true with image with managed annotation != true",
			allImages: newBool(true),
			images: testutil.ImageList(
				testutil.UnmanagedImage("sha256:0000000000000000000000000000000000000000000000000000000000000000", "someregistry/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000", true, imageapi.ManagedByOpenShiftAnnotation, "false"),
				testutil.UnmanagedImage("sha256:0000000000000000000000000000000000000000000000000000000000000001", "someregistry/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000", true, imageapi.ManagedByOpenShiftAnnotation, "0"),
				testutil.UnmanagedImage("sha256:0000000000000000000000000000000000000000000000000000000000000002", "someregistry/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000", true, imageapi.ManagedByOpenShiftAnnotation, "1"),
				testutil.UnmanagedImage("sha256:0000000000000000000000000000000000000000000000000000000000000003", "someregistry/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000", true, imageapi.ManagedByOpenShiftAnnotation, "True"),
				testutil.UnmanagedImage("sha256:0000000000000000000000000000000000000000000000000000000000000004", "someregistry/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000", true, imageapi.ManagedByOpenShiftAnnotation, "yes"),
				testutil.UnmanagedImage("sha256:0000000000000000000000000000000000000000000000000000000000000005", "someregistry/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000", true, imageapi.ManagedByOpenShiftAnnotation, "Yes"),
			),
			expectedImageDeletions: []string{
				"sha256:0000000000000000000000000000000000000000000000000000000000000000",
				"sha256:0000000000000000000000000000000000000000000000000000000000000001",
				"sha256:0000000000000000000000000000000000000000000000000000000000000002",
				"sha256:0000000000000000000000000000000000000000000000000000000000000003",
				"sha256:0000000000000000000000000000000000000000000000000000000000000004",
				"sha256:0000000000000000000000000000000000000000000000000000000000000005",
			},
			expectedStreamUpdates: []string{},
		},

		{
			name:      "prune all-images=false with image missing managed annotation",
			allImages: newBool(false),
			images: testutil.ImageList(
				testutil.UnmanagedImage("sha256:0000000000000000000000000000000000000000000000000000000000000000", "someregistry/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000", true, "foo", "bar"),
			),
			expectedImageDeletions: []string{},
			expectedStreamUpdates:  []string{},
		},

		{
			name:      "prune all-images=false with image with managed annotation != true",
			allImages: newBool(false),
			images: testutil.ImageList(
				testutil.UnmanagedImage("sha256:0000000000000000000000000000000000000000000000000000000000000000", "someregistry/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000", true, imageapi.ManagedByOpenShiftAnnotation, "false"),
				testutil.UnmanagedImage("sha256:0000000000000000000000000000000000000000000000000000000000000001", "someregistry/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000", true, imageapi.ManagedByOpenShiftAnnotation, "0"),
				testutil.UnmanagedImage("sha256:0000000000000000000000000000000000000000000000000000000000000002", "someregistry/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000", true, imageapi.ManagedByOpenShiftAnnotation, "1"),
				testutil.UnmanagedImage("sha256:0000000000000000000000000000000000000000000000000000000000000003", "someregistry/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000", true, imageapi.ManagedByOpenShiftAnnotation, "True"),
				testutil.UnmanagedImage("sha256:0000000000000000000000000000000000000000000000000000000000000004", "someregistry/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000", true, imageapi.ManagedByOpenShiftAnnotation, "yes"),
				testutil.UnmanagedImage("sha256:0000000000000000000000000000000000000000000000000000000000000005", "someregistry/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000", true, imageapi.ManagedByOpenShiftAnnotation, "Yes"),
			),
			expectedImageDeletions: []string{},
			expectedStreamUpdates:  []string{},
		},

		{
			name: "image with layers",
			images: testutil.ImageList(
				testutil.ImageWithLayers("sha256:0000000000000000000000000000000000000000000000000000000000000001", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000001", &testutil.Config1, "layer1", "layer2", "layer3", "layer4"),
				testutil.ImageWithLayers("sha256:0000000000000000000000000000000000000000000000000000000000000002", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000002", &testutil.Config2, "layer1", "layer2", "layer3", "layer4"),
				testutil.ImageWithLayers("sha256:0000000000000000000000000000000000000000000000000000000000000003", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000003", nil, "layer1", "layer2", "layer3", "layer4"),
				testutil.ImageWithLayers("sha256:0000000000000000000000000000000000000000000000000000000000000004", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000004", nil, "layer5", "layer6", "layer7", "layer8"),
			),
			streams: testutil.StreamList(
				testutil.Stream(registryHost, "foo", "bar", testutil.Tags(
					testutil.Tag("latest",
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000001", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000001"),
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000002", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000002"),
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000003", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000003"),
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000004", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000004"),
					),
				)),
			),
			expectedImageDeletions: []string{"sha256:0000000000000000000000000000000000000000000000000000000000000004"},
			expectedStreamUpdates:  []string{"foo/bar|sha256:0000000000000000000000000000000000000000000000000000000000000004"},
			expectedLayerLinkDeletions: []string{
				registryURL + "|foo/bar|layer5",
				registryURL + "|foo/bar|layer6",
				registryURL + "|foo/bar|layer7",
				registryURL + "|foo/bar|layer8",
			},
			expectedBlobDeletions: []string{
				registryURL + "|layer5",
				registryURL + "|layer6",
				registryURL + "|layer7",
				registryURL + "|layer8",
			},
		},

		{
			name: "images with duplicate layers and configs",
			images: testutil.ImageList(
				testutil.ImageWithLayers("sha256:0000000000000000000000000000000000000000000000000000000000000001", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000001", &testutil.Config1, "layer1", "layer2", "layer3", "layer4"),
				testutil.ImageWithLayers("sha256:0000000000000000000000000000000000000000000000000000000000000002", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000002", &testutil.Config1, "layer1", "layer2", "layer3", "layer4"),
				testutil.ImageWithLayers("sha256:0000000000000000000000000000000000000000000000000000000000000003", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000003", &testutil.Config1, "layer1", "layer2", "layer3", "layer4"),
				testutil.ImageWithLayers("sha256:0000000000000000000000000000000000000000000000000000000000000004", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000004", &testutil.Config2, "layer5", "layer6", "layer7", "layer8"),
				testutil.ImageWithLayers("sha256:0000000000000000000000000000000000000000000000000000000000000005", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000005", &testutil.Config2, "layer5", "layer6", "layer9", "layerX"),
			),
			streams: testutil.StreamList(
				testutil.Stream(registryHost, "foo", "bar", testutil.Tags(
					testutil.Tag("latest",
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000001", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000001"),
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000002", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000002"),
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000003", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000003"),
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000004", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000004"),
					),
				)),
			),
			expectedImageDeletions: []string{"sha256:0000000000000000000000000000000000000000000000000000000000000004", "sha256:0000000000000000000000000000000000000000000000000000000000000005"},
			expectedStreamUpdates:  []string{"foo/bar|sha256:0000000000000000000000000000000000000000000000000000000000000004"},
			expectedLayerLinkDeletions: []string{
				registryURL + "|foo/bar|" + testutil.Config2,
				registryURL + "|foo/bar|layer5",
				registryURL + "|foo/bar|layer6",
				registryURL + "|foo/bar|layer7",
				registryURL + "|foo/bar|layer8",
			},
			expectedBlobDeletions: []string{
				registryURL + "|" + testutil.Config2,
				registryURL + "|layer5",
				registryURL + "|layer6",
				registryURL + "|layer7",
				registryURL + "|layer8",
				registryURL + "|layer9",
				registryURL + "|layerX",
			},
		},

		{
			name: "layers shared with young images are not pruned",
			images: testutil.ImageList(
				testutil.AgedImage("sha256:0000000000000000000000000000000000000000000000000000000000000001", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000001", 43200),
				testutil.AgedImage("sha256:0000000000000000000000000000000000000000000000000000000000000002", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000002", 5),
			),
			expectedImageDeletions: []string{"sha256:0000000000000000000000000000000000000000000000000000000000000001"},
		},

		{
			name:               "image exceeding limits",
			pruneOverSizeLimit: newBool(true),
			images: testutil.ImageList(
				testutil.UnmanagedImage("sha256:0000000000000000000000000000000000000000000000000000000000000000", "otherregistry/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000", false, "", ""),
				testutil.SizedImage("sha256:0000000000000000000000000000000000000000000000000000000000000002", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000002", 100, nil),
				testutil.SizedImage("sha256:0000000000000000000000000000000000000000000000000000000000000003", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000003", 200, nil),
			),
			streams: testutil.StreamList(
				testutil.Stream(registryHost, "foo", "bar", testutil.Tags(
					testutil.Tag("latest",
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000000", "otherregistry/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000"),
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000002", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000002"),
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000003", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000003"),
					),
				)),
			),
			limits: map[string][]*kapi.LimitRange{
				"foo": testutil.LimitList(100, 200),
			},
			expectedImageDeletions: []string{"sha256:0000000000000000000000000000000000000000000000000000000000000003"},
			expectedStreamUpdates:  []string{"foo/bar|sha256:0000000000000000000000000000000000000000000000000000000000000003"},
		},

		{
			name:               "multiple images in different namespaces exceeding different limits",
			pruneOverSizeLimit: newBool(true),
			images: testutil.ImageList(
				testutil.SizedImage("sha256:0000000000000000000000000000000000000000000000000000000000000001", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000001", 100, nil),
				testutil.SizedImage("sha256:0000000000000000000000000000000000000000000000000000000000000002", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000002", 200, nil),
				testutil.SizedImage("sha256:0000000000000000000000000000000000000000000000000000000000000003", registryHost+"/bar/foo@sha256:0000000000000000000000000000000000000000000000000000000000000003", 500, nil),
				testutil.SizedImage("sha256:0000000000000000000000000000000000000000000000000000000000000004", registryHost+"/bar/foo@sha256:0000000000000000000000000000000000000000000000000000000000000004", 600, nil),
			),
			streams: testutil.StreamList(
				testutil.Stream(registryHost, "foo", "bar", testutil.Tags(
					testutil.Tag("latest",
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000001", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000001"),
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000002", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000002"),
					),
				)),
				testutil.Stream(registryHost, "bar", "foo", testutil.Tags(
					testutil.Tag("latest",
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000003", registryHost+"/bar/foo@sha256:0000000000000000000000000000000000000000000000000000000000000003"),
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000004", registryHost+"/bar/foo@sha256:0000000000000000000000000000000000000000000000000000000000000004"),
					),
				)),
			),
			limits: map[string][]*kapi.LimitRange{
				"foo": testutil.LimitList(150),
				"bar": testutil.LimitList(550),
			},
			expectedImageDeletions: []string{"sha256:0000000000000000000000000000000000000000000000000000000000000002", "sha256:0000000000000000000000000000000000000000000000000000000000000004"},
			expectedStreamUpdates:  []string{"foo/bar|sha256:0000000000000000000000000000000000000000000000000000000000000002", "bar/foo|sha256:0000000000000000000000000000000000000000000000000000000000000004"},
		},

		{
			name:               "image within allowed limits",
			pruneOverSizeLimit: newBool(true),
			images: testutil.ImageList(
				testutil.UnmanagedImage("sha256:0000000000000000000000000000000000000000000000000000000000000000", "otherregistry/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000", false, "", ""),
				testutil.SizedImage("sha256:0000000000000000000000000000000000000000000000000000000000000002", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000002", 100, nil),
				testutil.SizedImage("sha256:0000000000000000000000000000000000000000000000000000000000000003", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000003", 200, nil),
			),
			streams: testutil.StreamList(
				testutil.Stream(registryHost, "foo", "bar", testutil.Tags(
					testutil.Tag("latest",
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000000", "otherregistry/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000"),
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000002", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000002"),
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000003", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000003"),
					),
				)),
			),
			limits: map[string][]*kapi.LimitRange{
				"foo": testutil.LimitList(300),
			},
			expectedImageDeletions: []string{},
			expectedStreamUpdates:  []string{},
		},

		{
			name:               "image exceeding limits with namespace specified",
			pruneOverSizeLimit: newBool(true),
			namespace:          "foo",
			images: testutil.ImageList(
				testutil.UnmanagedImage("sha256:0000000000000000000000000000000000000000000000000000000000000000", "otherregistry/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000", false, "", ""),
				testutil.SizedImage("sha256:0000000000000000000000000000000000000000000000000000000000000002", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000002", 100, nil),
				testutil.SizedImage("sha256:0000000000000000000000000000000000000000000000000000000000000003", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000003", 200, nil),
			),
			streams: testutil.StreamList(
				testutil.Stream(registryHost, "foo", "bar", testutil.Tags(
					testutil.Tag("latest",
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000000", "otherregistry/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000"),
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000002", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000002"),
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000003", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000003"),
					),
				)),
			),
			limits: map[string][]*kapi.LimitRange{
				"foo": testutil.LimitList(100, 200),
			},
			expectedStreamUpdates: []string{"foo/bar|sha256:0000000000000000000000000000000000000000000000000000000000000003"},
		},

		{
			name:                "build with bad image reference",
			builds:              testutil.BuildList(testutil.Build("foo", "build1", "source", "DockerImage", "foo", registryHost+"/foo/bar@invalid-digest")),
			expectedErrorString: fmt.Sprintf(`Build[foo/build1]: invalid docker image reference "%s/foo/bar@invalid-digest": invalid reference format`, registryHost),
		},

		{
			name: "buildconfig with bad imagestreamtag",
			bcs:  testutil.BCList(testutil.BC("foo", "bc1", "source", "ImageStreamTag", "ns", "bad/tag@name")),
			expectedErrorString: `BuildConfig[foo/bc1]: invalid ImageStreamTag reference "bad/tag@name":` +
				` "bad/tag@name" is an image stream image, not an image stream tag`,
		},

		{
			name:        "more parsing errors",
			bcs:         testutil.BCList(testutil.BC("foo", "bc1", "source", "ImageStreamImage", "ns", "bad:isi")),
			deployments: testutil.DeploymentList(testutil.Deployment("nm", "dep1", "garbage")),
			rss:         testutil.RSList(testutil.RS("nm", "rs1", "I am certainly a valid reference")),
			expectedErrorString: `[BuildConfig[foo/bc1]: invalid ImageStreamImage reference "bad:isi":` +
				` expected exactly one @ in the isimage name "bad:isi",` +
				` ReplicaSet[nm/rs1]: invalid docker image reference "I am certainly a valid reference":` +
				` invalid reference format]`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			options := PrunerOptions{
				Namespace:   test.namespace,
				AllImages:   test.allImages,
				Images:      &test.images,
				Streams:     &test.streams,
				Pods:        &test.pods,
				RCs:         &test.rcs,
				BCs:         &test.bcs,
				Builds:      &test.builds,
				DSs:         &test.dss,
				Deployments: &test.deployments,
				DCs:         &test.dcs,
				RSs:         &test.rss,
				LimitRanges: test.limits,
				RegistryURL: &url.URL{Scheme: "https", Host: registryHost},
			}
			if test.pruneOverSizeLimit != nil {
				options.PruneOverSizeLimit = test.pruneOverSizeLimit
			} else {
				youngerThan := time.Hour
				tagRevisions := 3
				if test.keepTagRevisions != nil {
					tagRevisions = *test.keepTagRevisions
				}
				options.KeepYoungerThan = &youngerThan
				options.KeepTagRevisions = &tagRevisions
			}
			if test.pruneRegistry != nil {
				options.PruneRegistry = test.pruneRegistry
			}
			p, err := NewPruner(options)
			if err != nil {
				if len(test.expectedErrorString) > 0 {
					if a, e := err.Error(), test.expectedErrorString; a != e {
						t.Fatalf("got unexpected error: %q != %q", a, e)
					}
				} else {
					t.Fatalf("got unexpected error: %v", test.expectedErrorString)
				}
				return
			} else if len(test.expectedErrorString) > 0 {
				t.Fatalf("got no error while expecting: %s", test.expectedErrorString)
				return
			}

			imageDeleter := &fakeImageDeleter{invocations: sets.NewString()}
			streamDeleter := &fakeImageStreamDeleter{invocations: sets.NewString()}
			layerLinkDeleter := &fakeLayerLinkDeleter{invocations: sets.NewString()}
			blobDeleter := &fakeBlobDeleter{invocations: sets.NewString()}
			manifestDeleter := &fakeManifestDeleter{invocations: sets.NewString()}

			if err := p.Prune(imageDeleter, streamDeleter, layerLinkDeleter, blobDeleter, manifestDeleter); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			expectedImageDeletions := sets.NewString(test.expectedImageDeletions...)
			if a, e := imageDeleter.invocations, expectedImageDeletions; !reflect.DeepEqual(a, e) {
				t.Errorf("unexpected image deletions: %s", diff.ObjectDiff(a, e))
			}

			expectedStreamUpdates := sets.NewString(test.expectedStreamUpdates...)
			if a, e := streamDeleter.invocations, expectedStreamUpdates; !reflect.DeepEqual(a, e) {
				t.Errorf("unexpected stream updates: %s", diff.ObjectDiff(a, e))
			}

			expectedLayerLinkDeletions := sets.NewString(test.expectedLayerLinkDeletions...)
			if a, e := layerLinkDeleter.invocations, expectedLayerLinkDeletions; !reflect.DeepEqual(a, e) {
				t.Errorf("unexpected layer link deletions: %s", diff.ObjectDiff(a, e))
			}

			expectedBlobDeletions := sets.NewString(test.expectedBlobDeletions...)
			if a, e := blobDeleter.invocations, expectedBlobDeletions; !reflect.DeepEqual(a, e) {
				t.Errorf("unexpected blob deletions: %s", diff.ObjectDiff(a, e))
			}
		})
	}
}

func TestImageDeleter(t *testing.T) {
	flag.Lookup("v").Value.Set(fmt.Sprint(*logLevel))

	tests := map[string]struct {
		imageDeletionError error
	}{
		"no error": {},
		"delete error": {
			imageDeletionError: fmt.Errorf("foo"),
		},
	}

	for name, test := range tests {
		imageClient := imageclient.Clientset{}
		imageClient.AddReactor("delete", "images", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
			return true, nil, test.imageDeletionError
		})
		imageDeleter := NewImageDeleter(imageClient.Image())
		err := imageDeleter.DeleteImage(&imageapi.Image{ObjectMeta: metav1.ObjectMeta{Name: "sha256:0000000000000000000000000000000000000000000000000000000000000002"}})
		if test.imageDeletionError != nil {
			if e, a := test.imageDeletionError, err; e != a {
				t.Errorf("%s: err: expected %v, got %v", name, e, a)
			}
			continue
		}

		if e, a := 1, len(imageClient.Actions()); e != a {
			t.Errorf("%s: expected %d actions, got %d: %#v", name, e, a, imageClient.Actions())
			continue
		}

		if !imageClient.Actions()[0].Matches("delete", "images") {
			t.Errorf("%s: expected action %s, got %v", name, "delete-images", imageClient.Actions()[0])
		}
	}
}

func TestLayerDeleter(t *testing.T) {
	flag.Lookup("v").Value.Set(fmt.Sprint(*logLevel))

	var actions []string
	client := fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
		actions = append(actions, req.Method+":"+req.URL.String())
		return &http.Response{StatusCode: http.StatusServiceUnavailable, Body: ioutil.NopCloser(bytes.NewReader([]byte{}))}, nil
	})
	layerLinkDeleter := NewLayerLinkDeleter()
	layerLinkDeleter.DeleteLayerLink(client, &url.URL{Scheme: "http", Host: "registry1"}, "repo", "layer1")

	if e := []string{"DELETE:http://registry1/v2/repo/blobs/layer1"}; !reflect.DeepEqual(actions, e) {
		t.Errorf("unexpected actions: %s", diff.ObjectDiff(actions, e))
	}
}

func TestNotFoundLayerDeleter(t *testing.T) {
	flag.Lookup("v").Value.Set(fmt.Sprint(*logLevel))

	var actions []string
	client := fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
		actions = append(actions, req.Method+":"+req.URL.String())
		return &http.Response{StatusCode: http.StatusNotFound, Body: ioutil.NopCloser(bytes.NewReader([]byte{}))}, nil
	})
	layerLinkDeleter := NewLayerLinkDeleter()
	layerLinkDeleter.DeleteLayerLink(client, &url.URL{Scheme: "https", Host: "registry1"}, "repo", "layer1")

	if e := []string{"DELETE:https://registry1/v2/repo/blobs/layer1"}; !reflect.DeepEqual(actions, e) {
		t.Errorf("unexpected actions: %s", diff.ObjectDiff(actions, e))
	}
}

func TestRegistryPruning(t *testing.T) {
	flag.Lookup("v").Value.Set(fmt.Sprint(*logLevel))

	tests := []struct {
		name                       string
		images                     imageapi.ImageList
		streams                    imageapi.ImageStreamList
		expectedLayerLinkDeletions sets.String
		expectedBlobDeletions      sets.String
		expectedManifestDeletions  sets.String
		pruneRegistry              bool
		pingErr                    error
	}{
		{
			name:          "layers unique to id1 pruned",
			pruneRegistry: true,
			images: testutil.ImageList(
				testutil.ImageWithLayers("sha256:0000000000000000000000000000000000000000000000000000000000000001", "registry1.io/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000001", &testutil.Config1, "layer1", "layer2", "layer3", "layer4"),
				testutil.ImageWithLayers("sha256:0000000000000000000000000000000000000000000000000000000000000002", "registry1.io/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000002", &testutil.Config2, "layer3", "layer4", "layer5", "layer6"),
			),
			streams: testutil.StreamList(
				testutil.Stream("registry1.io", "foo", "bar", testutil.Tags(
					testutil.Tag("latest",
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000002", "registry1.io/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000002"),
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000001", "registry1.io/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000001"),
					),
				)),
				testutil.Stream("registry1.io", "foo", "other", testutil.Tags(
					testutil.Tag("latest",
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000002", "registry1.io/foo/other@sha256:0000000000000000000000000000000000000000000000000000000000000002"),
					),
				)),
			),
			expectedLayerLinkDeletions: sets.NewString(
				"https://registry1.io|foo/bar|"+testutil.Config1,
				"https://registry1.io|foo/bar|layer1",
				"https://registry1.io|foo/bar|layer2",
			),
			expectedBlobDeletions: sets.NewString(
				"https://registry1.io|"+testutil.Config1,
				"https://registry1.io|layer1",
				"https://registry1.io|layer2",
			),
			expectedManifestDeletions: sets.NewString(
				"https://registry1.io|foo/bar|sha256:0000000000000000000000000000000000000000000000000000000000000001",
			),
		},

		{
			name:          "no pruning when no images are pruned",
			pruneRegistry: true,
			images: testutil.ImageList(
				testutil.ImageWithLayers("sha256:0000000000000000000000000000000000000000000000000000000000000001", "registry1.io/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000001", &testutil.Config1, "layer1", "layer2", "layer3", "layer4"),
			),
			streams: testutil.StreamList(
				testutil.Stream("registry1.io", "foo", "bar", testutil.Tags(
					testutil.Tag("latest",
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000001", "registry1.io/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000001"),
					),
				)),
			),
			expectedLayerLinkDeletions: sets.NewString(),
			expectedBlobDeletions:      sets.NewString(),
			expectedManifestDeletions:  sets.NewString(),
		},

		{
			name:          "blobs pruned when streams have already been deleted",
			pruneRegistry: true,
			images: testutil.ImageList(
				testutil.ImageWithLayers("sha256:0000000000000000000000000000000000000000000000000000000000000001", "registry1.io/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000001", &testutil.Config1, "layer1", "layer2", "layer3", "layer4"),
				testutil.ImageWithLayers("sha256:0000000000000000000000000000000000000000000000000000000000000002", "registry1.io/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000002", &testutil.Config2, "layer3", "layer4", "layer5", "layer6"),
			),
			expectedLayerLinkDeletions: sets.NewString(),
			expectedBlobDeletions: sets.NewString(
				"https://registry1.io|"+testutil.Config1,
				"https://registry1.io|"+testutil.Config2,
				"https://registry1.io|layer1",
				"https://registry1.io|layer2",
				"https://registry1.io|layer3",
				"https://registry1.io|layer4",
				"https://registry1.io|layer5",
				"https://registry1.io|layer6",
			),
			expectedManifestDeletions: sets.NewString(),
		},

		{
			name:          "config used as a layer",
			pruneRegistry: true,
			images: testutil.ImageList(
				testutil.ImageWithLayers("sha256:0000000000000000000000000000000000000000000000000000000000000001", "registry1.io/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000001", &testutil.Config1, "layer1", "layer2", "layer3", testutil.Config1),
				testutil.ImageWithLayers("sha256:0000000000000000000000000000000000000000000000000000000000000002", "registry1.io/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000002", &testutil.Config2, "layer3", "layer4", "layer5", testutil.Config1),
				testutil.ImageWithLayers("sha256:0000000000000000000000000000000000000000000000000000000000000003", "registry1.io/foo/other@sha256:0000000000000000000000000000000000000000000000000000000000000003", nil, "layer3", "layer4", "layer6", testutil.Config1),
			),
			streams: testutil.StreamList(
				testutil.Stream("registry1.io", "foo", "bar", testutil.Tags(
					testutil.Tag("latest",
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000002", "registry1.io/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000002"),
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000001", "registry1.io/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000001"),
					),
				)),
				testutil.Stream("registry1.io", "foo", "other", testutil.Tags(
					testutil.Tag("latest",
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000003", "registry1.io/foo/other@sha256:0000000000000000000000000000000000000000000000000000000000000003"),
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000002", "registry1.io/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000002"),
					),
				)),
			),
			expectedLayerLinkDeletions: sets.NewString(
				"https://registry1.io|foo/bar|layer1",
				"https://registry1.io|foo/bar|layer2",
				// TODO: ideally, pruner should remove layers of id2 from foo/bar as well
			),
			expectedBlobDeletions: sets.NewString(
				"https://registry1.io|layer1",
				"https://registry1.io|layer2",
			),
			expectedManifestDeletions: sets.NewString(
				"https://registry1.io|foo/bar|sha256:0000000000000000000000000000000000000000000000000000000000000001",
			),
		},

		{
			name:          "config used as a layer, but leave registry alone",
			pruneRegistry: false,
			images: testutil.ImageList(
				testutil.ImageWithLayers("sha256:0000000000000000000000000000000000000000000000000000000000000001", "registry1.io/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000001", &testutil.Config1, "layer1", "layer2", "layer3", testutil.Config1),
				testutil.ImageWithLayers("sha256:0000000000000000000000000000000000000000000000000000000000000002", "registry1.io/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000002", &testutil.Config2, "layer3", "layer4", "layer5", testutil.Config1),
				testutil.ImageWithLayers("sha256:0000000000000000000000000000000000000000000000000000000000000003", "registry1.io/foo/other@sha256:0000000000000000000000000000000000000000000000000000000000000003", nil, "layer3", "layer4", "layer6", testutil.Config1),
			),
			streams: testutil.StreamList(
				testutil.Stream("registry1.io", "foo", "bar", testutil.Tags(
					testutil.Tag("latest",
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000002", "registry1.io/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000002"),
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000001", "registry1.io/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000001"),
					),
				)),
				testutil.Stream("registry1.io", "foo", "other", testutil.Tags(
					testutil.Tag("latest",
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000003", "registry1.io/foo/other@sha256:0000000000000000000000000000000000000000000000000000000000000003"),
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000002", "registry1.io/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000002"),
					),
				)),
			),
			expectedLayerLinkDeletions: sets.NewString(),
			expectedBlobDeletions:      sets.NewString(),
			expectedManifestDeletions:  sets.NewString(),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			keepYoungerThan := 60 * time.Minute
			keepTagRevisions := 1
			options := PrunerOptions{
				KeepYoungerThan:  &keepYoungerThan,
				KeepTagRevisions: &keepTagRevisions,
				PruneRegistry:    &test.pruneRegistry,
				Images:           &test.images,
				Streams:          &test.streams,
				Pods:             &kapi.PodList{},
				RCs:              &kapi.ReplicationControllerList{},
				BCs:              &buildapi.BuildConfigList{},
				Builds:           &buildapi.BuildList{},
				DSs:              &kapisext.DaemonSetList{},
				Deployments:      &kapisext.DeploymentList{},
				DCs:              &appsapi.DeploymentConfigList{},
				RSs:              &kapisext.ReplicaSetList{},
				RegistryURL:      &url.URL{Scheme: "https", Host: "registry1.io"},
			}
			p, err := NewPruner(options)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			imageDeleter := &fakeImageDeleter{invocations: sets.NewString()}
			streamDeleter := &fakeImageStreamDeleter{invocations: sets.NewString()}
			layerLinkDeleter := &fakeLayerLinkDeleter{invocations: sets.NewString()}
			blobDeleter := &fakeBlobDeleter{invocations: sets.NewString()}
			manifestDeleter := &fakeManifestDeleter{invocations: sets.NewString()}

			p.Prune(imageDeleter, streamDeleter, layerLinkDeleter, blobDeleter, manifestDeleter)

			if a, e := layerLinkDeleter.invocations, test.expectedLayerLinkDeletions; !reflect.DeepEqual(a, e) {
				t.Errorf("unexpected layer link deletions: %s", diff.ObjectDiff(a, e))
			}
			if a, e := blobDeleter.invocations, test.expectedBlobDeletions; !reflect.DeepEqual(a, e) {
				t.Errorf("unexpected blob deletions: %s", diff.ObjectDiff(a, e))
			}
			if a, e := manifestDeleter.invocations, test.expectedManifestDeletions; !reflect.DeepEqual(a, e) {
				t.Errorf("unexpected manifest deletions: %s", diff.ObjectDiff(a, e))
			}
		})
	}
}

func newBool(a bool) *bool {
	r := new(bool)
	*r = a
	return r
}

func TestImageWithStrongAndWeakRefsIsNotPruned(t *testing.T) {
	flag.Lookup("v").Value.Set(fmt.Sprint(*logLevel))

	images := testutil.ImageList(
		testutil.AgedImage("0000000000000000000000000000000000000000000000000000000000000001", "registry1.io/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000001", 1540),
		testutil.AgedImage("0000000000000000000000000000000000000000000000000000000000000002", "registry1.io/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000002", 1540),
		testutil.AgedImage("0000000000000000000000000000000000000000000000000000000000000003", "registry1.io/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000003", 1540),
	)
	streams := testutil.StreamList(
		testutil.Stream("registry1", "foo", "bar", testutil.Tags(
			testutil.Tag("latest",
				testutil.TagEvent("0000000000000000000000000000000000000000000000000000000000000003", "registry1.io/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000003"),
				testutil.TagEvent("0000000000000000000000000000000000000000000000000000000000000002", "registry1.io/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000002"),
				testutil.TagEvent("0000000000000000000000000000000000000000000000000000000000000001", "registry1.io/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000001"),
			),
			testutil.Tag("strong",
				testutil.TagEvent("0000000000000000000000000000000000000000000000000000000000000001", "registry1.io/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000001"),
			),
		)),
	)
	pods := testutil.PodList()
	rcs := testutil.RCList()
	bcs := testutil.BCList()
	builds := testutil.BuildList()
	dss := testutil.DSList()
	deployments := testutil.DeploymentList()
	dcs := testutil.DCList()
	rss := testutil.RSList()

	options := PrunerOptions{
		Images:      &images,
		Streams:     &streams,
		Pods:        &pods,
		RCs:         &rcs,
		BCs:         &bcs,
		Builds:      &builds,
		DSs:         &dss,
		Deployments: &deployments,
		DCs:         &dcs,
		RSs:         &rss,
	}
	keepYoungerThan := 24 * time.Hour
	keepTagRevisions := 2
	options.KeepYoungerThan = &keepYoungerThan
	options.KeepTagRevisions = &keepTagRevisions
	p, err := NewPruner(options)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	imageDeleter := &fakeImageDeleter{invocations: sets.NewString()}
	streamDeleter := &fakeImageStreamDeleter{invocations: sets.NewString()}
	layerLinkDeleter := &fakeLayerLinkDeleter{invocations: sets.NewString()}
	blobDeleter := &fakeBlobDeleter{invocations: sets.NewString()}
	manifestDeleter := &fakeManifestDeleter{invocations: sets.NewString()}

	if err := p.Prune(imageDeleter, streamDeleter, layerLinkDeleter, blobDeleter, manifestDeleter); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if imageDeleter.invocations.Len() > 0 {
		t.Fatalf("unexpected imageDeleter invocations: %v", imageDeleter.invocations)
	}
	if streamDeleter.invocations.Len() > 0 {
		t.Fatalf("unexpected streamDeleter invocations: %v", streamDeleter.invocations)
	}
	if layerLinkDeleter.invocations.Len() > 0 {
		t.Fatalf("unexpected layerLinkDeleter invocations: %v", layerLinkDeleter.invocations)
	}
	if blobDeleter.invocations.Len() > 0 {
		t.Fatalf("unexpected blobDeleter invocations: %v", blobDeleter.invocations)
	}
	if manifestDeleter.invocations.Len() > 0 {
		t.Fatalf("unexpected manifestDeleter invocations: %v", manifestDeleter.invocations)
	}
}

func TestImageIsPrunable(t *testing.T) {
	g := genericgraph.New()
	imageNode := imagegraph.EnsureImageNode(g, &imageapi.Image{ObjectMeta: metav1.ObjectMeta{Name: "myImage"}})
	streamNode := imagegraph.EnsureImageStreamNode(g, &imageapi.ImageStream{ObjectMeta: metav1.ObjectMeta{Name: "myStream"}})
	g.AddEdge(streamNode, imageNode, ReferencedImageEdgeKind)
	g.AddEdge(streamNode, imageNode, WeakReferencedImageEdgeKind)

	if imageIsPrunable(g, imageNode.(*imagegraph.ImageNode), pruneAlgorithm{}) {
		t.Fatalf("Image is prunable although it should not")
	}
}

func keepTagRevisions(n int) *int {
	return &n
}

type fakeImageDeleter struct {
	invocations sets.String
	err         error
}

var _ ImageDeleter = &fakeImageDeleter{}

func (p *fakeImageDeleter) DeleteImage(image *imageapi.Image) error {
	p.invocations.Insert(image.Name)
	return p.err
}

type fakeImageStreamDeleter struct {
	invocations  sets.String
	err          error
	streamImages map[string][]string
	streamTags   map[string][]string
}

var _ ImageStreamDeleter = &fakeImageStreamDeleter{}

func (p *fakeImageStreamDeleter) GetImageStream(stream *imageapi.ImageStream) (*imageapi.ImageStream, error) {
	if p.streamImages == nil {
		p.streamImages = make(map[string][]string)
	}
	if p.streamTags == nil {
		p.streamTags = make(map[string][]string)
	}
	for tag, history := range stream.Status.Tags {
		streamName := fmt.Sprintf("%s/%s", stream.Namespace, stream.Name)
		p.streamTags[streamName] = append(p.streamTags[streamName], tag)

		for _, tagEvent := range history.Items {
			p.streamImages[streamName] = append(p.streamImages[streamName], tagEvent.Image)
		}
	}
	return stream, p.err
}

func (p *fakeImageStreamDeleter) UpdateImageStream(stream *imageapi.ImageStream) (*imageapi.ImageStream, error) {
	streamImages := make(map[string]struct{})
	streamTags := make(map[string]struct{})

	for tag, history := range stream.Status.Tags {
		streamTags[tag] = struct{}{}
		for _, tagEvent := range history.Items {
			streamImages[tagEvent.Image] = struct{}{}
		}
	}

	streamName := fmt.Sprintf("%s/%s", stream.Namespace, stream.Name)

	for _, tag := range p.streamTags[streamName] {
		if _, ok := streamTags[tag]; !ok {
			p.invocations.Insert(fmt.Sprintf("%s:%s", streamName, tag))
		}
	}

	for _, imageName := range p.streamImages[streamName] {
		if _, ok := streamImages[imageName]; !ok {
			p.invocations.Insert(fmt.Sprintf("%s|%s", streamName, imageName))
		}
	}

	return stream, p.err
}

func (p *fakeImageStreamDeleter) NotifyImageStreamPrune(stream *imageapi.ImageStream, updatedTags []string, deletedTags []string) {
	return
}

type fakeBlobDeleter struct {
	invocations sets.String
	err         error
}

var _ BlobDeleter = &fakeBlobDeleter{}

func (p *fakeBlobDeleter) DeleteBlob(registryClient *http.Client, registryURL *url.URL, blob string) error {
	p.invocations.Insert(fmt.Sprintf("%s|%s", registryURL.String(), blob))
	return p.err
}

type fakeLayerLinkDeleter struct {
	invocations sets.String
	err         error
}

var _ LayerLinkDeleter = &fakeLayerLinkDeleter{}

func (p *fakeLayerLinkDeleter) DeleteLayerLink(registryClient *http.Client, registryURL *url.URL, repo, layer string) error {
	p.invocations.Insert(fmt.Sprintf("%s|%s|%s", registryURL.String(), repo, layer))
	return p.err
}

type fakeManifestDeleter struct {
	invocations sets.String
	err         error
}

var _ ManifestDeleter = &fakeManifestDeleter{}

func (p *fakeManifestDeleter) DeleteManifest(registryClient *http.Client, registryURL *url.URL, repo, manifest string) error {
	p.invocations.Insert(fmt.Sprintf("%s|%s|%s", registryURL.String(), repo, manifest))
	return p.err
}
