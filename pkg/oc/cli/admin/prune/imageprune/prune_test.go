package imageprune

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"reflect"
	"regexp"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/golang/glog"

	kappsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/diff"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/rest/fake"
	clienttesting "k8s.io/client-go/testing"
	"k8s.io/kubernetes/pkg/kubectl/scheme"

	"github.com/openshift/api"
	appsv1 "github.com/openshift/api/apps/v1"
	buildv1 "github.com/openshift/api/build/v1"
	imagev1 "github.com/openshift/api/image/v1"
	fakeimageclient "github.com/openshift/client-go/image/clientset/versioned/fake"
	imagev1client "github.com/openshift/client-go/image/clientset/versioned/typed/image/v1"
	fakeimagev1client "github.com/openshift/client-go/image/clientset/versioned/typed/image/v1/fake"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	"github.com/openshift/origin/pkg/oc/cli/admin/prune/imageprune/testutil"
	"github.com/openshift/origin/pkg/oc/lib/graph/genericgraph"
	imagegraph "github.com/openshift/origin/pkg/oc/lib/graph/imagegraph/nodes"
)

var logLevel = flag.Int("loglevel", 0, "")

func TestImagePruning(t *testing.T) {
	flag.Lookup("v").Value.Set(fmt.Sprint(*logLevel))
	registryHost := "registry.io"
	registryURL := "https://" + registryHost

	tests := []struct {
		name                          string
		pruneOverSizeLimit            *bool
		allImages                     *bool
		pruneRegistry                 *bool
		ignoreInvalidRefs             *bool
		keepTagRevisions              *int
		namespace                     string
		images                        imagev1.ImageList
		pods                          corev1.PodList
		streams                       imagev1.ImageStreamList
		rcs                           corev1.ReplicationControllerList
		bcs                           buildv1.BuildConfigList
		builds                        buildv1.BuildList
		dss                           kappsv1.DaemonSetList
		deployments                   kappsv1.DeploymentList
		dcs                           appsv1.DeploymentConfigList
		rss                           kappsv1.ReplicaSetList
		limits                        map[string][]*corev1.LimitRange
		imageDeleterErr               error
		imageStreamDeleterErr         error
		layerDeleterErr               error
		manifestDeleterErr            error
		blobDeleterErrorGetter        errorForSHA
		expectedImageDeletions        []string
		expectedStreamUpdates         []string
		expectedLayerLinkDeletions    []string
		expectedManifestLinkDeletions []string
		expectedBlobDeletions         []string
		expectedFailures              []string
		expectedErrorString           string
	}{
		{
			name:   "1 pod - phase pending - don't prune",
			images: testutil.ImageList(testutil.Image("sha256:0000000000000000000000000000000000000000000000000000000000000000", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000")),
			pods:   testutil.PodList(testutil.Pod("foo", "pod1", corev1.PodPending, registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000")),
			expectedImageDeletions: []string{},
		},

		{
			name:   "3 pods - last phase pending - don't prune",
			images: testutil.ImageList(testutil.Image("sha256:0000000000000000000000000000000000000000000000000000000000000000", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000")),
			pods: testutil.PodList(
				testutil.Pod("foo", "pod1", corev1.PodSucceeded, registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000"),
				testutil.Pod("foo", "pod2", corev1.PodSucceeded, registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000"),
				testutil.Pod("foo", "pod3", corev1.PodPending, registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000"),
			),
			expectedImageDeletions: []string{},
		},

		{
			name:   "1 pod - phase running - don't prune",
			images: testutil.ImageList(testutil.Image("sha256:0000000000000000000000000000000000000000000000000000000000000000", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000")),
			pods:   testutil.PodList(testutil.Pod("foo", "pod1", corev1.PodRunning, registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000")),
			expectedImageDeletions: []string{},
		},

		{
			name:   "3 pods - last phase running - don't prune",
			images: testutil.ImageList(testutil.Image("sha256:0000000000000000000000000000000000000000000000000000000000000000", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000")),
			pods: testutil.PodList(
				testutil.Pod("foo", "pod1", corev1.PodSucceeded, registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000"),
				testutil.Pod("foo", "pod2", corev1.PodSucceeded, registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000"),
				testutil.Pod("foo", "pod3", corev1.PodRunning, registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000"),
			),
			expectedImageDeletions: []string{},
		},

		{
			name:   "pod phase succeeded - prune",
			images: testutil.ImageList(testutil.Image("sha256:0000000000000000000000000000000000000000000000000000000000000000", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000")),
			pods:   testutil.PodList(testutil.Pod("foo", "pod1", corev1.PodSucceeded, registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000")),
			expectedImageDeletions: []string{"sha256:0000000000000000000000000000000000000000000000000000000000000000"},
			expectedBlobDeletions: []string{
				registryURL + "|sha256:0000000000000000000000000000000000000000000000000000000000000000",
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
			pods:          testutil.PodList(testutil.Pod("foo", "pod1", corev1.PodSucceeded, registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000")),
			expectedImageDeletions: []string{"sha256:0000000000000000000000000000000000000000000000000000000000000000"},
			expectedBlobDeletions:  []string{},
		},

		{
			name:   "pod phase succeeded, pod less than min pruning age - don't prune",
			images: testutil.ImageList(testutil.Image("sha256:0000000000000000000000000000000000000000000000000000000000000000", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000")),
			pods:   testutil.PodList(testutil.AgedPod("foo", "pod1", corev1.PodSucceeded, 5, registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000")),
			expectedImageDeletions: []string{},
		},

		{
			name:   "pod phase succeeded, image less than min pruning age - don't prune",
			images: testutil.ImageList(testutil.AgedImage("sha256:0000000000000000000000000000000000000000000000000000000000000000", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000", 5)),
			pods:   testutil.PodList(testutil.Pod("foo", "pod1", corev1.PodSucceeded, registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000")),
			expectedImageDeletions: []string{},
		},

		{
			name:   "pod phase failed - prune",
			images: testutil.ImageList(testutil.Image("sha256:0000000000000000000000000000000000000000000000000000000000000000", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000")),
			pods: testutil.PodList(
				testutil.Pod("foo", "pod1", corev1.PodFailed, registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000"),
				testutil.Pod("foo", "pod2", corev1.PodFailed, registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000"),
				testutil.Pod("foo", "pod3", corev1.PodFailed, registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000"),
			),
			expectedImageDeletions: []string{"sha256:0000000000000000000000000000000000000000000000000000000000000000"},
			expectedBlobDeletions: []string{
				registryURL + "|sha256:0000000000000000000000000000000000000000000000000000000000000000",
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
				testutil.Pod("foo", "pod1", corev1.PodUnknown, registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000"),
				testutil.Pod("foo", "pod2", corev1.PodUnknown, registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000"),
				testutil.Pod("foo", "pod3", corev1.PodUnknown, registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000"),
			),
			expectedImageDeletions: []string{"sha256:0000000000000000000000000000000000000000000000000000000000000000"},
			expectedBlobDeletions: []string{
				registryURL + "|sha256:0000000000000000000000000000000000000000000000000000000000000000",
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
				testutil.Pod("foo", "pod1", corev1.PodRunning, "a/b/c/d/e"),
			),
			expectedImageDeletions: []string{"sha256:0000000000000000000000000000000000000000000000000000000000000000"},
			expectedBlobDeletions: []string{
				registryURL + "|sha256:0000000000000000000000000000000000000000000000000000000000000000",
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
				testutil.Pod("foo", "pod1", corev1.PodRunning, "foo/bar:latest"),
			),
			expectedImageDeletions: []string{"sha256:0000000000000000000000000000000000000000000000000000000000000000"},
			expectedBlobDeletions: []string{
				registryURL + "|sha256:0000000000000000000000000000000000000000000000000000000000000000",
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
				testutil.Pod("foo", "pod1", corev1.PodRunning, registryHost+"/foo/bar@sha256:ABC0000000000000000000000000000000000000000000000000000000000002"),
			),
			expectedImageDeletions: []string{"sha256:0000000000000000000000000000000000000000000000000000000000000000"},
			expectedBlobDeletions: []string{
				registryURL + "|sha256:0000000000000000000000000000000000000000000000000000000000000000",
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
			expectedBlobDeletions:  []string{registryURL + "|sha256:0000000000000000000000000000000000000000000000000000000000000001"},
		},

		{
			name: "referenced by replicaset - don't prune",
			images: testutil.ImageList(
				testutil.Image("sha256:0000000000000000000000000000000000000000000000000000000000000000", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000"),
				testutil.Image("sha256:0000000000000000000000000000000000000000000000000000000000000001", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000001"),
			),
			rss: testutil.RSList(testutil.RS("foo", "rc1", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000")),
			expectedImageDeletions: []string{"sha256:0000000000000000000000000000000000000000000000000000000000000001"},
			expectedBlobDeletions:  []string{registryURL + "|sha256:0000000000000000000000000000000000000000000000000000000000000001"},
		},

		{
			name: "referenced by upstream deployment - don't prune",
			images: testutil.ImageList(
				testutil.Image("sha256:0000000000000000000000000000000000000000000000000000000000000000", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000"),
				testutil.Image("sha256:0000000000000000000000000000000000000000000000000000000000000001", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000001"),
			),
			deployments:            testutil.DeploymentList(testutil.Deployment("foo", "rc1", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000")),
			expectedImageDeletions: []string{"sha256:0000000000000000000000000000000000000000000000000000000000000001"},
			expectedBlobDeletions:  []string{registryURL + "|sha256:0000000000000000000000000000000000000000000000000000000000000001"},
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
				testutil.Stream(registryHost, "foo", "bar", []imagev1.NamedTagEventList{
					testutil.Tag("latest",
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000000", "otherregistry/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000"),
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000002", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000002"),
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000003", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000003"),
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000004", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000004"),
					),
				}),
			),
			expectedImageDeletions:        []string{"sha256:0000000000000000000000000000000000000000000000000000000000000004"},
			expectedStreamUpdates:         []string{"foo/bar|sha256:0000000000000000000000000000000000000000000000000000000000000004"},
			expectedManifestLinkDeletions: []string{registryURL + "|foo/bar|sha256:0000000000000000000000000000000000000000000000000000000000000004"},
			expectedBlobDeletions:         []string{registryURL + "|" + "sha256:0000000000000000000000000000000000000000000000000000000000000004"},
		},

		{
			name: "continue on blob deletion failure",
			images: testutil.ImageList(
				testutil.UnmanagedImage("sha256:0000000000000000000000000000000000000000000000000000000000000000", "otherregistry/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000", false, "", ""),
				testutil.Image("sha256:0000000000000000000000000000000000000000000000000000000000000002", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000002"),
				testutil.Image("sha256:0000000000000000000000000000000000000000000000000000000000000003", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000003"),
				testutil.ImageWithLayers("sha256:0000000000000000000000000000000000000000000000000000000000000004", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000004", nil, "layer1", "layer2"),
			),
			streams: testutil.StreamList(
				testutil.Stream(registryHost, "foo", "bar", []imagev1.NamedTagEventList{
					testutil.Tag("latest",
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000000", "otherregistry/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000"),
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000002", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000002"),
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000003", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000003"),
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000004", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000004"),
					),
				}),
			),
			blobDeleterErrorGetter: func(dgst string) error {
				if dgst == "layer1" {
					return errors.New("err")
				}
				return nil
			},
			expectedImageDeletions:        []string{"sha256:0000000000000000000000000000000000000000000000000000000000000004"},
			expectedStreamUpdates:         []string{"foo/bar|sha256:0000000000000000000000000000000000000000000000000000000000000004"},
			expectedManifestLinkDeletions: []string{registryURL + "|foo/bar|sha256:0000000000000000000000000000000000000000000000000000000000000004"},
			expectedLayerLinkDeletions:    []string{registryURL + "|foo/bar|layer1", registryURL + "|foo/bar|layer2"},
			expectedBlobDeletions: []string{
				registryURL + "|" + "layer1",
				registryURL + "|" + "layer2",
				registryURL + "|" + "sha256:0000000000000000000000000000000000000000000000000000000000000004",
			},
			expectedFailures: []string{registryURL + "|" + "layer1|err"},
		},

		{
			name: "keep image when all blob deletions fail",
			images: testutil.ImageList(
				testutil.UnmanagedImage("sha256:0000000000000000000000000000000000000000000000000000000000000000", "otherregistry/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000", false, "", ""),
				testutil.Image("sha256:0000000000000000000000000000000000000000000000000000000000000002", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000002"),
				testutil.Image("sha256:0000000000000000000000000000000000000000000000000000000000000003", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000003"),
				testutil.ImageWithLayers("sha256:0000000000000000000000000000000000000000000000000000000000000004", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000004", nil, "layer1", "layer2"),
			),
			streams: testutil.StreamList(
				testutil.Stream(registryHost, "foo", "bar", []imagev1.NamedTagEventList{
					testutil.Tag("latest",
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000000", "otherregistry/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000"),
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000002", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000002"),
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000003", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000003"),
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000004", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000004"),
					),
				}),
			),
			blobDeleterErrorGetter:        func(dgst string) error { return errors.New("err") },
			expectedImageDeletions:        []string{},
			expectedStreamUpdates:         []string{"foo/bar|sha256:0000000000000000000000000000000000000000000000000000000000000004"},
			expectedManifestLinkDeletions: []string{registryURL + "|foo/bar|sha256:0000000000000000000000000000000000000000000000000000000000000004"},
			expectedLayerLinkDeletions:    []string{registryURL + "|foo/bar|layer1", registryURL + "|foo/bar|layer2"},
			expectedBlobDeletions:         []string{registryURL + "|layer1", registryURL + "|layer2", registryURL + "|" + "sha256:0000000000000000000000000000000000000000000000000000000000000004"},
			expectedFailures:              []string{registryURL + "|" + "layer1|err", registryURL + "|" + "layer2|err", registryURL + "|sha256:0000000000000000000000000000000000000000000000000000000000000004|err"},
		},

		{
			name: "continue on manifest link deletion failure",
			images: testutil.ImageList(
				testutil.UnmanagedImage("sha256:0000000000000000000000000000000000000000000000000000000000000000", "otherregistry/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000", false, "", ""),
				testutil.Image("sha256:0000000000000000000000000000000000000000000000000000000000000002", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000002"),
				testutil.Image("sha256:0000000000000000000000000000000000000000000000000000000000000003", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000003"),
				testutil.Image("sha256:0000000000000000000000000000000000000000000000000000000000000004", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000004"),
			),
			streams: testutil.StreamList(
				testutil.Stream(registryHost, "foo", "bar", []imagev1.NamedTagEventList{
					testutil.Tag("latest",
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000000", "otherregistry/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000"),
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000002", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000002"),
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000003", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000003"),
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000004", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000004"),
					),
				}),
			),
			manifestDeleterErr:            fmt.Errorf("err"),
			expectedImageDeletions:        []string{"sha256:0000000000000000000000000000000000000000000000000000000000000004"},
			expectedStreamUpdates:         []string{"foo/bar|sha256:0000000000000000000000000000000000000000000000000000000000000004"},
			expectedManifestLinkDeletions: []string{registryURL + "|foo/bar|sha256:0000000000000000000000000000000000000000000000000000000000000004"},
			expectedBlobDeletions:         []string{registryURL + "|" + "sha256:0000000000000000000000000000000000000000000000000000000000000004"},
			expectedFailures:              []string{registryURL + "|foo/bar|sha256:0000000000000000000000000000000000000000000000000000000000000004|err"},
		},

		{
			name: "stop on image stream update failure",
			images: testutil.ImageList(
				testutil.UnmanagedImage("sha256:0000000000000000000000000000000000000000000000000000000000000000", "otherregistry/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000", false, "", ""),
				testutil.Image("sha256:0000000000000000000000000000000000000000000000000000000000000002", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000002"),
				testutil.Image("sha256:0000000000000000000000000000000000000000000000000000000000000003", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000003"),
				testutil.Image("sha256:0000000000000000000000000000000000000000000000000000000000000004", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000004"),
			),
			streams: testutil.StreamList(
				testutil.Stream(registryHost, "foo", "bar", []imagev1.NamedTagEventList{
					testutil.Tag("latest",
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000000", "otherregistry/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000"),
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000002", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000002"),
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000003", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000003"),
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000004", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000004"),
					),
				}),
			),
			imageStreamDeleterErr: fmt.Errorf("err"),
			expectedFailures:      []string{"foo/bar|err"},
		},

		{
			name: "image stream - same manifest listed multiple times in tag history",
			images: testutil.ImageList(
				testutil.Image("sha256:0000000000000000000000000000000000000000000000000000000000000001", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000001"),
				testutil.Image("sha256:0000000000000000000000000000000000000000000000000000000000000002", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000002"),
			),
			streams: testutil.StreamList(
				testutil.Stream(registryHost, "foo", "bar", []imagev1.NamedTagEventList{
					testutil.Tag("latest",
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000001", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000001"),
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000002", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000002"),
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000001", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000001"),
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000002", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000002"),
					),
				}),
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
				testutil.AgedStream(registryHost, "foo", "bar", 5, []imagev1.NamedTagEventList{
					testutil.Tag("latest",
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000000", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000"),
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000002", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000002"),
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000003", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000003"),
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000004", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000004"),
					),
				}),
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
				testutil.Stream(registryHost, "foo", "bar", []imagev1.NamedTagEventList{
					testutil.Tag("latest",
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000000", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000"),
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000001", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000001"),
					),
				}),
			),
			expectedStreamUpdates: []string{"foo/bar|sha256:0000000000000000000000000000000000000000000000000000000000000000"},
		},

		{
			name: "image stream with dangling references - delete tags",
			images: testutil.ImageList(
				testutil.ImageWithLayers("sha256:0000000000000000000000000000000000000000000000000000000000000001", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000001", nil, "layer1"),
			),
			streams: testutil.StreamList(
				testutil.Stream(registryHost, "foo", "bar", []imagev1.NamedTagEventList{
					testutil.Tag("latest",
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000000", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000"),
					),
					testutil.Tag("tag",
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000002", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000002"),
					),
				}),
			),
			expectedImageDeletions: []string{"sha256:0000000000000000000000000000000000000000000000000000000000000001"},
			expectedStreamUpdates: []string{
				"foo/bar:latest",
				"foo/bar:tag",
				"foo/bar|sha256:0000000000000000000000000000000000000000000000000000000000000000",
				"foo/bar|sha256:0000000000000000000000000000000000000000000000000000000000000002",
			},
			expectedBlobDeletions: []string{registryURL + "|sha256:0000000000000000000000000000000000000000000000000000000000000001", registryURL + "|layer1"},
		},

		{
			name: "image stream - keep reference to a young absent image",
			images: testutil.ImageList(
				testutil.Image("sha256:0000000000000000000000000000000000000000000000000000000000000001", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000001"),
				testutil.ImageWithLayers("sha256:0000000000000000000000000000000000000000000000000000000000000002", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000002", nil),
			),
			streams: testutil.StreamList(
				testutil.Stream(registryHost, "foo", "bar", []imagev1.NamedTagEventList{
					testutil.Tag("latest",
						testutil.YoungTagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000000", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000", metav1.Now()),
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000001", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000001"),
					),
				}),
			),
			expectedImageDeletions: []string{"sha256:0000000000000000000000000000000000000000000000000000000000000002"},
			expectedBlobDeletions:  []string{registryURL + "|sha256:0000000000000000000000000000000000000000000000000000000000000002"},
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
				testutil.Stream(registryHost, "foo", "bar", []imagev1.NamedTagEventList{
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
				}),
				testutil.Stream(registryHost, "foo", "baz", []imagev1.NamedTagEventList{
					testutil.Tag("late", // kept because replicaset references the tagged image
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000002", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000002"),
					),
					testutil.Tag("keepme", // kept because a deployment references the tagged image
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000006", registryHost+"/foo/baz@sha256:0000000000000000000000000000000000000000000000000000000000000006"),
					),
				}),
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
			expectedManifestLinkDeletions: []string{
				registryURL + "|foo/bar|sha256:0000000000000000000000000000000000000000000000000000000000000001",
				registryURL + "|foo/bar|sha256:0000000000000000000000000000000000000000000000000000000000000003",
				registryURL + "|foo/bar|sha256:0000000000000000000000000000000000000000000000000000000000000004",
				registryURL + "|foo/bar|sha256:0000000000000000000000000000000000000000000000000000000000000005",
			},
			expectedBlobDeletions: []string{
				registryURL + "|sha256:0000000000000000000000000000000000000000000000000000000000000001",
				registryURL + "|" + "sha256:0000000000000000000000000000000000000000000000000000000000000003",
				registryURL + "|" + "sha256:0000000000000000000000000000000000000000000000000000000000000004",
				registryURL + "|" + "sha256:0000000000000000000000000000000000000000000000000000000000000005",
			},
		},

		{
			name: "multiple resources pointing to image - don't prune",
			images: testutil.ImageList(
				testutil.Image("sha256:0000000000000000000000000000000000000000000000000000000000000000", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000"),
				testutil.Image("sha256:0000000000000000000000000000000000000000000000000000000000000002", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000002"),
			),
			streams: testutil.StreamList(
				testutil.Stream(registryHost, "foo", "bar", []imagev1.NamedTagEventList{
					testutil.Tag("latest",
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000000", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000"),
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000002", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000002"),
					),
				}),
			),
			rcs:                    testutil.RCList(testutil.RC("foo", "rc1", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000002")),
			pods:                   testutil.PodList(testutil.Pod("foo", "pod1", corev1.PodRunning, registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000002")),
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
			expectedBlobDeletions:  []string{registryURL + "|sha256:0000000000000000000000000000000000000000000000000000000000000000"},
		},

		{
			name:      "prune all-images=true image with nil annotations",
			allImages: newBool(true),
			images: testutil.ImageList(
				testutil.UnmanagedImage("sha256:0000000000000000000000000000000000000000000000000000000000000000", "someregistry/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000", false, "", ""),
			),
			expectedImageDeletions: []string{"sha256:0000000000000000000000000000000000000000000000000000000000000000"},
			expectedStreamUpdates:  []string{},
			expectedBlobDeletions:  []string{registryURL + "|sha256:0000000000000000000000000000000000000000000000000000000000000000"},
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
			expectedBlobDeletions:  []string{registryURL + "|sha256:0000000000000000000000000000000000000000000000000000000000000000"},
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
			expectedBlobDeletions: []string{
				registryURL + "|sha256:0000000000000000000000000000000000000000000000000000000000000000",
				registryURL + "|sha256:0000000000000000000000000000000000000000000000000000000000000001",
				registryURL + "|" + "sha256:0000000000000000000000000000000000000000000000000000000000000002",
				registryURL + "|" + "sha256:0000000000000000000000000000000000000000000000000000000000000003",
				registryURL + "|" + "sha256:0000000000000000000000000000000000000000000000000000000000000004",
				registryURL + "|" + "sha256:0000000000000000000000000000000000000000000000000000000000000005",
			},
		},

		{
			name:      "prune all-images=true with image missing managed annotation",
			allImages: newBool(true),
			images: testutil.ImageList(
				testutil.UnmanagedImage("sha256:0000000000000000000000000000000000000000000000000000000000000000", "someregistry/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000", true, "foo", "bar"),
			),
			expectedImageDeletions: []string{"sha256:0000000000000000000000000000000000000000000000000000000000000000"},
			expectedStreamUpdates:  []string{},
			expectedBlobDeletions:  []string{registryURL + "|sha256:0000000000000000000000000000000000000000000000000000000000000000"},
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
			expectedBlobDeletions: []string{
				registryURL + "|sha256:0000000000000000000000000000000000000000000000000000000000000000",
				registryURL + "|sha256:0000000000000000000000000000000000000000000000000000000000000001",
				registryURL + "|sha256:0000000000000000000000000000000000000000000000000000000000000002",
				registryURL + "|sha256:0000000000000000000000000000000000000000000000000000000000000003",
				registryURL + "|sha256:0000000000000000000000000000000000000000000000000000000000000004",
				registryURL + "|sha256:0000000000000000000000000000000000000000000000000000000000000005",
			},
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
				testutil.Stream(registryHost, "foo", "bar", []imagev1.NamedTagEventList{
					testutil.Tag("latest",
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000001", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000001"),
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000002", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000002"),
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000003", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000003"),
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000004", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000004"),
					),
				}),
			),
			expectedImageDeletions: []string{"sha256:0000000000000000000000000000000000000000000000000000000000000004"},
			expectedStreamUpdates:  []string{"foo/bar|sha256:0000000000000000000000000000000000000000000000000000000000000004"},
			expectedLayerLinkDeletions: []string{
				registryURL + "|foo/bar|layer5",
				registryURL + "|foo/bar|layer6",
				registryURL + "|foo/bar|layer7",
				registryURL + "|foo/bar|layer8",
			},
			expectedManifestLinkDeletions: []string{registryURL + "|foo/bar|sha256:0000000000000000000000000000000000000000000000000000000000000004"},
			expectedBlobDeletions: []string{
				registryURL + "|sha256:0000000000000000000000000000000000000000000000000000000000000004",
				registryURL + "|layer5",
				registryURL + "|layer6",
				registryURL + "|layer7",
				registryURL + "|layer8",
			},
		},

		{
			name: "continue on layer link error",
			images: testutil.ImageList(
				testutil.ImageWithLayers("sha256:0000000000000000000000000000000000000000000000000000000000000001", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000001", &testutil.Config1, "layer1", "layer2", "layer3", "layer4"),
				testutil.ImageWithLayers("sha256:0000000000000000000000000000000000000000000000000000000000000002", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000002", &testutil.Config2, "layer1", "layer2", "layer3", "layer4"),
				testutil.ImageWithLayers("sha256:0000000000000000000000000000000000000000000000000000000000000003", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000003", nil, "layer1", "layer2", "layer3", "layer4"),
				testutil.ImageWithLayers("sha256:0000000000000000000000000000000000000000000000000000000000000004", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000004", nil, "layer5", "layer6", "layer7", "layer8"),
			),
			streams: testutil.StreamList(
				testutil.Stream(registryHost, "foo", "bar", []imagev1.NamedTagEventList{
					testutil.Tag("latest",
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000001", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000001"),
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000002", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000002"),
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000003", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000003"),
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000004", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000004"),
					),
				}),
			),
			layerDeleterErr:               fmt.Errorf("err"),
			expectedImageDeletions:        []string{"sha256:0000000000000000000000000000000000000000000000000000000000000004"},
			expectedStreamUpdates:         []string{"foo/bar|sha256:0000000000000000000000000000000000000000000000000000000000000004"},
			expectedManifestLinkDeletions: []string{registryURL + "|foo/bar|sha256:0000000000000000000000000000000000000000000000000000000000000004"},
			expectedBlobDeletions: []string{
				registryURL + "|sha256:0000000000000000000000000000000000000000000000000000000000000004",
				registryURL + "|layer5",
				registryURL + "|layer6",
				registryURL + "|layer7",
				registryURL + "|layer8",
			},
			expectedLayerLinkDeletions: []string{
				registryURL + "|foo/bar|layer5",
				registryURL + "|foo/bar|layer6",
				registryURL + "|foo/bar|layer7",
				registryURL + "|foo/bar|layer8",
			},
			expectedFailures: []string{
				registryURL + "|foo/bar|layer5|err",
				registryURL + "|foo/bar|layer6|err",
				registryURL + "|foo/bar|layer7|err",
				registryURL + "|foo/bar|layer8|err",
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
				testutil.Stream(registryHost, "foo", "bar", []imagev1.NamedTagEventList{
					testutil.Tag("latest",
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000001", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000001"),
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000002", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000002"),
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000003", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000003"),
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000004", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000004"),
					),
				}),
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
			expectedManifestLinkDeletions: []string{registryURL + "|foo/bar|sha256:0000000000000000000000000000000000000000000000000000000000000004"},
			expectedBlobDeletions: []string{
				registryURL + "|sha256:0000000000000000000000000000000000000000000000000000000000000004",
				registryURL + "|sha256:0000000000000000000000000000000000000000000000000000000000000005",
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
			name: "continue on image deletion failure",
			images: testutil.ImageList(
				testutil.ImageWithLayers("sha256:0000000000000000000000000000000000000000000000000000000000000001", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000001", &testutil.Config1, "layer1", "layer2", "layer3", "layer4"),
				testutil.ImageWithLayers("sha256:0000000000000000000000000000000000000000000000000000000000000002", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000002", &testutil.Config1, "layer1", "layer2", "layer3", "layer4"),
				testutil.ImageWithLayers("sha256:0000000000000000000000000000000000000000000000000000000000000003", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000003", &testutil.Config1, "layer1", "layer2", "layer3", "layer4"),
				testutil.ImageWithLayers("sha256:0000000000000000000000000000000000000000000000000000000000000004", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000004", &testutil.Config2, "layer5", "layer6", "layer7", "layer8"),
				testutil.ImageWithLayers("sha256:0000000000000000000000000000000000000000000000000000000000000005", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000005", &testutil.Config2, "layer5", "layer6", "layer9", "layerX"),
			),
			streams: testutil.StreamList(
				testutil.Stream(registryHost, "foo", "bar", []imagev1.NamedTagEventList{
					testutil.Tag("latest",
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000001", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000001"),
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000002", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000002"),
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000003", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000003"),
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000004", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000004"),
					),
				}),
			),
			imageDeleterErr:        fmt.Errorf("err"),
			expectedImageDeletions: []string{"sha256:0000000000000000000000000000000000000000000000000000000000000004", "sha256:0000000000000000000000000000000000000000000000000000000000000005"},
			expectedStreamUpdates:  []string{"foo/bar|sha256:0000000000000000000000000000000000000000000000000000000000000004"},
			expectedLayerLinkDeletions: []string{
				registryURL + "|foo/bar|" + testutil.Config2,
				registryURL + "|foo/bar|layer5",
				registryURL + "|foo/bar|layer6",
				registryURL + "|foo/bar|layer7",
				registryURL + "|foo/bar|layer8",
			},
			expectedManifestLinkDeletions: []string{registryURL + "|foo/bar|sha256:0000000000000000000000000000000000000000000000000000000000000004"},
			expectedBlobDeletions: []string{
				registryURL + "|sha256:0000000000000000000000000000000000000000000000000000000000000004",
				registryURL + "|sha256:0000000000000000000000000000000000000000000000000000000000000005",
				registryURL + "|layer7",
				registryURL + "|layer8",
				registryURL + "|layer9",
				registryURL + "|layerX",
			},
			expectedFailures: []string{"sha256:0000000000000000000000000000000000000000000000000000000000000004|err", "sha256:0000000000000000000000000000000000000000000000000000000000000005|err"},
		},

		{
			name: "layers shared with young images are not pruned",
			images: testutil.ImageList(
				testutil.AgedImage("sha256:0000000000000000000000000000000000000000000000000000000000000001", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000001", 43200),
				testutil.AgedImage("sha256:0000000000000000000000000000000000000000000000000000000000000002", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000002", 5),
			),
			expectedImageDeletions: []string{"sha256:0000000000000000000000000000000000000000000000000000000000000001"},
			expectedBlobDeletions:  []string{registryURL + "|sha256:0000000000000000000000000000000000000000000000000000000000000001"},
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
				testutil.Stream(registryHost, "foo", "bar", []imagev1.NamedTagEventList{
					testutil.Tag("latest",
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000000", "otherregistry/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000"),
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000002", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000002"),
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000003", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000003"),
					),
				}),
			),
			limits: map[string][]*corev1.LimitRange{
				"foo": testutil.LimitList(100, 200),
			},
			expectedImageDeletions:        []string{"sha256:0000000000000000000000000000000000000000000000000000000000000003"},
			expectedStreamUpdates:         []string{"foo/bar|sha256:0000000000000000000000000000000000000000000000000000000000000003"},
			expectedManifestLinkDeletions: []string{registryURL + "|foo/bar|sha256:0000000000000000000000000000000000000000000000000000000000000003"},
			expectedBlobDeletions:         []string{registryURL + "|sha256:0000000000000000000000000000000000000000000000000000000000000003"},
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
				testutil.Stream(registryHost, "foo", "bar", []imagev1.NamedTagEventList{
					testutil.Tag("latest",
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000001", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000001"),
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000002", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000002"),
					),
				}),
				testutil.Stream(registryHost, "bar", "foo", []imagev1.NamedTagEventList{
					testutil.Tag("latest",
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000003", registryHost+"/bar/foo@sha256:0000000000000000000000000000000000000000000000000000000000000003"),
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000004", registryHost+"/bar/foo@sha256:0000000000000000000000000000000000000000000000000000000000000004"),
					),
				}),
			),
			limits: map[string][]*corev1.LimitRange{
				"foo": testutil.LimitList(150),
				"bar": testutil.LimitList(550),
			},
			expectedImageDeletions: []string{"sha256:0000000000000000000000000000000000000000000000000000000000000002", "sha256:0000000000000000000000000000000000000000000000000000000000000004"},
			expectedStreamUpdates:  []string{"foo/bar|sha256:0000000000000000000000000000000000000000000000000000000000000002", "bar/foo|sha256:0000000000000000000000000000000000000000000000000000000000000004"},
			expectedManifestLinkDeletions: []string{
				registryURL + "|foo/bar|sha256:0000000000000000000000000000000000000000000000000000000000000002",
				registryURL + "|bar/foo|sha256:0000000000000000000000000000000000000000000000000000000000000004",
			},
			expectedBlobDeletions: []string{
				registryURL + "|sha256:0000000000000000000000000000000000000000000000000000000000000002",
				registryURL + "|sha256:0000000000000000000000000000000000000000000000000000000000000004",
			},
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
				testutil.Stream(registryHost, "foo", "bar", []imagev1.NamedTagEventList{
					testutil.Tag("latest",
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000000", "otherregistry/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000"),
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000002", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000002"),
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000003", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000003"),
					),
				}),
			),
			limits: map[string][]*corev1.LimitRange{
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
				testutil.Stream(registryHost, "foo", "bar", []imagev1.NamedTagEventList{
					testutil.Tag("latest",
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000000", "otherregistry/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000"),
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000002", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000002"),
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000003", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000003"),
					),
				}),
			),
			limits: map[string][]*corev1.LimitRange{
				"foo": testutil.LimitList(100, 200),
			},
			expectedStreamUpdates: []string{"foo/bar|sha256:0000000000000000000000000000000000000000000000000000000000000003"},
		},

		{
			name:               "build with ignored bad image reference",
			pruneOverSizeLimit: newBool(true),
			ignoreInvalidRefs:  newBool(true),
			images: testutil.ImageList(
				testutil.UnmanagedImage("sha256:0000000000000000000000000000000000000000000000000000000000000000", "otherregistry/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000", false, "", ""),
				testutil.SizedImage("sha256:0000000000000000000000000000000000000000000000000000000000000002", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000002", 100, nil),
				testutil.SizedImage("sha256:0000000000000000000000000000000000000000000000000000000000000003", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000003", 200, nil),
			),
			streams: testutil.StreamList(
				testutil.Stream(registryHost, "foo", "bar", []imagev1.NamedTagEventList{
					testutil.Tag("latest",
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000000", "otherregistry/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000"),
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000002", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000002"),
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000003", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000003"),
					),
				}),
			),
			builds: testutil.BuildList(
				testutil.Build("foo", "build1", "source", "DockerImage", "foo", registryHost+"/foo/bar@sha256:many-zeros-and-3"),
			),
			limits: map[string][]*corev1.LimitRange{
				"foo": testutil.LimitList(100, 200),
			},
			expectedImageDeletions:        []string{"sha256:0000000000000000000000000000000000000000000000000000000000000003"},
			expectedManifestLinkDeletions: []string{registryURL + "|foo/bar|sha256:0000000000000000000000000000000000000000000000000000000000000003"},
			expectedBlobDeletions:         []string{registryURL + "|sha256:0000000000000000000000000000000000000000000000000000000000000003"},
			expectedStreamUpdates:         []string{"foo/bar|sha256:0000000000000000000000000000000000000000000000000000000000000003"},
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

	// we need to install OpenShift API types to kubectl's scheme for GetReference to work
	api.Install(scheme.Scheme)

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			options := PrunerOptions{
				Namespace:             test.namespace,
				AllImages:             test.allImages,
				Images:                &test.images,
				ImageWatcher:          watch.NewFake(),
				Streams:               &test.streams,
				StreamWatcher:         watch.NewFake(),
				Pods:                  &test.pods,
				RCs:                   &test.rcs,
				BCs:                   &test.bcs,
				Builds:                &test.builds,
				DSs:                   &test.dss,
				Deployments:           &test.deployments,
				DCs:                   &test.dcs,
				RSs:                   &test.rss,
				LimitRanges:           test.limits,
				RegistryClientFactory: FakeRegistryClientFactory,
				RegistryURL:           &url.URL{Scheme: "https", Host: registryHost},
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
			if test.ignoreInvalidRefs != nil {
				options.IgnoreInvalidRefs = *test.ignoreInvalidRefs
			}
			p, err := NewPruner(options)
			if err != nil {
				if len(test.expectedErrorString) > 0 {
					if a, e := err.Error(), test.expectedErrorString; a != e {
						t.Fatalf("got unexpected error: %q != %q", a, e)
					}
				} else {
					t.Fatalf("got unexpected error: %v", err)
				}
				return
			} else if len(test.expectedErrorString) > 0 {
				t.Fatalf("got no error while expecting: %s", test.expectedErrorString)
				return
			}

			imageDeleter, imageDeleterFactory := newFakeImageDeleter(test.imageDeleterErr)
			streamDeleter := &fakeImageStreamDeleter{err: test.imageStreamDeleterErr, invocations: sets.NewString()}
			layerLinkDeleter := &fakeLayerLinkDeleter{err: test.layerDeleterErr, invocations: sets.NewString()}
			blobDeleter := &fakeBlobDeleter{getError: test.blobDeleterErrorGetter, invocations: sets.NewString()}
			manifestDeleter := &fakeManifestDeleter{err: test.manifestDeleterErr, invocations: sets.NewString()}

			deletions, failures := p.Prune(imageDeleterFactory, streamDeleter, layerLinkDeleter, blobDeleter, manifestDeleter)

			expectedFailures := sets.NewString(test.expectedFailures...)
			renderedFailures := sets.NewString()
			for _, f := range failures {
				rendered := renderFailure(registryURL, &f)
				if renderedFailures.Has(rendered) {
					t.Errorf("got the following failure more than once: %v", rendered)
					continue
				}
				renderedFailures.Insert(rendered)
			}
			for f := range renderedFailures {
				if expectedFailures.Has(f) {
					expectedFailures.Delete(f)
					continue
				}
				t.Errorf("got unexpected failure: %v", f)
			}
			for f := range expectedFailures {
				t.Errorf("the following expected failure was not returned: %v", f)
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

			expectedManifestLinkDeletions := sets.NewString(test.expectedManifestLinkDeletions...)
			if a, e := manifestDeleter.invocations, expectedManifestLinkDeletions; !reflect.DeepEqual(a, e) {
				t.Errorf("unexpected manifest link deletions: %s", diff.ObjectDiff(a, e))
			}

			expectedBlobDeletions := sets.NewString(test.expectedBlobDeletions...)
			if a, e := blobDeleter.invocations, expectedBlobDeletions; !reflect.DeepEqual(a, e) {
				t.Errorf("unexpected blob deletions: %s", diff.ObjectDiff(a, e))
			}

			// TODO: shall we return deletion for each layer link unlinked from the image stream??
			imageStreamUpdates := sets.NewString()
			expectedAllDeletions := sets.NewString()
			for _, s := range []sets.String{expectedImageDeletions, expectedLayerLinkDeletions, expectedBlobDeletions} {
				expectedAllDeletions.Insert(s.List()...)
			}
			for _, d := range deletions {
				rendered, isImageStreamUpdate, isManifestLinkDeletion := renderDeletion(registryURL, &d)
				if isManifestLinkDeletion {
					continue
				}
				if isImageStreamUpdate {
					imageStreamUpdates.Insert(rendered)
					continue
				}
				if expectedAllDeletions.Has(rendered) {
					expectedAllDeletions.Delete(rendered)
				} else {
					t.Errorf("got unexpected deletion: %#+v (rendered: %q)", d, rendered)
				}
			}
			for _, f := range failures {
				rendered, _, _ := renderDeletion(registryURL, &Deletion{Node: f.Node, Parent: f.Parent})
				expectedAllDeletions.Delete(rendered)
			}
			for del, ok := expectedAllDeletions.PopAny(); ok; del, ok = expectedAllDeletions.PopAny() {
				t.Errorf("expected deletion %q did not happen", del)
			}

			expectedStreamUpdateNames := sets.NewString()
			for u := range expectedStreamUpdates {
				expectedStreamUpdateNames.Insert(regexp.MustCompile(`[@|:]`).Split(u, 2)[0])
			}
			if a, e := imageStreamUpdates, expectedStreamUpdateNames; !reflect.DeepEqual(a, e) {
				t.Errorf("unexpected image stream updates in deletions: %s", diff.ObjectDiff(a, e))
			}
		})
	}
}

func renderDeletion(registryURL string, deletion *Deletion) (rendered string, isImageStreamUpdate, isManifestLinkDeletion bool) {
	switch t := deletion.Node.(type) {
	case *imagegraph.ImageNode:
		return t.Image.Name, false, false
	case *imagegraph.ImageComponentNode:
		// deleting blob
		if deletion.Parent == nil {
			return fmt.Sprintf("%s|%s", registryURL, t.Component), false, false
		}
		streamName := "unknown"
		if sn, ok := deletion.Parent.(*imagegraph.ImageStreamNode); ok {
			streamName = getName(sn.ImageStream)
		}
		return fmt.Sprintf("%s|%s|%s", registryURL, streamName, t.Component), false, t.Type == imagegraph.ImageComponentTypeManifest
	case *imagegraph.ImageStreamNode:
		return getName(t.ImageStream), true, false
	}
	return "unknown", false, false
}

func renderFailure(registryURL string, failure *Failure) string {
	rendered, _, _ := renderDeletion(registryURL, &Deletion{Node: failure.Node, Parent: failure.Parent})
	return rendered + "|" + failure.Err.Error()
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
		imageClient := &fakeimagev1client.FakeImageV1{Fake: &clienttesting.Fake{}}
		imageClient.AddReactor("delete", "images", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
			return true, nil, test.imageDeletionError
		})
		imageDeleter := NewImageDeleter(imageClient)
		err := imageDeleter.DeleteImage(&imagev1.Image{ObjectMeta: metav1.ObjectMeta{Name: "sha256:0000000000000000000000000000000000000000000000000000000000000002"}})
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
		images                     imagev1.ImageList
		streams                    imagev1.ImageStreamList
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
				testutil.Stream("registry1.io", "foo", "bar", []imagev1.NamedTagEventList{
					testutil.Tag("latest",
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000002", "registry1.io/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000002"),
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000001", "registry1.io/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000001"),
					),
				}),
				testutil.Stream("registry1.io", "foo", "other", []imagev1.NamedTagEventList{
					testutil.Tag("latest",
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000002", "registry1.io/foo/other@sha256:0000000000000000000000000000000000000000000000000000000000000002"),
					),
				}),
			),
			expectedLayerLinkDeletions: sets.NewString(
				"https://registry1.io|foo/bar|"+testutil.Config1,
				"https://registry1.io|foo/bar|layer1",
				"https://registry1.io|foo/bar|layer2",
			),
			expectedBlobDeletions: sets.NewString(
				"https://registry1.io|sha256:0000000000000000000000000000000000000000000000000000000000000001",
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
				testutil.Stream("registry1.io", "foo", "bar", []imagev1.NamedTagEventList{
					testutil.Tag("latest",
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000001", "registry1.io/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000001"),
					),
				}),
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
				"https://registry1.io|sha256:0000000000000000000000000000000000000000000000000000000000000001",
				"https://registry1.io|sha256:0000000000000000000000000000000000000000000000000000000000000002",
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
				testutil.Stream("registry1.io", "foo", "bar", []imagev1.NamedTagEventList{
					testutil.Tag("latest",
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000002", "registry1.io/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000002"),
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000001", "registry1.io/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000001"),
					),
				}),
				testutil.Stream("registry1.io", "foo", "other", []imagev1.NamedTagEventList{
					testutil.Tag("latest",
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000003", "registry1.io/foo/other@sha256:0000000000000000000000000000000000000000000000000000000000000003"),
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000002", "registry1.io/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000002"),
					),
				}),
			),
			expectedLayerLinkDeletions: sets.NewString(
				"https://registry1.io|foo/bar|layer1",
				"https://registry1.io|foo/bar|layer2",
			),
			expectedBlobDeletions: sets.NewString(
				"https://registry1.io|sha256:0000000000000000000000000000000000000000000000000000000000000001",
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
				testutil.Stream("registry1.io", "foo", "bar", []imagev1.NamedTagEventList{
					testutil.Tag("latest",
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000002", "registry1.io/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000002"),
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000001", "registry1.io/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000001"),
					),
				}),
				testutil.Stream("registry1.io", "foo", "other", []imagev1.NamedTagEventList{
					testutil.Tag("latest",
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000003", "registry1.io/foo/other@sha256:0000000000000000000000000000000000000000000000000000000000000003"),
						testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000002", "registry1.io/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000002"),
					),
				}),
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
				ImageWatcher:     watch.NewFake(),
				Streams:          &test.streams,
				StreamWatcher:    watch.NewFake(),
				Pods:             &corev1.PodList{},
				RCs:              &corev1.ReplicationControllerList{},
				BCs:              &buildv1.BuildConfigList{},
				Builds:           &buildv1.BuildList{},
				DSs:              &kappsv1.DaemonSetList{},
				Deployments:      &kappsv1.DeploymentList{},
				DCs:              &appsv1.DeploymentConfigList{},
				RSs:              &kappsv1.ReplicaSetList{},
				RegistryClientFactory: FakeRegistryClientFactory,
				RegistryURL:           &url.URL{Scheme: "https", Host: "registry1.io"},
			}
			p, err := NewPruner(options)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			_, imageDeleterFactory := newFakeImageDeleter(nil)
			streamDeleter := &fakeImageStreamDeleter{invocations: sets.NewString()}
			layerLinkDeleter := &fakeLayerLinkDeleter{invocations: sets.NewString()}
			blobDeleter := &fakeBlobDeleter{invocations: sets.NewString()}
			manifestDeleter := &fakeManifestDeleter{invocations: sets.NewString()}

			p.Prune(imageDeleterFactory, streamDeleter, layerLinkDeleter, blobDeleter, manifestDeleter)

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
		testutil.Stream("registry1", "foo", "bar", []imagev1.NamedTagEventList{
			testutil.Tag("latest",
				testutil.TagEvent("0000000000000000000000000000000000000000000000000000000000000003", "registry1.io/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000003"),
				testutil.TagEvent("0000000000000000000000000000000000000000000000000000000000000002", "registry1.io/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000002"),
				testutil.TagEvent("0000000000000000000000000000000000000000000000000000000000000001", "registry1.io/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000001"),
			),
			testutil.Tag("strong",
				testutil.TagEvent("0000000000000000000000000000000000000000000000000000000000000001", "registry1.io/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000001"),
			),
		}),
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
		Images:        &images,
		ImageWatcher:  watch.NewFake(),
		Streams:       &streams,
		StreamWatcher: watch.NewFake(),
		Pods:          &pods,
		RCs:           &rcs,
		BCs:           &bcs,
		Builds:        &builds,
		DSs:           &dss,
		Deployments:   &deployments,
		DCs:           &dcs,
		RSs:           &rss,
	}
	keepYoungerThan := 24 * time.Hour
	keepTagRevisions := 2
	options.KeepYoungerThan = &keepYoungerThan
	options.KeepTagRevisions = &keepTagRevisions
	p, err := NewPruner(options)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	imageDeleter, imageDeleterFactory := newFakeImageDeleter(nil)
	streamDeleter := &fakeImageStreamDeleter{invocations: sets.NewString()}
	layerLinkDeleter := &fakeLayerLinkDeleter{invocations: sets.NewString()}
	blobDeleter := &fakeBlobDeleter{invocations: sets.NewString()}
	manifestDeleter := &fakeManifestDeleter{invocations: sets.NewString()}

	deletions, failures := p.Prune(imageDeleterFactory, streamDeleter, layerLinkDeleter, blobDeleter, manifestDeleter)
	if len(failures) != 0 {
		t.Errorf("got unexpected failures: %#+v", failures)
	}

	if len(deletions) > 0 {
		t.Fatalf("got unexpected deletions: %#+v", deletions)
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
	imageNode := imagegraph.EnsureImageNode(g, &imagev1.Image{ObjectMeta: metav1.ObjectMeta{Name: "myImage"}})
	streamNode := imagegraph.EnsureImageStreamNode(g, &imagev1.ImageStream{ObjectMeta: metav1.ObjectMeta{Name: "myStream"}})
	g.AddEdge(streamNode, imageNode, ReferencedImageEdgeKind)
	g.AddEdge(streamNode, imageNode, WeakReferencedImageEdgeKind)

	if imageIsPrunable(g, imageNode.(*imagegraph.ImageNode), pruneAlgorithm{}) {
		t.Fatalf("Image is prunable although it should not")
	}
}

func TestPrunerGetNextJob(t *testing.T) {
	flag.Lookup("v").Value.Set(fmt.Sprint(*logLevel))

	glog.V(2).Infof("debug")
	algo := pruneAlgorithm{
		keepYoungerThan: time.Now(),
	}
	p := &pruner{algorithm: algo, processedImages: make(map[*imagegraph.ImageNode]*Job)}
	images := testutil.ImageList(
		testutil.AgedImage("sha256:0000000000000000000000000000000000000000000000000000000000000003", "registry1.io/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000003", 1, "layer1"),
		testutil.AgedImage("sha256:0000000000000000000000000000000000000000000000000000000000000002", "registry1.io/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000002", 2, "layer1", "layer2"),
		testutil.AgedImage("sha256:0000000000000000000000000000000000000000000000000000000000000001", "registry1.io/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000001", 3, "Layer1", "Layer2", "Layer3"),
		testutil.AgedImage("sha256:0000000000000000000000000000000000000000000000000000000000000013", "registry1.io/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000013", 4, "Layer1", "LayeR2", "LayeR3"),
		testutil.AgedImage("sha256:0000000000000000000000000000000000000000000000000000000000000012", "registry1.io/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000012", 5, "LayeR1", "LayeR2"),
		testutil.AgedImage("sha256:0000000000000000000000000000000000000000000000000000000000000011", "registry1.io/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000011", 6, "layer1", "Layer2", "LAYER3", "LAYER4"),
		testutil.AgedImage("sha256:0000000000000000000000000000000000000000000000000000000000000010", "registry1.io/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000010", 7, "layer1", "layer2", "layer3", "layer4"),
	)
	p.g = genericgraph.New()
	err := p.addImagesToGraph(&images)
	if err != nil {
		t.Fatalf("failed to add images: %v", err)
	}

	is := images.Items
	imageStreams := testutil.StreamList(
		testutil.Stream("example.com", "foo", "bar", []imagev1.NamedTagEventList{
			testutil.Tag("latest",
				testutil.TagEvent(is[3].Name, is[3].DockerImageReference),
				testutil.TagEvent(is[4].Name, is[4].DockerImageReference),
				testutil.TagEvent(is[5].Name, is[5].DockerImageReference))}),
		testutil.Stream("example.com", "foo", "baz", []imagev1.NamedTagEventList{
			testutil.Tag("devel",
				testutil.TagEvent(is[3].Name, is[3].DockerImageReference),
				testutil.TagEvent(is[2].Name, is[2].DockerImageReference),
				testutil.TagEvent(is[1].Name, is[1].DockerImageReference)),
			testutil.Tag("prod",
				testutil.TagEvent(is[2].Name, is[2].DockerImageReference))}))
	if err := p.addImageStreamsToGraph(&imageStreams, nil); err != nil {
		t.Fatalf("failed to add image streams: %v", err)
	}

	imageNodes := getImageNodes(p.g.Nodes())
	if len(imageNodes) == 0 {
		t.Fatalf("not images nodes")
	}
	prunable := calculatePrunableImages(p.g, imageNodes, algo)
	sort.Sort(byLayerCountAndAge(prunable))
	p.queue = makeQueue(prunable)

	checkQueue := func(desc string, expected ...*imagev1.Image) {
		for i, item := 0, p.queue; i < len(expected) || item != nil; i++ {
			if i >= len(expected) {
				t.Errorf("[%s] unexpected image at #%d: %s", desc, i, item.node.Image.Name)
			} else if item == nil {
				t.Errorf("[%s] expected image %q not found at #%d", desc, expected[i].Name, i)
			} else if item.node.Image.Name != expected[i].Name {
				t.Errorf("[%s] unexpected image at #%d: %s != %s", desc, i, item.node.Image.Name, expected[i].Name)
			}
			if item != nil {
				item = item.next
			}
		}
		if t.Failed() {
			t.FailNow()
		}
	}

	/* layerrefs: layer1:4, Layer1:2, LayeR1:1, layer2:2, Layer2:2, LayeR2:2,
	 * layer3:1, Layer3:1, LayeR3:1, LAYER3:1, layer4:1, LAYER4:1 */
	checkQueue("initial state", &is[6], &is[5], &is[3], &is[2], &is[4], &is[1], &is[0])
	job := expectBlockedOrJob(t, p, "pop first", false, &is[6], []string{"layer4", "layer3"})(p.getNextJob())
	p.processedImages[job.Image] = job
	imgnd6 := job.Image

	/* layerrefs: layer1:3, Layer1:2, LayeR1:1, layer2:1, Layer2:2, LayeR2:2,
	 * layer3:0, Layer3:1, LayeR3:1, LAYER3:1, layer4:0, LAYER4:1 */
	checkQueue("1 removed", &is[5], &is[3], &is[2], &is[4], &is[1], &is[0])
	job = expectBlockedOrJob(t, p, "pop second", false, &is[5], []string{"LAYER3", "LAYER4"})(p.getNextJob())
	p.processedImages[job.Image] = job
	imgnd5 := job.Image

	/* layerrefs: layer1:2, Layer1:2, LayeR1:1, layer2:1, Layer2:1, LayeR2:2,
	 * Layer3:1, LayeR3:1, LAYER3:0, LAYER4:0 */
	checkQueue("2 removed", &is[3], &is[2], &is[4], &is[1], &is[0])
	job = expectBlockedOrJob(t, p, "pop third", false, &is[3], []string{"LayeR3"})(p.getNextJob())
	p.processedImages[job.Image] = job
	imgnd3 := job.Image

	// layerrefs: layer1:2, Layer1:1, LayeR1:1, layer2:1, Layer2:1, LayeR2:1, Layer3:1, LayeR3:0
	checkQueue("3 removed", &is[2], &is[4], &is[1], &is[0])
	// all the remaining images are blocked now except for the is[0]
	job = expectBlockedOrJob(t, p, "pop fourth", false, &is[0], nil)(p.getNextJob())
	p.processedImages[job.Image] = job
	imgnd0 := job.Image

	// layerrefs: layer1:1, Layer1:1, LayeR1:1, layer2:1, Layer2:1, LayeR2:1, Layer3:1
	checkQueue("4 removed and blocked", &is[2], &is[4], &is[1])
	// all the remaining images are blocked now
	expectBlockedOrJob(t, p, "blocked", true, nil, nil)(p.getNextJob())

	// layerrefs: layer1:1, Layer1:2, LayeR1:1, layer2:1, Layer2:1, LayeR2:1, Layer3:1
	checkQueue("3 to go", &is[2], &is[4], &is[1])
	// unblock one of the images
	p.g.RemoveNode(imgnd3)
	job = expectBlockedOrJob(t, p, "pop fifth", false, &is[4],
		[]string{"LayeR1", "LayeR2"})(p.getNextJob())
	p.processedImages[job.Image] = job
	imgnd4 := job.Image

	// layerrefs: layer1:1, Layer1:2, LayeR1:0, layer2:1, Layer2:1, LayeR2:0, Layer3:1
	checkQueue("2 to go", &is[2], &is[1])
	expectBlockedOrJob(t, p, "blocked with two items#1", true, nil, nil)(p.getNextJob())
	checkQueue("still 2 to go", &is[2], &is[1])

	p.g.RemoveNode(imgnd0)
	delete(p.processedImages, imgnd0)
	expectBlockedOrJob(t, p, "blocked with two items#2", true, nil, nil)(p.getNextJob())
	p.g.RemoveNode(imgnd6)
	delete(p.processedImages, imgnd6)
	expectBlockedOrJob(t, p, "blocked with two items#3", true, nil, nil)(p.getNextJob())
	p.g.RemoveNode(imgnd4)
	delete(p.processedImages, imgnd4)
	expectBlockedOrJob(t, p, "blocked with two items#4", true, nil, nil)(p.getNextJob())
	p.g.RemoveNode(imgnd5)
	delete(p.processedImages, imgnd5)

	job = expectBlockedOrJob(t, p, "pop sixth", false, &is[2],
		[]string{"Layer1", "Layer2", "Layer3"})(p.getNextJob())
	p.processedImages[job.Image] = job

	// layerrefs: layer1:1, Layer1:0, layer2:1, Layer2:0, Layer3:0
	checkQueue("1 to go", &is[1])
	job = expectBlockedOrJob(t, p, "pop last", false, &is[1],
		[]string{"layer1", "layer2"})(p.getNextJob())
	p.processedImages[job.Image] = job

	// layerrefs: layer1:0, layer2:0
	checkQueue("queue empty")
	expectBlockedOrJob(t, p, "empty", false, nil, nil)(p.getNextJob())
}

func expectBlockedOrJob(
	t *testing.T,
	p *pruner,
	desc string,
	blocked bool,
	image *imagev1.Image,
	layers []string,
) func(job *Job, blocked bool) *Job {
	return func(job *Job, b bool) *Job {
		if b != blocked {
			t.Fatalf("[%s] unexpected blocked: %t != %t", desc, b, blocked)
		}

		if blocked {
			return job
		}

		if image == nil && job != nil {
			t.Fatalf("[%s] got unexpected job %#+v", desc, job)
		}
		if image != nil && job == nil {
			t.Fatalf("[%s] got nil instead of job", desc)
		}
		if job == nil {
			return nil
		}

		if a, e := job.Image.Image.Name, image.Name; a != e {
			t.Errorf("[%s] unexpected image in job: %s != %s", desc, a, e)
		}

		expLayers := sets.NewString(imagegraph.EnsureImageComponentManifestNode(
			p.g, job.Image.Image.Name).(*imagegraph.ImageComponentNode).String())
		for _, l := range layers {
			expLayers.Insert(imagegraph.EnsureImageComponentLayerNode(
				p.g, l).(*imagegraph.ImageComponentNode).String())
		}
		actLayers := sets.NewString()
		for c, ret := range job.Components {
			if ret.PrunableGlobally {
				actLayers.Insert(c.String())
			}
		}
		if a, e := actLayers, expLayers; !reflect.DeepEqual(a, e) {
			t.Errorf("[%s] unexpected image components: %s", desc, diff.ObjectDiff(a.List(), e.List()))
		}

		if t.Failed() {
			t.FailNow()
		}

		return job
	}
}

func TestChangeImageStreamsWhilePruning(t *testing.T) {
	t.Skip("failed after commenting out")
	flag.Lookup("v").Value.Set(fmt.Sprint(*logLevel))

	images := testutil.ImageList(
		testutil.AgedImage("sha256:0000000000000000000000000000000000000000000000000000000000000001", "registry1.io/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000001", 5),
		testutil.AgedImage("sha256:0000000000000000000000000000000000000000000000000000000000000002", "registry1.io/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000002", 4),
		testutil.AgedImage("sha256:0000000000000000000000000000000000000000000000000000000000000003", "registry1.io/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000003", 3),
		testutil.AgedImage("sha256:0000000000000000000000000000000000000000000000000000000000000004", "registry1.io/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000003", 2),
		testutil.AgedImage("sha256:0000000000000000000000000000000000000000000000000000000000000005", "registry1.io/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000003", 1),
	)

	streams := testutil.StreamList(testutil.Stream("registry1", "foo", "bar", []imagev1.NamedTagEventList{}))
	streamWatcher := watch.NewFake()
	pods := testutil.PodList()
	rcs := testutil.RCList()
	bcs := testutil.BCList()
	builds := testutil.BuildList()
	dss := testutil.DSList()
	deployments := testutil.DeploymentList()
	dcs := testutil.DCList()
	rss := testutil.RSList()

	options := PrunerOptions{
		Images:        &images,
		ImageWatcher:  watch.NewFake(),
		Streams:       &streams,
		StreamWatcher: streamWatcher,
		Pods:          &pods,
		RCs:           &rcs,
		BCs:           &bcs,
		Builds:        &builds,
		DSs:           &dss,
		Deployments:   &deployments,
		DCs:           &dcs,
		RSs:           &rss,
		RegistryClientFactory: FakeRegistryClientFactory,
		RegistryURL:           &url.URL{Scheme: "https", Host: "registry1.io"},
		NumWorkers:            1,
	}
	keepYoungerThan := 30 * time.Second
	keepTagRevisions := 2
	options.KeepYoungerThan = &keepYoungerThan
	options.KeepTagRevisions = &keepTagRevisions
	p, err := NewPruner(options)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	pruneFinished := make(chan struct{})
	deletions, failures := []Deletion{}, []Failure{}
	imageDeleter, imageDeleterFactory := newBlockingImageDeleter(t)

	// run the pruning loop in a go routine
	go func() {
		deletions, failures = p.Prune(
			imageDeleterFactory,
			&fakeImageStreamDeleter{invocations: sets.NewString()},
			&fakeLayerLinkDeleter{invocations: sets.NewString()},
			&fakeBlobDeleter{invocations: sets.NewString()},
			&fakeManifestDeleter{invocations: sets.NewString()},
		)
		if len(failures) != 0 {
			t.Errorf("got unexpected failures: %#+v", failures)
		}
		close(pruneFinished)
	}()

	expectedImageDeletions := sets.NewString()
	expectedBlobDeletions := sets.NewString()

	img := imageDeleter.waitForRequest()
	if a, e := img.Name, images.Items[0].Name; a != e {
		t.Fatalf("got unexpected image deletion request: %s != %s", a, e)
	}
	expectedImageDeletions.Insert(images.Items[0].Name)
	expectedBlobDeletions.Insert("registry1|" + images.Items[0].Name)

	// let the pruner wait for reply and meanwhile reference an image with a new image stream
	stream := testutil.Stream("registry1", "foo", "new", []imagev1.NamedTagEventList{
		testutil.Tag("latest",
			testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000002", "registry1/foo/new@sha256:0000000000000000000000000000000000000000000000000000000000000002"),
		)})
	streamWatcher.Add(&stream)
	imageDeleter.unblock()

	// the pruner shall skip the newly referenced image
	img = imageDeleter.waitForRequest()
	if a, e := img.Name, images.Items[2].Name; a != e {
		t.Fatalf("got unexpected image deletion request: %s != %s", a, e)
	}
	expectedImageDeletions.Insert(images.Items[2].Name)
	expectedBlobDeletions.Insert("registry1|" + images.Items[2].Name)

	// now lets modify the existing image stream to reference some more images
	stream = testutil.Stream("registry1", "foo", "bar", []imagev1.NamedTagEventList{
		testutil.Tag("latest",
			testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000000", "registry1/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000"),
			testutil.TagEvent("sha256:0000000000000000000000000000000000000000000000000000000000000004", "registry1/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000004"),
		)})
	streamWatcher.Modify(&stream)
	imageDeleter.unblock()

	// the pruner shall skip the newly referenced image
	img = imageDeleter.waitForRequest()
	if a, e := img.Name, images.Items[4].Name; a != e {
		t.Fatalf("got unexpected image deletion request: %s != %s", a, e)
	}
	expectedImageDeletions.Insert(images.Items[4].Name)
	expectedBlobDeletions.Insert("registry1|" + images.Items[4].Name)
	imageDeleter.unblock()

	// no more images - wait for the pruner to finish
	select {
	case <-pruneFinished:
	case <-time.After(time.Second):
		t.Errorf("tester: timeout while waiting for pruner to finish")
	}

	if a, e := imageDeleter.d.invocations, expectedImageDeletions; !reflect.DeepEqual(a, e) {
		t.Errorf("unexpected image deletions: %s", diff.ObjectDiff(a, e))
	}

	expectedAllDeletions := sets.NewString(
		append(expectedImageDeletions.List(), expectedBlobDeletions.List()...)...)
	for _, d := range deletions {
		rendered, _, isManifestLinkDeletion := renderDeletion("registry1", &d)
		if isManifestLinkDeletion {
			// TODO: update tests to count and verify the number of manifest link deletions
			continue
		}
		if expectedAllDeletions.Has(rendered) {
			expectedAllDeletions.Delete(rendered)
		} else {
			t.Errorf("got unexpected deletion: %#+v (rendered: %q)", d, rendered)
		}
	}
	for del, ok := expectedAllDeletions.PopAny(); ok; del, ok = expectedAllDeletions.PopAny() {
		t.Errorf("expected deletion %q did not happen", del)
	}
}

func streamListToClient(list *imagev1.ImageStreamList) imagev1client.ImageStreamsGetter {
	streams := make([]runtime.Object, 0, len(list.Items))
	for i := range list.Items {
		streams = append(streams, &list.Items[i])
	}

	return &fakeimagev1client.FakeImageV1{Fake: &(fakeimageclient.NewSimpleClientset(streams...).Fake)}
}

func keepTagRevisions(n int) *int {
	return &n
}

type fakeImageDeleter struct {
	mutex       sync.Mutex
	invocations sets.String
	err         error
}

var _ ImageDeleter = &fakeImageDeleter{}

func (p *fakeImageDeleter) DeleteImage(image *imagev1.Image) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.invocations.Insert(image.Name)
	return p.err
}

func newFakeImageDeleter(err error) (*fakeImageDeleter, ImagePrunerFactoryFunc) {
	deleter := &fakeImageDeleter{
		err:         err,
		invocations: sets.NewString(),
	}
	return deleter, func() (ImageDeleter, error) {
		return deleter, nil
	}
}

type blockingImageDeleter struct {
	t        *testing.T
	d        *fakeImageDeleter
	requests chan *imagev1.Image
	reply    chan struct{}
}

func (bid *blockingImageDeleter) DeleteImage(img *imagev1.Image) error {
	bid.requests <- img
	select {
	case <-bid.reply:
	case <-time.After(time.Second):
		bid.t.Fatalf("worker: timeout while waiting for image deletion confirmation")
	}
	return bid.d.DeleteImage(img)
}

func (bid *blockingImageDeleter) waitForRequest() *imagev1.Image {
	select {
	case img := <-bid.requests:
		return img
	case <-time.After(time.Second):
		bid.t.Fatalf("tester: timeout while waiting on worker's request")
		return nil
	}
}

func (bid *blockingImageDeleter) unblock() {
	bid.reply <- struct{}{}
}

func newBlockingImageDeleter(t *testing.T) (*blockingImageDeleter, ImagePrunerFactoryFunc) {
	deleter, _ := newFakeImageDeleter(nil)
	blocking := blockingImageDeleter{
		t:        t,
		d:        deleter,
		requests: make(chan *imagev1.Image),
		reply:    make(chan struct{}),
	}
	return &blocking, func() (ImageDeleter, error) {
		return &blocking, nil
	}
}

type fakeImageStreamDeleter struct {
	mutex        sync.Mutex
	invocations  sets.String
	err          error
	streamImages map[string][]string
	streamTags   map[string][]string
}

var _ ImageStreamDeleter = &fakeImageStreamDeleter{}

func (p *fakeImageStreamDeleter) GetImageStream(stream *imagev1.ImageStream) (*imagev1.ImageStream, error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	if p.streamImages == nil {
		p.streamImages = make(map[string][]string)
	}
	if p.streamTags == nil {
		p.streamTags = make(map[string][]string)
	}
	for _, tag := range stream.Status.Tags {
		streamName := fmt.Sprintf("%s/%s", stream.Namespace, stream.Name)
		p.streamTags[streamName] = append(p.streamTags[streamName], tag.Tag)

		for _, tagEvent := range tag.Items {
			p.streamImages[streamName] = append(p.streamImages[streamName], tagEvent.Image)
		}
	}
	return stream, p.err
}

func (p *fakeImageStreamDeleter) UpdateImageStream(stream *imagev1.ImageStream) (*imagev1.ImageStream, error) {
	streamImages := make(map[string]struct{})
	streamTags := make(map[string]struct{})

	for _, tag := range stream.Status.Tags {
		streamTags[tag.Tag] = struct{}{}
		for _, tagEvent := range tag.Items {
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

func (p *fakeImageStreamDeleter) NotifyImageStreamPrune(stream *imagev1.ImageStream, updatedTags []string, deletedTags []string) {
	return
}

type errorForSHA func(dgst string) error

type fakeBlobDeleter struct {
	mutex       sync.Mutex
	invocations sets.String
	getError    errorForSHA
}

var _ BlobDeleter = &fakeBlobDeleter{}

func (p *fakeBlobDeleter) DeleteBlob(registryClient *http.Client, registryURL *url.URL, blob string) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.invocations.Insert(fmt.Sprintf("%s|%s", registryURL.String(), blob))
	if p.getError == nil {
		return nil
	}
	return p.getError(blob)
}

type fakeLayerLinkDeleter struct {
	mutex       sync.Mutex
	invocations sets.String
	err         error
}

var _ LayerLinkDeleter = &fakeLayerLinkDeleter{}

func (p *fakeLayerLinkDeleter) DeleteLayerLink(registryClient *http.Client, registryURL *url.URL, repo, layer string) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.invocations.Insert(fmt.Sprintf("%s|%s|%s", registryURL.String(), repo, layer))
	return p.err
}

type fakeManifestDeleter struct {
	mutex       sync.Mutex
	invocations sets.String
	err         error
}

var _ ManifestDeleter = &fakeManifestDeleter{}

func (p *fakeManifestDeleter) DeleteManifest(registryClient *http.Client, registryURL *url.URL, repo, manifest string) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.invocations.Insert(fmt.Sprintf("%s|%s|%s", registryURL.String(), repo, manifest))
	return p.err
}
