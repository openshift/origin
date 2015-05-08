package prune

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/testclient"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	buildapi "github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/client"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	"github.com/openshift/origin/pkg/dockerregistry/server"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

func imageList(images ...imageapi.Image) imageapi.ImageList {
	return imageapi.ImageList{
		Items: images,
	}
}

func agedImage(id, ref string, ageInMinutes int64) imageapi.Image {
	image := imageWithLayers(id, ref,
		"tarsum.dev+sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		"tarsum.dev+sha256:b194de3772ebbcdc8f244f663669799ac1cb141834b7cb8b69100285d357a2b0",
		"tarsum.dev+sha256:c937c4bb1c1a21cc6d94340812262c6472092028972ae69b551b1a70d4276171",
		"tarsum.dev+sha256:2aaacc362ac6be2b9e9ae8c6029f6f616bb50aec63746521858e47841b90fabd",
		"tarsum.dev+sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
	)

	if ageInMinutes >= 0 {
		image.CreationTimestamp = util.NewTime(util.Now().Add(time.Duration(-1*ageInMinutes) * time.Minute))
	}

	return image
}

func image(id, ref string) imageapi.Image {
	return agedImage(id, ref, -1)
}

func imageWithLayers(id, ref string, layers ...string) imageapi.Image {
	image := imageapi.Image{
		ObjectMeta: kapi.ObjectMeta{
			Name: id,
			Annotations: map[string]string{
				imageapi.ManagedByOpenShiftAnnotation: "true",
			},
		},
		DockerImageReference: ref,
	}

	manifest := imageapi.DockerImageManifest{
		FSLayers: []imageapi.DockerFSLayer{},
	}

	for _, layer := range layers {
		manifest.FSLayers = append(manifest.FSLayers, imageapi.DockerFSLayer{DockerBlobSum: layer})
	}

	manifestBytes, err := json.Marshal(&manifest)
	if err != nil {
		panic(err)
	}

	image.DockerImageManifest = string(manifestBytes)

	return image
}

func unmanagedImage(id, ref string, hasAnnotations bool, annotation, value string) imageapi.Image {
	image := imageWithLayers(id, ref)
	if !hasAnnotations {
		image.Annotations = nil
	} else {
		delete(image.Annotations, imageapi.ManagedByOpenShiftAnnotation)
		image.Annotations[annotation] = value
	}
	return image
}

func imageWithBadManifest(id, ref string) imageapi.Image {
	image := image(id, ref)
	image.DockerImageManifest = "asdf"
	return image
}

func podList(pods ...kapi.Pod) kapi.PodList {
	return kapi.PodList{
		Items: pods,
	}
}

func pod(namespace, name string, phase kapi.PodPhase, containerImages ...string) kapi.Pod {
	return agedPod(namespace, name, phase, -1, containerImages...)
}

func agedPod(namespace, name string, phase kapi.PodPhase, ageInMinutes int64, containerImages ...string) kapi.Pod {
	pod := kapi.Pod{
		ObjectMeta: kapi.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Spec: podSpec(containerImages...),
		Status: kapi.PodStatus{
			Phase: phase,
		},
	}

	if ageInMinutes >= 0 {
		pod.CreationTimestamp = util.NewTime(util.Now().Add(time.Duration(-1*ageInMinutes) * time.Minute))
	}

	return pod
}

func podSpec(containerImages ...string) kapi.PodSpec {
	spec := kapi.PodSpec{
		Containers: []kapi.Container{},
	}
	for _, image := range containerImages {
		container := kapi.Container{
			Image: image,
		}
		spec.Containers = append(spec.Containers, container)
	}
	return spec
}

func streamList(streams ...imageapi.ImageStream) imageapi.ImageStreamList {
	return imageapi.ImageStreamList{
		Items: streams,
	}
}

func stream(registry, namespace, name string, tags map[string]imageapi.TagEventList) imageapi.ImageStream {
	return agedStream(registry, namespace, name, -1, tags)
}

func agedStream(registry, namespace, name string, ageInMinutes int64, tags map[string]imageapi.TagEventList) imageapi.ImageStream {
	stream := imageapi.ImageStream{
		ObjectMeta: kapi.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Status: imageapi.ImageStreamStatus{
			DockerImageRepository: fmt.Sprintf("%s/%s/%s", registry, namespace, name),
			Tags: tags,
		},
	}

	if ageInMinutes >= 0 {
		stream.CreationTimestamp = util.NewTime(util.Now().Add(time.Duration(-1*ageInMinutes) * time.Minute))
	}

	return stream
}

func streamPtr(registry, namespace, name string, tags map[string]imageapi.TagEventList) *imageapi.ImageStream {
	s := stream(registry, namespace, name, tags)
	return &s
}

func tags(list ...namedTagEventList) map[string]imageapi.TagEventList {
	m := make(map[string]imageapi.TagEventList, len(list))
	for _, tag := range list {
		m[tag.name] = tag.events
	}
	return m
}

type namedTagEventList struct {
	name   string
	events imageapi.TagEventList
}

func tag(name string, events ...imageapi.TagEvent) namedTagEventList {
	return namedTagEventList{
		name: name,
		events: imageapi.TagEventList{
			Items: events,
		},
	}
}

func tagEvent(id, ref string) imageapi.TagEvent {
	return imageapi.TagEvent{
		Image:                id,
		DockerImageReference: ref,
	}
}

func rcList(rcs ...kapi.ReplicationController) kapi.ReplicationControllerList {
	return kapi.ReplicationControllerList{
		Items: rcs,
	}
}

func rc(namespace, name string, containerImages ...string) kapi.ReplicationController {
	return kapi.ReplicationController{
		ObjectMeta: kapi.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Spec: kapi.ReplicationControllerSpec{
			Template: &kapi.PodTemplateSpec{
				Spec: podSpec(containerImages...),
			},
		},
	}
}

func dcList(dcs ...deployapi.DeploymentConfig) deployapi.DeploymentConfigList {
	return deployapi.DeploymentConfigList{
		Items: dcs,
	}
}

func dc(namespace, name string, containerImages ...string) deployapi.DeploymentConfig {
	return deployapi.DeploymentConfig{
		ObjectMeta: kapi.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Template: deployapi.DeploymentTemplate{
			ControllerTemplate: kapi.ReplicationControllerSpec{
				Template: &kapi.PodTemplateSpec{
					Spec: podSpec(containerImages...),
				},
			},
		},
	}
}

func bcList(bcs ...buildapi.BuildConfig) buildapi.BuildConfigList {
	return buildapi.BuildConfigList{
		Items: bcs,
	}
}

func bc(namespace, name string, strategyType buildapi.BuildStrategyType, fromKind, fromNamespace, fromName string) buildapi.BuildConfig {
	return buildapi.BuildConfig{
		ObjectMeta: kapi.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Parameters: buildParameters(strategyType, fromKind, fromNamespace, fromName),
	}
}

func buildList(builds ...buildapi.Build) buildapi.BuildList {
	return buildapi.BuildList{
		Items: builds,
	}
}

func build(namespace, name string, strategyType buildapi.BuildStrategyType, fromKind, fromNamespace, fromName string) buildapi.Build {
	return buildapi.Build{
		ObjectMeta: kapi.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Parameters: buildParameters(strategyType, fromKind, fromNamespace, fromName),
	}
}

func buildParameters(strategyType buildapi.BuildStrategyType, fromKind, fromNamespace, fromName string) buildapi.BuildParameters {
	params := buildapi.BuildParameters{
		Strategy: buildapi.BuildStrategy{
			Type: strategyType,
		},
	}
	switch strategyType {
	case buildapi.STIBuildStrategyType:
		params.Strategy.STIStrategy = &buildapi.STIBuildStrategy{
			From: &kapi.ObjectReference{
				Kind:      fromKind,
				Namespace: fromNamespace,
				Name:      fromName,
			},
		}
	case buildapi.DockerBuildStrategyType:
		params.Strategy.DockerStrategy = &buildapi.DockerBuildStrategy{
			From: &kapi.ObjectReference{
				Kind:      fromKind,
				Namespace: fromNamespace,
				Name:      fromName,
			},
		}
	case buildapi.CustomBuildStrategyType:
		params.Strategy.CustomStrategy = &buildapi.CustomBuildStrategy{
			From: &kapi.ObjectReference{
				Kind:      fromKind,
				Namespace: fromNamespace,
				Name:      fromName,
			},
		}
	}

	return params
}

var logLevel = flag.Int("loglevel", 0, "")
var testCase = flag.String("testcase", "", "")

func TestImagePruning(t *testing.T) {
	flag.Lookup("v").Value.Set(fmt.Sprint(*logLevel))
	registryURL := "registry"

	tests := map[string]struct {
		registryURLs           []string
		images                 imageapi.ImageList
		pods                   kapi.PodList
		streams                imageapi.ImageStreamList
		rcs                    kapi.ReplicationControllerList
		bcs                    buildapi.BuildConfigList
		builds                 buildapi.BuildList
		dcs                    deployapi.DeploymentConfigList
		expectedDeletions      []string
		expectedUpdatedStreams []string
	}{
		"1 pod - phase pending - don't prune": {
			images:            imageList(image("id", registryURL+"/foo/bar@id")),
			pods:              podList(pod("foo", "pod1", kapi.PodPending, registryURL+"/foo/bar@id")),
			expectedDeletions: []string{},
		},
		"3 pods - last phase pending - don't prune": {
			images: imageList(image("id", registryURL+"/foo/bar@id")),
			pods: podList(
				pod("foo", "pod1", kapi.PodSucceeded, registryURL+"/foo/bar@id"),
				pod("foo", "pod2", kapi.PodSucceeded, registryURL+"/foo/bar@id"),
				pod("foo", "pod3", kapi.PodPending, registryURL+"/foo/bar@id"),
			),
			expectedDeletions: []string{},
		},
		"1 pod - phase running - don't prune": {
			images:            imageList(image("id", registryURL+"/foo/bar@id")),
			pods:              podList(pod("foo", "pod1", kapi.PodRunning, registryURL+"/foo/bar@id")),
			expectedDeletions: []string{},
		},
		"3 pods - last phase running - don't prune": {
			images: imageList(image("id", registryURL+"/foo/bar@id")),
			pods: podList(
				pod("foo", "pod1", kapi.PodSucceeded, registryURL+"/foo/bar@id"),
				pod("foo", "pod2", kapi.PodSucceeded, registryURL+"/foo/bar@id"),
				pod("foo", "pod3", kapi.PodRunning, registryURL+"/foo/bar@id"),
			),
			expectedDeletions: []string{},
		},
		"pod phase succeeded - prune": {
			images:            imageList(image("id", registryURL+"/foo/bar@id")),
			pods:              podList(pod("foo", "pod1", kapi.PodSucceeded, registryURL+"/foo/bar@id")),
			expectedDeletions: []string{"id"},
		},
		"pod phase succeeded, pod less than min pruning age - don't prune": {
			images:            imageList(image("id", registryURL+"/foo/bar@id")),
			pods:              podList(agedPod("foo", "pod1", kapi.PodSucceeded, 5, registryURL+"/foo/bar@id")),
			expectedDeletions: []string{},
		},
		"pod phase succeeded, image less than min pruning age - don't prune": {
			images:            imageList(agedImage("id", registryURL+"/foo/bar@id", 5)),
			pods:              podList(pod("foo", "pod1", kapi.PodSucceeded, registryURL+"/foo/bar@id")),
			expectedDeletions: []string{},
		},
		"pod phase failed - prune": {
			images: imageList(image("id", registryURL+"/foo/bar@id")),
			pods: podList(
				pod("foo", "pod1", kapi.PodFailed, registryURL+"/foo/bar@id"),
				pod("foo", "pod2", kapi.PodFailed, registryURL+"/foo/bar@id"),
				pod("foo", "pod3", kapi.PodFailed, registryURL+"/foo/bar@id"),
			),
			expectedDeletions: []string{"id"},
		},
		"pod phase unknown - prune": {
			images: imageList(image("id", registryURL+"/foo/bar@id")),
			pods: podList(
				pod("foo", "pod1", kapi.PodUnknown, registryURL+"/foo/bar@id"),
				pod("foo", "pod2", kapi.PodUnknown, registryURL+"/foo/bar@id"),
				pod("foo", "pod3", kapi.PodUnknown, registryURL+"/foo/bar@id"),
			),
			expectedDeletions: []string{"id"},
		},
		"pod container image not parsable": {
			images: imageList(image("id", registryURL+"/foo/bar@id")),
			pods: podList(
				pod("foo", "pod1", kapi.PodRunning, "a/b/c/d/e"),
			),
			expectedDeletions: []string{"id"},
		},
		"pod container image doesn't have an id": {
			images: imageList(image("id", registryURL+"/foo/bar@id")),
			pods: podList(
				pod("foo", "pod1", kapi.PodRunning, "foo/bar:latest"),
			),
			expectedDeletions: []string{"id"},
		},
		"pod refers to image not in graph": {
			images: imageList(image("id", registryURL+"/foo/bar@id")),
			pods: podList(
				pod("foo", "pod1", kapi.PodRunning, registryURL+"/foo/bar@otherid"),
			),
			expectedDeletions: []string{"id"},
		},
		"referenced by rc - don't prune": {
			images:            imageList(image("id", registryURL+"/foo/bar@id")),
			rcs:               rcList(rc("foo", "rc1", registryURL+"/foo/bar@id")),
			expectedDeletions: []string{},
		},
		"referenced by dc - don't prune": {
			images:            imageList(image("id", registryURL+"/foo/bar@id")),
			dcs:               dcList(dc("foo", "rc1", registryURL+"/foo/bar@id")),
			expectedDeletions: []string{},
		},
		"referenced by bc - sti - ImageStreamImage - don't prune": {
			images:            imageList(image("id", registryURL+"/foo/bar@id")),
			bcs:               bcList(bc("foo", "bc1", buildapi.STIBuildStrategyType, "ImageStreamImage", "foo", "bar@id")),
			expectedDeletions: []string{},
		},
		"referenced by bc - docker - ImageStreamImage - don't prune": {
			images:            imageList(image("id", registryURL+"/foo/bar@id")),
			bcs:               bcList(bc("foo", "bc1", buildapi.DockerBuildStrategyType, "ImageStreamImage", "foo", "bar@id")),
			expectedDeletions: []string{},
		},
		"referenced by bc - custom - ImageStreamImage - don't prune": {
			images:            imageList(image("id", registryURL+"/foo/bar@id")),
			bcs:               bcList(bc("foo", "bc1", buildapi.CustomBuildStrategyType, "ImageStreamImage", "foo", "bar@id")),
			expectedDeletions: []string{},
		},
		"referenced by bc - sti - DockerImage - don't prune": {
			images:            imageList(image("id", registryURL+"/foo/bar@id")),
			bcs:               bcList(bc("foo", "bc1", buildapi.STIBuildStrategyType, "DockerImage", "foo", registryURL+"/foo/bar@id")),
			expectedDeletions: []string{},
		},
		"referenced by bc - docker - DockerImage - don't prune": {
			images:            imageList(image("id", registryURL+"/foo/bar@id")),
			bcs:               bcList(bc("foo", "bc1", buildapi.DockerBuildStrategyType, "DockerImage", "foo", registryURL+"/foo/bar@id")),
			expectedDeletions: []string{},
		},
		"referenced by bc - custom - DockerImage - don't prune": {
			images:            imageList(image("id", registryURL+"/foo/bar@id")),
			bcs:               bcList(bc("foo", "bc1", buildapi.CustomBuildStrategyType, "DockerImage", "foo", registryURL+"/foo/bar@id")),
			expectedDeletions: []string{},
		},
		"referenced by build - sti - ImageStreamImage - don't prune": {
			images:            imageList(image("id", registryURL+"/foo/bar@id")),
			builds:            buildList(build("foo", "build1", buildapi.STIBuildStrategyType, "ImageStreamImage", "foo", "bar@id")),
			expectedDeletions: []string{},
		},
		"referenced by build - docker - ImageStreamImage - don't prune": {
			images:            imageList(image("id", registryURL+"/foo/bar@id")),
			builds:            buildList(build("foo", "build1", buildapi.DockerBuildStrategyType, "ImageStreamImage", "foo", "bar@id")),
			expectedDeletions: []string{},
		},
		"referenced by build - custom - ImageStreamImage - don't prune": {
			images:            imageList(image("id", registryURL+"/foo/bar@id")),
			builds:            buildList(build("foo", "build1", buildapi.CustomBuildStrategyType, "ImageStreamImage", "foo", "bar@id")),
			expectedDeletions: []string{},
		},
		"referenced by build - sti - DockerImage - don't prune": {
			images:            imageList(image("id", registryURL+"/foo/bar@id")),
			builds:            buildList(build("foo", "build1", buildapi.STIBuildStrategyType, "DockerImage", "foo", registryURL+"/foo/bar@id")),
			expectedDeletions: []string{},
		},
		"referenced by build - docker - DockerImage - don't prune": {
			images:            imageList(image("id", registryURL+"/foo/bar@id")),
			builds:            buildList(build("foo", "build1", buildapi.DockerBuildStrategyType, "DockerImage", "foo", registryURL+"/foo/bar@id")),
			expectedDeletions: []string{},
		},
		"referenced by build - custom - DockerImage - don't prune": {
			images:            imageList(image("id", registryURL+"/foo/bar@id")),
			builds:            buildList(build("foo", "build1", buildapi.CustomBuildStrategyType, "DockerImage", "foo", registryURL+"/foo/bar@id")),
			expectedDeletions: []string{},
		},
		"image stream - keep most recent n images": {
			images: imageList(
				unmanagedImage("id", "otherregistry/foo/bar@id", false, "", ""),
				image("id2", registryURL+"/foo/bar@id2"),
				image("id3", registryURL+"/foo/bar@id3"),
				image("id4", registryURL+"/foo/bar@id4"),
			),
			streams: streamList(
				stream(registryURL, "foo", "bar", tags(
					tag("latest",
						tagEvent("id", "otherregistry/foo/bar@id"),
						tagEvent("id2", registryURL+"/foo/bar@id2"),
						tagEvent("id3", registryURL+"/foo/bar@id3"),
						tagEvent("id4", registryURL+"/foo/bar@id4"),
					),
				)),
			),
			expectedDeletions:      []string{"id4"},
			expectedUpdatedStreams: []string{"foo/bar"},
		},
		"image stream age less than min pruning age - don't prune": {
			images: imageList(
				image("id", registryURL+"/foo/bar@id"),
				image("id2", registryURL+"/foo/bar@id2"),
				image("id3", registryURL+"/foo/bar@id3"),
				image("id4", registryURL+"/foo/bar@id4"),
			),
			streams: streamList(
				agedStream(registryURL, "foo", "bar", 5, tags(
					tag("latest",
						tagEvent("id", registryURL+"/foo/bar@id"),
						tagEvent("id2", registryURL+"/foo/bar@id2"),
						tagEvent("id3", registryURL+"/foo/bar@id3"),
						tagEvent("id4", registryURL+"/foo/bar@id4"),
					),
				)),
			),
			expectedDeletions:      []string{},
			expectedUpdatedStreams: []string{},
		},
		"multiple resources pointing to image - don't prune": {
			images: imageList(
				image("id", registryURL+"/foo/bar@id"),
				image("id2", registryURL+"/foo/bar@id2"),
			),
			streams: streamList(
				stream(registryURL, "foo", "bar", tags(
					tag("latest",
						tagEvent("id", registryURL+"/foo/bar@id"),
						tagEvent("id2", registryURL+"/foo/bar@id2"),
					),
				)),
			),
			rcs:                    rcList(rc("foo", "rc1", registryURL+"/foo/bar@id2")),
			pods:                   podList(pod("foo", "pod1", kapi.PodRunning, registryURL+"/foo/bar@id2")),
			dcs:                    dcList(dc("foo", "rc1", registryURL+"/foo/bar@id")),
			bcs:                    bcList(bc("foo", "bc1", buildapi.STIBuildStrategyType, "DockerImage", "foo", registryURL+"/foo/bar@id")),
			builds:                 buildList(build("foo", "build1", buildapi.CustomBuildStrategyType, "ImageStreamImage", "foo", "bar@id")),
			expectedDeletions:      []string{},
			expectedUpdatedStreams: []string{},
		},
		"image with nil annotations": {
			images: imageList(
				unmanagedImage("id", "someregistry/foo/bar@id", false, "", ""),
			),
			expectedDeletions:      []string{},
			expectedUpdatedStreams: []string{},
		},
		"image missing managed annotation": {
			images: imageList(
				unmanagedImage("id", "someregistry/foo/bar@id", true, "foo", "bar"),
			),
			expectedDeletions:      []string{},
			expectedUpdatedStreams: []string{},
		},
		"image with managed annotation != true": {
			images: imageList(
				unmanagedImage("id", "someregistry/foo/bar@id", true, imageapi.ManagedByOpenShiftAnnotation, "false"),
				unmanagedImage("id", "someregistry/foo/bar@id", true, imageapi.ManagedByOpenShiftAnnotation, "0"),
				unmanagedImage("id", "someregistry/foo/bar@id", true, imageapi.ManagedByOpenShiftAnnotation, "1"),
				unmanagedImage("id", "someregistry/foo/bar@id", true, imageapi.ManagedByOpenShiftAnnotation, "True"),
				unmanagedImage("id", "someregistry/foo/bar@id", true, imageapi.ManagedByOpenShiftAnnotation, "yes"),
				unmanagedImage("id", "someregistry/foo/bar@id", true, imageapi.ManagedByOpenShiftAnnotation, "Yes"),
			),
			expectedDeletions:      []string{},
			expectedUpdatedStreams: []string{},
		},
		"image with bad manifest is pruned ok": {
			images: imageList(
				imageWithBadManifest("id", "someregistry/foo/bar@id"),
			),
			expectedDeletions:      []string{"id"},
			expectedUpdatedStreams: []string{},
		},
	}

	for name, test := range tests {
		tcFilter := flag.Lookup("testcase").Value.String()
		if len(tcFilter) > 0 && name != tcFilter {
			continue
		}
		p := newImagePruner(60*time.Minute, 3, &test.images, &test.streams, &test.pods, &test.rcs, &test.bcs, &test.builds, &test.dcs)
		actualDeletions := util.NewStringSet()
		actualUpdatedStreams := util.NewStringSet()

		imagePruneFunc := func(image *imageapi.Image, streams []*imageapi.ImageStream) []error {
			actualDeletions.Insert(image.Name)
			for _, stream := range streams {
				actualUpdatedStreams.Insert(fmt.Sprintf("%s/%s", stream.Namespace, stream.Name))
			}
			return []error{}
		}

		layerPruneFunc := func(registryURL string, req server.DeleteLayersRequest) (error, map[string][]error) {
			return nil, map[string][]error{}
		}

		p.Run(imagePruneFunc, layerPruneFunc)

		expectedDeletions := util.NewStringSet(test.expectedDeletions...)
		if !reflect.DeepEqual(expectedDeletions, actualDeletions) {
			t.Errorf("%s: expected image deletions %q, got %q", name, expectedDeletions.List(), actualDeletions.List())
		}

		expectedUpdatedStreams := util.NewStringSet(test.expectedUpdatedStreams...)
		if !reflect.DeepEqual(expectedUpdatedStreams, actualUpdatedStreams) {
			t.Errorf("%s: expected stream updates %q, got %q", name, expectedUpdatedStreams.List(), actualUpdatedStreams.List())
		}
	}
}

func TestDeletingImagePruneFunc(t *testing.T) {
	registryURL := "registry"

	tests := map[string]struct {
		referencedStreams  []*imageapi.ImageStream
		expectedUpdates    []*imageapi.ImageStream
		imageDeletionError error
		streamUpdateError  error
	}{
		"no referenced streams": {
			referencedStreams: []*imageapi.ImageStream{},
			expectedUpdates:   []*imageapi.ImageStream{},
		},
		"1 tag, 1 image revision": {
			referencedStreams: []*imageapi.ImageStream{
				streamPtr(registryURL, "foo", "bar", tags(
					tag("latest",
						tagEvent("id", "registry/foo/bar@id"),
					),
				)),
			},
			expectedUpdates: []*imageapi.ImageStream{},
		},
		"1 tag, multiple image revisions": {
			referencedStreams: []*imageapi.ImageStream{
				streamPtr(registryURL, "foo", "bar", tags(
					tag("latest",
						tagEvent("id", "registry/foo/bar@id"),
						tagEvent("id2", "registry/foo/bar@id2"),
					),
				)),
			},
			expectedUpdates: []*imageapi.ImageStream{
				streamPtr(registryURL, "foo", "bar", tags(
					tag("latest",
						tagEvent("id", "registry/foo/bar@id"),
					),
				)),
			},
		},
		"image deletion error": {
			referencedStreams: []*imageapi.ImageStream{
				streamPtr(registryURL, "foo", "bar", tags(
					tag("latest",
						tagEvent("id", "registry/foo/bar@id"),
					),
				)),
			},
			imageDeletionError: fmt.Errorf("foo"),
		},
		"stream update error": {
			referencedStreams: []*imageapi.ImageStream{
				streamPtr(registryURL, "foo", "bar", tags(
					tag("latest",
						tagEvent("id", "registry/foo/bar@id"),
					),
				)),
				streamPtr(registryURL, "bar", "baz", tags(
					tag("latest",
						tagEvent("id", "registry/foo/bar@id"),
					),
				)),
			},
			streamUpdateError: fmt.Errorf("foo"),
		},
	}

	for name, test := range tests {
		imageClient := client.Fake{
			Err: test.imageDeletionError,
		}
		streamClient := client.Fake{
			Err: test.streamUpdateError,
		}
		pruneFunc := DeletingImagePruneFunc(imageClient.Images(), &streamClient)
		errs := pruneFunc(&imageapi.Image{ObjectMeta: kapi.ObjectMeta{Name: "id2"}}, test.referencedStreams)
		if test.imageDeletionError != nil {
			if e, a := 1, len(errs); e != a {
				t.Errorf("%s: # of errors: expected %v, got %v", name, e, a)
				continue
			}
			if e, a := fmt.Sprintf("Error deleting image: %v", test.imageDeletionError), errs[0].Error(); e != a {
				t.Errorf("%s: errs: expected %v, got %v", name, e, a)
			}
			continue
		}

		if test.streamUpdateError != nil {
			if e, a := len(test.referencedStreams), len(errs); e != a {
				t.Errorf("%s: # of errors: expected %v, got %v", name, e, a)
				continue
			}
			for i, stream := range test.referencedStreams {
				if e, a := fmt.Sprintf("Unable to update image stream status %s/%s: %v", stream.Namespace, stream.Name, test.streamUpdateError), errs[i].Error(); e != a {
					t.Errorf("%s: errs: expected %v, got %v", name, e, a)
				}
			}
			continue
		}

		if len(imageClient.Actions) < 1 {
			t.Fatalf("%s: expected image deletion", name)
		}

		if e, a := len(test.referencedStreams), len(streamClient.Actions); e != a {
			t.Errorf("%s: expected %d stream updates, got %d", name, e, a)
		}

		for i := range test.expectedUpdates {
			if e, a := "update-status-imagestream", streamClient.Actions[i].Action; e != a {
				t.Errorf("%s: unexpected action %q", name, a)
			}
			updatedStream := streamClient.Actions[i].Value.(*imageapi.ImageStream)
			if e, a := test.expectedUpdates[i], updatedStream; !reflect.DeepEqual(e, a) {
				t.Errorf("%s: unexpected updated stream: %s", name, util.ObjectDiff(e, a))
			}
		}
	}
}

func TestLayerPruning(t *testing.T) {
	flag.Lookup("v").Value.Set(fmt.Sprint(*logLevel))
	registryURL := "registry1"

	tests := map[string]struct {
		images                imageapi.ImageList
		streams               imageapi.ImageStreamList
		expectedDeletions     map[string]util.StringSet
		expectedStreamUpdates map[string]util.StringSet
	}{
		"layers unique to id1 pruned": {
			images: imageList(
				imageWithLayers("id1", "registry1/foo/bar@id1", "layer1", "layer2", "layer3", "layer4"),
				imageWithLayers("id2", "registry1/foo/bar@id2", "layer3", "layer4", "layer5", "layer6"),
			),
			streams: streamList(
				stream(registryURL, "foo", "bar", tags(
					tag("latest",
						tagEvent("id2", registryURL+"/foo/bar@id2"),
						tagEvent("id1", registryURL+"/foo/bar@id1"),
					),
				)),
				stream(registryURL, "foo", "other", tags(
					tag("latest",
						tagEvent("id2", registryURL+"/foo/other@id2"),
					),
				)),
			),
			expectedDeletions: map[string]util.StringSet{
				"registry1": util.NewStringSet("layer1", "layer2"),
			},
			expectedStreamUpdates: map[string]util.StringSet{
				"registry1": util.NewStringSet("foo/bar"),
			},
		},
		"no pruning when no images are pruned": {
			images: imageList(
				imageWithLayers("id1", "registry1/foo/bar@id1", "layer1", "layer2", "layer3", "layer4"),
			),
			streams: streamList(
				stream(registryURL, "foo", "bar", tags(
					tag("latest",
						tagEvent("id1", registryURL+"/foo/bar@id1"),
					),
				)),
			),
			expectedDeletions:     map[string]util.StringSet{},
			expectedStreamUpdates: map[string]util.StringSet{},
		},
	}

	for name, test := range tests {
		actualDeletions := map[string]util.StringSet{}
		actualUpdatedStreams := map[string]util.StringSet{}

		imagePruneFunc := func(image *imageapi.Image, streams []*imageapi.ImageStream) []error {
			return []error{}
		}

		layerPruneFunc := func(registryURL string, req server.DeleteLayersRequest) (error, map[string][]error) {
			registryDeletions, ok := actualDeletions[registryURL]
			if !ok {
				registryDeletions = util.NewStringSet()
			}
			streamUpdates, ok := actualUpdatedStreams[registryURL]
			if !ok {
				streamUpdates = util.NewStringSet()
			}

			for layer, streams := range req {
				registryDeletions.Insert(layer)
				streamUpdates.Insert(streams...)
			}

			actualDeletions[registryURL] = registryDeletions
			actualUpdatedStreams[registryURL] = streamUpdates

			return nil, map[string][]error{}
		}

		p := newImagePruner(60, 1, &test.images, &test.streams, &kapi.PodList{}, &kapi.ReplicationControllerList{}, &buildapi.BuildConfigList{}, &buildapi.BuildList{}, &deployapi.DeploymentConfigList{})

		p.Run(imagePruneFunc, layerPruneFunc)

		if !reflect.DeepEqual(test.expectedDeletions, actualDeletions) {
			t.Errorf("%s: expected layer deletions %#v, got %#v", name, test.expectedDeletions, actualDeletions)
		}

		if !reflect.DeepEqual(test.expectedStreamUpdates, actualUpdatedStreams) {
			t.Errorf("%s: expected stream updates %q, got %q", name, test.expectedStreamUpdates, actualUpdatedStreams)
		}
	}
}

func TestNewImagePruner(t *testing.T) {
	osFake := &client.Fake{}

	kFake := &testclient.Fake{}
	p, err := NewImagePruner(60, 3, osFake, osFake, kFake, kFake, osFake, osFake, osFake)
	if err != nil {
		t.Fatalf("unexpected error creating image pruner: %v", err)
	}
	if p == nil {
		t.Fatalf("unexpected nil pruner")
	}

	seen := util.NewStringSet()
	for _, action := range osFake.Actions {
		seen.Insert(action.Action)
	}
	for _, action := range kFake.Actions {
		seen.Insert(action.Action)
	}

	expected := util.NewStringSet(
		"list-images",
		"list-imagestreams",
		"list-pods",
		"list-replicationControllers",
		"list-buildconfig",
		"list-builds",
		"list-deploymentconfig",
	)

	if e, a := expected, seen; !reflect.DeepEqual(e, a) {
		t.Errorf("Expected actions=%v, got: %v", e.List(), a.List())
	}
}

func TestDeletingLayerPruneFunc(t *testing.T) {
	tests := map[string]struct {
		simulateClientError        bool
		registryResponseStatusCode int
		registryResponse           string
		expectedRequestError       string
		expectedErrors             []string
	}{
		"client error": {
			simulateClientError:  true,
			expectedRequestError: "Error sending request:",
		},
		"non-200 response": {
			registryResponseStatusCode: http.StatusInternalServerError,
			expectedRequestError:       fmt.Sprintf("Unexpected status code %d in response", http.StatusInternalServerError),
			registryResponse:           "{}",
		},
		"error unmarshaling response body": {
			registryResponseStatusCode: http.StatusOK,
			registryResponse:           "foo",
			expectedRequestError:       "Error unmarshaling response:",
		},
		"happy path - no response errors": {
			registryResponseStatusCode: http.StatusOK,
			registryResponse:           `{"result":"success"}`,
			expectedErrors:             []string{},
		},
		"happy path - with response errors": {
			registryResponseStatusCode: http.StatusOK,
			registryResponse:           `{"result":"failure","errors":{"layer1":["error1","error2","error3"]}}`,
			expectedErrors:             []string{"error1", "error2", "error3"},
		},
	}

	for name, test := range tests {
		client := http.DefaultClient

		testServer := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.WriteHeader(test.registryResponseStatusCode)
			w.Write([]byte(test.registryResponse))
		}))
		registry := testServer.Listener.Addr().String()

		if !test.simulateClientError {
			testServer.Start()
			defer testServer.Close()
		} else {
			registry = "noregistryhere!"
		}

		pruneFunc := DeletingLayerPruneFunc(client)

		deletions := server.DeleteLayersRequest{
			"layer1": {"aaa/stream1", "bbb/stream2"},
		}

		requestError, layerErrors := pruneFunc(registry, deletions)

		gotError := requestError != nil
		expectError := len(test.expectedRequestError) != 0
		if e, a := expectError, gotError; e != a {
			t.Errorf("%s: requestError: expected %t, got %t: %v", name, e, a, requestError)
			continue
		}
		if gotError {
			if e, a := test.expectedRequestError, requestError; !strings.HasPrefix(a.Error(), e) {
				t.Errorf("%s: expected request error %q, got %q", name, e, a)
			}
		}

		errs := layerErrors["layer1"]
		if e, a := len(test.expectedErrors), len(errs); e != a {
			t.Errorf("%s: expected %d errors (%v), got %d (%v)", name, e, test.expectedErrors, a, errs)
			continue
		}
		for i, e := range test.expectedErrors {
			a := errs[i].Error()
			if !strings.HasPrefix(a, e) {
				t.Errorf("%s: expected error starting with %q, got %q", name, e, a)
			}
		}
	}
}
