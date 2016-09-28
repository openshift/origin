package integration

import (
	"os"
	"reflect"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/util/diff"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	stratsupport "github.com/openshift/origin/pkg/deploy/strategy/support"
	imageapi "github.com/openshift/origin/pkg/image/api"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

func TestImageStreamList(t *testing.T) {
	testutil.RequireEtcd(t)
	defer testutil.DumpEtcdOnFailure(t)
	_, clusterAdminKubeConfig, err := testserver.StartTestMaster()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	clusterAdminClient, err := testutil.GetClusterAdminClient(clusterAdminKubeConfig)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	err = testutil.CreateNamespace(clusterAdminKubeConfig, testutil.Namespace())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	builds, err := clusterAdminClient.ImageStreams(testutil.Namespace()).List(kapi.ListOptions{})
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}
	if len(builds.Items) != 0 {
		t.Errorf("Expected no builds, got %#v", builds.Items)
	}
}

func mockImageStream() *imageapi.ImageStream {
	return &imageapi.ImageStream{ObjectMeta: kapi.ObjectMeta{Name: "test"}}
}

func TestImageStreamCreate(t *testing.T) {
	testutil.RequireEtcd(t)
	defer testutil.DumpEtcdOnFailure(t)
	_, clusterAdminKubeConfig, err := testserver.StartTestMasterAPI()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	clusterAdminClient, err := testutil.GetClusterAdminClient(clusterAdminKubeConfig)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	err = testutil.CreateNamespace(clusterAdminKubeConfig, testutil.Namespace())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	stream := mockImageStream()

	if _, err := clusterAdminClient.ImageStreams(testutil.Namespace()).Create(&imageapi.ImageStream{}); err == nil || !errors.IsInvalid(err) {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected, err := clusterAdminClient.ImageStreams(testutil.Namespace()).Create(stream)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if expected.Name == "" {
		t.Errorf("Unexpected empty image Name %v", expected)
	}

	actual, err := clusterAdminClient.ImageStreams(testutil.Namespace()).Get(stream.Name)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("unexpected object: %s", diff.ObjectDiff(expected, actual))
	}

	streams, err := clusterAdminClient.ImageStreams(testutil.Namespace()).List(kapi.ListOptions{})
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}
	if len(streams.Items) != 1 {
		t.Errorf("Expected one image, got %#v", streams.Items)
	}
}

func TestImageStreamMappingCreate(t *testing.T) {
	testutil.RequireEtcd(t)
	defer testutil.DumpEtcdOnFailure(t)
	_, clusterAdminKubeConfig, err := testserver.StartTestMasterAPI()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	clusterAdminClient, err := testutil.GetClusterAdminClient(clusterAdminKubeConfig)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	err = testutil.CreateNamespace(clusterAdminKubeConfig, testutil.Namespace())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	stream := mockImageStream()

	expected, err := clusterAdminClient.ImageStreams(testutil.Namespace()).Create(stream)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if expected.Name == "" {
		t.Errorf("Unexpected empty image Name %v", expected)
	}

	// create a mapping to an image that doesn't exist
	mapping := &imageapi.ImageStreamMapping{
		ObjectMeta: kapi.ObjectMeta{Name: stream.Name},
		Tag:        "newer",
		Image: imageapi.Image{
			ObjectMeta: kapi.ObjectMeta{
				Name: "image1",
			},
			DockerImageReference: "some/other/name",
		},
	}
	if err := clusterAdminClient.ImageStreamMappings(testutil.Namespace()).Create(mapping); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// verify we can tag a second time with the same data, and nothing changes
	if err := clusterAdminClient.ImageStreamMappings(testutil.Namespace()).Create(mapping); err != nil {
		t.Fatalf("unexpected non-error or type: %v", err)
	}

	// create an image directly
	image := &imageapi.Image{
		ObjectMeta: kapi.ObjectMeta{Name: "image2"},
		DockerImageMetadata: imageapi.DockerImage{
			Config: &imageapi.DockerConfig{
				Env: []string{"A=B"},
			},
		},
	}
	if _, err := clusterAdminClient.Images().Create(image); err == nil {
		t.Error("unexpected non-error")
	}
	image.DockerImageReference = "some/other/name" // can reuse references across multiple images
	actual, err := clusterAdminClient.Images().Create(image)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if actual == nil || actual.Name != image.Name {
		t.Errorf("unexpected object: %#v", actual)
	}

	// verify that image stream mappings cannot mutate / overwrite the image (images are immutable)
	mapping = &imageapi.ImageStreamMapping{
		ObjectMeta: kapi.ObjectMeta{Name: stream.Name},
		Tag:        "newest",
		Image:      *image,
	}
	mapping.Image.DockerImageReference = "different"
	if err := clusterAdminClient.ImageStreamMappings(testutil.Namespace()).Create(mapping); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	image, err = clusterAdminClient.Images().Get(image.Name)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if image.DockerImageReference != "some/other/name" {
		t.Fatalf("image was unexpectedly mutated: %#v", image)
	}

	// ensure the correct tags are set
	updated, err := clusterAdminClient.ImageStreams(testutil.Namespace()).Get(stream.Name)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if updated.Spec.Tags != nil && len(updated.Spec.Tags) > 0 {
		t.Errorf("unexpected object: %#v", updated.Spec.Tags)
	}

	fromTag, err := clusterAdminClient.ImageStreamTags(testutil.Namespace()).Get(stream.Name, "newer")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if fromTag.Name != "test:newer" || fromTag.Image.UID == "" || fromTag.Image.DockerImageReference != "some/other/name" {
		t.Errorf("unexpected object: %#v", fromTag)
	}

	fromTag, err = clusterAdminClient.ImageStreamTags(testutil.Namespace()).Get(stream.Name, "newest")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if fromTag.Name != "test:newest" || fromTag.Image.UID == "" || fromTag.Image.DockerImageReference != "different" {
		t.Errorf("unexpected object: %#v", fromTag)
	}

	// verify that image stream mappings can use the same image for different tags
	image.ResourceVersion = ""
	mapping = &imageapi.ImageStreamMapping{
		ObjectMeta: kapi.ObjectMeta{Name: stream.Name},
		Tag:        "anothertag",
		Image:      *image,
	}
	if err := clusterAdminClient.ImageStreamMappings(testutil.Namespace()).Create(mapping); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// ensure the correct tags are set
	updated, err = clusterAdminClient.ImageStreams(testutil.Namespace()).Get(stream.Name)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if updated.Spec.Tags != nil && len(updated.Spec.Tags) > 0 {
		t.Errorf("unexpected object: %#v", updated.Spec.Tags)
	}

	if _, err := clusterAdminClient.ImageStreamTags(testutil.Namespace()).Get(stream.Name, "doesnotexist"); err == nil || !errors.IsNotFound(err) {
		t.Fatalf("Unexpected error: %v", err)
	}

	fromTag, err = clusterAdminClient.ImageStreamTags(testutil.Namespace()).Get(stream.Name, "newer")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if fromTag.Name != "test:newer" || fromTag.Image.UID == "" || fromTag.Image.DockerImageReference != "some/other/name" {
		t.Errorf("unexpected object: %#v", fromTag)
	}

	fromTag, err = clusterAdminClient.ImageStreamTags(testutil.Namespace()).Get(stream.Name, "newest")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if fromTag.Name != "test:newest" || fromTag.Image.UID == "" || fromTag.Image.DockerImageReference != "different" {
		t.Errorf("unexpected object: %#v", fromTag)
	}
	fromTag, err = clusterAdminClient.ImageStreamTags(testutil.Namespace()).Get(stream.Name, "anothertag")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if fromTag.Name != "test:anothertag" || fromTag.Image.UID == "" || fromTag.Image.DockerImageReference != "some/other/name" {
		t.Errorf("unexpected object: %#v", fromTag)
	}

	// try an update with an incorrect resource version
	if _, err := clusterAdminClient.ImageStreamTags(testutil.Namespace()).Update(&imageapi.ImageStreamTag{
		ObjectMeta: kapi.ObjectMeta{Namespace: stream.Namespace, Name: stream.Name + ":brandnew", ResourceVersion: fromTag.ResourceVersion + "0"},
		Tag: &imageapi.TagReference{
			From: &kapi.ObjectReference{
				Kind: "ImageStreamTag",
				Name: "newest",
			},
		},
	}); !errors.IsConflict(err) {
		t.Fatalf("should have returned conflict error: %v", err)
	}

	// update and create a new tag
	fromTag, err = clusterAdminClient.ImageStreamTags(testutil.Namespace()).Update(&imageapi.ImageStreamTag{
		ObjectMeta: kapi.ObjectMeta{Namespace: stream.Namespace, Name: stream.Name + ":brandnew", ResourceVersion: fromTag.ResourceVersion},
		Tag: &imageapi.TagReference{
			From: &kapi.ObjectReference{
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

func TestImageStreamTagLifecycleHook(t *testing.T) {
	testutil.RequireEtcd(t)
	defer testutil.DumpEtcdOnFailure(t)
	_, clusterAdminKubeConfig, err := testserver.StartTestMaster()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	clusterAdminKubeClient, err := testutil.GetClusterAdminKubeClient(clusterAdminKubeConfig)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	clusterAdminClient, err := testutil.GetClusterAdminClient(clusterAdminKubeConfig)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	err = testutil.CreateNamespace(clusterAdminKubeConfig, testutil.Namespace())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	stream := mockImageStream()
	if _, err := clusterAdminClient.ImageStreams(testutil.Namespace()).Create(stream); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// can tag to a stream that exists
	exec := stratsupport.NewHookExecutor(nil, clusterAdminClient, clusterAdminKubeClient, os.Stdout, kapi.Codecs.UniversalDecoder())
	err = exec.Execute(
		&deployapi.LifecycleHook{
			TagImages: []deployapi.TagImageHook{
				{
					ContainerName: "test",
					To:            kapi.ObjectReference{Kind: "ImageStreamTag", Name: stream.Name + ":test"},
				},
			},
		},
		&kapi.ReplicationController{
			ObjectMeta: kapi.ObjectMeta{Name: "rc-1", Namespace: testutil.Namespace()},
			Spec: kapi.ReplicationControllerSpec{
				Template: &kapi.PodTemplateSpec{
					Spec: kapi.PodSpec{
						Containers: []kapi.Container{
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
	if stream, err = clusterAdminClient.ImageStreams(testutil.Namespace()).Get(stream.Name); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tag, ok := stream.Spec.Tags["test"]; !ok || tag.From == nil || tag.From.Name != "someimage:other" {
		t.Fatalf("unexpected object: %#v", tag)
	}

	// can execute a second time the same tag and it should work
	exec = stratsupport.NewHookExecutor(nil, clusterAdminClient, clusterAdminKubeClient, os.Stdout, kapi.Codecs.UniversalDecoder())
	err = exec.Execute(
		&deployapi.LifecycleHook{
			TagImages: []deployapi.TagImageHook{
				{
					ContainerName: "test",
					To:            kapi.ObjectReference{Kind: "ImageStreamTag", Name: stream.Name + ":test"},
				},
			},
		},
		&kapi.ReplicationController{
			ObjectMeta: kapi.ObjectMeta{Name: "rc-1", Namespace: testutil.Namespace()},
			Spec: kapi.ReplicationControllerSpec{
				Template: &kapi.PodTemplateSpec{
					Spec: kapi.PodSpec{
						Containers: []kapi.Container{
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
	exec = stratsupport.NewHookExecutor(nil, clusterAdminClient, clusterAdminKubeClient, os.Stdout, kapi.Codecs.UniversalDecoder())
	err = exec.Execute(
		&deployapi.LifecycleHook{
			TagImages: []deployapi.TagImageHook{
				{
					ContainerName: "test",
					To:            kapi.ObjectReference{Kind: "ImageStreamTag", Name: "test2:test"},
				},
			},
		},
		&kapi.ReplicationController{
			ObjectMeta: kapi.ObjectMeta{Name: "rc-1", Namespace: testutil.Namespace()},
			Spec: kapi.ReplicationControllerSpec{
				Template: &kapi.PodTemplateSpec{
					Spec: kapi.PodSpec{
						Containers: []kapi.Container{
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
	if stream, err = clusterAdminClient.ImageStreams(testutil.Namespace()).Get("test2"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tag, ok := stream.Spec.Tags["test"]; !ok || tag.From == nil || tag.From.Name != "someimage:other" {
		t.Fatalf("unexpected object: %#v", tag)
	}
}
