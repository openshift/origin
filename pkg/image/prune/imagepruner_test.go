package prune

import (
	"encoding/json"
	"flag"
	"fmt"
	"reflect"
	"testing"
	"time"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	buildapi "github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/dockerregistry"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
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
				image("id", registryURL+"/foo/bar@id"),
				image("id2", registryURL+"/foo/bar@id2"),
				image("id3", registryURL+"/foo/bar@id3"),
				image("id4", registryURL+"/foo/bar@id4"),
			),
			streams: streamList(
				stream(registryURL, "foo", "bar", tags(
					tag("latest",
						tagEvent("id", registryURL+"/foo/bar@id"),
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
			),
			streams: streamList(
				agedStream(registryURL, "foo", "bar", 5, tags(
					tag("latest",
						tagEvent("id", registryURL+"/foo/bar@id"),
						tagEvent("id2", registryURL+"/foo/bar@id2"),
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
	}

	for name, test := range tests {
		p := newImagePruner(60, 3, &test.images, &test.streams, &test.pods, &test.rcs, &test.bcs, &test.builds, &test.dcs)
		actualDeletions := util.NewStringSet()
		actualUpdatedStreams := util.NewStringSet()

		imagePruneFunc := func(image *imageapi.Image, streams []*imageapi.ImageStream) []error {
			actualDeletions.Insert(image.Name)
			for _, stream := range streams {
				actualUpdatedStreams.Insert(fmt.Sprintf("%s/%s", stream.Namespace, stream.Name))
			}
			return []error{}
		}

		layerPruneFunc := func(registryURL string, req dockerregistry.DeleteLayersRequest) []error {
			return []error{}
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

func TestDefaultImagePruneFunc(t *testing.T) {
	registryURL := "registry"

	tests := map[string]struct {
		referencedStreams []*imageapi.ImageStream
		expectedUpdates   []*imageapi.ImageStream
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
	}

	for name, test := range tests {
		fakeClient := client.Fake{}
		pruneFunc := DeletingImagePruneFunc(fakeClient.Images(), &fakeClient)
		err := pruneFunc(&imageapi.Image{ObjectMeta: kapi.ObjectMeta{Name: "id2"}}, test.referencedStreams)
		_ = err

		if len(fakeClient.Actions) < 1 {
			t.Fatalf("%s: expected image deletion", name)
		}

		if e, a := len(test.referencedStreams), len(fakeClient.Actions)-1; e != a {
			t.Errorf("%s: expected %d stream updates, got %d", name, e, a)
		}

		for i := range test.expectedUpdates {
			if e, a := "update-status-imagestream", fakeClient.Actions[i+1].Action; e != a {
				t.Errorf("%s: unexpected action %q", name, a)
			}
			updatedStream := fakeClient.Actions[i+1].Value.(*imageapi.ImageStream)
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
			),
			expectedDeletions: map[string]util.StringSet{
				"registry1": util.NewStringSet("layer1", "layer2"),
			},
		},
	}

	for name, test := range tests {
		actualDeletions := map[string]util.StringSet{}
		actualUpdatedStreams := map[string]util.StringSet{}

		imagePruneFunc := func(image *imageapi.Image, streams []*imageapi.ImageStream) []error {
			return []error{}
		}

		layerPruneFunc := func(registryURL string, req dockerregistry.DeleteLayersRequest) []error {
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

			return []error{}
		}

		p := newImagePruner(60, 1, &test.images, &test.streams, &kapi.PodList{}, &kapi.ReplicationControllerList{}, &buildapi.BuildConfigList{}, &buildapi.BuildList{}, &deployapi.DeploymentConfigList{})

		p.Run(imagePruneFunc, layerPruneFunc)

		if !reflect.DeepEqual(test.expectedDeletions, actualDeletions) {
			t.Errorf("%s: expected layer deletions %#v, got %#v", name, test.expectedDeletions, actualDeletions)
		}

		/*
			expectedUpdatedStreams := util.NewStringSet(test.expectedUpdatedStreams...)
			if !reflect.DeepEqual(expectedUpdatedStreams, actualUpdatedStreams) {
				t.Errorf("%s: expected stream updates %q, got %q", name, expectedUpdatedStreams.List(), actualUpdatedStreams.List())
			}
		*/

	}
}
