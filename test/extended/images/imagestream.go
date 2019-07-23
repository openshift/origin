package images

import (
	"encoding/json"
	"os"

	"k8s.io/apimachinery/pkg/runtime"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	g "github.com/onsi/ginkgo"
	appsv1 "github.com/openshift/api/apps/v1"
	"github.com/openshift/api/image/docker10"
	imagev1 "github.com/openshift/api/image/v1"
	"github.com/openshift/library-go/pkg/image/imageutil"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[Feature:Image][triggers][Serial] ImageStream API", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLI("imagestream-api", exutil.KubeConfigPath())

	g.It("TestImageStreamMappingCreate", func() {
		TestImageStreamMappingCreate(g.GinkgoT(), oc)
	})
	g.It("TestImageStreamWithoutDockerImageConfig", func() {
		TestImageStreamWithoutDockerImageConfig(g.GinkgoT(), oc)
	})
	g.It("TestImageStreamTagLifecycleHook", func() {
		TestImageStreamTagLifecycleHook(g.GinkgoT(), oc)
	})
})

func mockImageStream() *imagev1.ImageStream {
	return &imagev1.ImageStream{ObjectMeta: metav1.ObjectMeta{Name: "test"}}
}

func TestImageStreamMappingCreate(t g.GinkgoTInterface, oc *exutil.CLI) {
	clusterAdminImageClient := oc.AdminImageClient().ImageV1()
	stream := mockImageStream()

	expected, err := clusterAdminImageClient.ImageStreams(oc.Namespace()).Create(stream)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if expected.Name == "" {
		t.Errorf("Unexpected empty image Name %v", expected)
	}

	// create a mapping to an image that doesn't exist
	mapping := &imagev1.ImageStreamMapping{
		ObjectMeta: metav1.ObjectMeta{Name: stream.Name},
		Tag:        "newer",
		Image: imagev1.Image{
			ObjectMeta: metav1.ObjectMeta{
				Name: "image1",
			},
			DockerImageReference: "some/other/name",
		},
	}
	if _, err := clusterAdminImageClient.ImageStreamMappings(oc.Namespace()).Create(mapping); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// verify we can tag a second time with the same data, and nothing changes
	if _, err := clusterAdminImageClient.ImageStreamMappings(oc.Namespace()).Create(mapping); err != nil {
		t.Fatalf("unexpected non-error or type: %v", err)
	}

	// create an image directly
	image := &imagev1.Image{
		ObjectMeta: metav1.ObjectMeta{Name: "image2"},
		DockerImageMetadata: runtime.RawExtension{
			Object: &docker10.DockerImage{
				Config: &docker10.DockerConfig{
					Env: []string{"A=B"},
				},
			},
		},
	}
	if _, err := clusterAdminImageClient.Images().Create(image); err == nil {
		t.Error("unexpected non-error")
	}
	image.DockerImageReference = "some/other/name" // can reuse references across multiple images
	actual, err := clusterAdminImageClient.Images().Create(image)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if actual == nil || actual.Name != image.Name {
		t.Errorf("unexpected object: %#v", actual)
	}

	// verify that image stream mappings cannot mutate / overwrite the image (images are immutable)
	mapping = &imagev1.ImageStreamMapping{
		ObjectMeta: metav1.ObjectMeta{Name: stream.Name},
		Tag:        "newest",
		Image:      *image,
	}
	mapping.Image.DockerImageReference = "different"
	if _, err := clusterAdminImageClient.ImageStreamMappings(oc.Namespace()).Create(mapping); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	image, err = clusterAdminImageClient.Images().Get(image.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if image.DockerImageReference != "some/other/name" {
		t.Fatalf("image was unexpectedly mutated: %#v", image)
	}

	// ensure the correct tags are set
	updated, err := clusterAdminImageClient.ImageStreams(oc.Namespace()).Get(stream.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if updated.Spec.Tags != nil && len(updated.Spec.Tags) > 0 {
		t.Errorf("unexpected object: %#v", updated.Spec.Tags)
	}

	fromTag, err := clusterAdminImageClient.ImageStreamTags(oc.Namespace()).Get(stream.Name+":newer", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if fromTag.Name != "test:newer" || fromTag.Image.UID == "" || fromTag.Image.DockerImageReference != "some/other/name" {
		t.Errorf("unexpected object: %#v", fromTag)
	}

	fromTag, err = clusterAdminImageClient.ImageStreamTags(oc.Namespace()).Get(stream.Name+":newest", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if fromTag.Name != "test:newest" || fromTag.Image.UID == "" || fromTag.Image.DockerImageReference != "different" {
		t.Errorf("unexpected object: %#v", fromTag)
	}

	// verify that image stream mappings can use the same image for different tags
	image.ResourceVersion = ""
	mapping = &imagev1.ImageStreamMapping{
		ObjectMeta: metav1.ObjectMeta{Name: stream.Name},
		Tag:        "anothertag",
		Image:      *image,
	}
	if _, err := clusterAdminImageClient.ImageStreamMappings(oc.Namespace()).Create(mapping); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// ensure the correct tags are set
	updated, err = clusterAdminImageClient.ImageStreams(oc.Namespace()).Get(stream.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if updated.Spec.Tags != nil && len(updated.Spec.Tags) > 0 {
		t.Errorf("unexpected object: %#v", updated.Spec.Tags)
	}

	if _, err := clusterAdminImageClient.ImageStreamTags(oc.Namespace()).Get(stream.Name+":doesnotexist", metav1.GetOptions{}); err == nil || !errors.IsNotFound(err) {
		t.Fatalf("Unexpected error: %v", err)
	}

	fromTag, err = clusterAdminImageClient.ImageStreamTags(oc.Namespace()).Get(stream.Name+":newer", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if fromTag.Name != "test:newer" || fromTag.Image.UID == "" || fromTag.Image.DockerImageReference != "some/other/name" {
		t.Errorf("unexpected object: %#v", fromTag)
	}

	fromTag, err = clusterAdminImageClient.ImageStreamTags(oc.Namespace()).Get(stream.Name+":newest", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if fromTag.Name != "test:newest" || fromTag.Image.UID == "" || fromTag.Image.DockerImageReference != "different" {
		t.Errorf("unexpected object: %#v", fromTag)
	}
	fromTag, err = clusterAdminImageClient.ImageStreamTags(oc.Namespace()).Get(stream.Name+":anothertag", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if fromTag.Name != "test:anothertag" || fromTag.Image.UID == "" || fromTag.Image.DockerImageReference != "some/other/name" {
		t.Errorf("unexpected object: %#v", fromTag)
	}

	// try an update with an incorrect resource version
	if _, err := clusterAdminImageClient.ImageStreamTags(oc.Namespace()).Update(&imagev1.ImageStreamTag{
		ObjectMeta: metav1.ObjectMeta{Namespace: stream.Namespace, Name: stream.Name + ":brandnew", ResourceVersion: fromTag.ResourceVersion + "0"},
		Tag: &imagev1.TagReference{
			From: &corev1.ObjectReference{
				Kind: "ImageStreamTag",
				Name: "newest",
			},
		},
	}); !errors.IsConflict(err) {
		t.Fatalf("should have returned conflict error: %v", err)
	}

	// update and create a new tag
	fromTag, err = clusterAdminImageClient.ImageStreamTags(oc.Namespace()).Update(&imagev1.ImageStreamTag{
		ObjectMeta: metav1.ObjectMeta{Namespace: stream.Namespace, Name: stream.Name + ":brandnew", ResourceVersion: fromTag.ResourceVersion},
		Tag: &imagev1.TagReference{
			From: &corev1.ObjectReference{
				Kind: "ImageStreamTag",
				Name: "newest",
			},
		},
	})
	if err != nil {
		t.Fatalf("should have returned conflict error: %v", err)
	}
	if fromTag.Name != "test:brandnew" || fromTag.Image.UID == "" || fromTag.Tag.From.Name != "newest" {
		t.Errorf("unexpected object: %#v", fromTag)
	}
}

func TestImageStreamWithoutDockerImageConfig(t g.GinkgoTInterface, oc *exutil.CLI) {
	clusterAdminImageClient := oc.AdminImageClient().ImageV1()
	stream := mockImageStream()

	expected, err := clusterAdminImageClient.ImageStreams(oc.Namespace()).Create(stream)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if expected.Name == "" {
		t.Errorf("Unexpected empty image Name %v", expected)
	}

	imageConfig := docker10.DockerConfig{
		Hostname: "example.com",
		Env:      []string{"A=B"},
	}

	imageConfigBytes, err := json.Marshal(imageConfig)
	if err != nil {
		t.Fatalf("error marshaling image config: %s", err)
	}

	image := imagev1.Image{
		ObjectMeta: metav1.ObjectMeta{
			Name: BaseImageWith1LayerDigest,
		},
		DockerImageMetadata: runtime.RawExtension{
			Object: &docker10.DockerImage{
				Config: &docker10.DockerConfig{
					Hostname: "example.com",
					Env:      []string{"A=B"},
				},
			},
		},
		DockerImageConfig:    string(imageConfigBytes),
		DockerImageReference: "some/namespace/name",
	}

	// create a mapping to an image that doesn't exist
	mapping := &imagev1.ImageStreamMapping{
		ObjectMeta: metav1.ObjectMeta{
			Name: stream.Name,
		},
		Tag:   "newer",
		Image: image,
	}
	if _, err := clusterAdminImageClient.ImageStreamMappings(oc.Namespace()).Create(mapping); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	img, err := clusterAdminImageClient.Images().Get(image.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if img.Name != image.Name {
		t.Fatalf("unexpected image: %#v", img)
	}
	if len(img.DockerImageConfig) == 0 {
		t.Fatalf("image has an empty config: %#v", img)
	}

	ist, err := clusterAdminImageClient.ImageStreamTags(oc.Namespace()).Get(stream.Name+":newer", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if ist.Image.Name != image.Name {
		t.Fatalf("unexpected image: %#v", img)
	}
	if len(ist.Image.DockerImageConfig) != 0 {
		t.Errorf("image has a not empty config: %#v", ist)
	}

	isi, err := clusterAdminImageClient.ImageStreamImages(oc.Namespace()).Get(imageutil.JoinImageStreamImage(stream.Name, BaseImageWith1LayerDigest), metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if isi.Image.Name != image.Name {
		t.Fatalf("unexpected image: %#v", img)
	}
	if len(isi.Image.DockerImageConfig) != 0 {
		t.Errorf("image has a not empty config: %#v", isi)
	}

}

func TestImageStreamTagLifecycleHook(t g.GinkgoTInterface, oc *exutil.CLI) {
	clusterAdminImageClient := oc.AdminImageClient().ImageV1()

	stream := mockImageStream()
	if _, err := clusterAdminImageClient.ImageStreams(oc.Namespace()).Create(stream); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	imageClientset := oc.AdminImageClient()
	coreClient := oc.AdminKubeClient()

	// can tag to a stream that exists
	exec := NewHookExecutor(coreClient, imageClientset.ImageV1(), os.Stdout)
	err := exec.Execute(
		&appsv1.LifecycleHook{
			TagImages: []appsv1.TagImageHook{
				{
					ContainerName: "test",
					To:            corev1.ObjectReference{Kind: "ImageStreamTag", Name: stream.Name + ":test"},
				},
			},
		},
		&corev1.ReplicationController{
			ObjectMeta: metav1.ObjectMeta{Name: "rc-1", Namespace: oc.Namespace()},
			Spec: corev1.ReplicationControllerSpec{
				Template: &corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "test",
								Image: "someimage:other",
							},
						},
					},
				},
			},
		},
		"test", "test",
	)
	if err != nil {
		t.Fatal(err)
	}
	if stream, err = clusterAdminImageClient.ImageStreams(oc.Namespace()).Get(stream.Name, metav1.GetOptions{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if tag, ok := imageutil.SpecHasTag(stream, "test"); !ok || tag.From == nil || tag.From.Name != "someimage:other" {
		t.Fatalf("unexpected object: %#v", tag)
	}

	// can execute a second time the same tag and it should work
	exec = NewHookExecutor(coreClient, imageClientset.ImageV1(), os.Stdout)
	err = exec.Execute(
		&appsv1.LifecycleHook{
			TagImages: []appsv1.TagImageHook{
				{
					ContainerName: "test",
					To:            corev1.ObjectReference{Kind: "ImageStreamTag", Name: stream.Name + ":test"},
				},
			},
		},
		&corev1.ReplicationController{
			ObjectMeta: metav1.ObjectMeta{Name: "rc-1", Namespace: oc.Namespace()},
			Spec: corev1.ReplicationControllerSpec{
				Template: &corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "test",
								Image: "someimage:other",
							},
						},
					},
				},
			},
		},
		"test", "test",
	)
	if err != nil {
		t.Fatal(err)
	}

	// can lifecycle tag a new image stream
	exec = NewHookExecutor(coreClient, imageClientset.ImageV1(), os.Stdout)
	err = exec.Execute(
		&appsv1.LifecycleHook{
			TagImages: []appsv1.TagImageHook{
				{
					ContainerName: "test",
					To:            corev1.ObjectReference{Kind: "ImageStreamTag", Name: "test2:test"},
				},
			},
		},
		&corev1.ReplicationController{
			ObjectMeta: metav1.ObjectMeta{Name: "rc-1", Namespace: oc.Namespace()},
			Spec: corev1.ReplicationControllerSpec{
				Template: &corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "test",
								Image: "someimage:other",
							},
						},
					},
				},
			},
		},
		"test", "test",
	)
	if err != nil {
		t.Fatal(err)
	}
	if stream, err = clusterAdminImageClient.ImageStreams(oc.Namespace()).Get("test2", metav1.GetOptions{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tag, ok := imageutil.SpecHasTag(stream, "test"); !ok || tag.From == nil || tag.From.Name != "someimage:other" {
		t.Fatalf("unexpected object: %#v", tag)
	}
}
