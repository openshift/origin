package integration

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/fields"
	watchapi "k8s.io/kubernetes/pkg/watch"

	buildapi "github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	"github.com/openshift/origin/pkg/cmd/server/origin"
	imageapi "github.com/openshift/origin/pkg/image/api"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

var (
	//TODO: Make these externally configurable

	// ConcurrentBuildControllersTestWait is the time that TestConcurrentBuildControllers waits
	// for any other changes to happen when testing whether only a single build got processed
	ConcurrentBuildControllersTestWait = 10 * time.Second

	// ConcurrentBuildPodControllersTestWait is the time that TestConcurrentBuildPodControllers waits
	// after a state transition to make sure other state transitions don't occur unexpectedly
	ConcurrentBuildPodControllersTestWait = 10 * time.Second

	// BuildControllersWatchTimeout is used by all tests to wait for watch events. In case where only
	// a single watch event is expected, the test will fail after the timeout.
	BuildControllersWatchTimeout = 60 * time.Second
)

type controllerCount struct {
	BuildControllers,
	BuildPodControllers,
	ImageChangeControllers,
	ConfigChangeControllers int
}

func mockBuild() *buildapi.Build {
	return &buildapi.Build{
		ObjectMeta: kapi.ObjectMeta{
			GenerateName: "mock-build",
			Labels: map[string]string{
				"label1":                     "value1",
				"label2":                     "value2",
				buildapi.BuildConfigLabel:    "mock-build-config",
				buildapi.BuildRunPolicyLabel: string(buildapi.BuildRunPolicyParallel),
			},
		},
		Spec: buildapi.BuildSpec{
			CommonSpec: buildapi.CommonSpec{
				Source: buildapi.BuildSource{
					Git: &buildapi.GitBuildSource{
						URI: "http://my.docker/build",
					},
					ContextDir: "context",
				},
				Strategy: buildapi.BuildStrategy{
					DockerStrategy: &buildapi.DockerBuildStrategy{},
				},
				Output: buildapi.BuildOutput{
					To: &kapi.ObjectReference{
						Kind: "DockerImage",
						Name: "namespace/builtimage",
					},
				},
			},
		},
	}
}

// TestConcurrentBuildControllers tests the transition of a build from new to pending. Ensures that only a single New -> Pending
// transition happens and that only a single pod is created during a set period of time.
func TestConcurrentBuildControllers(t *testing.T) {
	defer testutil.DumpEtcdOnFailure(t)
	// Start a master with multiple BuildControllers
	osClient, kClient := setupBuildControllerTest(controllerCount{BuildControllers: 5}, t)

	// Setup an error channel
	errChan := make(chan error) // go routines will send a message on this channel if an error occurs. Once this happens the test is over

	// Create a build
	ns := testutil.Namespace()
	b, err := osClient.Builds(ns).Create(mockBuild())
	if err != nil {
		t.Fatal(err)
	}

	// Start watching builds for New -> Pending transition
	buildWatch, err := osClient.Builds(ns).Watch(kapi.ListOptions{FieldSelector: fields.OneTermEqualSelector("metadata.name", b.Name), ResourceVersion: b.ResourceVersion})
	if err != nil {
		t.Fatal(err)
	}
	defer buildWatch.Stop()
	buildModifiedCount := int32(0)
	go func() {
		for e := range buildWatch.ResultChan() {
			if e.Type != watchapi.Modified {
				errChan <- fmt.Errorf("received an unexpected event of type: %s with object: %#v", e.Type, e.Object)
			}
			build, ok := e.Object.(*buildapi.Build)
			if !ok {
				errChan <- fmt.Errorf("received something other than build: %#v", e.Object)
				break
			}
			// If unexpected status, throw error
			if build.Status.Phase != buildapi.BuildPhasePending && build.Status.Phase != buildapi.BuildPhaseNew {
				errChan <- fmt.Errorf("received unexpected build status: %s", build.Status.Phase)
				break
			}
			atomic.AddInt32(&buildModifiedCount, 1)
		}
	}()

	// Watch build pods as they are created
	podWatch, err := kClient.Pods(ns).Watch(kapi.ListOptions{FieldSelector: fields.OneTermEqualSelector("metadata.name", buildapi.GetBuildPodName(b))})
	if err != nil {
		t.Fatal(err)
	}
	defer podWatch.Stop()
	podAddedCount := int32(0)
	go func() {
		for e := range podWatch.ResultChan() {
			// Look for creation events
			if e.Type == watchapi.Added {
				atomic.AddInt32(&podAddedCount, 1)
			}
		}
	}()

	select {
	case err := <-errChan:
		t.Errorf("Error: %v", err)
	case <-time.After(ConcurrentBuildControllersTestWait):
		if atomic.LoadInt32(&buildModifiedCount) < 1 {
			t.Errorf("The build was modified an unexpected number of times. Got: %d, Expected: >= 1", buildModifiedCount)
		}
		if atomic.LoadInt32(&podAddedCount) != 1 {
			t.Errorf("The build pod was created an unexpected number of times. Got: %d, Expected: 1", podAddedCount)
		}
	}
}

type buildControllerPodState struct {
	PodPhase   kapi.PodPhase
	BuildPhase buildapi.BuildPhase
}

type buildControllerPodTest struct {
	Name   string
	States []buildControllerPodState
}

// TestConcurrentBuildPodControllers tests the lifecycle of a build pod when running multiple controllers.
func TestConcurrentBuildPodControllers(t *testing.T) {
	defer testutil.DumpEtcdOnFailure(t)
	// Start a master with multiple BuildPodControllers
	osClient, kClient := setupBuildControllerTest(controllerCount{BuildPodControllers: 5}, t)

	ns := testutil.Namespace()
	waitTime := ConcurrentBuildPodControllersTestWait

	tests := []buildControllerPodTest{
		{
			Name: "running state test",
			States: []buildControllerPodState{
				{
					PodPhase:   kapi.PodRunning,
					BuildPhase: buildapi.BuildPhaseRunning,
				},
			},
		},
		{
			Name: "build succeeded",
			States: []buildControllerPodState{
				{
					PodPhase:   kapi.PodRunning,
					BuildPhase: buildapi.BuildPhaseRunning,
				},
				{
					PodPhase:   kapi.PodSucceeded,
					BuildPhase: buildapi.BuildPhaseComplete,
				},
			},
		},
		{
			Name: "build failed",
			States: []buildControllerPodState{
				{
					PodPhase:   kapi.PodRunning,
					BuildPhase: buildapi.BuildPhaseRunning,
				},
				{
					PodPhase:   kapi.PodFailed,
					BuildPhase: buildapi.BuildPhaseFailed,
				},
			},
		},
	}
	for _, test := range tests {
		// Setup communications channels
		podReadyChan := make(chan *kapi.Pod) // Will receive a value when a build pod is ready
		errChan := make(chan error)          // Will receive a value when an error occurs
		stateReached := int32(0)

		// Create a build
		b, err := osClient.Builds(ns).Create(mockBuild())
		if err != nil {
			t.Fatal(err)
		}

		// Watch build pod for transition to pending
		podWatch, err := kClient.Pods(ns).Watch(kapi.ListOptions{FieldSelector: fields.OneTermEqualSelector("metadata.name", buildapi.GetBuildPodName(b))})
		if err != nil {
			t.Fatal(err)
		}
		go func() {
			for e := range podWatch.ResultChan() {
				pod, ok := e.Object.(*kapi.Pod)
				if !ok {
					t.Fatalf("%s: unexpected object received: %#v\n", test.Name, e.Object)
				}
				if pod.Status.Phase == kapi.PodPending {
					podReadyChan <- pod
					break
				}
			}
		}()

		var pod *kapi.Pod
		select {
		case pod = <-podReadyChan:
			if pod.Status.Phase != kapi.PodPending {
				t.Errorf("Got wrong pod phase: %s", pod.Status.Phase)
				podWatch.Stop()
				continue
			}

		case <-time.After(BuildControllersWatchTimeout):
			t.Errorf("Timed out waiting for build pod to be ready")
			podWatch.Stop()
			continue
		}
		podWatch.Stop()

		for _, state := range test.States {
			if err := kclient.RetryOnConflict(kclient.DefaultRetry, func() error {
				// Update pod state and verify that corresponding build state happens accordingly
				pod, err := kClient.Pods(ns).Get(pod.Name)
				if err != nil {
					return err
				}
				if pod.Status.Phase == state.PodPhase {
					return fmt.Errorf("another client altered the pod phase to %s: %#v", state.PodPhase, pod)
				}
				pod.Status.Phase = state.PodPhase
				_, err = kClient.Pods(ns).UpdateStatus(pod)
				return err
			}); err != nil {
				t.Fatal(err)
			}

			buildWatch, err := osClient.Builds(ns).Watch(kapi.ListOptions{FieldSelector: fields.OneTermEqualSelector("metadata.name", b.Name), ResourceVersion: b.ResourceVersion})
			if err != nil {
				t.Fatal(err)
			}
			defer buildWatch.Stop()
			go func() {
				done := false
				for e := range buildWatch.ResultChan() {
					var ok bool
					b, ok = e.Object.(*buildapi.Build)
					if !ok {
						errChan <- fmt.Errorf("%s: unexpected object received: %#v", test.Name, e.Object)
					}
					if e.Type != watchapi.Modified {
						errChan <- fmt.Errorf("%s: unexpected event received: %s, object: %#v", test.Name, e.Type, e.Object)
					}
					if done {
						errChan <- fmt.Errorf("%s: unexpected build state: %#v", test.Name, e.Object)
					} else if b.Status.Phase == state.BuildPhase {
						done = true
						atomic.StoreInt32(&stateReached, 1)
					}
				}
			}()

			select {
			case err := <-errChan:
				buildWatch.Stop()
				t.Errorf("%s: Error: %v\n", test.Name, err)
				break
			case <-time.After(waitTime):
				buildWatch.Stop()
				if atomic.LoadInt32(&stateReached) != 1 {
					t.Errorf("%s: Did not reach desired build state: %s", test.Name, state.BuildPhase)
					break
				}
			}
		}
	}
}

func TestConcurrentBuildImageChangeTriggerControllers(t *testing.T) {
	defer testutil.DumpEtcdOnFailure(t)
	// Start a master with multiple ImageChangeTrigger controllers
	osClient, _ := setupBuildControllerTest(controllerCount{ImageChangeControllers: 5}, t)
	tag := "latest"
	streamName := "test-image-trigger-repo"

	imageStream := mockImageStream2(tag)
	imageStreamMapping := mockImageStreamMapping(imageStream.Name, "someimage", tag, "registry:8080/openshift/test-image-trigger:"+tag)
	strategy := stiStrategy("ImageStreamTag", streamName+":"+tag)
	config := imageChangeBuildConfig("sti-imagestreamtag", strategy)
	runImageChangeTriggerTest(t, osClient, imageStream, imageStreamMapping, config, tag)
}

func TestBuildDeleteController(t *testing.T) {
	defer testutil.DumpEtcdOnFailure(t)
	osClient, kClient := setupBuildControllerTest(controllerCount{}, t)
	runBuildDeleteTest(t, osClient, kClient)
}

func TestBuildRunningPodDeleteController(t *testing.T) {
	defer testutil.DumpEtcdOnFailure(t)
	osClient, kClient := setupBuildControllerTest(controllerCount{}, t)
	runBuildRunningPodDeleteTest(t, osClient, kClient)
}

func TestBuildCompletePodDeleteController(t *testing.T) {
	defer testutil.DumpEtcdOnFailure(t)
	osClient, kClient := setupBuildControllerTest(controllerCount{}, t)
	runBuildCompletePodDeleteTest(t, osClient, kClient)
}

func TestConcurrentBuildConfigControllers(t *testing.T) {
	defer testutil.DumpEtcdOnFailure(t)
	osClient, kClient := setupBuildControllerTest(controllerCount{ConfigChangeControllers: 5}, t)
	runBuildConfigChangeControllerTest(t, osClient, kClient)
}

func setupBuildControllerTest(counts controllerCount, t *testing.T) (*client.Client, *kclient.Client) {
	testutil.RequireEtcd(t)
	master, clusterAdminKubeConfig, err := testserver.StartTestMaster()
	if err != nil {
		t.Fatal(err)
	}

	clusterAdminClient, err := testutil.GetClusterAdminClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatal(err)
	}

	clusterAdminKubeClient, err := testutil.GetClusterAdminKubeClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatal(err)
	}
	_, err = clusterAdminKubeClient.Namespaces().Create(&kapi.Namespace{
		ObjectMeta: kapi.ObjectMeta{Name: testutil.Namespace()},
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := testserver.WaitForServiceAccounts(clusterAdminKubeClient, testutil.Namespace(), []string{bootstrappolicy.BuilderServiceAccountName, bootstrappolicy.DefaultServiceAccountName}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	openshiftConfig, err := origin.BuildMasterConfig(*master)
	if err != nil {
		t.Fatal(err)
	}

	// Get the build controller clients, since those rely on service account tokens
	// We don't want to proceed with the rest of the test until those are available
	openshiftConfig.BuildControllerClients()

	for i := 0; i < counts.BuildControllers; i++ {
		openshiftConfig.RunBuildController(openshiftConfig.Informers)
	}
	for i := 0; i < counts.BuildPodControllers; i++ {
		openshiftConfig.RunBuildPodController()
	}
	for i := 0; i < counts.ImageChangeControllers; i++ {
		openshiftConfig.RunBuildImageChangeTriggerController()
	}
	for i := 0; i < counts.ConfigChangeControllers; i++ {
		openshiftConfig.RunBuildConfigChangeController()
	}
	return clusterAdminClient, clusterAdminKubeClient
}

func waitForWatch(t *testing.T, name string, w watchapi.Interface) *watchapi.Event {
	select {
	case e := <-w.ResultChan():
		return &e
	case <-time.After(BuildControllersWatchTimeout):
		t.Fatalf("Timed out waiting for watch: %s", name)
		return nil
	}
}

func runImageChangeTriggerTest(t *testing.T, clusterAdminClient *client.Client, imageStream *imageapi.ImageStream, imageStreamMapping *imageapi.ImageStreamMapping, config *buildapi.BuildConfig, tag string) {
	created, err := clusterAdminClient.BuildConfigs(testutil.Namespace()).Create(config)
	if err != nil {
		t.Fatalf("Couldn't create BuildConfig: %v", err)
	}

	watch, err := clusterAdminClient.Builds(testutil.Namespace()).Watch(kapi.ListOptions{ResourceVersion: created.ResourceVersion})
	if err != nil {
		t.Fatalf("Couldn't subscribe to Builds %v", err)
	}

	watch2, err := clusterAdminClient.BuildConfigs(testutil.Namespace()).Watch(kapi.ListOptions{ResourceVersion: created.ResourceVersion})
	if err != nil {
		t.Fatalf("Couldn't subscribe to BuildConfigs %v", err)
	}
	defer watch2.Stop()

	imageStream, err = clusterAdminClient.ImageStreams(testutil.Namespace()).Create(imageStream)
	if err != nil {
		t.Fatalf("Couldn't create ImageStream: %v", err)
	}

	err = clusterAdminClient.ImageStreamMappings(testutil.Namespace()).Create(imageStreamMapping)
	if err != nil {
		t.Fatalf("Couldn't create Image: %v", err)
	}

	// wait for initial build event from the creation of the imagerepo with tag latest
	event := waitForWatch(t, "initial build added", watch)
	if e, a := watchapi.Added, event.Type; e != a {
		t.Fatalf("expected watch event type %s, got %s", e, a)
	}
	newBuild := event.Object.(*buildapi.Build)
	strategy := newBuild.Spec.Strategy
	switch {
	case strategy.SourceStrategy != nil:
		if strategy.SourceStrategy.From.Name != "registry:8080/openshift/test-image-trigger:"+tag {
			i, _ := clusterAdminClient.ImageStreams(testutil.Namespace()).Get(imageStream.Name)
			bc, _ := clusterAdminClient.BuildConfigs(testutil.Namespace()).Get(config.Name)
			t.Fatalf("Expected build with base image %s, got %s\n, imagerepo is %v\ntrigger is %s\n", "registry:8080/openshift/test-image-trigger:"+tag, strategy.SourceStrategy.From.Name, i, bc.Spec.Triggers[0].ImageChange)
		}
	case strategy.DockerStrategy != nil:
		if strategy.DockerStrategy.From.Name != "registry:8080/openshift/test-image-trigger:"+tag {
			i, _ := clusterAdminClient.ImageStreams(testutil.Namespace()).Get(imageStream.Name)
			bc, _ := clusterAdminClient.BuildConfigs(testutil.Namespace()).Get(config.Name)
			t.Fatalf("Expected build with base image %s, got %s\n, imagerepo is %v\ntrigger is %s\n", "registry:8080/openshift/test-image-trigger:"+tag, strategy.DockerStrategy.From.Name, i, bc.Spec.Triggers[0].ImageChange)
		}
	case strategy.CustomStrategy != nil:
		if strategy.CustomStrategy.From.Name != "registry:8080/openshift/test-image-trigger:"+tag {
			i, _ := clusterAdminClient.ImageStreams(testutil.Namespace()).Get(imageStream.Name)
			bc, _ := clusterAdminClient.BuildConfigs(testutil.Namespace()).Get(config.Name)
			t.Fatalf("Expected build with base image %s, got %s\n, imagerepo is %v\ntrigger is %s\n", "registry:8080/openshift/test-image-trigger:"+tag, strategy.CustomStrategy.From.Name, i, bc.Spec.Triggers[0].ImageChange)
		}
	}
	// Wait for an update on the specific build that was added
	watch3, err := clusterAdminClient.Builds(testutil.Namespace()).Watch(kapi.ListOptions{FieldSelector: fields.OneTermEqualSelector("metadata.name", newBuild.Name), ResourceVersion: newBuild.ResourceVersion})
	defer watch3.Stop()
	if err != nil {
		t.Fatalf("Couldn't subscribe to Builds %v", err)
	}
	event = waitForWatch(t, "initial build update", watch3)
	if e, a := watchapi.Modified, event.Type; e != a {
		t.Fatalf("expected watch event type %s, got %s", e, a)
	}
	newBuild = event.Object.(*buildapi.Build)
	// Make sure the resolution of the build's docker image pushspec didn't mutate the persisted API object
	if newBuild.Spec.Output.To.Name != "test-image-trigger-repo:outputtag" {
		t.Fatalf("unexpected build output: %#v %#v", newBuild.Spec.Output.To, newBuild.Spec.Output)
	}
	if newBuild.Labels["testlabel"] != "testvalue" {
		t.Fatalf("Expected build with label %s=%s from build config got %s=%s", "testlabel", "testvalue", "testlabel", newBuild.Labels["testlabel"])
	}

	// wait for build config to be updated
WaitLoop:
	for {
		select {
		case e := <-watch2.ResultChan():
			event = &e
			continue
		case <-time.After(BuildControllersWatchTimeout):
			break WaitLoop
		}
	}
	updatedConfig := event.Object.(*buildapi.BuildConfig)
	if err != nil {
		t.Fatalf("Couldn't get BuildConfig: %v", err)
	}
	// the first tag did not have an image id, so the last trigger field is the pull spec
	if updatedConfig.Spec.Triggers[0].ImageChange.LastTriggeredImageID != "registry:8080/openshift/test-image-trigger:"+tag {
		t.Fatalf("Expected imageID equal to pull spec, got %#v", updatedConfig.Spec.Triggers[0].ImageChange)
	}

	// clear out the build/buildconfig watches before triggering a new build
WaitLoop2:
	for {
		select {
		case <-watch.ResultChan():
			continue
		case <-watch2.ResultChan():
			continue
		case <-time.After(BuildControllersWatchTimeout):
			break WaitLoop2
		}
	}

	// trigger a build by posting a new image
	if err := clusterAdminClient.ImageStreamMappings(testutil.Namespace()).Create(&imageapi.ImageStreamMapping{
		ObjectMeta: kapi.ObjectMeta{
			Namespace: testutil.Namespace(),
			Name:      imageStream.Name,
		},
		Tag: tag,
		Image: imageapi.Image{
			ObjectMeta: kapi.ObjectMeta{
				Name: "ref-2-random",
			},
			DockerImageReference: "registry:8080/openshift/test-image-trigger:ref-2-random",
		},
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	event = waitForWatch(t, "second build created", watch)
	if e, a := watchapi.Added, event.Type; e != a {
		t.Fatalf("expected watch event type %s, got %s", e, a)
	}
	newBuild = event.Object.(*buildapi.Build)
	strategy = newBuild.Spec.Strategy
	switch {
	case strategy.SourceStrategy != nil:
		if strategy.SourceStrategy.From.Name != "registry:8080/openshift/test-image-trigger:ref-2-random" {
			i, _ := clusterAdminClient.ImageStreams(testutil.Namespace()).Get(imageStream.Name)
			bc, _ := clusterAdminClient.BuildConfigs(testutil.Namespace()).Get(config.Name)
			t.Fatalf("Expected build with base image %s, got %s\n, imagerepo is %v\trigger is %s\n", "registry:8080/openshift/test-image-trigger:ref-2-random", strategy.SourceStrategy.From.Name, i, bc.Spec.Triggers[3].ImageChange)
		}
	case strategy.DockerStrategy != nil:
		if strategy.DockerStrategy.From.Name != "registry:8080/openshift/test-image-trigger:ref-2-random" {
			i, _ := clusterAdminClient.ImageStreams(testutil.Namespace()).Get(imageStream.Name)
			bc, _ := clusterAdminClient.BuildConfigs(testutil.Namespace()).Get(config.Name)
			t.Fatalf("Expected build with base image %s, got %s\n, imagerepo is %v\trigger is %s\n", "registry:8080/openshift/test-image-trigger:ref-2-random", strategy.DockerStrategy.From.Name, i, bc.Spec.Triggers[3].ImageChange)
		}
	case strategy.CustomStrategy != nil:
		if strategy.CustomStrategy.From.Name != "registry:8080/openshift/test-image-trigger:ref-2-random" {
			i, _ := clusterAdminClient.ImageStreams(testutil.Namespace()).Get(imageStream.Name)
			bc, _ := clusterAdminClient.BuildConfigs(testutil.Namespace()).Get(config.Name)
			t.Fatalf("Expected build with base image %s, got %s\n, imagerepo is %v\trigger is %s\n", "registry:8080/openshift/test-image-trigger:ref-2-random", strategy.CustomStrategy.From.Name, i, bc.Spec.Triggers[3].ImageChange)
		}
	}

	// Listen to events on specific  build
	watch4, err := clusterAdminClient.Builds(testutil.Namespace()).Watch(kapi.ListOptions{FieldSelector: fields.OneTermEqualSelector("metadata.name", newBuild.Name), ResourceVersion: newBuild.ResourceVersion})
	defer watch4.Stop()

	event = waitForWatch(t, "update on second build", watch4)
	if e, a := watchapi.Modified, event.Type; e != a {
		t.Fatalf("expected watch event type %s, got %s", e, a)
	}
	newBuild = event.Object.(*buildapi.Build)
	// Make sure the resolution of the build's docker image pushspec didn't mutate the persisted API object
	if newBuild.Spec.Output.To.Name != "test-image-trigger-repo:outputtag" {
		t.Fatalf("unexpected build output: %#v %#v", newBuild.Spec.Output.To, newBuild.Spec.Output)
	}
	if newBuild.Labels["testlabel"] != "testvalue" {
		t.Fatalf("Expected build with label %s=%s from build config got %s=%s", "testlabel", "testvalue", "testlabel", newBuild.Labels["testlabel"])
	}

WaitLoop3:
	for {
		select {
		case e := <-watch2.ResultChan():
			event = &e
			continue
		case <-time.After(BuildControllersWatchTimeout):
			break WaitLoop3
		}
	}
	updatedConfig = event.Object.(*buildapi.BuildConfig)
	if e, a := "registry:8080/openshift/test-image-trigger:ref-2-random", updatedConfig.Spec.Triggers[0].ImageChange.LastTriggeredImageID; e != a {
		t.Errorf("unexpected trigger id: expected %v, got %v", e, a)
	}
}

func runBuildDeleteTest(t *testing.T, clusterAdminClient *client.Client, clusterAdminKubeClient *kclient.Client) {

	buildWatch, err := clusterAdminClient.Builds(testutil.Namespace()).Watch(kapi.ListOptions{})
	if err != nil {
		t.Fatalf("Couldn't subscribe to Builds %v", err)
	}
	defer buildWatch.Stop()

	created, err := clusterAdminClient.Builds(testutil.Namespace()).Create(mockBuild())
	if err != nil {
		t.Fatalf("Couldn't create Build: %v", err)
	}

	podWatch, err := clusterAdminKubeClient.Pods(testutil.Namespace()).Watch(kapi.ListOptions{ResourceVersion: created.ResourceVersion})
	if err != nil {
		t.Fatalf("Couldn't subscribe to Pods %v", err)
	}
	defer podWatch.Stop()

	// wait for initial build event from the creation of the imagerepo with tag latest
	event := waitForWatch(t, "initial build added", buildWatch)
	if e, a := watchapi.Added, event.Type; e != a {
		t.Fatalf("expected watch event type %s, got %s", e, a)
	}
	newBuild := event.Object.(*buildapi.Build)

	// initial pod creation for build
	event = waitForWatch(t, "build pod created", podWatch)
	if e, a := watchapi.Added, event.Type; e != a {
		t.Fatalf("expected watch event type %s, got %s", e, a)
	}

	clusterAdminClient.Builds(testutil.Namespace()).Delete(newBuild.Name)

	event = waitForWatchType(t, "pod deleted due to build deleted", podWatch, watchapi.Deleted)
	if e, a := watchapi.Deleted, event.Type; e != a {
		t.Fatalf("expected watch event type %s, got %s", e, a)
	}
	pod := event.Object.(*kapi.Pod)
	if expected := buildapi.GetBuildPodName(newBuild); pod.Name != expected {
		t.Fatalf("Expected pod %s to be deleted, but pod %s was deleted", expected, pod.Name)
	}

}

// waitForWatchType tolerates receiving 3 events before failing while watching for a particular event
// type.
func waitForWatchType(t *testing.T, name string, w watchapi.Interface, expect watchapi.EventType) *watchapi.Event {
	tries := 3
	for i := 0; i < tries; i++ {
		select {
		case e := <-w.ResultChan():
			if e.Type != expect {
				continue
			}
			return &e
		case <-time.After(BuildControllersWatchTimeout):
			t.Fatalf("Timed out waiting for watch: %s", name)
			return nil
		}
	}
	t.Fatalf("Waited for a %v event with %d tries but never received one", expect, tries)
	return nil
}

func runBuildRunningPodDeleteTest(t *testing.T, clusterAdminClient *client.Client, clusterAdminKubeClient *kclient.Client) {

	buildWatch, err := clusterAdminClient.Builds(testutil.Namespace()).Watch(kapi.ListOptions{})
	if err != nil {
		t.Fatalf("Couldn't subscribe to Builds %v", err)
	}
	defer buildWatch.Stop()

	created, err := clusterAdminClient.Builds(testutil.Namespace()).Create(mockBuild())
	if err != nil {
		t.Fatalf("Couldn't create Build: %v", err)
	}

	podWatch, err := clusterAdminKubeClient.Pods(testutil.Namespace()).Watch(kapi.ListOptions{ResourceVersion: created.ResourceVersion})
	if err != nil {
		t.Fatalf("Couldn't subscribe to Pods %v", err)
	}
	defer podWatch.Stop()

	// wait for initial build event from the creation of the imagerepo with tag latest
	event := waitForWatch(t, "initial build added", buildWatch)
	if e, a := watchapi.Added, event.Type; e != a {
		t.Fatalf("expected watch event type %s, got %s", e, a)
	}
	newBuild := event.Object.(*buildapi.Build)
	buildName := newBuild.Name
	podName := newBuild.Name + "-build"

	// initial pod creation for build
	for {
		event = waitForWatch(t, "build pod created", podWatch)
		newPod := event.Object.(*kapi.Pod)
		if newPod.Name == podName {
			break
		}
	}
	if e, a := watchapi.Added, event.Type; e != a {
		t.Fatalf("expected watch event type %s, got %s", e, a)
	}

	// throw away events from other builds, we only care about the new build
	// we just triggered
	for {
		event = waitForWatch(t, "build updated to pending", buildWatch)
		newBuild = event.Object.(*buildapi.Build)
		if newBuild.Name == buildName {
			break
		}
	}
	if e, a := watchapi.Modified, event.Type; e != a {
		t.Fatalf("expected watch event type %s, got %s", e, a)
	}
	if newBuild.Status.Phase != buildapi.BuildPhasePending {
		t.Fatalf("expected build status to be marked pending, but was marked %s", newBuild.Status.Phase)
	}

	clusterAdminKubeClient.Pods(testutil.Namespace()).Delete(buildapi.GetBuildPodName(newBuild), kapi.NewDeleteOptions(0))
	event = waitForWatch(t, "build updated to error", buildWatch)
	if e, a := watchapi.Modified, event.Type; e != a {
		t.Fatalf("expected watch event type %s, got %s", e, a)
	}
	newBuild = event.Object.(*buildapi.Build)
	if newBuild.Status.Phase != buildapi.BuildPhaseError {
		t.Fatalf("expected build status to be marked error, but was marked %s", newBuild.Status.Phase)
	}
}

func runBuildCompletePodDeleteTest(t *testing.T, clusterAdminClient *client.Client, clusterAdminKubeClient *kclient.Client) {

	buildWatch, err := clusterAdminClient.Builds(testutil.Namespace()).Watch(kapi.ListOptions{})
	if err != nil {
		t.Fatalf("Couldn't subscribe to Builds %v", err)
	}
	defer buildWatch.Stop()

	created, err := clusterAdminClient.Builds(testutil.Namespace()).Create(mockBuild())
	if err != nil {
		t.Fatalf("Couldn't create Build: %v", err)
	}

	podWatch, err := clusterAdminKubeClient.Pods(testutil.Namespace()).Watch(kapi.ListOptions{ResourceVersion: created.ResourceVersion})
	if err != nil {
		t.Fatalf("Couldn't subscribe to Pods %v", err)
	}
	defer podWatch.Stop()

	// wait for initial build event from the creation of the imagerepo with tag latest
	event := waitForWatch(t, "initial build added", buildWatch)
	if e, a := watchapi.Added, event.Type; e != a {
		t.Fatalf("expected watch event type %s, got %s", e, a)
	}
	newBuild := event.Object.(*buildapi.Build)

	// initial pod creation for build
	event = waitForWatch(t, "build pod created", podWatch)
	if e, a := watchapi.Added, event.Type; e != a {
		t.Fatalf("expected watch event type %s, got %s", e, a)
	}

	event = waitForWatch(t, "build updated to pending", buildWatch)
	if e, a := watchapi.Modified, event.Type; e != a {
		t.Fatalf("expected watch event type %s, got %s", e, a)
	}

	newBuild = event.Object.(*buildapi.Build)
	if newBuild.Status.Phase != buildapi.BuildPhasePending {
		t.Fatalf("expected build status to be marked pending, but was marked %s", newBuild.Status.Phase)
	}

	newBuild.Status.Phase = buildapi.BuildPhaseComplete
	clusterAdminClient.Builds(testutil.Namespace()).Update(newBuild)
	event = waitForWatch(t, "build updated to complete", buildWatch)
	if e, a := watchapi.Modified, event.Type; e != a {
		t.Fatalf("expected watch event type %s, got %s", e, a)
	}
	newBuild = event.Object.(*buildapi.Build)
	if newBuild.Status.Phase != buildapi.BuildPhaseComplete {
		t.Fatalf("expected build status to be marked complete, but was marked %s", newBuild.Status.Phase)
	}

	clusterAdminKubeClient.Pods(testutil.Namespace()).Delete(buildapi.GetBuildPodName(newBuild), kapi.NewDeleteOptions(0))
	time.Sleep(10 * time.Second)
	newBuild, err = clusterAdminClient.Builds(testutil.Namespace()).Get(newBuild.Name)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if newBuild.Status.Phase != buildapi.BuildPhaseComplete {
		t.Fatalf("build status was updated to %s after deleting pod, should have stayed as %s", newBuild.Status.Phase, buildapi.BuildPhaseComplete)
	}
}

func runBuildConfigChangeControllerTest(t *testing.T, clusterAdminClient *client.Client, clusterAdminKubeClient *kclient.Client) {
	config := configChangeBuildConfig()
	created, err := clusterAdminClient.BuildConfigs(testutil.Namespace()).Create(config)
	if err != nil {
		t.Fatalf("Couldn't create BuildConfig: %v", err)
	}

	watch, err := clusterAdminClient.Builds(testutil.Namespace()).Watch(kapi.ListOptions{ResourceVersion: created.ResourceVersion})
	if err != nil {
		t.Fatalf("Couldn't subscribe to Builds %v", err)
	}
	defer watch.Stop()

	watch2, err := clusterAdminClient.BuildConfigs(testutil.Namespace()).Watch(kapi.ListOptions{ResourceVersion: created.ResourceVersion})
	if err != nil {
		t.Fatalf("Couldn't subscribe to BuildConfigs %v", err)
	}
	defer watch2.Stop()

	// wait for initial build event
	event := waitForWatch(t, "config change initial build added", watch)
	if e, a := watchapi.Added, event.Type; e != a {
		t.Fatalf("expected watch event type %s, got %s", e, a)
	}

	event = waitForWatch(t, "config change config updated", watch2)
	if e, a := watchapi.Modified, event.Type; e != a {
		t.Fatalf("expected watch event type %s, got %s", e, a)
	}
	if bc := event.Object.(*buildapi.BuildConfig); bc.Status.LastVersion == 0 {
		t.Fatalf("expected build config lastversion to be greater than zero after build")
	}
}

func configChangeBuildConfig() *buildapi.BuildConfig {
	bc := &buildapi.BuildConfig{}
	bc.Name = "testcfgbc"
	bc.Namespace = testutil.Namespace()
	bc.Spec.Source.Git = &buildapi.GitBuildSource{}
	bc.Spec.Source.Git.URI = "git://github.com/openshift/ruby-hello-world.git"
	bc.Spec.Strategy.DockerStrategy = &buildapi.DockerBuildStrategy{}
	configChangeTrigger := buildapi.BuildTriggerPolicy{Type: buildapi.ConfigChangeBuildTriggerType}
	bc.Spec.Triggers = append(bc.Spec.Triggers, configChangeTrigger)
	return bc
}
