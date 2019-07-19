package integration

import (
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	imagev1 "github.com/openshift/api/image/v1"
	imagev1client "github.com/openshift/client-go/image/clientset/versioned"
	"github.com/openshift/library-go/pkg/image/imageutil"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

func mockImageStream() *imagev1.ImageStream {
	return &imagev1.ImageStream{ObjectMeta: metav1.ObjectMeta{Name: "test"}}
}

func upsertSpecTag(tags *[]imagev1.TagReference, tagReference imagev1.TagReference) {
	for i := range *tags {
		curr := (*tags)[i]
		if curr.Name == tagReference.Name {
			(*tags)[i] = tagReference
			return
		}
	}
	*tags = append(*tags, tagReference)
}

func TestRegistryWhitelistingValidation(t *testing.T) {
	testutil.AddAdditionalAllowedRegistries("my.insecure.registry:80")
	masterConfig, clusterAdminKubeConfig, err := testserver.StartTestMasterAPI()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer testserver.CleanupMasterEtcd(t, masterConfig)

	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	clusterAdminImageClient := imagev1client.NewForConfigOrDie(clusterAdminClientConfig).ImageV1()
	err = testutil.CreateNamespace(clusterAdminKubeConfig, testutil.Namespace())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	stream := mockImageStream()
	stream.Spec = imagev1.ImageStreamSpec{
		Tags: []imagev1.TagReference{
			{
				Name: "latest",
				From: &corev1.ObjectReference{
					Kind: "DockerImage",
					Name: "my.test.registry/repo/sitory:latest",
				},
			},
		},
	}

	_, err = clusterAdminImageClient.ImageStreams(testutil.Namespace()).Create(stream)
	if err == nil || !errors.IsInvalid(err) {
		t.Fatalf("expected invalid error, got: %T %v", err, err)
	}
	if e, a := `spec.tags[latest].from.name: Forbidden: registry "my.test.registry" not allowed by whitelist`, err.Error(); !strings.Contains(a, e) {
		t.Fatalf("expected string %q not contained in error: %s", e, a)
	}

	latestSpecTag, _ := imageutil.SpecHasTag(stream, "latest")
	latestSpecTag.From.Name = "docker.io/busybox"
	upsertSpecTag(&stream.Spec.Tags, latestSpecTag)
	stream, err = clusterAdminImageClient.ImageStreams(testutil.Namespace()).Create(stream)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	upsertSpecTag(&stream.Spec.Tags, imagev1.TagReference{
		Name: "fail",
		From: &corev1.ObjectReference{
			Kind: "DockerImage",
			Name: "this.will.fail/repo:tag",
		},
	})
	_, err = clusterAdminImageClient.ImageStreams(testutil.Namespace()).Update(stream)
	if err == nil || !errors.IsInvalid(err) {
		t.Fatalf("expected invalid error, got: %T %v", err, err)
	}
	if e, a := `spec.tags[fail].from.name: Forbidden: registry "this.will.fail" not allowed by whitelist`, err.Error(); !strings.Contains(a, e) {
		t.Fatalf("expected string %q not contained in error: %s", e, a)
	}

	stream.Annotations = map[string]string{imagev1.InsecureRepositoryAnnotation: "true"}
	newTags := []imagev1.TagReference{}
	for i := range stream.Spec.Tags {
		if stream.Spec.Tags[i].Name != "fail" {
			newTags = append(newTags, stream.Spec.Tags[i])
		}
	}
	stream.Spec.Tags = newTags

	upsertSpecTag(&stream.Spec.Tags, imagev1.TagReference{
		Name: "pass",
		From: &corev1.ObjectReference{
			Kind: "DockerImage",
			Name: "127.0.0.1:5000/repo:tag",
		},
	})
	_, err = clusterAdminImageClient.ImageStreams(testutil.Namespace()).Update(stream)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	istag := &imagev1.ImageStreamTag{
		ObjectMeta: metav1.ObjectMeta{
			Name: stream.Name + ":new",
		},
		Tag: &imagev1.TagReference{
			Name: "new",
			From: &corev1.ObjectReference{
				Kind: "DockerImage",
				Name: "my.insecure.registry/repo:new",
			},
		},
	}

	_, err = clusterAdminImageClient.ImageStreamTags(testutil.Namespace()).Create(istag)
	if err == nil || !errors.IsInvalid(err) {
		t.Fatalf("expected invalid error, got: %T %v", err, err)
	}
	if e, a := `tag.from.name: Forbidden: registry "my.insecure.registry" not allowed by whitelist`, err.Error(); !strings.Contains(a, e) {
		t.Fatalf("expected string %q not contained in error: %s", e, a)
	}

	istag.Annotations = map[string]string{imagev1.InsecureRepositoryAnnotation: "true"}
	istag, err = clusterAdminImageClient.ImageStreamTags(testutil.Namespace()).Create(istag)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	istag.Tag.From = &corev1.ObjectReference{
		Kind: "DockerImage",
		Name: "example.com/repo:tag",
	}
	istag.ObjectMeta = metav1.ObjectMeta{
		Name:            istag.Name,
		ResourceVersion: istag.ResourceVersion,
	}
	_, err = clusterAdminImageClient.ImageStreamTags(testutil.Namespace()).Update(istag)
	if err == nil || !errors.IsInvalid(err) {
		t.Fatalf("expected invalid error, got: %T %v", err, err)
	}
	if e, a := `tag.from.name: Forbidden: registry "example.com" not allowed by whitelist`, err.Error(); !strings.Contains(a, e) {
		t.Fatalf("expected string %q not contained in error: %s", e, a)
	}

	istag.Tag.From = &corev1.ObjectReference{
		Kind: "DockerImage",
		Name: "myupstream/repo:latest",
	}
	_, err = clusterAdminImageClient.ImageStreamTags(testutil.Namespace()).Update(istag)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
