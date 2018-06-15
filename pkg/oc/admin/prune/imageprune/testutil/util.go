package testutil

import (
	"fmt"
	"time"

	"github.com/docker/distribution/manifest/schema1"
	"github.com/docker/distribution/manifest/schema2"

	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kapisext "k8s.io/kubernetes/pkg/apis/extensions"

	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
)

const (
	Layer1 = "tarsum.dev+sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	Layer2 = "tarsum.dev+sha256:b194de3772ebbcdc8f244f663669799ac1cb141834b7cb8b69100285d357a2b0"
	Layer3 = "tarsum.dev+sha256:c937c4bb1c1a21cc6d94340812262c6472092028972ae69b551b1a70d4276171"
	Layer4 = "tarsum.dev+sha256:2aaacc362ac6be2b9e9ae8c6029f6f616bb50aec63746521858e47841b90fabd"
	Layer5 = "tarsum.dev+sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
)

var (
	Config1 = "sha256:2b8fd9751c4c0f5dd266fcae00707e67a2545ef34f9a29354585f93dac906749"
	Config2 = "sha256:8ddc19f16526912237dd8af81971d5e4dd0587907234be2b83e249518d5b673f"
)

// ImageList turns the given images into ImageList.
func ImageList(images ...imageapi.Image) imageapi.ImageList {
	return imageapi.ImageList{
		Items: images,
	}
}

// AgedImage creates a test image with specified age.
func AgedImage(id, ref string, ageInMinutes int64) imageapi.Image {
	return CreatedImage(id, ref, time.Now().Add(time.Duration(ageInMinutes)*time.Minute*-1))
}

// CreatedImage creates a test image with the CreationTime set to the given timestamp.
func CreatedImage(id, ref string, created time.Time) imageapi.Image {
	image := ImageWithLayers(id, ref, nil, Layer1, Layer2, Layer3, Layer4, Layer5)
	image.CreationTimestamp = metav1.NewTime(created)
	return image
}

// SizedImage returns a test image of given size.
func SizedImage(id, ref string, size int64, configName *string) imageapi.Image {
	image := ImageWithLayers(id, ref, configName, Layer1, Layer2, Layer3, Layer4, Layer5)
	image.CreationTimestamp = metav1.NewTime(metav1.Now().Add(time.Duration(-1) * time.Minute))
	image.DockerImageMetadata.Size = size

	return image
}

// Image returns a default test image object 120 minutes old.
func Image(id, ref string) imageapi.Image {
	return AgedImage(id, ref, 120)
}

// Image returns a default test image referencing the given layers.
func ImageWithLayers(id, ref string, configName *string, layers ...string) imageapi.Image {
	image := imageapi.Image{
		ObjectMeta: metav1.ObjectMeta{
			Name: id,
			Annotations: map[string]string{
				imageapi.ManagedByOpenShiftAnnotation: "true",
			},
		},
		DockerImageReference:         ref,
		DockerImageManifestMediaType: schema1.MediaTypeManifest,
	}

	if configName != nil {
		image.DockerImageMetadata = imageapi.DockerImage{
			ID: *configName,
		}
		image.DockerImageConfig = fmt.Sprintf("{Digest: %s}", *configName)
		image.DockerImageManifestMediaType = schema2.MediaTypeManifest
	}

	image.DockerImageLayers = []imageapi.ImageLayer{}
	for _, layer := range layers {
		image.DockerImageLayers = append(image.DockerImageLayers, imageapi.ImageLayer{Name: layer})
	}

	return image
}

// UnmanagedImage creates a test image object lacking managed by OpenShift annotation.
func UnmanagedImage(id, ref string, hasAnnotations bool, annotation, value string) imageapi.Image {
	image := ImageWithLayers(id, ref, nil)
	if !hasAnnotations {
		image.Annotations = nil
	} else {
		delete(image.Annotations, imageapi.ManagedByOpenShiftAnnotation)
		image.Annotations[annotation] = value
	}
	return image
}

// PodList turns the given pods into PodList.
func PodList(pods ...kapi.Pod) kapi.PodList {
	return kapi.PodList{
		Items: pods,
	}
}

// Pod creates and returns a pod having the given docker image references.
func Pod(namespace, name string, phase kapi.PodPhase, containerImages ...string) kapi.Pod {
	return AgedPod(namespace, name, phase, -1, containerImages...)
}

// AgedPod creates and returns a pod of particular age.
func AgedPod(namespace, name string, phase kapi.PodPhase, ageInMinutes int64, containerImages ...string) kapi.Pod {
	pod := kapi.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
			SelfLink:  "/pod/" + name,
		},
		Spec: PodSpec(containerImages...),
		Status: kapi.PodStatus{
			Phase: phase,
		},
	}

	if ageInMinutes >= 0 {
		pod.CreationTimestamp = metav1.NewTime(metav1.Now().Add(time.Duration(-1*ageInMinutes) * time.Minute))
	}

	return pod
}

// PodSpec creates a pod specification having the given docker image references.
func PodSpec(containerImages ...string) kapi.PodSpec {
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

// StreamList turns the given streams into StreamList.
func StreamList(streams ...imageapi.ImageStream) imageapi.ImageStreamList {
	return imageapi.ImageStreamList{
		Items: streams,
	}
}

// Stream creates and returns a test ImageStream object 1 minute old
func Stream(registry, namespace, name string, tags map[string]imageapi.TagEventList) imageapi.ImageStream {
	return AgedStream(registry, namespace, name, -1, tags)
}

// Stream creates and returns a test ImageStream object of given age.
func AgedStream(registry, namespace, name string, ageInMinutes int64, tags map[string]imageapi.TagEventList) imageapi.ImageStream {
	stream := imageapi.ImageStream{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Status: imageapi.ImageStreamStatus{
			DockerImageRepository: fmt.Sprintf("%s/%s/%s", registry, namespace, name),
			Tags: tags,
		},
	}

	if ageInMinutes >= 0 {
		stream.CreationTimestamp = metav1.NewTime(metav1.Now().Add(time.Duration(-1*ageInMinutes) * time.Minute))
	}

	return stream
}

// Stream creates an ImageStream object and returns a pointer to it.
func StreamPtr(registry, namespace, name string, tags map[string]imageapi.TagEventList) *imageapi.ImageStream {
	s := Stream(registry, namespace, name, tags)
	return &s
}

// Tags creates a map of tags for image stream status.
func Tags(list ...namedTagEventList) map[string]imageapi.TagEventList {
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

// Tag creates tag entries for Tags function.
func Tag(name string, events ...imageapi.TagEvent) namedTagEventList {
	return namedTagEventList{
		name: name,
		events: imageapi.TagEventList{
			Items: events,
		},
	}
}

// TagEvent creates a TagEvent object.
func TagEvent(id, ref string) imageapi.TagEvent {
	return imageapi.TagEvent{
		Image:                id,
		DockerImageReference: ref,
	}
}

// YoungTagEvent creates a TagEvent with the given created timestamp.
func YoungTagEvent(id, ref string, created metav1.Time) imageapi.TagEvent {
	return imageapi.TagEvent{
		Image:                id,
		Created:              created,
		DockerImageReference: ref,
	}
}

// RCList turns the given replication controllers into RCList.
func RCList(rcs ...kapi.ReplicationController) kapi.ReplicationControllerList {
	return kapi.ReplicationControllerList{
		Items: rcs,
	}
}

// RC creates and returns a ReplicationController.
func RC(namespace, name string, containerImages ...string) kapi.ReplicationController {
	return kapi.ReplicationController{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
			SelfLink:  "/rc/" + name,
		},
		Spec: kapi.ReplicationControllerSpec{
			Template: &kapi.PodTemplateSpec{
				Spec: PodSpec(containerImages...),
			},
		},
	}
}

// DSList turns the given daemon sets into DaemonSetList.
func DSList(dss ...kapisext.DaemonSet) kapisext.DaemonSetList {
	return kapisext.DaemonSetList{
		Items: dss,
	}
}

// DS creates and returns a DaemonSet object.
func DS(namespace, name string, containerImages ...string) kapisext.DaemonSet {
	return kapisext.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
			SelfLink:  "/ds/" + name,
		},
		Spec: kapisext.DaemonSetSpec{
			Template: kapi.PodTemplateSpec{
				Spec: PodSpec(containerImages...),
			},
		},
	}
}

// DeploymentList turns the given deployments into DeploymentList.
func DeploymentList(deployments ...kapisext.Deployment) kapisext.DeploymentList {
	return kapisext.DeploymentList{
		Items: deployments,
	}
}

// Deployment creates and returns aDeployment object.
func Deployment(namespace, name string, containerImages ...string) kapisext.Deployment {
	return kapisext.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
			SelfLink:  "/deployment/" + name,
		},
		Spec: kapisext.DeploymentSpec{
			Template: kapi.PodTemplateSpec{
				Spec: PodSpec(containerImages...),
			},
		},
	}
}

// DCList turns the given deployment configs into DeploymentConfigList.
func DCList(dcs ...appsapi.DeploymentConfig) appsapi.DeploymentConfigList {
	return appsapi.DeploymentConfigList{
		Items: dcs,
	}
}

// DC creates and returns a DeploymentConfig object.
func DC(namespace, name string, containerImages ...string) appsapi.DeploymentConfig {
	return appsapi.DeploymentConfig{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
			SelfLink:  "/dc/" + name,
		},
		Spec: appsapi.DeploymentConfigSpec{
			Template: &kapi.PodTemplateSpec{
				Spec: PodSpec(containerImages...),
			},
		},
	}
}

// RSList turns the given replica set into ReplicaSetList.
func RSList(rss ...kapisext.ReplicaSet) kapisext.ReplicaSetList {
	return kapisext.ReplicaSetList{
		Items: rss,
	}
}

// RS creates and returns a ReplicaSet object.
func RS(namespace, name string, containerImages ...string) kapisext.ReplicaSet {
	return kapisext.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
			SelfLink:  "/rs/" + name,
		},
		Spec: kapisext.ReplicaSetSpec{
			Template: kapi.PodTemplateSpec{
				Spec: PodSpec(containerImages...),
			},
		},
	}
}

// BCList turns the given build configs into BuildConfigList.
func BCList(bcs ...buildapi.BuildConfig) buildapi.BuildConfigList {
	return buildapi.BuildConfigList{
		Items: bcs,
	}
}

// BC creates and returns a BuildConfig object.
func BC(namespace, name, strategyType, fromKind, fromNamespace, fromName string) buildapi.BuildConfig {
	return buildapi.BuildConfig{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
			SelfLink:  "/bc/" + name,
		},
		Spec: buildapi.BuildConfigSpec{
			CommonSpec: CommonSpec(strategyType, fromKind, fromNamespace, fromName),
		},
	}
}

// BuildList turns the given builds into BuildList.
func BuildList(builds ...buildapi.Build) buildapi.BuildList {
	return buildapi.BuildList{
		Items: builds,
	}
}

// Build creates and returns a Build object.
func Build(namespace, name, strategyType, fromKind, fromNamespace, fromName string) buildapi.Build {
	return buildapi.Build{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
			SelfLink:  "/build/" + name,
		},
		Spec: buildapi.BuildSpec{
			CommonSpec: CommonSpec(strategyType, fromKind, fromNamespace, fromName),
		},
	}
}

// LimitList turns the given limits into LimitRanges.
func LimitList(limits ...int64) []*kapi.LimitRange {
	list := make([]*kapi.LimitRange, len(limits))
	for _, limit := range limits {
		quantity := resource.NewQuantity(limit, resource.BinarySI)
		list = append(list, &kapi.LimitRange{
			Spec: kapi.LimitRangeSpec{
				Limits: []kapi.LimitRangeItem{
					{
						Type: imageapi.LimitTypeImage,
						Max: kapi.ResourceList{
							kapi.ResourceStorage: *quantity,
						},
					},
				},
			},
		})
	}
	return list
}

// CommonSpec creates and returns CommonSpec object.
func CommonSpec(strategyType, fromKind, fromNamespace, fromName string) buildapi.CommonSpec {
	spec := buildapi.CommonSpec{
		Strategy: buildapi.BuildStrategy{},
	}
	switch strategyType {
	case "source":
		spec.Strategy.SourceStrategy = &buildapi.SourceBuildStrategy{
			From: kapi.ObjectReference{
				Kind:      fromKind,
				Namespace: fromNamespace,
				Name:      fromName,
			},
		}
	case "docker":
		spec.Strategy.DockerStrategy = &buildapi.DockerBuildStrategy{
			From: &kapi.ObjectReference{
				Kind:      fromKind,
				Namespace: fromNamespace,
				Name:      fromName,
			},
		}
	case "custom":
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
