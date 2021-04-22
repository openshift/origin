package builds

import (
	"context"
	"fmt"
	"os"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/apitesting"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/wait"
	watchapi "k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"

	buildv1 "github.com/openshift/api/build/v1"
	imagev1 "github.com/openshift/api/image/v1"
	buildv1clienttyped "github.com/openshift/client-go/build/clientset/versioned/typed/build/v1"
	imagev1clienttyped "github.com/openshift/client-go/image/clientset/versioned/typed/image/v1"
)

const (
	// BuildStartedEventReason is the reason associated with the event registered when a build is started (pod is created).
	BuildStartedEventReason = "BuildStarted"
	// BuildStartedEventMessage is the message associated with the event registered when a build is started (pod is created).
	BuildStartedEventMessage = "Build %s/%s is now running"
	// BuildCompletedEventReason is the reason associated with the event registered when build completes successfully.
	BuildCompletedEventReason = "BuildCompleted"
	// BuildCompletedEventMessage is the message associated with the event registered when build completes successfully.
	BuildCompletedEventMessage = "Build %s/%s completed successfully"
	// BuildFailedEventReason is the reason associated with the event registered when build fails.
	BuildFailedEventReason = "BuildFailed"
	// BuildFailedEventMessage is the message associated with the event registered when build fails.
	BuildFailedEventMessage = "Build %s/%s failed"
	// BuildCancelledEventReason is the reason associated with the event registered when build is cancelled.
	BuildCancelledEventReason = "BuildCancelled"
	// BuildCancelledEventMessage is the message associated with the event registered when build is cancelled.
	BuildCancelledEventMessage        = "Build %s/%s has been cancelled"
	additionalAllowedRegistriesEnvVar = "ADDITIONAL_ALLOWED_REGISTRIES"
)

var (
	//TODO: Make these externally configurable

	// BuildControllerTestWait is the time that RunBuildControllerTest waits
	// for any other changes to happen when testing whether only a single build got processed
	BuildControllerTestWait = 10 * time.Second

	// BuildControllerTestTransitionTimeout is the time RunBuildControllerPodSyncTest waits
	// for a build trasition to occur after the pod's status has been updated
	BuildControllerTestTransitionTimeout = 60 * time.Second

	// BuildControllersWatchTimeout is used by all tests to wait for watch events. In case where only
	// a single watch event is expected, the test will fail after the timeout.
	// The value is 6 minutes to allow for a resync to occur, which allows for necessarily
	// reconciliation to occur in tests where events occur in a non-deterministic order.
	BuildControllersWatchTimeout = 360 * time.Second

	buildScheme = runtime.NewScheme()
	buildCodecs = serializer.CodecFactory{}
)

func init() {
	buildScheme, buildCodecs = apitesting.SchemeForOrDie(buildv1.Install)
}

type testingT interface {
	Fail()
	Error(args ...interface{})
	Errorf(format string, args ...interface{})
	FailNow()
	Fatal(args ...interface{})
	Fatalf(format string, args ...interface{})
	Log(args ...interface{})
	Logf(format string, args ...interface{})
	Failed() bool
	Parallel()
	Skip(args ...interface{})
	Skipf(format string, args ...interface{})
	SkipNow()
	Skipped() bool
}

func mockBuild() *buildv1.Build {
	return &buildv1.Build{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "mock-build",
			Labels: map[string]string{
				"label1":                    "value1",
				"label2":                    "value2",
				buildv1.BuildConfigLabel:    "mock-build-config",
				buildv1.BuildRunPolicyLabel: string(buildv1.BuildRunPolicyParallel),
			},
		},
		Spec: buildv1.BuildSpec{
			CommonSpec: buildv1.CommonSpec{
				Source: buildv1.BuildSource{
					Git: &buildv1.GitBuildSource{
						URI: "http://my.docker/build",
					},
					ContextDir: "context",
				},
				Strategy: buildv1.BuildStrategy{
					DockerStrategy: &buildv1.DockerBuildStrategy{},
				},
				Output: buildv1.BuildOutput{
					To: &corev1.ObjectReference{
						Kind: "DockerImage",
						Name: "namespace/builtimage",
					},
				},
			},
		},
	}
}

type buildControllerPodState struct {
	PodPhase   corev1.PodPhase
	BuildPhase buildv1.BuildPhase
}

type buildControllerPodTest struct {
	Name   string
	States []buildControllerPodState
}

func waitForWatch(t testingT, name string, w watchapi.Interface) *watchapi.Event {
	select {
	case e, ok := <-w.ResultChan():
		if !ok {
			t.Fatalf("Channel closed waiting for watch: %s", name)
		}
		return &e
	case <-time.After(BuildControllersWatchTimeout):
		t.Fatalf("Timed out waiting for watch: %s", name)
		return nil
	}
}

func RunImageChangeTriggerTest(t testingT, clusterAdminBuildClient buildv1clienttyped.BuildV1Interface, clusterAdminImageClient imagev1clienttyped.ImageV1Interface, ns string) {
	const (
		tag              = "latest"
		streamName       = "test-image-trigger-repo"
		registryHostname = "registry:8080"
	)

	os.Setenv(additionalAllowedRegistriesEnvVar, registryHostname)

	imageStream := mockImageStream2(registryHostname, tag)
	imageStreamMapping := mockImageStreamMapping(imageStream.Name, "someimage", tag, registryHostname+"/openshift/test-image-trigger:"+tag)

	config := imageChangeBuildConfig(ns, "sti-imagestreamtag", stiStrategy("ImageStreamTag", streamName+":"+tag))
	_, err := clusterAdminBuildClient.BuildConfigs(ns).Create(context.Background(), config, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Couldn't create BuildConfig: %v", err)
	}

	watch, err := clusterAdminBuildClient.Builds(ns).Watch(context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("Couldn't subscribe to Builds %v", err)
	}
	defer watch.Stop()

	watch2, err := clusterAdminBuildClient.BuildConfigs(ns).Watch(context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("Couldn't subscribe to BuildConfigs %v", err)
	}
	defer watch2.Stop()

	imageStream, err = clusterAdminImageClient.ImageStreams(ns).Create(context.Background(), imageStream, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Couldn't create ImageStream: %v", err)
	}

	// give the imagechangecontroller's buildconfig cache time to be updated with the buildconfig object
	// so it doesn't get a miss when looking up the BC while processing the imagestream update event.
	time.Sleep(10 * time.Second)
	_, err = clusterAdminImageClient.ImageStreamMappings(ns).Create(context.Background(), imageStreamMapping, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Couldn't create Image: %v", err)
	}

	// wait for initial build event from the creation of the imagerepo with tag latest
	event := waitForWatch(t, "initial build added", watch)
	if e, a := watchapi.Added, event.Type; e != a {
		t.Fatalf("expected watch event type %s, got %s", e, a)
	}
	newBuild := event.Object.(*buildv1.Build)
	strategy := newBuild.Spec.Strategy
	if strategy.SourceStrategy.From.Name != registryHostname+"/openshift/test-image-trigger:"+tag {
		i, _ := clusterAdminImageClient.ImageStreams(ns).Get(context.Background(), imageStream.Name, metav1.GetOptions{})
		bc, _ := clusterAdminBuildClient.BuildConfigs(ns).Get(context.Background(), config.Name, metav1.GetOptions{})
		t.Fatalf("Expected build with base image %s, got %s\n, imagerepo is %v\ntrigger is %s\n", registryHostname+"/openshift/test-image-trigger:"+tag, strategy.SourceStrategy.From.Name, i, bc.Spec.Triggers[0].ImageChange)
	}
	// Wait for an update on the specific build that was added
	watch3, err := clusterAdminBuildClient.Builds(ns).Watch(context.Background(), metav1.ListOptions{FieldSelector: fields.OneTermEqualSelector("metadata.name", newBuild.Name).String(), ResourceVersion: newBuild.ResourceVersion})
	defer watch3.Stop()
	if err != nil {
		t.Fatalf("Couldn't subscribe to Builds %v", err)
	}
	event = waitForWatch(t, "initial build update", watch3)
	if e, a := watchapi.Modified, event.Type; e != a {
		t.Fatalf("expected watch event type %s, got %s", e, a)
	}
	newBuild = event.Object.(*buildv1.Build)
	// Make sure the resolution of the build's docker image pushspec didn't mutate the persisted API object
	if newBuild.Spec.Output.To.Name != "test-image-trigger-repo:outputtag" {
		t.Fatalf("unexpected build output: %#v %#v", newBuild.Spec.Output.To, newBuild.Spec.Output)
	}
	if newBuild.Labels["testlabel"] != "testvalue" {
		t.Fatalf("Expected build with label %s=%s from build config got %s=%s", "testlabel", "testvalue", "testlabel", newBuild.Labels["testlabel"])
	}

	// wait for build config to be updated
	timeout := time.After(BuildControllerTestWait)
WaitLoop:
	for {
		select {
		case e, ok := <-watch2.ResultChan():
			if !ok {
				t.Fatalf("Channel closed waiting for watch: build config update in WaitLoop")
			}
			event = &e
			continue
		case <-timeout:
			break WaitLoop
		}
	}
	updatedConfig := event.Object.(*buildv1.BuildConfig)
	if err != nil {
		t.Fatalf("Couldn't get BuildConfig: %v", err)
	}
	// the first tag did not have an image id, so the last trigger field is the pull spec
	if updatedConfig.Spec.Triggers[0].ImageChange.LastTriggeredImageID != registryHostname+"/openshift/test-image-trigger:"+tag {
		t.Fatalf("Expected imageID equal to pull spec, got %#v", updatedConfig.Spec.Triggers[0].ImageChange)
	}

	// clear out the build/buildconfig watches before triggering a new build
	timeout = time.After(60 * time.Second)
WaitLoop2:
	for {
		select {
		case _, ok := <-watch.ResultChan():
			if !ok {
				t.Fatalf("Channel closed waiting for watch: build update in WaitLoop2")
			}
			continue
		case _, ok := <-watch2.ResultChan():
			if !ok {
				t.Fatalf("Channel closed waiting for watch: build config update in WaitLoop2")
			}
			continue
		case <-timeout:
			break WaitLoop2
		}
	}

	// trigger a build by posting a new image
	if _, err := clusterAdminImageClient.ImageStreamMappings(ns).Create(context.Background(), &imagev1.ImageStreamMapping{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      imageStream.Name,
		},
		Tag: tag,
		Image: imagev1.Image{
			ObjectMeta: metav1.ObjectMeta{
				Name: "ref-2-random",
			},
			DockerImageReference: registryHostname + "/openshift/test-image-trigger:ref-2-random",
		},
	}, metav1.CreateOptions{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	event = waitForWatch(t, "second build created", watch)
	if e, a := watchapi.Added, event.Type; e != a {
		t.Fatalf("expected watch event type %s, got %s", e, a)
	}
	newBuild = event.Object.(*buildv1.Build)
	strategy = newBuild.Spec.Strategy
	if strategy.SourceStrategy.From.Name != registryHostname+"/openshift/test-image-trigger:ref-2-random" {
		i, _ := clusterAdminImageClient.ImageStreams(ns).Get(context.Background(), imageStream.Name, metav1.GetOptions{})
		bc, _ := clusterAdminBuildClient.BuildConfigs(ns).Get(context.Background(), config.Name, metav1.GetOptions{})
		t.Fatalf("Expected build with base image %s, got %s\n, imagerepo is %v\trigger is %s\n", registryHostname+"/openshift/test-image-trigger:ref-2-random", strategy.SourceStrategy.From.Name, i, bc.Spec.Triggers[3].ImageChange)
	}

	// Listen to events on specific  build
	watch4, err := clusterAdminBuildClient.Builds(ns).Watch(context.Background(), metav1.ListOptions{FieldSelector: fields.OneTermEqualSelector("metadata.name", newBuild.Name).String(), ResourceVersion: newBuild.ResourceVersion})
	defer watch4.Stop()

	event = waitForWatch(t, "update on second build", watch4)
	if e, a := watchapi.Modified, event.Type; e != a {
		t.Fatalf("expected watch event type %s, got %s", e, a)
	}
	newBuild = event.Object.(*buildv1.Build)
	// Make sure the resolution of the build's docker image pushspec didn't mutate the persisted API object
	if newBuild.Spec.Output.To.Name != "test-image-trigger-repo:outputtag" {
		t.Fatalf("unexpected build output: %#v %#v", newBuild.Spec.Output.To, newBuild.Spec.Output)
	}
	if newBuild.Labels["testlabel"] != "testvalue" {
		t.Fatalf("Expected build with label %s=%s from build config got %s=%s", "testlabel", "testvalue", "testlabel", newBuild.Labels["testlabel"])
	}

	timeout = time.After(BuildControllerTestWait)
WaitLoop3:
	for {
		select {
		case e, ok := <-watch2.ResultChan():
			if !ok {
				t.Fatalf("Channel closed waiting for watch: build config update in WaitLoop3")
			}
			event = &e
			continue
		case <-timeout:
			break WaitLoop3
		}
	}
	updatedConfig = event.Object.(*buildv1.BuildConfig)
	if e, a := registryHostname+"/openshift/test-image-trigger:ref-2-random", updatedConfig.Spec.Triggers[0].ImageChange.LastTriggeredImageID; e != a {
		t.Errorf("unexpected trigger id: expected %v, got %v", e, a)
	}
}

func RunBuildDeleteTest(t testingT, clusterAdminClient buildv1clienttyped.BuildsGetter, clusterAdminKubeClientset kubernetes.Interface, ns string) {
	buildWatch, err := clusterAdminClient.Builds(ns).Watch(context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("Couldn't subscribe to Builds %v", err)
	}
	defer buildWatch.Stop()

	_, err = clusterAdminClient.Builds(ns).Create(context.Background(), mockBuild(), metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Couldn't create Build: %v", err)
	}

	podWatch, err := clusterAdminKubeClientset.CoreV1().Pods(ns).Watch(context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("Couldn't subscribe to Pods %v", err)
	}
	defer podWatch.Stop()

	// wait for initial build event from the creation of the imagerepo with tag latest
	event := waitForWatch(t, "initial build added", buildWatch)
	if e, a := watchapi.Added, event.Type; e != a {
		t.Fatalf("expected watch event type %s, got %s", e, a)
	}
	newBuild := event.Object.(*buildv1.Build)

	// initial pod creation for build
	event = waitForWatch(t, "build pod created", podWatch)
	if e, a := watchapi.Added, event.Type; e != a {
		t.Fatalf("expected watch event type %s, got %s", e, a)
	}

	if err := clusterAdminClient.Builds(ns).Delete(context.Background(), newBuild.Name, metav1.DeleteOptions{}); err != nil {
		t.Fatal(err)
	}

	event = waitForWatchType(t, "pod deleted due to build deleted", podWatch, watchapi.Deleted)
	if e, a := watchapi.Deleted, event.Type; e != a {
		t.Fatalf("expected watch event type %s, got %s", e, a)
	}
	pod := event.Object.(*corev1.Pod)
	if expected := GetBuildPodName(newBuild); pod.Name != expected {
		t.Fatalf("Expected pod %s to be deleted, but pod %s was deleted", expected, pod.Name)
	}

}

// waitForWatchType tolerates receiving 10 events before failing while watching for a particular event
// type.
func waitForWatchType(t testingT, name string, w watchapi.Interface, expect watchapi.EventType) *watchapi.Event {
	tries := 10
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

func RunBuildRunningPodDeleteTest(t testingT, clusterAdminClient buildv1clienttyped.BuildsGetter, clusterAdminKubeClientset kubernetes.Interface, ns string) {

	buildWatch, err := clusterAdminClient.Builds(ns).Watch(context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("Couldn't subscribe to Builds %v", err)
	}
	defer buildWatch.Stop()

	_, err = clusterAdminClient.Builds(ns).Create(context.Background(), mockBuild(), metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Couldn't create Build: %v", err)
	}

	podWatch, err := clusterAdminKubeClientset.CoreV1().Pods(ns).Watch(context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("Couldn't subscribe to Pods %v", err)
	}
	defer podWatch.Stop()

	// wait for initial build event from the creation of the imagerepo with tag latest
	event := waitForWatch(t, "initial build added", buildWatch)
	if e, a := watchapi.Added, event.Type; e != a {
		t.Fatalf("expected watch event type %s, got %s", e, a)
	}
	newBuild := event.Object.(*buildv1.Build)
	buildName := newBuild.Name
	podName := newBuild.Name + "-build"

	// initial pod creation for build
	for {
		event = waitForWatch(t, "build pod created", podWatch)
		newPod := event.Object.(*corev1.Pod)
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
		newBuild = event.Object.(*buildv1.Build)
		if newBuild.Name == buildName {
			break
		}
	}
	if e, a := watchapi.Modified, event.Type; e != a {
		t.Fatalf("expected watch event type %s, got %s", e, a)
	}
	if newBuild.Status.Phase != buildv1.BuildPhasePending {
		t.Fatalf("expected build status to be marked pending, but was marked %s", newBuild.Status.Phase)
	}

	clusterAdminKubeClientset.CoreV1().Pods(ns).Delete(context.Background(), GetBuildPodName(newBuild), *metav1.NewDeleteOptions(0))
	event = waitForWatch(t, "build updated to error", buildWatch)
	if e, a := watchapi.Modified, event.Type; e != a {
		t.Fatalf("expected watch event type %s, got %s", e, a)
	}
	newBuild = event.Object.(*buildv1.Build)
	if newBuild.Status.Phase != buildv1.BuildPhaseError {
		t.Fatalf("expected build status to be marked error, but was marked %s", newBuild.Status.Phase)
	}

	foundFailed := false

	err = wait.Poll(time.Second, 30*time.Second, func() (bool, error) {
		events, err := clusterAdminKubeClientset.CoreV1().Events(ns).Search(buildScheme, newBuild)
		if err != nil {
			t.Fatalf("error getting build events: %v", err)
			return false, fmt.Errorf("error getting build events: %v", err)
		}
		for _, event := range events.Items {
			if event.Reason == BuildFailedEventReason {
				foundFailed = true
				expect := fmt.Sprintf(BuildFailedEventMessage, newBuild.Namespace, newBuild.Name)
				if event.Message != expect {
					return false, fmt.Errorf("expected failed event message to be %s, got %s", expect, event.Message)
				}
				return true, nil
			}
		}
		return false, nil
	})

	if err != nil {
		t.Fatalf("unexpected: %v", err)
		return
	}

	if !foundFailed {
		t.Fatalf("expected to find a failed event on the build %s/%s", newBuild.Namespace, newBuild.Name)
	}
}

func RunBuildCompletePodDeleteTest(t testingT, clusterAdminClient buildv1clienttyped.BuildsGetter, clusterAdminKubeClientset kubernetes.Interface, ns string) {

	buildWatch, err := clusterAdminClient.Builds(ns).Watch(context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("Couldn't subscribe to Builds %v", err)
	}
	defer buildWatch.Stop()

	_, err = clusterAdminClient.Builds(ns).Create(context.Background(), mockBuild(), metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Couldn't create Build: %v", err)
	}

	podWatch, err := clusterAdminKubeClientset.CoreV1().Pods(ns).Watch(context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("Couldn't subscribe to Pods %v", err)
	}
	defer podWatch.Stop()

	// wait for initial build event from the creation of the imagerepo with tag latest
	event := waitForWatch(t, "initial build added", buildWatch)
	if e, a := watchapi.Added, event.Type; e != a {
		t.Fatalf("expected watch event type %s, got %s", e, a)
	}
	newBuild := event.Object.(*buildv1.Build)

	// initial pod creation for build
	event = waitForWatch(t, "build pod created", podWatch)
	if e, a := watchapi.Added, event.Type; e != a {
		t.Fatalf("expected watch event type %s, got %s", e, a)
	}

	event = waitForWatch(t, "build updated to pending", buildWatch)
	if e, a := watchapi.Modified, event.Type; e != a {
		t.Fatalf("expected watch event type %s, got %s", e, a)
	}

	newBuild = event.Object.(*buildv1.Build)
	if newBuild.Status.Phase != buildv1.BuildPhasePending {
		t.Fatalf("expected build status to be marked pending, but was marked %s", newBuild.Status.Phase)
	}

	newBuild.Status.Phase = buildv1.BuildPhaseComplete
	clusterAdminClient.Builds(ns).Update(context.Background(), newBuild, metav1.UpdateOptions{})
	event = waitForWatch(t, "build updated to complete", buildWatch)
	if e, a := watchapi.Modified, event.Type; e != a {
		t.Fatalf("expected watch event type %s, got %s", e, a)
	}
	newBuild = event.Object.(*buildv1.Build)
	if newBuild.Status.Phase != buildv1.BuildPhaseComplete {
		t.Fatalf("expected build status to be marked complete, but was marked %s", newBuild.Status.Phase)
	}

	clusterAdminKubeClientset.CoreV1().Pods(ns).Delete(context.Background(), GetBuildPodName(newBuild), *metav1.NewDeleteOptions(0))
	time.Sleep(10 * time.Second)
	newBuild, err = clusterAdminClient.Builds(ns).Get(context.Background(), newBuild.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if newBuild.Status.Phase != buildv1.BuildPhaseComplete {
		t.Fatalf("build status was updated to %s after deleting pod, should have stayed as %s", newBuild.Status.Phase, buildv1.BuildPhaseComplete)
	}
}

func RunBuildConfigChangeControllerTest(t testingT, clusterAdminBuildClient buildv1clienttyped.BuildV1Interface, ns string) {
	config := configChangeBuildConfig(ns)
	created, err := clusterAdminBuildClient.BuildConfigs(ns).Create(context.Background(), config, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Couldn't create BuildConfig: %v", err)
	}

	watch, err := clusterAdminBuildClient.Builds(ns).Watch(context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("Couldn't subscribe to Builds %v", err)
	}
	defer watch.Stop()

	watch2, err := clusterAdminBuildClient.BuildConfigs(ns).Watch(context.Background(), metav1.ListOptions{ResourceVersion: created.ResourceVersion})
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
	if bc := event.Object.(*buildv1.BuildConfig); bc.Status.LastVersion == 0 {
		t.Fatalf("expected build config lastversion to be greater than zero after build")
	}
}

func configChangeBuildConfig(ns string) *buildv1.BuildConfig {
	bc := &buildv1.BuildConfig{}
	bc.Name = "testcfgbc"
	bc.Namespace = ns
	bc.Spec.Source.Git = &buildv1.GitBuildSource{}
	bc.Spec.Source.Git.URI = "https://github.com/openshift/ruby-hello-world.git"
	bc.Spec.Strategy.DockerStrategy = &buildv1.DockerBuildStrategy{}
	configChangeTrigger := buildv1.BuildTriggerPolicy{Type: buildv1.ConfigChangeBuildTriggerType}
	bc.Spec.Triggers = append(bc.Spec.Triggers, configChangeTrigger)
	return bc
}

func mockImageStream2(registryHostname, tag string) *imagev1.ImageStream {
	return &imagev1.ImageStream{
		ObjectMeta: metav1.ObjectMeta{Name: "test-image-trigger-repo"},

		Spec: imagev1.ImageStreamSpec{
			DockerImageRepository: registryHostname + "/openshift/test-image-trigger",
			Tags: []imagev1.TagReference{
				{
					Name: tag,
					From: &corev1.ObjectReference{
						Kind: "DockerImage",
						Name: registryHostname + "/openshift/test-image-trigger:" + tag,
					},
				},
			},
		},
	}
}

func mockImageStreamMapping(stream, image, tag, reference string) *imagev1.ImageStreamMapping {
	// create a mapping to an image that doesn't exist
	return &imagev1.ImageStreamMapping{
		ObjectMeta: metav1.ObjectMeta{Name: stream},
		Tag:        tag,
		Image: imagev1.Image{
			ObjectMeta: metav1.ObjectMeta{
				Name: image,
			},
			DockerImageReference: reference,
		},
	}
}

func imageChangeBuildConfig(ns, name string, strategy buildv1.BuildStrategy) *buildv1.BuildConfig {
	return &buildv1.BuildConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
			Labels:    map[string]string{"testlabel": "testvalue"},
		},
		Spec: buildv1.BuildConfigSpec{

			RunPolicy: buildv1.BuildRunPolicyParallel,
			CommonSpec: buildv1.CommonSpec{
				Source: buildv1.BuildSource{
					Git: &buildv1.GitBuildSource{
						URI: "https://github.com/openshift/ruby-hello-world.git",
					},
					ContextDir: "contextimage",
				},
				Strategy: strategy,
				Output: buildv1.BuildOutput{
					To: &corev1.ObjectReference{
						Kind: "ImageStreamTag",
						Name: "test-image-trigger-repo:outputtag",
					},
				},
			},
			Triggers: []buildv1.BuildTriggerPolicy{
				{
					Type:        buildv1.ImageChangeBuildTriggerType,
					ImageChange: &buildv1.ImageChangeTrigger{},
				},
			},
		},
	}
}

func stiStrategy(kind, name string) buildv1.BuildStrategy {
	return buildv1.BuildStrategy{
		SourceStrategy: &buildv1.SourceBuildStrategy{
			From: corev1.ObjectReference{
				Kind: kind,
				Name: name,
			},
		},
	}
}
