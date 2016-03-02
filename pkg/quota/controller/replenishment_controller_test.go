package controller

import (
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/controller"
	kresourcequota "k8s.io/kubernetes/pkg/controller/resourcequota"
	"k8s.io/kubernetes/pkg/runtime"

	imageapi "github.com/openshift/origin/pkg/image/api"
)

// testReplenishment lets us test replenishment functions are invoked
type testReplenishment struct {
	groupKind unversioned.GroupKind
	namespace string
}

// mock function that holds onto the last kind that was replenished
func (t *testReplenishment) Replenish(groupKind unversioned.GroupKind, namespace string, object runtime.Object) {
	t.groupKind = groupKind
	t.namespace = namespace
}

func TestImageStreamReplenishmentUpdateFunc(t *testing.T) {
	for _, tc := range []struct {
		name           string
		oldISStatus    imageapi.ImageStreamStatus
		newISStatus    imageapi.ImageStreamStatus
		expectedUpdate bool
	}{
		{
			name:           "empty",
			expectedUpdate: false,
		},
		{
			name: "no change",
			oldISStatus: imageapi.ImageStreamStatus{
				Tags: map[string]imageapi.TagEventList{
					"foo": {
						Items: []imageapi.TagEvent{
							{DockerImageReference: "foo-ref"},
						},
					},
				},
			},
			newISStatus: imageapi.ImageStreamStatus{
				Tags: map[string]imageapi.TagEventList{
					"foo": {
						Items: []imageapi.TagEvent{
							{DockerImageReference: "foo-ref"},
						},
					},
				},
			},
			expectedUpdate: false,
		},
		{
			name: "first image stream tag",
			newISStatus: imageapi.ImageStreamStatus{
				Tags: map[string]imageapi.TagEventList{
					"latest": {
						Items: []imageapi.TagEvent{
							{DockerImageReference: "latest-ref"},
							{DockerImageReference: "older"},
						},
					},
				},
			},
			expectedUpdate: true,
		},
		{
			name: "image stream tag event deleted",
			oldISStatus: imageapi.ImageStreamStatus{
				Tags: map[string]imageapi.TagEventList{
					"latest": {
						Items: []imageapi.TagEvent{
							{DockerImageReference: "latest-ref"},
							{DockerImageReference: "older"},
						},
					},
				},
			},
			newISStatus: imageapi.ImageStreamStatus{
				Tags: map[string]imageapi.TagEventList{
					"latest": {
						Items: []imageapi.TagEvent{
							{DockerImageReference: "latest-ref"},
						},
					},
				},
			},
			expectedUpdate: true,
		},
	} {
		mockReplenish := &testReplenishment{}
		options := kresourcequota.ReplenishmentControllerOptions{
			GroupKind:         kapi.Kind("ImageStream"),
			ReplenishmentFunc: mockReplenish.Replenish,
			ResyncPeriod:      controller.NoResyncPeriodFunc,
		}
		oldIS := &imageapi.ImageStream{
			ObjectMeta: kapi.ObjectMeta{Namespace: "test", Name: "is"},
			Status:     tc.oldISStatus,
		}
		newIS := &imageapi.ImageStream{
			ObjectMeta: kapi.ObjectMeta{Namespace: "test", Name: "is"},
			Status:     tc.newISStatus,
		}
		updateFunc := ImageStreamReplenishmentUpdateFunc(&options)
		updateFunc(oldIS, newIS)
		if tc.expectedUpdate {
			if mockReplenish.groupKind != kapi.Kind("ImageStream") {
				t.Errorf("[%s]: Unexpected group kind %v", tc.name, mockReplenish.groupKind)
			}
			if mockReplenish.namespace != oldIS.Namespace {
				t.Errorf("[%s]: Unexpected namespace %v", tc.name, mockReplenish.namespace)
			}
		} else {
			if mockReplenish.groupKind.Group != "" || mockReplenish.groupKind.Kind != "" || mockReplenish.namespace != "" {
				t.Errorf("[%s]: Update function unexpectedly called on %s in namespace %s", tc.name, mockReplenish.groupKind, mockReplenish.namespace)
			}
		}
	}
}
