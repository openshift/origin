package prune

import (
	"encoding/json"
	"flag"
	"fmt"
	"reflect"
	"testing"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	ktc "k8s.io/kubernetes/pkg/client/unversioned/testclient"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util"
	"k8s.io/kubernetes/pkg/util/sets"

	buildapi "github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/client/testclient"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

type fakeRegistryPinger struct {
	err      error
	requests []string
}

func (f *fakeRegistryPinger) ping(registry string) error {
	f.requests = append(f.requests, registry)
	return f.err
}

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
		Spec: buildapi.BuildConfigSpec{
			BuildSpec: buildSpec(strategyType, fromKind, fromNamespace, fromName),
		},
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
		Spec: buildSpec(strategyType, fromKind, fromNamespace, fromName),
	}
}

func buildSpec(strategyType buildapi.BuildStrategyType, fromKind, fromNamespace, fromName string) buildapi.BuildSpec {
	spec := buildapi.BuildSpec{
		Strategy: buildapi.BuildStrategy{
			Type: strategyType,
		},
	}
	switch strategyType {
	case buildapi.SourceBuildStrategyType:
		spec.Strategy.SourceStrategy = &buildapi.SourceBuildStrategy{
			From: kapi.ObjectReference{
				Kind:      fromKind,
				Namespace: fromNamespace,
				Name:      fromName,
			},
		}
	case buildapi.DockerBuildStrategyType:
		spec.Strategy.DockerStrategy = &buildapi.DockerBuildStrategy{
			From: &kapi.ObjectReference{
				Kind:      fromKind,
				Namespace: fromNamespace,
				Name:      fromName,
			},
		}
	case buildapi.CustomBuildStrategyType:
		spec.Strategy.CustomStrategy = &buildapi.CustomBuildStrategy{
			From: kapi.ObjectReference{
				Kind:      fromKind,
				Namespace: fromNamespace,
				Name:      fromName,
			},
		}
	}

	return spec
}

type fakeImagePruner struct {
	invocations sets.String
	err         error
}

var _ ImagePruner = &fakeImagePruner{}

func (p *fakeImagePruner) PruneImage(image *imageapi.Image) error {
	p.invocations.Insert(image.Name)
	return p.err
}

type fakeImageStreamPruner struct {
	invocations sets.String
	err         error
}

var _ ImageStreamPruner = &fakeImageStreamPruner{}

func (p *fakeImageStreamPruner) PruneImageStream(stream *imageapi.ImageStream, image *imageapi.Image, updatedTags []string) (*imageapi.ImageStream, error) {
	p.invocations.Insert(fmt.Sprintf("%s/%s|%s", stream.Namespace, stream.Name, image.Name))
	return stream, p.err
}

type fakeBlobPruner struct {
	invocations sets.String
	err         error
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
			bcs:               bcList(bc("foo", "bc1", buildapi.SourceBuildStrategyType, "ImageStreamImage", "foo", "bar@id")),
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
			bcs:               bcList(bc("foo", "bc1", buildapi.SourceBuildStrategyType, "DockerImage", "foo", registryURL+"/foo/bar@id")),
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
			builds:            buildList(build("foo", "build1", buildapi.SourceBuildStrategyType, "ImageStreamImage", "foo", "bar@id")),
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
			builds:            buildList(build("foo", "build1", buildapi.SourceBuildStrategyType, "DockerImage", "foo", registryURL+"/foo/bar@id")),
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
			expectedUpdatedStreams: []string{"foo/bar|id4"},
		},
		"image stream - same manifest listed multiple times in tag history": {
			images: imageList(
				image("id1", registryURL+"/foo/bar@id1"),
				image("id2", registryURL+"/foo/bar@id2"),
			),
			streams: streamList(
				stream(registryURL, "foo", "bar", tags(
					tag("latest",
						tagEvent("id1", registryURL+"/foo/bar@id1"),
						tagEvent("id2", registryURL+"/foo/bar@id2"),
						tagEvent("id1", registryURL+"/foo/bar@id1"),
						tagEvent("id2", registryURL+"/foo/bar@id2"),
					),
				)),
			),
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
			bcs:                    bcList(bc("foo", "bc1", buildapi.SourceBuildStrategyType, "DockerImage", "foo", registryURL+"/foo/bar@id")),
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

		options := ImageRegistryPrunerOptions{
			KeepYoungerThan:  60 * time.Minute,
			KeepTagRevisions: 3,
			Images:           &test.images,
			Streams:          &test.streams,
			Pods:             &test.pods,
			RCs:              &test.rcs,
			BCs:              &test.bcs,
			Builds:           &test.builds,
			DCs:              &test.dcs,
		}
		p := NewImageRegistryPruner(options)

		imagePruner := &fakeImagePruner{invocations: sets.NewString()}
		streamPruner := &fakeImageStreamPruner{invocations: sets.NewString()}

		p.Prune(imagePruner, streamPruner)

		expectedDeletions := sets.NewString(test.expectedDeletions...)
		if !reflect.DeepEqual(expectedDeletions, imagePruner.invocations) {
			t.Errorf("%s: expected image deletions %q, got %q", name, expectedDeletions.List(), imagePruner.invocations.List())
		}

		expectedUpdatedStreams := sets.NewString(test.expectedUpdatedStreams...)
		if !reflect.DeepEqual(expectedUpdatedStreams, streamPruner.invocations) {
			t.Errorf("%s: expected stream updates %q, got %q", name, expectedUpdatedStreams.List(), streamPruner.invocations.List())
		}
	}
}

func TestDeletingImagePruner(t *testing.T) {
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
		imageClient := testclient.Fake{}
		imageClient.AddReactor("delete", "images", func(action ktc.Action) (handled bool, ret runtime.Object, err error) {
			return true, nil, test.imageDeletionError
		})
		imagePruner := NewDeletingImagePruner(imageClient.Images())
		err := imagePruner.PruneImage(&imageapi.Image{ObjectMeta: kapi.ObjectMeta{Name: "id2"}})
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

func TestRegistryPruning(t *testing.T) {
	flag.Lookup("v").Value.Set(fmt.Sprint(*logLevel))

	tests := map[string]struct {
		images                    imageapi.ImageList
		streams                   imageapi.ImageStreamList
		expectedManifestDeletions sets.String
	}{
		"layers unique to id1 pruned": {
			images: imageList(
				imageWithLayers("id1", "registry1/foo/bar@id1", "layer1", "layer2", "layer3", "layer4"),
				imageWithLayers("id2", "registry1/foo/bar@id2", "layer3", "layer4", "layer5", "layer6"),
			),
			streams: streamList(
				stream("registry1", "foo", "bar", tags(
					tag("latest",
						tagEvent("id2", "registry1/foo/bar@id2"),
						tagEvent("id1", "registry1/foo/bar@id1"),
					),
				)),
				stream("registry1", "foo", "other", tags(
					tag("latest",
						tagEvent("id2", "registry1/foo/other@id2"),
					),
				)),
			),
		},
		"no pruning when no images are pruned": {
			images: imageList(
				imageWithLayers("id1", "registry1/foo/bar@id1", "layer1", "layer2", "layer3", "layer4"),
			),
			streams: streamList(
				stream("registry1", "foo", "bar", tags(
					tag("latest",
						tagEvent("id1", "registry1/foo/bar@id1"),
					),
				)),
			),
			expectedManifestDeletions: sets.NewString(),
		},
		"blobs pruned when streams have already been deleted": {
			images: imageList(
				imageWithLayers("id1", "registry1/foo/bar@id1", "layer1", "layer2", "layer3", "layer4"),
				imageWithLayers("id2", "registry1/foo/bar@id2", "layer3", "layer4", "layer5", "layer6"),
			),
		},
		"ping error": {
			images: imageList(
				imageWithLayers("id1", "registry1/foo/bar@id1", "layer1", "layer2", "layer3", "layer4"),
				imageWithLayers("id2", "registry1/foo/bar@id2", "layer3", "layer4", "layer5", "layer6"),
			),
			streams: streamList(
				stream("registry1", "foo", "bar", tags(
					tag("latest",
						tagEvent("id2", "registry1/foo/bar@id2"),
						tagEvent("id1", "registry1/foo/bar@id1"),
					),
				)),
				stream("registry1", "foo", "other", tags(
					tag("latest",
						tagEvent("id2", "registry1/foo/other@id2"),
					),
				)),
			),
		},
	}

	for name, test := range tests {
		tcFilter := flag.Lookup("testcase").Value.String()
		if len(tcFilter) > 0 && name != tcFilter {
			continue
		}

		t.Logf("Running test case %s", name)

		options := ImageRegistryPrunerOptions{
			KeepYoungerThan:  60 * time.Minute,
			KeepTagRevisions: 1,
			Images:           &test.images,
			Streams:          &test.streams,
			Pods:             &kapi.PodList{},
			RCs:              &kapi.ReplicationControllerList{},
			BCs:              &buildapi.BuildConfigList{},
			Builds:           &buildapi.BuildList{},
			DCs:              &deployapi.DeploymentConfigList{},
		}
		p := NewImageRegistryPruner(options)

		imagePruner := &fakeImagePruner{invocations: sets.NewString()}
		streamPruner := &fakeImageStreamPruner{invocations: sets.NewString()}

		p.Prune(imagePruner, streamPruner)
	}
}
