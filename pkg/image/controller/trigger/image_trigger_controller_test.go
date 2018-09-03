package trigger

import (
	"context"
	"fmt"
	"math/rand"
	"reflect"
	"sort"
	"strconv"
	"sync"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	kapierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/diff"
	"k8s.io/apimachinery/pkg/watch"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	appsv1 "github.com/openshift/api/apps/v1"
	buildv1 "github.com/openshift/api/build/v1"
	imagev1 "github.com/openshift/api/image/v1"
	imagev1lister "github.com/openshift/client-go/image/listers/image/v1"
	buildgenerator "github.com/openshift/origin/pkg/build/generator"
	buildutil "github.com/openshift/origin/pkg/build/util"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	triggerapi "github.com/openshift/origin/pkg/image/apis/image/v1/trigger"
	"github.com/openshift/origin/pkg/image/trigger"
	"github.com/openshift/origin/pkg/image/trigger/annotations"
	"github.com/openshift/origin/pkg/image/trigger/buildconfigs"
	"github.com/openshift/origin/pkg/image/trigger/deploymentconfigs"
)

type fakeTagResponse struct {
	Namespace string
	Name      string
	Ref       string
	RV        int64
}

type fakeTagRetriever []fakeTagResponse

func (r fakeTagRetriever) ImageStreamTag(namespace, name string) (string, int64, bool) {
	for _, resp := range r {
		if resp.Namespace != namespace || resp.Name != name {
			continue
		}
		return resp.Ref, resp.RV, true
	}
	return "", 0, false
}

type mockOperationQueue struct {
	lock   sync.Mutex
	queued []interface{}
}

func (q *mockOperationQueue) Add(key interface{}) {
	q.lock.Lock()
	defer q.lock.Unlock()
	q.queued = append(q.queued, key)
}
func (q *mockOperationQueue) AddRateLimited(key interface{})            {}
func (q *mockOperationQueue) AddAfter(key interface{}, d time.Duration) {}
func (q *mockOperationQueue) NumRequeues(key interface{}) int           { return 0 }
func (q *mockOperationQueue) Get() (key interface{}, shutdown bool) {
	return "", false
}
func (q *mockOperationQueue) Done(key interface{})   {}
func (q *mockOperationQueue) Forget(key interface{}) {}
func (q *mockOperationQueue) All() []interface{} {
	q.lock.Lock()
	defer q.lock.Unlock()
	return q.queued
}
func (q *mockOperationQueue) Len() int {
	q.lock.Lock()
	defer q.lock.Unlock()
	return len(q.queued)
}
func (q *mockOperationQueue) ShutDown()          {}
func (q *mockOperationQueue) ShuttingDown() bool { return false }

type streamTagResults struct {
	ref string
	rv  int64
}
type namespaceTags map[string]streamTagResults
type mockTags map[string]namespaceTags

type mockTagRetriever struct {
	calls int
	tags  mockTags
}

func (r *mockTagRetriever) ImageStreamTag(namespace, name string) (string, int64, bool) {
	r.calls++
	if i, ok := r.tags[namespace]; ok {
		if j, ok := i[name]; ok {
			return j.ref, j.rv, true
		}
	}
	return "", 0, false
}

type mockImageStreamLister struct {
	namespace string

	stream *imagev1.ImageStream
	err    error
}

func (l *mockImageStreamLister) List(selector labels.Selector) (ret []*imagev1.ImageStream, err error) {
	return nil, l.err
}
func (l *mockImageStreamLister) ImageStreams(namespace string) imagev1lister.ImageStreamNamespaceLister {
	l.namespace = namespace
	return l
}
func (l *mockImageStreamLister) Get(name string) (*imagev1.ImageStream, error) {
	return l.stream, l.err
}

type imageStreamInformer struct {
	informer cache.SharedIndexInformer
}

func (f *imageStreamInformer) Informer() cache.SharedIndexInformer {
	return f.informer
}

func (f *imageStreamInformer) Lister() imagev1lister.ImageStreamLister {
	return imagev1lister.NewImageStreamLister(f.informer.GetIndexer())
}

type fakeInstantiator struct {
	err error

	namespace          string
	req                *buildv1.BuildRequest
	generator          *buildgenerator.BuildGenerator
	buildConfigUpdater *fakeBuildConfigUpdater
}

func (i *fakeInstantiator) Instantiate(namespace string, req *buildv1.BuildRequest) (*buildv1.Build, error) {
	if i.err != nil {
		return nil, i.err
	}
	i.req, i.namespace = req, namespace
	if i.generator == nil {
		return nil, nil
	}
	return i.generator.Instantiate(apirequest.WithNamespace(apirequest.NewContext(), namespace), req)
}

type fakeBuildConfigUpdater struct {
	updateCount int
	buildcfg    *buildv1.BuildConfig
	err         error
}

func (m *fakeBuildConfigUpdater) Update(buildcfg *buildv1.BuildConfig) error {
	m.buildcfg = buildcfg
	m.updateCount++
	return m.err
}

func fakeBuildConfigInstantiator(buildcfg *buildv1.BuildConfig, imageStream *imagev1.ImageStream) *fakeInstantiator {
	builderAccount := corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{Name: bootstrappolicy.BuilderServiceAccountName, Namespace: buildcfg.Namespace},
		Secrets:    []corev1.ObjectReference{},
	}
	instantiator := &fakeInstantiator{}
	instantiator.buildConfigUpdater = &fakeBuildConfigUpdater{}
	generator := &buildgenerator.BuildGenerator{
		Secrets:         fake.NewSimpleClientset().Core(),
		ServiceAccounts: fake.NewSimpleClientset(&builderAccount).Core(),
		Client: buildgenerator.TestingClient{
			GetBuildConfigFunc: func(ctx context.Context, name string, options *metav1.GetOptions) (*buildv1.BuildConfig, error) {
				return buildcfg, nil
			},
			UpdateBuildConfigFunc: func(ctx context.Context, buildConfig *buildv1.BuildConfig) error {
				return instantiator.buildConfigUpdater.Update(buildcfg)
			},
			CreateBuildFunc: func(ctx context.Context, build *buildv1.Build) error {
				return nil
			},
			GetBuildFunc: func(ctx context.Context, name string, options *metav1.GetOptions) (*buildv1.Build, error) {
				return nil, nil
			},
			GetImageStreamFunc: func(ctx context.Context, name string, options *metav1.GetOptions) (*imagev1.ImageStream, error) {
				return imageStream, nil
			},
			GetImageStreamTagFunc: func(ctx context.Context, name string, options *metav1.GetOptions) (*imagev1.ImageStreamTag, error) {
				return nil, nil
			},
			GetImageStreamImageFunc: func(ctx context.Context, name string, options *metav1.GetOptions) (*imagev1.ImageStreamImage, error) {
				return nil, nil
			},
		}}
	instantiator.generator = generator
	return instantiator
}

func TestTriggerControllerSyncImageStream(t *testing.T) {
	queue := &mockOperationQueue{}
	lister := &mockImageStreamLister{
		stream: scenario_1_imageStream_single("test", "stream", "10"),
	}
	controller := TriggerController{
		triggerCache:     NewTriggerCache(),
		lister:           lister,
		imageChangeQueue: queue,
	}
	controller.triggerCache.Add("buildconfigs/test/build1", &trigger.CacheEntry{
		Key:       "buildconfigs/test/build1",
		Namespace: "test",
		Triggers: []triggerapi.ObjectFieldTrigger{
			{From: triggerapi.ObjectReference{Kind: "ImageStreamTag", Name: "stream:1"}},
			{From: triggerapi.ObjectReference{Kind: "ImageStreamTag", Name: "stream:2"}},
			{From: triggerapi.ObjectReference{Kind: "DockerImage", Name: "test/stream:1"}},
		},
	})
	if err := controller.syncImageStream("test/stream"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	queued := queue.All()
	if len(queued) != 1 || queued[0] != "buildconfigs/test/build1" {
		t.Errorf("unexpected changes: %#v", queued)
	}
}

func TestTriggerControllerSyncBuildConfigResource(t *testing.T) {
	tests := []struct {
		name    string
		is      *imagev1.ImageStream
		bc      *buildv1.BuildConfig
		tagResp []fakeTagResponse
		req     *buildv1.BuildRequest
	}{
		{
			name:    "NewImageID",
			is:      scenario_1_imageStream_single("test", "stream", "10"),
			bc:      scenario_1_buildConfig_imageSource(),
			tagResp: []fakeTagResponse{{Namespace: "other", Name: "stream:2", Ref: "image/result:1"}},
			req: &buildv1.BuildRequest{
				ObjectMeta:       metav1.ObjectMeta{Name: "build2", Namespace: "test2"},
				From:             &corev1.ObjectReference{Kind: "ImageStreamTag", Name: "stream:2", Namespace: "other"},
				TriggeredByImage: &corev1.ObjectReference{Kind: "DockerImage", Name: "image/result:1"},
				TriggeredBy: []buildv1.BuildTriggerCause{
					{
						Message: "Image change",
						ImageChangeBuild: &buildv1.ImageChangeCause{
							ImageID: "image/result:1",
							FromRef: &corev1.ObjectReference{Kind: "ImageStreamTag", Name: "stream:2", Namespace: "other"},
						},
					},
				},
			},
		},
		{
			name:    "NewImageIDDefaultTag",
			is:      scenario_1_imageStream_single_defaultImageTag("test", "stream", "10"),
			bc:      scenario_1_buildConfig_imageSource_defaultImageTag(),
			tagResp: []fakeTagResponse{{Namespace: "other", Name: "stream:latest", Ref: "image/result:1"}},
			req: &buildv1.BuildRequest{
				ObjectMeta: metav1.ObjectMeta{Name: "build2", Namespace: "test2"},
				From: &corev1.ObjectReference{APIVersion: imagev1.GroupVersion.Version, Kind: "ImageStreamTag", Name: "stream:latest",
					Namespace: "other"},
				TriggeredByImage: &corev1.ObjectReference{Kind: "DockerImage", Name: "image/result:1"},
				TriggeredBy: []buildv1.BuildTriggerCause{
					{
						Message: "Image change",
						ImageChangeBuild: &buildv1.ImageChangeCause{
							ImageID: "image/result:1",
							FromRef: &corev1.ObjectReference{APIVersion: imagev1.GroupVersion.Version, Kind: "ImageStreamTag", Name: "stream:latest",
								Namespace: "other"},
						},
					},
				},
			},
		},
	}

	for _, test := range tests {
		lister := &mockImageStreamLister{
			stream: test.is,
		}

		store := &cache.FakeCustomStore{
			GetByKeyFunc: func(key string) (interface{}, bool, error) {
				return test.bc, true, nil
			},
		}
		inst := fakeBuildConfigInstantiator(test.bc, test.is)
		reaction := buildconfigs.NewBuildConfigReactor(inst, nil)
		controller := TriggerController{
			triggerCache: NewTriggerCache(),
			lister:       lister,
			triggerSources: map[string]TriggerSource{
				"buildconfigs": {
					Store:   store,
					Reactor: reaction,
				},
			},
			tagRetriever: fakeTagRetriever(test.tagResp),
		}
		if err := controller.syncResource("buildconfigs/test/build1"); err != nil {
			t.Errorf("For test %s unexpected error: %v", test.name, err)
		}
		if inst.namespace != "test2" || !reflect.DeepEqual(inst.req, test.req) {
			t.Errorf("For test %s unexpected: %s %s", test.name, inst.namespace, diff.ObjectReflectDiff(test.req, inst.req))
		}
		if inst.buildConfigUpdater.buildcfg == nil {
			t.Fatalf("For test %s expected buildConfig update when new image was created!", test.name)
		}
		found := false
		imageIDs := ""
		for _, trigger := range inst.buildConfigUpdater.buildcfg.Spec.Triggers {
			if trigger.ImageChange != nil {
				if actual, expected := trigger.ImageChange.LastTriggeredImageID, "image/result:2"; actual == expected {
					found = true
				}
				imageIDs = imageIDs + trigger.ImageChange.LastTriggeredImageID + "\n"
			}
		}
		if !found {
			t.Errorf("For test %s instead of 'image/result:2' found the following last triggered image ID's: %s", test.name, imageIDs)
		}
	}
}

func TestTriggerControllerSyncBuildConfigResourceErrorHandling(t *testing.T) {
	tests := []struct {
		name    string
		bc      *buildv1.BuildConfig
		err     error
		tagResp []fakeTagResponse
	}{
		{
			name: "NonExistentImageStrem",
			bc:   scenario_1_buildConfig_imageSource(),
		},
		{
			name:    "DifferentTagUpdate",
			bc:      scenario_1_buildConfig_imageSource(),
			tagResp: []fakeTagResponse{{Namespace: "other", Name: "stream2:3", Ref: "image/result:3"}},
		},
		{
			name:    "DifferentTagUpdate2",
			bc:      scenario_1_buildConfig_imageSource_previousBuildForTag(),
			tagResp: []fakeTagResponse{{Namespace: "other", Name: "stream2:3", Ref: "image/result:3"}},
		},
		{
			name:    "DifferentImageUpdate",
			bc:      scenario_1_buildConfig_imageSource(),
			tagResp: []fakeTagResponse{{Namespace: "other2", Name: "stream:3", Ref: "image/result:4"}},
		},
		{
			name:    "DifferentNamespace",
			bc:      scenario_1_buildConfig_imageSource(),
			tagResp: []fakeTagResponse{{Namespace: "foo", Name: "stream:2", Ref: "image/result:1"}},
		},
		{
			name:    "DifferentTriggerType",
			bc:      scenario_1_buildConfig_otherTrigger(),
			tagResp: []fakeTagResponse{{Namespace: "foo", Name: "stream:2", Ref: "image/result:1"}},
		},
		{
			name:    "NoImageIDChange",
			bc:      scenario_1_buildConfig_imageSource_noImageIDChange(),
			tagResp: []fakeTagResponse{{Namespace: "other", Name: "stream:2", Ref: "image/result:1"}},
		},
		{
			name:    "InstantiationError",
			bc:      scenario_1_buildConfig_imageSource(),
			err:     fmt.Errorf("instantiation error"),
			tagResp: []fakeTagResponse{{Namespace: "other", Name: "stream:2", Ref: "image/result:1"}},
		},
	}

	for _, test := range tests {
		lister := &mockImageStreamLister{
			stream: scenario_1_imageStream_single("test", "stream", "10"),
		}
		store := &cache.FakeCustomStore{
			GetByKeyFunc: func(key string) (interface{}, bool, error) {
				return test.bc, true, nil
			},
		}
		inst := fakeBuildConfigInstantiator(test.bc, nil)
		if test.err != nil {
			inst.err = test.err
		}
		reaction := buildconfigs.NewBuildConfigReactor(inst, nil)
		controller := TriggerController{
			triggerCache: NewTriggerCache(),
			lister:       lister,
			triggerSources: map[string]TriggerSource{
				"buildconfigs": {
					Store:   store,
					Reactor: reaction,
				},
			},
			tagRetriever: fakeTagRetriever(test.tagResp),
		}

		err := controller.syncResource("buildconfigs/test/build1")
		if err == nil && test.err != nil {
			t.Errorf("Test for %s expected error but got nil", test.name)
		}
		if err != nil && test.err == nil {
			t.Errorf("Test for %s got unexpected error %v", test.name, err)
		}
		if err != nil && test.err != nil && !reflect.DeepEqual(err, inst.err) {
			t.Errorf("Test for %s expected error %v but got %v", test.name, inst.err, err)
		}
		if inst.req != nil {
			t.Errorf("Test for %s generated build unexpectedly", test.name)
		}
		if inst.buildConfigUpdater.buildcfg != nil {
			t.Errorf("Test for %s updated the build config unexpectedly", test.name)
		}
	}
}

func TestBuildConfigTriggerIndexer(t *testing.T) {
	stopCh := make(chan struct{})
	defer close(stopCh)
	informer, fw := newFakeInformer(&buildv1.BuildConfig{}, &buildv1.BuildConfigList{ListMeta: metav1.ListMeta{ResourceVersion: "1"}})

	c := NewTriggerCache()
	r := &mockTagRetriever{}

	queue := &mockOperationQueue{}
	sources := []TriggerSource{
		{
			Resource: schema.GroupResource{Resource: "buildconfigs"},
			Informer: informer,
			TriggerFn: func(prefix string) trigger.Indexer {
				return buildconfigs.NewBuildConfigTriggerIndexer(prefix)
			},
		},
	}
	_, syncs, err := setupTriggerSources(c, r, sources, queue)
	if err != nil {
		t.Fatal(err)
	}
	go informer.Run(stopCh)
	if !cache.WaitForCacheSync(stopCh, syncs...) {
		t.Fatal("Unsynced")
	}

	// Verifies that two builds added to the informer:
	// - Perform a proper index of the triggers
	// - Queue the right changes, representing the changed/not-available images
	r.tags = mockTags{
		"test": namespaceTags{
			"stream:1": streamTagResults{ref: "image/result:1", rv: 10},
		},
		"other": namespaceTags{
			"stream:2": streamTagResults{ref: "image/result:2", rv: 11},
		},
	}
	fw.Add(scenario_1_buildConfig_strategy())
	fw.Add(scenario_1_buildConfig_imageSource())

	for len(c.List()) != 2 {
		time.Sleep(1 * time.Millisecond)
	}

	actual, ok := c.Get("buildconfigs/test/build1")
	if e := scenario_1_buildConfig_strategy_cacheEntry(); !ok || !reflect.DeepEqual(e, actual) {
		t.Fatalf("unexpected: %s", diff.ObjectReflectDiff(e, actual))
	}
	if err := verifyEntriesAt(c, []interface{}{scenario_1_buildConfig_strategy_cacheEntry()}, "test/stream"); err != nil {
		t.Fatal(err)
	}

	// verify we create two index entries and can cross namespaces with trigger types
	actual, ok = c.Get("buildconfigs/test2/build2")
	if e := scenario_1_buildConfig_imageSource_cacheEntry(); !ok || !reflect.DeepEqual(e, actual) {
		t.Fatalf("unexpected: %s", diff.ObjectReflectDiff(e, actual))
	}
	if err := verifyEntriesAt(c, []interface{}{scenario_1_buildConfig_imageSource_cacheEntry()}, "other/stream", "test2/stream"); err != nil {
		t.Fatal(err)
	}

	// should have enqueued a single action (based on the image stream tag retriever)
	queued := queue.All()
	expected := []interface{}{
		"buildconfigs/test/build1",
		"buildconfigs/test2/build2",
	}
	if !reflect.DeepEqual(expected, queued) {
		t.Fatalf("changes: %#v", queued)
	}
}

func TestDeploymentConfigTriggerIndexer(t *testing.T) {
	stopCh := make(chan struct{})
	defer close(stopCh)
	informer, fw := newFakeInformer(&appsv1.DeploymentConfig{}, &appsv1.DeploymentConfigList{ListMeta: metav1.ListMeta{ResourceVersion: "1"}})

	c := NewTriggerCache()
	r := &mockTagRetriever{}

	queue := &mockOperationQueue{}
	sources := []TriggerSource{
		{
			Resource: schema.GroupResource{Resource: "deploymentconfigs"},
			Informer: informer,
			TriggerFn: func(prefix string) trigger.Indexer {
				return deploymentconfigs.NewDeploymentConfigTriggerIndexer(prefix)
			},
		},
	}
	_, syncs, err := setupTriggerSources(c, r, sources, queue)
	if err != nil {
		t.Fatal(err)
	}
	go informer.Run(stopCh)
	if !cache.WaitForCacheSync(stopCh, syncs...) {
		t.Fatal("Unsynced")
	}

	// Verifies that two builds added to the informer:
	// - Perform a proper index of the triggers
	// - Queue the right changes, representing the changed/not-available images
	r.tags = mockTags{
		"test": namespaceTags{
			"stream:1": streamTagResults{ref: "image/result:1", rv: 10},
		},
		"other": namespaceTags{
			"stream:2": streamTagResults{ref: "image/result:2", rv: 11},
		},
	}
	fw.Add(scenario_1_deploymentConfig_imageSource())

	for len(c.List()) != 1 {
		time.Sleep(1 * time.Millisecond)
	}

	actual, ok := c.Get("deploymentconfigs/test/deploy1")
	if e := scenario_1_deploymentConfig_imageSource_cacheEntry(); !ok || !reflect.DeepEqual(e, actual) {
		t.Fatalf("unexpected: %s\n%#v", diff.ObjectReflectDiff(e, actual), actual)
	}
	if err := verifyEntriesAt(c, []interface{}{scenario_1_deploymentConfig_imageSource_cacheEntry()}, "test/stream"); err != nil {
		t.Fatal(err)
	}

	// should have enqueued a single action (based on the image stream tag retriever)
	queued := queue.All()
	expected := []interface{}{"deploymentconfigs/test/deploy1"}
	if !reflect.DeepEqual(expected, queued) {
		t.Fatalf("changes: %#v", queued)
	}
}

func verifyEntriesAt(c cache.ThreadSafeStore, entries []interface{}, keys ...string) error {
	for _, key := range keys {
		indexed, err := c.ByIndex("images", key)
		if err != nil {
			return fmt.Errorf("unexpected error for key %s: %v", key, err)
		}
		if e, a := entries, indexed; !reflect.DeepEqual(e, a) {
			return fmt.Errorf("unexpected entry for key %s: %s", key, diff.ObjectReflectDiff(e, a))
		}
	}
	return nil
}

func scenario_1_buildConfig_strategy() *buildv1.BuildConfig {
	return &buildv1.BuildConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "build1", Namespace: "test"},
		Spec: buildv1.BuildConfigSpec{
			Triggers: []buildv1.BuildTriggerPolicy{
				{ImageChange: &buildv1.ImageChangeTrigger{}},
			},
			CommonSpec: buildv1.CommonSpec{
				Strategy: buildv1.BuildStrategy{
					DockerStrategy: &buildv1.DockerBuildStrategy{
						From: &corev1.ObjectReference{Kind: "ImageStreamTag", Name: "stream:1"},
					},
				},
			},
		},
	}
}

func scenario_1_imageStream_single(namespace, name, rv string) *imagev1.ImageStream {
	return &imagev1.ImageStream{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace, ResourceVersion: rv},
		Status: imagev1.ImageStreamStatus{
			Tags: []imagev1.NamedTagEventList{
				{
					Tag: "1",
					Items: []imagev1.TagEvent{
						{DockerImageReference: "image/result:1"},
					},
				},
			},
		},
	}
}

func scenario_1_imageStream_single_defaultImageTag(namespace, name, rv string) *imagev1.ImageStream {
	return &imagev1.ImageStream{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace, ResourceVersion: rv},
		Status: imagev1.ImageStreamStatus{
			Tags: []imagev1.NamedTagEventList{
				{
					Tag: "latest",
					Items: []imagev1.TagEvent{
						{DockerImageReference: "image/result:1"},
					},
				},
			},
		},
	}
}

func scenario_1_buildConfig_imageSource_defaultImageTag() *buildv1.BuildConfig {
	return &buildv1.BuildConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "build2", Namespace: "test2"},
		Spec: buildv1.BuildConfigSpec{
			Triggers: []buildv1.BuildTriggerPolicy{
				{ImageChange: &buildv1.ImageChangeTrigger{}},
				{ImageChange: &buildv1.ImageChangeTrigger{
					From: &corev1.ObjectReference{APIVersion: imagev1.GroupVersion.Version, Kind: "ImageStreamTag",
						Name:      "stream:latest",
						Namespace: "other"},
					LastTriggeredImageID: "image/result:2",
				}},
				{ImageChange: &buildv1.ImageChangeTrigger{From: &corev1.ObjectReference{Kind: "DockerImage", Name: "mysql", Namespace: "other"}}},
			},
			CommonSpec: buildv1.CommonSpec{
				Strategy: buildv1.BuildStrategy{
					DockerStrategy: &buildv1.DockerBuildStrategy{
						From: &corev1.ObjectReference{APIVersion: imagev1.GroupVersion.Version, Kind: "ImageStreamTag", Name: "stream:latest"},
					},
				},
			},
		},
	}
}
func scenario_1_buildConfig_strategy_cacheEntry() *trigger.CacheEntry {
	return &trigger.CacheEntry{
		Key:       "buildconfigs/test/build1",
		Namespace: "test",
		Triggers: []triggerapi.ObjectFieldTrigger{
			{From: triggerapi.ObjectReference{Kind: "ImageStreamTag", Name: "stream:1"}, FieldPath: "spec.strategy.*.from"},
		},
	}
}

func scenario_1_deploymentConfig_imageSource() *appsv1.DeploymentConfig {
	return &appsv1.DeploymentConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "deploy1", Namespace: "test"},
		Spec: appsv1.DeploymentConfigSpec{
			Triggers: []appsv1.DeploymentTriggerPolicy{
				{ImageChangeParams: &appsv1.DeploymentTriggerImageChangeParams{
					Automatic:          true,
					ContainerNames:     []string{"first", "second"},
					From:               corev1.ObjectReference{Kind: "ImageStreamTag", Name: "stream:1"},
					LastTriggeredImage: "image/result:2",
				}},
				{ImageChangeParams: &appsv1.DeploymentTriggerImageChangeParams{
					Automatic:      true,
					ContainerNames: []string{"third"},
					From:           corev1.ObjectReference{Kind: "DockerImage", Name: "mysql", Namespace: "other"},
				}},
			},
			Template: &corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "first", Image: "image/result:2"},
						{Name: "second", Image: ""},
						{Name: "third", Image: ""},
					},
				},
			},
		},
	}
}

func scenario_1_deploymentConfig_imageSource_cacheEntry() *trigger.CacheEntry {
	return &trigger.CacheEntry{
		Key:       "deploymentconfigs/test/deploy1",
		Namespace: "test",
		Triggers: []triggerapi.ObjectFieldTrigger{
			{From: triggerapi.ObjectReference{Kind: "ImageStreamTag", Name: "stream:1"}, FieldPath: "spec.template.spec.containers[@name==\"first\"].image"},
			{From: triggerapi.ObjectReference{Kind: "ImageStreamTag", Name: "stream:1"}, FieldPath: "spec.template.spec.containers[@name==\"second\"].image"},
		},
	}
}

func scenario_1_buildConfig_otherTrigger() *buildv1.BuildConfig {
	return &buildv1.BuildConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "build2", Namespace: "test2"},
		Spec: buildv1.BuildConfigSpec{
			Triggers: []buildv1.BuildTriggerPolicy{
				{
					Type:           buildv1.GenericWebHookBuildTriggerType,
					GenericWebHook: &buildv1.WebHookTrigger{},
				},
			},
			CommonSpec: buildv1.CommonSpec{
				Strategy: buildv1.BuildStrategy{
					DockerStrategy: &buildv1.DockerBuildStrategy{
						From: &corev1.ObjectReference{Kind: "ImageStreamTag", Name: "stream:1"},
					},
				},
			},
		},
	}
}

func scenario_1_buildConfig_imageSource() *buildv1.BuildConfig {
	return &buildv1.BuildConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "build2", Namespace: "test2"},
		Spec: buildv1.BuildConfigSpec{
			Triggers: []buildv1.BuildTriggerPolicy{
				{ImageChange: &buildv1.ImageChangeTrigger{}},
				{ImageChange: &buildv1.ImageChangeTrigger{
					From:                 &corev1.ObjectReference{Kind: "ImageStreamTag", Name: "stream:2", Namespace: "other"},
					LastTriggeredImageID: "image/result:2",
				}},
				{ImageChange: &buildv1.ImageChangeTrigger{From: &corev1.ObjectReference{Kind: "DockerImage", Name: "mysql", Namespace: "other"}}},
			},
			CommonSpec: buildv1.CommonSpec{
				Strategy: buildv1.BuildStrategy{
					DockerStrategy: &buildv1.DockerBuildStrategy{
						From: &corev1.ObjectReference{Kind: "ImageStreamTag", Name: "stream:1"},
					},
				},
			},
		},
	}
}

func scenario_1_buildConfig_imageSource_previousBuildForTag() *buildv1.BuildConfig {
	return &buildv1.BuildConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "build2", Namespace: "test2"},
		Spec: buildv1.BuildConfigSpec{
			Triggers: []buildv1.BuildTriggerPolicy{
				{ImageChange: &buildv1.ImageChangeTrigger{}},
				{ImageChange: &buildv1.ImageChangeTrigger{
					From:                 &corev1.ObjectReference{Kind: "ImageStreamTag", Name: "stream:2", Namespace: "other"},
					LastTriggeredImageID: "image/result:3",
				}},
				{ImageChange: &buildv1.ImageChangeTrigger{From: &corev1.ObjectReference{Kind: "DockerImage", Name: "mysql", Namespace: "other"}}},
			},
			CommonSpec: buildv1.CommonSpec{
				Strategy: buildv1.BuildStrategy{
					DockerStrategy: &buildv1.DockerBuildStrategy{
						From: &corev1.ObjectReference{Kind: "ImageStreamTag", Name: "stream:1"},
					},
				},
			},
		},
	}
}

func scenario_1_buildConfig_imageSource_noImageIDChange() *buildv1.BuildConfig {
	return &buildv1.BuildConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "build2", Namespace: "test2"},
		Spec: buildv1.BuildConfigSpec{
			Triggers: []buildv1.BuildTriggerPolicy{
				{ImageChange: &buildv1.ImageChangeTrigger{}},
				{ImageChange: &buildv1.ImageChangeTrigger{
					From:                 &corev1.ObjectReference{Kind: "ImageStreamTag", Name: "stream:2", Namespace: "other"},
					LastTriggeredImageID: "image/result:1",
				}},
				{ImageChange: &buildv1.ImageChangeTrigger{From: &corev1.ObjectReference{Kind: "DockerImage", Name: "mysql", Namespace: "other"}}},
			},
			CommonSpec: buildv1.CommonSpec{
				Strategy: buildv1.BuildStrategy{
					DockerStrategy: &buildv1.DockerBuildStrategy{
						From: &corev1.ObjectReference{Kind: "ImageStreamTag", Name: "stream:1"},
					},
				},
			},
		},
	}
}

func scenario_1_buildConfig_imageSource_cacheEntry() *trigger.CacheEntry {
	return &trigger.CacheEntry{
		Key:       "buildconfigs/test2/build2",
		Namespace: "test2",
		Triggers: []triggerapi.ObjectFieldTrigger{
			{From: triggerapi.ObjectReference{Kind: "ImageStreamTag", Name: "stream:1"}, FieldPath: "spec.strategy.*.from"},
			{From: triggerapi.ObjectReference{Kind: "ImageStreamTag", Name: "stream:2", Namespace: "other"}, FieldPath: "spec.triggers"},
		},
	}
}

func newFakeInformer(item, initialList runtime.Object) (cache.SharedIndexInformer, *watch.RaceFreeFakeWatcher) {
	fw := watch.NewRaceFreeFake()
	lw := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			return initialList, nil
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) { return fw, nil },
	}
	informer := cache.NewSharedIndexInformer(lw, item, 0, nil)
	return informer, fw
}

type fakeImageReactor struct {
	lock   sync.Mutex
	nested trigger.ImageReactor
	calls  int
	err    error
}

type imageReactorFunc func(obj runtime.Object, tagRetriever trigger.TagRetriever) error

func (fn imageReactorFunc) ImageChanged(obj runtime.Object, tagRetriever trigger.TagRetriever) error {
	return fn(obj, tagRetriever)
}

func (r *fakeImageReactor) ImageChanged(obj runtime.Object, tagRetriever trigger.TagRetriever) error {
	r.lock.Lock()
	defer r.lock.Unlock()
	err := r.err
	if r.nested != nil {
		if cerr := r.nested.ImageChanged(obj, tagRetriever); cerr != nil {
			err = cerr
		}
	}
	r.calls++
	return err
}

func (r *fakeImageReactor) Results() *fakeImageReactor {
	r.lock.Lock()
	defer r.lock.Unlock()
	return &fakeImageReactor{
		nested: r.nested,
		calls:  r.calls,
		err:    r.err,
	}
}

func randomStreamTag(r *rand.Rand, maxStreams, maxTags int32) string {
	return fmt.Sprintf("stream-%d:%d", r.Int31n(maxStreams), r.Int31n(maxTags))
}

func benchmark_1_buildConfig(r *rand.Rand, identity, maxStreams, maxTags, triggers int32) *buildv1.BuildConfig {
	bc := &buildv1.BuildConfig{
		ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("build-%d", identity), Namespace: "test"},
		Spec: buildv1.BuildConfigSpec{
			Triggers: []buildv1.BuildTriggerPolicy{
				{ImageChange: &buildv1.ImageChangeTrigger{}},
			},
			CommonSpec: buildv1.CommonSpec{
				Strategy: buildv1.BuildStrategy{
					DockerStrategy: &buildv1.DockerBuildStrategy{
						From: &corev1.ObjectReference{Kind: "ImageStreamTag", Name: randomStreamTag(r, maxStreams, maxTags)},
					},
				},
			},
		},
	}
	if triggers == 0 {
		bc.Spec.Triggers = nil
	}
	for i := int32(0); i < (triggers - 1); i++ {
		bc.Spec.Triggers = append(bc.Spec.Triggers, buildv1.BuildTriggerPolicy{
			ImageChange: &buildv1.ImageChangeTrigger{From: &corev1.ObjectReference{Kind: "ImageStreamTag", Name: randomStreamTag(r, maxStreams, maxTags)}},
		})
	}
	return bc
}

func benchmark_1_pod(r *rand.Rand, identity, maxStreams, maxTags, containers int32) *kapi.Pod {
	pod := &kapi.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("pod-%d", identity),
			Namespace: "test",
			Annotations: map[string]string{
				triggerapi.TriggerAnnotationKey: fmt.Sprintf(
					`[
						{"from":{"kind":"ImageStreamTag","name":"%s"},"fieldPath":"spec.containers[0].image"},
						{"from":{"kind":"ImageStreamTag","name":"%s"},"fieldPath":"spec.containers[1].image"}
					]`,
					randomStreamTag(r, maxStreams, maxTags),
					randomStreamTag(r, maxStreams, maxTags),
				),
			},
		},
		Spec: kapi.PodSpec{},
	}
	for i := int32(0); i < containers; i++ {
		pod.Spec.Containers = append(pod.Spec.Containers, kapi.Container{Name: fmt.Sprintf("container-%d", i), Image: "initial-image"})
	}
	return pod
}

func benchmark_1_deploymentConfig(r *rand.Rand, identity, maxStreams, maxTags, containers int32) *appsv1.DeploymentConfig {
	dc := &appsv1.DeploymentConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("dc-%d", identity),
			Namespace: "test",
		},
		Spec: appsv1.DeploymentConfigSpec{
			Template: &corev1.PodTemplateSpec{},
		},
	}
	for i := int32(0); i < containers; i++ {
		dc.Spec.Triggers = append(dc.Spec.Triggers, appsv1.DeploymentTriggerPolicy{
			ImageChangeParams: &appsv1.DeploymentTriggerImageChangeParams{
				Automatic:      true,
				ContainerNames: []string{fmt.Sprintf("container-%d", i)},
				From:           corev1.ObjectReference{Kind: "ImageStreamTag", Name: randomStreamTag(r, maxStreams, maxTags)},
			},
		})
		dc.Spec.Template.Spec.Containers = append(dc.Spec.Template.Spec.Containers, corev1.Container{Name: fmt.Sprintf("container-%d", i), Image: "initial-image"})
	}
	return dc
}

func benchmark_1_imageStream(identity, maxTags, sequence int32, round, index int) *imagev1.ImageStream {
	is := &imagev1.ImageStream{
		ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("stream-%d", identity), Namespace: "test"},
		Status:     imagev1.ImageStreamStatus{Tags: []imagev1.NamedTagEventList{}},
	}
	for i := int32(0); i < maxTags; i++ {
		is.Status.Tags = append(is.Status.Tags, imagev1.NamedTagEventList{
			Tag: strconv.Itoa(int(i)),
			Items: []imagev1.TagEvent{
				{DockerImageReference: fmt.Sprintf("image-%d-%d:%d-%d-%d", identity, i, round, index, sequence)},
			},
		})
	}
	return is
}

// updateBuildConfigImages updates the LastTriggeredImageID field on a build config.
func updateBuildConfigImages(bc *buildv1.BuildConfig, tagRetriever trigger.TagRetriever) (*buildv1.BuildConfig, error) {
	var updated *buildv1.BuildConfig
	for i, t := range bc.Spec.Triggers {
		p := t.ImageChange
		if p == nil || (p.From != nil && p.From.Kind != "ImageStreamTag") {
			continue
		}
		var from *corev1.ObjectReference
		if p.From != nil {
			from = p.From
		} else {
			from = buildutil.GetInputReference(bc.Spec.Strategy)
		}
		namespace := from.Namespace
		if len(namespace) == 0 {
			namespace = bc.Namespace
		}
		latest, _, found := tagRetriever.ImageStreamTag(namespace, from.Name)
		if !found || latest == p.LastTriggeredImageID {
			continue
		}
		if updated == nil {
			updated = bc.DeepCopy()
		}
		p = updated.Spec.Triggers[i].ImageChange
		p.LastTriggeredImageID = latest
	}
	return updated, nil
}

// alterBuildConfigFromTriggers will alter the incoming build config based on the trigger
// changes passed to it and send it back on the watch as a modification.
func alterBuildConfigFromTriggers(bcWatch *consistentWatch) imageReactorFunc {
	return imageReactorFunc(func(obj runtime.Object, tagRetriever trigger.TagRetriever) error {
		bc := obj.DeepCopyObject()
		updated, err := updateBuildConfigImages(bc.(*buildv1.BuildConfig), tagRetriever)
		if err != nil {
			return err
		}
		if updated != nil {
			return bcWatch.Modify(updated)
		}
		return nil
	})
}

func alterDeploymentConfigFromTriggers(dcWatch *consistentWatch) imageReactorFunc {
	return imageReactorFunc(func(obj runtime.Object, tagRetriever trigger.TagRetriever) error {
		dc := obj.DeepCopyObject()
		updated, resolvable, err := deploymentconfigs.UpdateDeploymentConfigImages(dc.(*appsv1.DeploymentConfig), tagRetriever)
		if err != nil {
			return err
		}
		if updated != nil && resolvable {
			return dcWatch.Modify(updated)
		}
		return nil
	})
}

// alterPodFromTriggers will alter the incoming pod based on the trigger
// changes passed to it and send it back on the watch as a modification.
func alterPodFromTriggers(podWatch *watch.RaceFreeFakeWatcher) imageReactorFunc {
	count := 2
	return imageReactorFunc(func(obj runtime.Object, tagRetriever trigger.TagRetriever) error {
		pod := obj.DeepCopyObject()

		updated, err := annotations.UpdateObjectFromImages(pod.(*kapi.Pod), tagRetriever)
		if err != nil {
			return err
		}
		if updated != nil {
			updated.(*kapi.Pod).ResourceVersion = strconv.Itoa(count)
			count++
			podWatch.Modify(updated)
		}
		return nil
	})
}

type consistentWatch struct {
	lock   sync.Mutex
	watch  *watch.RaceFreeFakeWatcher
	latest map[string]int64
}

func (w *consistentWatch) Add(obj runtime.Object) error {
	w.lock.Lock()
	defer w.lock.Unlock()
	m, err := meta.Accessor(obj)
	if err != nil {
		return err
	}
	if w.latest == nil {
		w.latest = make(map[string]int64)
	}
	if len(m.GetResourceVersion()) == 0 {
		m.SetResourceVersion("0")
	}
	rv, err := strconv.ParseInt(m.GetResourceVersion(), 10, 64)
	if err != nil {
		return err
	}
	key := m.GetNamespace() + "/" + m.GetName()
	if latest, ok := w.latest[key]; ok {
		if latest != rv {
			return kapierrs.NewAlreadyExists(schema.GroupResource{}, m.GetName())
		}
	}
	rv++
	w.latest[key] = rv
	m.SetResourceVersion(strconv.Itoa(int(rv)))
	w.watch.Add(obj)
	return nil
}

func (w *consistentWatch) Modify(obj runtime.Object) error {
	w.lock.Lock()
	defer w.lock.Unlock()
	m, err := meta.Accessor(obj)
	if err != nil {
		return err
	}
	if w.latest == nil {
		w.latest = make(map[string]int64)
	}
	if len(m.GetResourceVersion()) == 0 {
		m.SetResourceVersion("0")
	}
	rv, err := strconv.ParseInt(m.GetResourceVersion(), 10, 64)
	if err != nil {
		return err
	}
	key := m.GetNamespace() + "/" + m.GetName()
	if latest, ok := w.latest[key]; ok {
		if rv != 0 && latest != rv {
			return kapierrs.NewConflict(schema.GroupResource{}, m.GetName(), fmt.Errorf("unable to update, resource version %d does not match %d", rv, latest))
		}
	}
	rv++
	w.latest[key] = rv
	m.SetResourceVersion(strconv.Itoa(int(rv)))
	w.watch.Modify(obj)
	return nil
}

func TestTriggerController(t *testing.T) {
	// tuning
	var rounds, iterations = 100, 250
	var numStreams, numBuildConfigs, numPods, numDeploymentConfigs int32 = 10, 10, 10, 10
	var numTagsPerStream, maxTriggersPerBuild, maxContainersPerPod int32 = 5, 1, 2
	var ratioReferencedStreams, ratioTriggeredBuildConfigs float32 = 0.50, 1
	var ratioStreamChanges float32 = 0.50
	rnd := rand.New(rand.NewSource(1))

	stopCh := make(chan struct{})
	defer close(stopCh)
	bcInformer, bcFakeWatch := newFakeInformer(&buildv1.BuildConfig{}, &buildv1.BuildConfigList{ListMeta: metav1.ListMeta{ResourceVersion: "1"}})
	bcWatch := &consistentWatch{watch: bcFakeWatch}
	isInformer, isFakeWatch := newFakeInformer(&imagev1.ImageStream{}, &imagev1.ImageStreamList{ListMeta: metav1.ListMeta{ResourceVersion: "1"}})
	isWatch := &consistentWatch{watch: isFakeWatch}
	podInformer, podWatch := newFakeInformer(&kapi.Pod{}, &kapi.PodList{ListMeta: metav1.ListMeta{ResourceVersion: "1"}})
	dcInformer, dcFakeWatch := newFakeInformer(&appsv1.DeploymentConfig{}, &appsv1.DeploymentConfigList{ListMeta: metav1.ListMeta{ResourceVersion: "1"}})
	dcWatch := &consistentWatch{watch: dcFakeWatch}

	buildReactorFn := alterBuildConfigFromTriggers(bcWatch)
	buildReactor := &fakeImageReactor{nested: buildReactorFn}
	podReactor := &fakeImageReactor{nested: alterPodFromTriggers(podWatch)}
	deploymentReactor := &fakeImageReactor{nested: alterDeploymentConfigFromTriggers(dcWatch)}
	c := NewTriggerController(record.NewBroadcasterForTests(0), &imageStreamInformer{isInformer},
		TriggerSource{
			Resource: schema.GroupResource{Resource: "buildconfigs"},
			Informer: bcInformer,
			TriggerFn: func(prefix string) trigger.Indexer {
				return buildconfigs.NewBuildConfigTriggerIndexer(prefix)
			},
			Reactor: buildReactor,
		},
		TriggerSource{
			Resource: schema.GroupResource{Resource: "deploymentconfigs"},
			Informer: dcInformer,
			TriggerFn: func(prefix string) trigger.Indexer {
				return deploymentconfigs.NewDeploymentConfigTriggerIndexer(prefix)
			},
			Reactor: deploymentReactor,
		},
		TriggerSource{
			Resource: schema.GroupResource{Resource: "pods"},
			Informer: podInformer,
			TriggerFn: func(prefix string) trigger.Indexer {
				return annotations.NewAnnotationTriggerIndexer(prefix)
			},
			Reactor: podReactor,
		},
	)
	c.resourceFailureDelayFn = func(_ int) (time.Duration, bool) {
		return 0, true
	}
	isFn := c.syncImageStreamFn
	c.syncImageStreamFn = func(key string) error {
		if err := isFn(key); err != nil {
			if kapierrs.IsConflict(err) {
				return err
			}
			t.Fatalf("failure on %s: %v", key, err)
		}
		return nil
	}
	resFn := c.syncResourceFn
	c.syncResourceFn = func(key string) error {
		if err := resFn(key); err != nil {
			if kapierrs.IsConflict(err) {
				t.Logf("conflict syncing resource %s: %v", key, err)
				return err
			}
			t.Fatalf("failure on %s: %v", key, err)
		}
		return nil
	}
	go isInformer.Run(stopCh)
	go bcInformer.Run(stopCh)
	go podInformer.Run(stopCh)
	go dcInformer.Run(stopCh)
	go c.Run(8, stopCh)

	numReferencedStreams := int32(float32(numStreams) * ratioReferencedStreams)

	// generate an initial state
	for i := int32(0); i < numBuildConfigs; i++ {
		if i < int32(float32(numBuildConfigs)*ratioTriggeredBuildConfigs) {
			// builds that point to triggers
			if err := bcWatch.Add(benchmark_1_buildConfig(rnd, i, numReferencedStreams, numTagsPerStream, maxTriggersPerBuild)); err != nil {
				t.Fatal(err)
			}
		} else {
			// builds that have no image stream triggers
			if err := bcWatch.Add(benchmark_1_buildConfig(rnd, i, numStreams, numTagsPerStream, 0)); err != nil {
				t.Fatal(err)
			}
		}
	}
	for i := int32(0); i < numPods; i++ {
		// set initial pods
		podWatch.Add(benchmark_1_pod(rnd, i, numReferencedStreams, numTagsPerStream, maxContainersPerPod))
	}
	for i := int32(0); i < numDeploymentConfigs; i++ {
		// set initial deployments
		if err := dcWatch.Add(benchmark_1_deploymentConfig(rnd, i, numReferencedStreams, numTagsPerStream, maxContainersPerPod)); err != nil {
			t.Fatal(err)
		}
	}
	for i := int32(0); i < numStreams; i++ {
		// set initial image streams
		if err := isWatch.Add(benchmark_1_imageStream(i, numTagsPerStream, 1, 0, 0)); err != nil {
			t.Fatal(err)
		}
	}

	describe := map[string][]string{}

	// make a set of modifications to the streams or builds, verifying after each round
	for round := 1; round <= rounds; round++ {
		var changes []interface{}
		for i := 0; i < iterations; i++ {
			switch f := rnd.Float32(); {
			case f < ratioStreamChanges:
				streamNum := rnd.Int31n(numStreams)
				stream := benchmark_1_imageStream(streamNum, numTagsPerStream, int32(2+(round-1)*500+i), round, i)
				existing, ok, err := isInformer.GetStore().GetByKey(stream.Namespace + "/" + stream.Name)
				if !ok || err != nil {
					t.Logf("keys: %v", isInformer.GetStore().ListKeys())
					t.Logf("Unable to find %s in cache: %t %v", stream.Name, ok, err)
					i = i - 1
					continue
				}
				stream.ResourceVersion = existing.(*imagev1.ImageStream).ResourceVersion
				if err := isWatch.Modify(stream); err != nil {
					t.Logf("[round=%d change=%d] failed to modify image stream: %v", round, i, err)
				}
			default:
				items := bcInformer.GetStore().List()
				if len(items) == 0 {
					continue
				}
				originalBc := items[rnd.Int31n(int32(len(items)))].(*buildv1.BuildConfig)
				bc := originalBc.DeepCopy()
				if len(bc.Spec.Triggers) > 0 {
					index := rnd.Int31n(int32(len(bc.Spec.Triggers)))
					triggerPolicy := &bc.Spec.Triggers[index]
					if triggerPolicy.ImageChange.From != nil {
						old := triggerPolicy.ImageChange.From.Name
						triggerPolicy.ImageChange.From.Name = randomStreamTag(rnd, numStreams, numTagsPerStream)
						describe[bc.Namespace+"/"+bc.Name] = append(describe[bc.Namespace+"/"+bc.Name], fmt.Sprintf("[round=%d change=%d]: change trigger %d from %q to %q", round, i, index, old, triggerPolicy.ImageChange.From.Name))
					} else {
						old := bc.Spec.Strategy.DockerStrategy.From.Name
						bc.Spec.Strategy.DockerStrategy.From.Name = randomStreamTag(rnd, numStreams, numTagsPerStream)
						describe[bc.Namespace+"/"+bc.Name] = append(describe[bc.Namespace+"/"+bc.Name], fmt.Sprintf("[round=%d change=%d]: change docker strategy from %q to %q", round, i, old, bc.Spec.Strategy.DockerStrategy.From.Name))
					}
					if err := bcWatch.Modify(bc); err != nil {
						t.Logf("[round=%d change=%d] failed to modify build config: %v", round, i, err)
					}
				}
			}
		}

		if !verifyState(c, t, changes, describe, bcInformer, podInformer, dcInformer) {
			t.Fatalf("halted after %d rounds", round)
		}
	}
}

func verifyState(
	c *TriggerController,
	t *testing.T,
	expected []interface{},
	descriptions map[string][]string,
	bcInformer, podInformer, dcInformer cache.SharedInformer,
) bool {

	if !controllerDrained(c) {
		t.Errorf("queue=%d changes=%d", c.queue.Len(), c.imageChangeQueue.Len())
		return false
	}

	failed := false
	times := 100

	// verify every build config points to the latest stream
	for i := 0; i < times; i++ {
		var failures []string
		for _, obj := range bcInformer.GetStore().List() {
			if bc, err := updateBuildConfigImages(obj.(*buildv1.BuildConfig), c.tagRetriever); bc != nil || err != nil {
				failures = append(failures, fmt.Sprintf("%s is not fully resolved: %v %s", obj.(*buildv1.BuildConfig).Name, err, diff.ObjectReflectDiff(obj, bc)))
				continue
			}
		}
		if len(failures) == 0 {
			break
		}
		if i == times-1 {
			sort.Strings(failures)
			for _, s := range failures {
				t.Errorf(s)
			}
			failed = true
		}
		time.Sleep(time.Millisecond)
	}

	// verify every deployment config points to the latest stream
	for i := 0; i < times; i++ {
		var failures []string
		for _, obj := range dcInformer.GetStore().List() {
			if updated, resolved, err := deploymentconfigs.UpdateDeploymentConfigImages(obj.(*appsv1.DeploymentConfig), c.tagRetriever); updated != nil || !resolved || err != nil {
				failures = append(failures, fmt.Sprintf("%s is not fully resolved: %v", obj.(*appsv1.DeploymentConfig).Name, err))
				continue
			}
		}
		if len(failures) == 0 {
			break
		}
		if i == times-1 {
			sort.Strings(failures)
			for _, s := range failures {
				t.Errorf(s)
			}
			failed = true
		}
		time.Sleep(time.Millisecond)
	}

	// verify every pod points to the latest stream
	for i := 0; i < times; i++ {
		var failures []string
		for _, obj := range podInformer.GetStore().List() {
			if updated, err := annotations.UpdateObjectFromImages(obj.(*kapi.Pod), c.tagRetriever); updated != nil || err != nil {
				failures = append(failures, fmt.Sprintf("%s is not fully resolved: %v", obj.(*kapi.Pod).Name, err))
				continue
			}
		}
		if len(failures) == 0 {
			break
		}
		if i == times-1 {
			sort.Strings(failures)
			for _, s := range failures {
				t.Errorf(s)
			}
			failed = true
		}
		time.Sleep(time.Millisecond)
	}

	return !failed
}

func controllerDrained(c *TriggerController) bool {
	count := 0
	passed := 0
	for {
		if c.queue.Len() == 0 && c.imageChangeQueue.Len() == 0 {
			if passed > 5 {
				break
			}
			passed++
		} else {
			passed = 0
		}
		time.Sleep(time.Millisecond)
		count++
		if count > 3000 {
			return false
		}
	}
	return true
}
