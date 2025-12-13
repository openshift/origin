package images

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	appsv1 "github.com/openshift/api/apps/v1"
	"github.com/openshift/api/image/docker10"
	imagev1 "github.com/openshift/api/image/v1"
	"github.com/openshift/library-go/pkg/image/imageutil"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-imageregistry][Feature:ImageTriggers][Serial] ImageStream API", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLI("imagestream-api")

	g.It("TestImageStreamMappingCreate [apigroup:image.openshift.io]", g.Label("Size:M"), func() {
		TestImageStreamMappingCreate(g.GinkgoT(), oc)
	})
	g.It("TestImageStreamWithoutDockerImageConfig [apigroup:image.openshift.io]", g.Label("Size:M"), func() {
		TestImageStreamWithoutDockerImageConfig(g.GinkgoT(), oc)
	})
	g.It("TestImageStreamTagLifecycleHook [apigroup:image.openshift.io]", g.Label("Size:L"), func() {
		TestImageStreamTagLifecycleHook(g.GinkgoT(), oc)
	})
})

func mockImageStream() *imagev1.ImageStream {
	return &imagev1.ImageStream{ObjectMeta: metav1.ObjectMeta{Name: "test"}}
}

func TestImageStreamMappingCreate(t g.GinkgoTInterface, oc *exutil.CLI) {
	clusterAdminImageClient := oc.AdminImageClient().ImageV1()
	g.By("creating an imagestream and imagestream mappings")
	stream := mockImageStream()
	ctx := context.Background()

	expected, err := clusterAdminImageClient.ImageStreams(oc.Namespace()).Create(ctx, stream, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(expected.Name).NotTo(o.BeEmpty())

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
	_, err = clusterAdminImageClient.ImageStreamMappings(oc.Namespace()).Create(ctx, mapping, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	// verify we can tag a second time with the same data, and nothing changes
	_, err = clusterAdminImageClient.ImageStreamMappings(oc.Namespace()).Create(ctx, mapping, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	g.By("creating an image directly")
	name := fmt.Sprintf("image-%d", rand.Intn(10000))
	image := &imagev1.Image{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		DockerImageMetadata: runtime.RawExtension{
			Object: &docker10.DockerImage{
				Config: &docker10.DockerConfig{
					Env: []string{"A=B"},
				},
			},
		},
	}
	if _, err := clusterAdminImageClient.Images().Create(ctx, image, metav1.CreateOptions{}); err == nil {
		t.Fatalf("unexpected non-error")
	}
	defer clusterAdminImageClient.Images().Delete(ctx, image.Name, metav1.DeleteOptions{})
	image.DockerImageReference = "some/other/name" // can reuse references across multiple images
	actual, err := clusterAdminImageClient.Images().Create(ctx, image, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	if actual == nil || actual.Name != image.Name {
		t.Fatalf("unexpected object: %#v", actual)
	}

	// verify that image stream mappings cannot mutate / overwrite the image (images are immutable)
	g.By("attempting to overwrite imagestream mappings")
	mapping = &imagev1.ImageStreamMapping{
		ObjectMeta: metav1.ObjectMeta{Name: stream.Name},
		Tag:        "newest",
		Image:      *image,
	}
	mapping.Image.DockerImageReference = "different"
	_, err = clusterAdminImageClient.ImageStreamMappings(oc.Namespace()).Create(ctx, mapping, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	image, err = clusterAdminImageClient.Images().Get(ctx, image.Name, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(image.DockerImageReference).To(o.Equal("some/other/name"))

	// ensure the correct tags are set
	g.By("verifying imagestream tags are correct")
	updated, err := clusterAdminImageClient.ImageStreams(oc.Namespace()).Get(ctx, stream.Name, metav1.GetOptions{})

	o.Expect(err).NotTo(o.HaveOccurred())
	if updated.Spec.Tags != nil && len(updated.Spec.Tags) > 0 {
		t.Fatalf("unexpected object: %#v", updated.Spec.Tags)
	}

	fromTag, err := clusterAdminImageClient.ImageStreamTags(oc.Namespace()).Get(ctx, stream.Name+":newer", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	if fromTag.Name != "test:newer" || fromTag.Image.UID == "" || fromTag.Image.DockerImageReference != "some/other/name" {
		t.Fatalf("unexpected object: %#v", fromTag)
	}

	fromTag, err = clusterAdminImageClient.ImageStreamTags(oc.Namespace()).Get(ctx, stream.Name+":newest", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	if fromTag.Name != "test:newest" || fromTag.Image.UID == "" || fromTag.Image.DockerImageReference != "different" {
		t.Fatalf("unexpected object: %#v", fromTag)
	}

	// verify that image stream mappings can use the same image for different tags
	image.ResourceVersion = ""
	mapping = &imagev1.ImageStreamMapping{
		ObjectMeta: metav1.ObjectMeta{Name: stream.Name},
		Tag:        "anothertag",
		Image:      *image,
	}
	_, err = clusterAdminImageClient.ImageStreamMappings(oc.Namespace()).Create(ctx, mapping, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	// ensure the correct tags are set
	updated, err = clusterAdminImageClient.ImageStreams(oc.Namespace()).Get(ctx, stream.Name, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	if updated.Spec.Tags != nil && len(updated.Spec.Tags) > 0 {
		t.Fatalf("unexpected object: %#v", updated.Spec.Tags)
	}

	// expect not found error for non-existent imagestream tag
	_, err = clusterAdminImageClient.ImageStreamTags(oc.Namespace()).Get(ctx, stream.Name+":doesnotexist", metav1.GetOptions{})
	o.Expect(err).To(o.HaveOccurred())
	o.Expect(errors.IsNotFound(err)).To(o.BeTrue())

	fromTag, err = clusterAdminImageClient.ImageStreamTags(oc.Namespace()).Get(ctx, stream.Name+":newer", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	if fromTag.Name != "test:newer" || fromTag.Image.UID == "" || fromTag.Image.DockerImageReference != "some/other/name" {
		t.Fatalf("unexpected object: %#v", fromTag)
	}

	fromTag, err = clusterAdminImageClient.ImageStreamTags(oc.Namespace()).Get(ctx, stream.Name+":newest", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	if fromTag.Name != "test:newest" || fromTag.Image.UID == "" || fromTag.Image.DockerImageReference != "different" {
		t.Fatalf("unexpected object: %#v", fromTag)
	}
	fromTag, err = clusterAdminImageClient.ImageStreamTags(oc.Namespace()).Get(ctx, stream.Name+":anothertag", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	if fromTag.Name != "test:anothertag" || fromTag.Image.UID == "" || fromTag.Image.DockerImageReference != "some/other/name" {
		t.Fatalf("unexpected object: %#v", fromTag)
	}

	// try an update with an incorrect resource version - needs to have conflict error
	g.By("updating imagestreamtag expecting conflict")
	_, err = clusterAdminImageClient.ImageStreamTags(oc.Namespace()).Update(ctx, &imagev1.ImageStreamTag{
		ObjectMeta: metav1.ObjectMeta{Namespace: stream.Namespace, Name: stream.Name + ":brandnew", ResourceVersion: fromTag.ResourceVersion + "0"},
		Tag: &imagev1.TagReference{
			From: &corev1.ObjectReference{
				Kind: "ImageStreamTag",
				Name: "newest",
			},
		},
	}, metav1.UpdateOptions{})
	o.Expect(errors.IsConflict(err)).To(o.BeTrue())

	// update and create a new tag
	g.By("adding a new tag to an existing imagestream via update")
	fromTag, err = clusterAdminImageClient.ImageStreamTags(oc.Namespace()).Update(ctx, &imagev1.ImageStreamTag{
		ObjectMeta: metav1.ObjectMeta{Namespace: stream.Namespace, Name: stream.Name + ":brandnew", ResourceVersion: fromTag.ResourceVersion},
		Tag: &imagev1.TagReference{
			From: &corev1.ObjectReference{
				Kind: "ImageStreamTag",
				Name: "newest",
			},
		},
	}, metav1.UpdateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	if fromTag.Name != "test:brandnew" || fromTag.Image.UID == "" || fromTag.Tag.From.Name != "newest" {
		t.Fatalf("unexpected object: %#v", fromTag)
	}
}

func TestImageStreamWithoutDockerImageConfig(t g.GinkgoTInterface, oc *exutil.CLI) {
	ctx := context.Background()
	clusterAdminImageClient := oc.AdminImageClient().ImageV1()
	stream := mockImageStream()

	expected, err := clusterAdminImageClient.ImageStreams(oc.Namespace()).Create(ctx, stream, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	if expected.Name == "" {
		t.Fatalf("Unexpected empty image Name %v", expected)
	}

	imageConfig := docker10.DockerConfig{
		Hostname: "example.com",
		Env:      []string{"A=B"},
	}

	imageConfigBytes, err := json.Marshal(imageConfig)
	o.Expect(err).NotTo(o.HaveOccurred())

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
	_, err = clusterAdminImageClient.ImageStreamMappings(oc.Namespace()).Create(ctx, mapping, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	img, err := clusterAdminImageClient.Images().Get(ctx, image.Name, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(img.Name).To(o.Equal(image.Name))
	o.Expect(img.DockerImageConfig).NotTo(o.BeEmpty())

	ist, err := clusterAdminImageClient.ImageStreamTags(oc.Namespace()).Get(ctx, stream.Name+":newer", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(ist.Image.Name).To(o.Equal(image.Name))
	o.Expect(ist.Image.DockerImageConfig).To(o.BeEmpty())

	isi, err := clusterAdminImageClient.ImageStreamImages(oc.Namespace()).Get(ctx, imageutil.JoinImageStreamImage(stream.Name, BaseImageWith1LayerDigest), metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(isi.Image.Name).To(o.Equal(image.Name))
	o.Expect(isi.Image.DockerImageConfig).To(o.BeEmpty())
}

func TestImageStreamTagLifecycleHook(t g.GinkgoTInterface, oc *exutil.CLI) {
	clusterAdminImageClient := oc.AdminImageClient().ImageV1()

	stream := mockImageStream()
	_, err := clusterAdminImageClient.ImageStreams(oc.Namespace()).Create(context.Background(), stream, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	imageClientset := oc.AdminImageClient()
	coreClient := oc.AdminKubeClient()

	// can tag to a stream that exists
	exec := NewHookExecutor(coreClient, imageClientset.ImageV1(), os.Stdout)
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
	o.Expect(err).NotTo(o.HaveOccurred())
	stream, err = clusterAdminImageClient.ImageStreams(oc.Namespace()).Get(context.Background(), stream.Name, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	tag, ok := imageutil.SpecHasTag(stream, "test")
	o.Expect(ok).To(o.BeTrue())
	o.Expect(tag.From).NotTo(o.BeNil())
	o.Expect(tag.From.Name).To(o.Equal("someimage:other"))

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
	o.Expect(err).NotTo(o.HaveOccurred())

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
	o.Expect(err).NotTo(o.HaveOccurred())
	stream, err = clusterAdminImageClient.ImageStreams(oc.Namespace()).Get(context.Background(), "test2", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(ok).To(o.BeTrue())
	o.Expect(tag.From).NotTo(o.BeNil())
	o.Expect(tag.From.Name).To(o.Equal("someimage:other"))
}
