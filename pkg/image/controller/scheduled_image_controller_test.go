package controller

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	imageinformer "github.com/openshift/origin/pkg/image/generated/informers/internalversion"
	imageclient "github.com/openshift/origin/pkg/image/generated/internalclientset/fake"

	_ "github.com/openshift/origin/pkg/api/install"
)

func TestScheduledImport(t *testing.T) {
	one := int64(1)
	stream := &imageapi.ImageStream{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test", Namespace: "other", UID: "1", ResourceVersion: "1",
			Annotations: map[string]string{imageapi.DockerImageRepositoryCheckAnnotation: "done"},
			Generation:  1,
		},
		Spec: imageapi.ImageStreamSpec{
			Tags: map[string]imageapi.TagReference{
				"default": {
					From:         &kapi.ObjectReference{Kind: "DockerImage", Name: "mysql:latest"},
					Generation:   &one,
					ImportPolicy: imageapi.TagImportPolicy{Scheduled: true},
				},
			},
		},
		Status: imageapi.ImageStreamStatus{
			Tags: map[string]imageapi.TagEventList{
				"default": {Items: []imageapi.TagEvent{{Generation: 1}}},
			},
		},
	}

	imageInformers := imageinformer.NewSharedInformerFactory(imageclient.NewSimpleClientset(), 0)
	isInformer := imageInformers.Image().InternalVersion().ImageStreams()
	fake := imageclient.NewSimpleClientset()
	sched := NewScheduledImageStreamController(fake, isInformer, ScheduledImageStreamControllerOptions{
		Enabled:           true,
		Resync:            1 * time.Second,
		DefaultBucketSize: 4,
	})

	// queue, but don't import the stream
	sched.enqueueImageStream(stream)
	if sched.scheduler.Len() != 1 {
		t.Fatalf("should have scheduled: %#v", sched.scheduler)
	}
	if len(fake.Actions()) != 0 {
		t.Fatalf("should have made no calls: %#v", fake)
	}

	// encountering a not found error for image streams should drop the stream
	for i := 0; i < 3; i++ { // loop all the buckets (2 + the additional internal one)
		sched.scheduler.RunOnce()
	}
	if sched.scheduler.Len() != 0 {
		t.Fatalf("should have removed item in scheduler: %#v", sched.scheduler)
	}
	if len(fake.Actions()) != 0 {
		t.Fatalf("invalid actions: %#v", fake.Actions())
	}

	// queue back
	sched.enqueueImageStream(stream)
	// and add to informer
	isInformer.Informer().GetIndexer().Add(stream)

	// run a background import
	for i := 0; i < 3; i++ { // loop all the buckets (2 + the additional internal one)
		sched.scheduler.RunOnce()
	}
	if sched.scheduler.Len() != 1 {
		t.Fatalf("should have left item in scheduler: %#v", sched.scheduler)
	}
	if len(fake.Actions()) != 1 || !fake.Actions()[0].Matches("create", "imagestreamimports") {
		t.Fatalf("invalid actions: %#v", fake.Actions())
	}

	// disabling the scheduled import should drop the stream
	sched.enabled = false
	fake.ClearActions()

	for i := 0; i < 3; i++ { // loop all the buckets (2 + the additional internal one)
		sched.scheduler.RunOnce()
	}
	if sched.scheduler.Len() != 0 {
		t.Fatalf("should have removed item from scheduler: %#v", sched.scheduler)
	}
	if len(fake.Actions()) != 0 {
		t.Fatalf("invalid actions: %#v", fake.Actions())
	}

	// queuing when disabled should not add the stream
	sched.enqueueImageStream(stream)
	if sched.scheduler.Len() != 0 {
		t.Fatalf("should have not added item to scheduler: %#v", sched.scheduler)
	}
}
