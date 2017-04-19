package controller

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kapi "k8s.io/kubernetes/pkg/api"
	kexternalfake "k8s.io/kubernetes/pkg/client/clientset_generated/clientset/fake"
	kinternalfake "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/fake"
	kexternalinformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/externalversions"
	kinternalinformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/internalversion"

	"github.com/openshift/origin/pkg/client/testclient"
	"github.com/openshift/origin/pkg/controller/shared"
	"github.com/openshift/origin/pkg/image/api"

	_ "github.com/openshift/origin/pkg/api/install"
)

func TestScheduledImport(t *testing.T) {
	one := int64(1)
	stream := &api.ImageStream{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test", Namespace: "other", UID: "1", ResourceVersion: "1",
			Annotations: map[string]string{api.DockerImageRepositoryCheckAnnotation: "done"},
			Generation:  1,
		},
		Spec: api.ImageStreamSpec{
			Tags: map[string]api.TagReference{
				"default": {
					From:         &kapi.ObjectReference{Kind: "DockerImage", Name: "mysql:latest"},
					Generation:   &one,
					ImportPolicy: api.TagImportPolicy{Scheduled: true},
				},
			},
		},
		Status: api.ImageStreamStatus{
			Tags: map[string]api.TagEventList{
				"default": {Items: []api.TagEvent{{Generation: 1}}},
			},
		},
	}

	internalKubeClient := kinternalfake.NewSimpleClientset()
	externalKubeClient := kexternalfake.NewSimpleClientset()
	externalKubeInformerFactory := kexternalinformers.NewSharedInformerFactory(externalKubeClient, 10*time.Minute)
	internalKubeInformerFactory := kinternalinformers.NewSharedInformerFactory(internalKubeClient, 10*time.Minute)
	informerFactory := shared.NewInformerFactory(internalKubeInformerFactory, externalKubeInformerFactory,
		internalKubeClient, testclient.NewSimpleFake(), shared.DefaultListerWatcherOverrides{}, 10*time.Minute)
	isInformer := informerFactory.ImageStreams()
	fake := testclient.NewSimpleFake()
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
	sched.scheduler.RunOnce() // we need to run it twice since we have 2 buckets
	sched.scheduler.RunOnce()
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
	sched.scheduler.RunOnce() // we need to run it twice since we have 2 buckets
	sched.scheduler.RunOnce()
	if sched.scheduler.Len() != 1 {
		t.Fatalf("should have left item in scheduler: %#v", sched.scheduler)
	}
	if len(fake.Actions()) != 1 || !fake.Actions()[0].Matches("create", "imagestreamimports") {
		t.Fatalf("invalid actions: %#v", fake.Actions())
	}

	// disabling the scheduled import should drop the stream
	sched.enabled = false
	fake.ClearActions()

	sched.scheduler.RunOnce() // we need to run it twice since we have 2 buckets
	sched.scheduler.RunOnce()
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
