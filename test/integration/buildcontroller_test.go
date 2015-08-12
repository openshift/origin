// +build integration,!no-etcd

package integration

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	kclient "k8s.io/kubernetes/pkg/client"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	watchapi "k8s.io/kubernetes/pkg/watch"

	buildapi "github.com/openshift/origin/pkg/build/api"
	buildutil "github.com/openshift/origin/pkg/build/util"
	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	"github.com/openshift/origin/pkg/cmd/server/origin"
	imageapi "github.com/openshift/origin/pkg/image/api"
	testutil "github.com/openshift/origin/test/util"
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
	BuildControllersWatchTimeout = 10 * time.Second
)

func init() {
	testutil.RequireEtcd()
}

// TestConcurrentBuildControllers tests the transition of a build from new to pending. Ensures that only a single New -> Pending
// transition happens and that only a single pod is created during a set period of time.
func TestConcurrentBuildControllers(t *testing.T) {
	// Start a master with multiple BuildControllers
	osClient, kClient := setupBuildControllerTest(5, 0, 0, t)

	// Setup an error channel
	errChan := make(chan error) // go routines will send a message on this channel if an error occurs. Once this happens the test is over

	// Create a build
	ns := testutil.Namespace()
	b, err := osClient.Builds(ns).Create(mockBuild())
	checkErr(t, err)

	// Start watching builds for New -> Pending transition
	buildWatch, err := osClient.Builds(ns).Watch(labels.Everything(), fields.OneTermEqualSelector("name", b.Name), b.ResourceVersion)
	checkErr(t, err)
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
			if build.Status.Phase != buildapi.BuildPhasePending {
				errChan <- fmt.Errorf("received unexpected build status: %s", build.Status.Phase)
				break
			} else {
				atomic.AddInt32(&buildModifiedCount, 1)
			}
		}
	}()

	// Watch build pods as they are created
	podWatch, err := kClient.Pods(ns).Watch(labels.Everything(), fields.OneTermEqualSelector("metadata.name", buildutil.GetBuildPodName(b)), "")
	checkErr(t, err)
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
		if atomic.LoadInt32(&buildModifiedCount) != 1 {
			t.Errorf("The build was modified an unexpected number of times. Got: %d, Expected: 1", buildModifiedCount)
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
	// Start a master with multiple BuildPodControllers
	osClient, kClient := setupBuildControllerTest(0, 5, 0, t)

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
		checkErr(t, err)

		// Watch build pod for transition to pending
		podWatch, err := kClient.Pods(ns).Watch(labels.Everything(), fields.OneTermEqualSelector("metadata.name", buildutil.GetBuildPodName(b)), "")
		checkErr(t, err)
		go func() {
			for e := range podWatch.ResultChan() {
				pod, ok := e.Object.(*kapi.Pod)
				if !ok {
					checkErr(t, fmt.Errorf("%s: unexpected object received: %#v\n", test.Name, e.Object))
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
			// Update pod state and verify that corresponding build state happens accordingly
			pod, err := kClient.Pods(ns).Get(pod.Name)
			checkErr(t, err)
			pod.Status.Phase = state.PodPhase
			_, err = kClient.Pods(ns).UpdateStatus(pod)
			checkErr(t, err)

			buildWatch, err := osClient.Builds(ns).Watch(labels.Everything(), fields.OneTermEqualSelector("name", b.Name), b.ResourceVersion)
			checkErr(t, err)
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
						errChan <- fmt.Errorf("%s: unexpected event received: %s, object: %#v", e.Type, e.Object)
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
				t.Errorf("Error: %v\n", test.Name, err)
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
	// Start a master with multiple ImageChangeTrigger controllers
	osClient, _ := setupBuildControllerTest(0, 0, 5, t)
	tag := "latest"
	streamName := "test-image-trigger-repo"

	imageStream := mockImageStream2(tag)
	imageStreamMapping := mockImageStreamMapping(imageStream.Name, "someimage", tag, "registry:8080/openshift/test-image-trigger:"+tag)
	strategy := stiStrategy("ImageStreamTag", streamName+":"+tag)
	config := imageChangeBuildConfig("sti-imagestreamtag", strategy)
	runImageChangeTriggerTest(t, osClient, imageStream, imageStreamMapping, config, tag)
}

func TestBuildDeleteController(t *testing.T) {
	osClient, kClient := setupBuildControllerTest(0, 0, 0, t)
	runBuildDeleteTest(t, osClient, kClient)
}

func TestBuildRunningPodDeleteController(t *testing.T) {
	osClient, kClient := setupBuildControllerTest(0, 0, 0, t)
	runBuildRunningPodDeleteTest(t, osClient, kClient)
}

func TestBuildCompletePodDeleteController(t *testing.T) {
	osClient, kClient := setupBuildControllerTest(0, 0, 0, t)
	runBuildCompletePodDeleteTest(t, osClient, kClient)
}

func checkErr(t *testing.T, err error) {
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func setupBuildControllerTest(additionalBuildControllers, additionalBuildPodControllers, additionalImageChangeControllers int, t *testing.T) (*client.Client, *kclient.Client) {
	master, clusterAdminKubeConfig, err := testutil.StartTestMaster()
	checkErr(t, err)

	clusterAdminClient, err := testutil.GetClusterAdminClient(clusterAdminKubeConfig)
	checkErr(t, err)

	clusterAdminKubeClient, err := testutil.GetClusterAdminKubeClient(clusterAdminKubeConfig)
	checkErr(t, err)
	_, err = clusterAdminKubeClient.Namespaces().Create(&kapi.Namespace{
		ObjectMeta: kapi.ObjectMeta{Name: testutil.Namespace()},
	})
	checkErr(t, err)

	if err := testutil.WaitForServiceAccounts(clusterAdminKubeClient, testutil.Namespace(), []string{bootstrappolicy.BuilderServiceAccountName, bootstrappolicy.DefaultServiceAccountName}); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	openshiftConfig, err := origin.BuildMasterConfig(*master)
	checkErr(t, err)

	// Get the build controller clients, since those rely on service account tokens
	// We don't want to proceed with the rest of the test until those are available
	openshiftConfig.BuildControllerClients()

	for i := 0; i < additionalBuildControllers; i++ {
		openshiftConfig.RunBuildController()
	}
	for i := 0; i < additionalBuildPodControllers; i++ {
		openshiftConfig.RunBuildPodController()
	}
	for i := 0; i < additionalImageChangeControllers; i++ {
		openshiftConfig.RunBuildImageChangeTriggerController()
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
	return nil
}

func runImageChangeTriggerTest(t *testing.T, clusterAdminClient *client.Client, imageStream *imageapi.ImageStream, imageStreamMapping *imageapi.ImageStreamMapping, config *buildapi.BuildConfig, tag string) {
	created, err := clusterAdminClient.BuildConfigs(testutil.Namespace()).Create(config)
	if err != nil {
		t.Fatalf("Couldn't create BuildConfig: %v", err)
	}

	watch, err := clusterAdminClient.Builds(testutil.Namespace()).Watch(labels.Everything(), fields.Everything(), created.ResourceVersion)
	if err != nil {
		t.Fatalf("Couldn't subscribe to Builds %v", err)
	}

	watch2, err := clusterAdminClient.BuildConfigs(testutil.Namespace()).Watch(labels.Everything(), fields.Everything(), created.ResourceVersion)
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
	switch newBuild.Spec.Strategy.Type {
	case buildapi.SourceBuildStrategyType:
		if newBuild.Spec.Strategy.SourceStrategy.From.Name != "registry:8080/openshift/test-image-trigger:"+tag {
			i, _ := clusterAdminClient.ImageStreams(testutil.Namespace()).Get(imageStream.Name)
			bc, _ := clusterAdminClient.BuildConfigs(testutil.Namespace()).Get(config.Name)
			t.Fatalf("Expected build with base image %s, got %s\n, imagerepo is %v\ntrigger is %s\n", "registry:8080/openshift/test-image-trigger:"+tag, newBuild.Spec.Strategy.DockerStrategy.From.Name, i, bc.Spec.Triggers[0].ImageChange)
		}
	case buildapi.DockerBuildStrategyType:
		if newBuild.Spec.Strategy.DockerStrategy.From.Name != "registry:8080/openshift/test-image-trigger:"+tag {
			i, _ := clusterAdminClient.ImageStreams(testutil.Namespace()).Get(imageStream.Name)
			bc, _ := clusterAdminClient.BuildConfigs(testutil.Namespace()).Get(config.Name)
			t.Fatalf("Expected build with base image %s, got %s\n, imagerepo is %v\ntrigger is %s\n", "registry:8080/openshift/test-image-trigger:"+tag, newBuild.Spec.Strategy.DockerStrategy.From.Name, i, bc.Spec.Triggers[0].ImageChange)
		}
	case buildapi.CustomBuildStrategyType:
		if newBuild.Spec.Strategy.CustomStrategy.From.Name != "registry:8080/openshift/test-image-trigger:"+tag {
			i, _ := clusterAdminClient.ImageStreams(testutil.Namespace()).Get(imageStream.Name)
			bc, _ := clusterAdminClient.BuildConfigs(testutil.Namespace()).Get(config.Name)
			t.Fatalf("Expected build with base image %s, got %s\n, imagerepo is %v\ntrigger is %s\n", "registry:8080/openshift/test-image-trigger:"+tag, newBuild.Spec.Strategy.DockerStrategy.From.Name, i, bc.Spec.Triggers[0].ImageChange)
		}
	}
	// Wait for an update on the specific build that was added
	watch3, err := clusterAdminClient.Builds(testutil.Namespace()).Watch(labels.Everything(), fields.OneTermEqualSelector("name", newBuild.Name), newBuild.ResourceVersion)
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
	switch newBuild.Spec.Strategy.Type {
	case buildapi.SourceBuildStrategyType:
		if newBuild.Spec.Strategy.SourceStrategy.From.Name != "registry:8080/openshift/test-image-trigger:ref-2-random" {
			i, _ := clusterAdminClient.ImageStreams(testutil.Namespace()).Get(imageStream.Name)
			bc, _ := clusterAdminClient.BuildConfigs(testutil.Namespace()).Get(config.Name)
			t.Fatalf("Expected build with base image %s, got %s\n, imagerepo is %v\trigger is %s\n", "registry:8080/openshift/test-image-trigger:ref-2-random", newBuild.Spec.Strategy.DockerStrategy.From.Name, i, bc.Spec.Triggers[3].ImageChange)
		}
	case buildapi.DockerBuildStrategyType:
		if newBuild.Spec.Strategy.DockerStrategy.From.Name != "registry:8080/openshift/test-image-trigger:ref-2-random" {
			i, _ := clusterAdminClient.ImageStreams(testutil.Namespace()).Get(imageStream.Name)
			bc, _ := clusterAdminClient.BuildConfigs(testutil.Namespace()).Get(config.Name)
			t.Fatalf("Expected build with base image %s, got %s\n, imagerepo is %v\trigger is %s\n", "registry:8080/openshift/test-image-trigger:ref-2-random", newBuild.Spec.Strategy.DockerStrategy.From.Name, i, bc.Spec.Triggers[3].ImageChange)
		}
	case buildapi.CustomBuildStrategyType:
		if newBuild.Spec.Strategy.CustomStrategy.From.Name != "registry:8080/openshift/test-image-trigger:ref-2-random" {
			i, _ := clusterAdminClient.ImageStreams(testutil.Namespace()).Get(imageStream.Name)
			bc, _ := clusterAdminClient.BuildConfigs(testutil.Namespace()).Get(config.Name)
			t.Fatalf("Expected build with base image %s, got %s\n, imagerepo is %v\trigger is %s\n", "registry:8080/openshift/test-image-trigger:ref-2-random", newBuild.Spec.Strategy.DockerStrategy.From.Name, i, bc.Spec.Triggers[3].ImageChange)
		}
	}

	// Listen to events on specific  build
	watch4, err := clusterAdminClient.Builds(testutil.Namespace()).Watch(labels.Everything(), fields.OneTermEqualSelector("name", newBuild.Name), newBuild.ResourceVersion)
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

	buildWatch, err := clusterAdminClient.Builds(testutil.Namespace()).Watch(labels.Everything(), fields.Everything(), "0")
	if err != nil {
		t.Fatalf("Couldn't subscribe to Builds %v", err)
	}
	defer buildWatch.Stop()

	created, err := clusterAdminClient.Builds(testutil.Namespace()).Create(mockBuild())
	if err != nil {
		t.Fatalf("Couldn't create Build: %v", err)
	}

	podWatch, err := clusterAdminKubeClient.Pods(testutil.Namespace()).Watch(labels.Everything(), fields.Everything(), created.ResourceVersion)
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

	event = waitForWatch(t, "pod deleted due to build deleted", podWatch)
	if e, a := watchapi.Deleted, event.Type; e != a {
		t.Fatalf("expected watch event type %s, got %s", e, a)
	}
	pod := event.Object.(*kapi.Pod)
	if expected := buildutil.GetBuildPodName(newBuild); pod.Name != expected {
		t.Fatalf("Expected pod %s to be deleted, but pod %s was deleted", expected, pod.Name)
	}

}

func runBuildRunningPodDeleteTest(t *testing.T, clusterAdminClient *client.Client, clusterAdminKubeClient *kclient.Client) {

	buildWatch, err := clusterAdminClient.Builds(testutil.Namespace()).Watch(labels.Everything(), fields.Everything(), "0")
	if err != nil {
		t.Fatalf("Couldn't subscribe to Builds %v", err)
	}
	defer buildWatch.Stop()

	created, err := clusterAdminClient.Builds(testutil.Namespace()).Create(mockBuild())
	if err != nil {
		t.Fatalf("Couldn't create Build: %v", err)
	}

	podWatch, err := clusterAdminKubeClient.Pods(testutil.Namespace()).Watch(labels.Everything(), fields.Everything(), created.ResourceVersion)
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

	clusterAdminKubeClient.Pods(testutil.Namespace()).Delete(buildutil.GetBuildPodName(newBuild), kapi.NewDeleteOptions(0))
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

	buildWatch, err := clusterAdminClient.Builds(testutil.Namespace()).Watch(labels.Everything(), fields.Everything(), "0")
	if err != nil {
		t.Fatalf("Couldn't subscribe to Builds %v", err)
	}
	defer buildWatch.Stop()

	created, err := clusterAdminClient.Builds(testutil.Namespace()).Create(mockBuild())
	if err != nil {
		t.Fatalf("Couldn't create Build: %v", err)
	}

	podWatch, err := clusterAdminKubeClient.Pods(testutil.Namespace()).Watch(labels.Everything(), fields.Everything(), created.ResourceVersion)
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

	clusterAdminKubeClient.Pods(testutil.Namespace()).Delete(buildutil.GetBuildPodName(newBuild), kapi.NewDeleteOptions(0))
	time.Sleep(10 * time.Second)
	newBuild, err = clusterAdminClient.Builds(testutil.Namespace()).Get(newBuild.Name)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if newBuild.Status.Phase != buildapi.BuildPhaseComplete {
		t.Fatalf("build status was updated to %s after deleting pod, should have stayed as %s", newBuild.Status.Phase, buildapi.BuildPhaseComplete)
	}
}
