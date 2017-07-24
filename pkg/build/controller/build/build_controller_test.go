package build

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	clientgotesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/v1"
	kexternalclientset "k8s.io/kubernetes/pkg/client/clientset_generated/clientset"
	kexternalclientfake "k8s.io/kubernetes/pkg/client/clientset_generated/clientset/fake"
	kinternalclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	kinternalclientfake "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/fake"
	kexternalinformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/externalversions"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	"github.com/openshift/origin/pkg/build/apis/build/validation"
	builddefaults "github.com/openshift/origin/pkg/build/controller/build/defaults"
	buildoverrides "github.com/openshift/origin/pkg/build/controller/build/overrides"
	"github.com/openshift/origin/pkg/build/controller/common"
	"github.com/openshift/origin/pkg/build/controller/policy"
	strategy "github.com/openshift/origin/pkg/build/controller/strategy"
	buildinformersinternal "github.com/openshift/origin/pkg/build/generated/informers/internalversion"
	buildinternalclientset "github.com/openshift/origin/pkg/build/generated/internalclientset"
	buildinternalfakeclient "github.com/openshift/origin/pkg/build/generated/internalclientset/fake"
	buildutil "github.com/openshift/origin/pkg/build/util"
	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/client/testclient"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	imageinformersinternal "github.com/openshift/origin/pkg/image/generated/informers/internalversion"
	imageinternalclientset "github.com/openshift/origin/pkg/image/generated/internalclientset"
	imageinternalfakeclient "github.com/openshift/origin/pkg/image/generated/internalclientset/fake"
)

// TestHandleBuild is the main test for build updates through the controller
func TestHandleBuild(t *testing.T) {

	now := metav1.Now()
	before := metav1.NewTime(now.Time.Add(-1 * time.Hour))

	build := func(phase buildapi.BuildPhase) *buildapi.Build {
		b := dockerStrategy(mockBuild(phase, buildapi.BuildOutput{}))
		if phase != buildapi.BuildPhaseNew {
			podName := buildapi.GetBuildPodName(b)
			common.SetBuildPodNameAnnotation(b, podName)
		}
		return b
	}
	pod := func(phase v1.PodPhase) *v1.Pod {
		p := mockBuildPod(build(buildapi.BuildPhaseNew))
		p.Status.Phase = phase
		switch phase {
		case v1.PodRunning, v1.PodFailed:
			p.Status.StartTime = &now
		case v1.PodSucceeded:
			p.Status.StartTime = &now
			p.Status.ContainerStatuses = []v1.ContainerStatus{
				{
					Name: "container",
					State: v1.ContainerState{
						Terminated: &v1.ContainerStateTerminated{
							ExitCode: 0,
						},
					},
				},
			}
		}
		return p
	}
	cancelled := func(build *buildapi.Build) *buildapi.Build {
		build.Status.Cancelled = true
		return build
	}
	withCompletionTS := func(build *buildapi.Build) *buildapi.Build {
		build.Status.CompletionTimestamp = &now
		return build
	}
	withBuildCreationTS := func(build *buildapi.Build, tm metav1.Time) *buildapi.Build {
		build.CreationTimestamp = tm
		return build
	}
	withPodCreationTS := func(pod *v1.Pod, tm metav1.Time) *v1.Pod {
		pod.CreationTimestamp = tm
		return pod
	}

	tests := []struct {
		name string

		// Conditions
		build              *buildapi.Build
		pod                *v1.Pod
		runPolicy          *fakeRunPolicy
		errorOnPodDelete   bool
		errorOnPodCreate   bool
		errorOnBuildUpdate bool

		// Expected Result
		expectUpdate     *buildUpdate
		expectPodCreated bool
		expectPodDeleted bool
		expectError      bool
		expectOnComplete bool
	}{
		{
			name:  "cancel running build",
			build: cancelled(build(buildapi.BuildPhaseRunning)),
			pod:   pod(v1.PodRunning),
			expectUpdate: newUpdate().phase(buildapi.BuildPhaseCancelled).
				reason(buildapi.StatusReasonCancelledBuild).
				message(buildapi.StatusMessageCancelledBuild).
				completionTime(now).
				startTime(now).update,
			expectPodDeleted: true,
			expectOnComplete: true,
		},
		{
			name:         "cancel build in terminal state",
			build:        cancelled(withCompletionTS(build(buildapi.BuildPhaseComplete))),
			pod:          pod(v1.PodRunning),
			expectUpdate: nil,
		},
		{
			name:             "cancel build with delete pod error",
			build:            cancelled(build(buildapi.BuildPhaseRunning)),
			errorOnPodDelete: true,
			expectUpdate:     nil,
			expectError:      true,
		},
		{
			name:  "new -> pending",
			build: build(buildapi.BuildPhaseNew),
			expectUpdate: newUpdate().
				phase(buildapi.BuildPhasePending).
				reason("").
				message("").
				podNameAnnotation(pod(v1.PodPending).Name).
				update,
			expectPodCreated: true,
		},
		{
			name:  "new with existing newer pod",
			build: withBuildCreationTS(build(buildapi.BuildPhaseNew), before),
			pod:   withPodCreationTS(pod(v1.PodRunning), now),
			expectUpdate: newUpdate().
				phase(buildapi.BuildPhaseRunning).
				reason("").
				message("").
				startTime(now).
				podNameAnnotation(pod(v1.PodRunning).Name).
				update,
		},
		{
			name:  "new with existing older pod",
			build: withBuildCreationTS(build(buildapi.BuildPhaseNew), now),
			pod:   withPodCreationTS(pod(v1.PodRunning), before),
			expectUpdate: newUpdate().
				phase(buildapi.BuildPhaseError).
				reason(buildapi.StatusReasonBuildPodExists).
				message(buildapi.StatusMessageBuildPodExists).
				podNameAnnotation(pod(v1.PodRunning).Name).
				startTime(now).
				completionTime(now).
				update,
			expectOnComplete: true,
		},
		{
			name:         "new not runnable by policy",
			build:        build(buildapi.BuildPhaseNew),
			runPolicy:    &fakeRunPolicy{notRunnable: true},
			expectUpdate: nil,
		},
		{
			name:               "new -> pending with update error",
			build:              build(buildapi.BuildPhaseNew),
			errorOnBuildUpdate: true,
			expectUpdate:       nil,
			expectPodCreated:   true,
			expectError:        true,
		},
		{
			name:  "pending -> running",
			build: build(buildapi.BuildPhasePending),
			pod:   pod(v1.PodRunning),
			expectUpdate: newUpdate().
				phase(buildapi.BuildPhaseRunning).
				reason("").
				message("").
				startTime(now).
				update,
		},
		{
			name:               "pending -> running with update error",
			build:              build(buildapi.BuildPhasePending),
			pod:                pod(v1.PodRunning),
			errorOnBuildUpdate: true,
			expectUpdate:       nil,
			expectError:        true,
		},
		{
			name:             "pending -> failed",
			build:            build(buildapi.BuildPhasePending),
			pod:              pod(v1.PodFailed),
			expectOnComplete: true,
			expectUpdate: newUpdate().
				phase(buildapi.BuildPhaseFailed).
				reason(buildapi.StatusReasonGenericBuildFailed).
				message(buildapi.StatusMessageGenericBuildFailed).
				startTime(now).
				completionTime(now).
				update,
		},
		{
			name:         "pending -> pending",
			build:        build(buildapi.BuildPhasePending),
			pod:          pod(v1.PodPending),
			expectUpdate: nil,
		},
		{
			name:             "running -> complete",
			build:            build(buildapi.BuildPhaseRunning),
			pod:              pod(v1.PodSucceeded),
			expectOnComplete: true,
			expectUpdate: newUpdate().
				phase(buildapi.BuildPhaseComplete).
				reason("").
				message("").
				startTime(now).
				completionTime(now).
				update,
		},
		{
			name:         "running -> running",
			build:        build(buildapi.BuildPhaseRunning),
			pod:          pod(v1.PodRunning),
			expectUpdate: nil,
		},
		{
			name:             "running with missing pod",
			build:            build(buildapi.BuildPhaseRunning),
			expectOnComplete: true,
			expectUpdate: newUpdate().
				phase(buildapi.BuildPhaseError).
				reason(buildapi.StatusReasonBuildPodDeleted).
				message(buildapi.StatusMessageBuildPodDeleted).
				startTime(now).
				completionTime(now).
				update,
		},
		{
			name:             "failed -> failed with no completion timestamp",
			build:            build(buildapi.BuildPhaseFailed),
			pod:              pod(v1.PodFailed),
			expectOnComplete: true,
			expectUpdate: newUpdate().
				startTime(now).
				completionTime(now).
				update,
		},
		{
			name:             "failed -> failed with completion timestamp+message",
			build:            withCompletionTS(build(buildapi.BuildPhaseFailed)),
			pod:              pod(v1.PodFailed),
			expectOnComplete: true,
			expectUpdate: newUpdate().
				startTime(now).
				completionTime(now).
				logSnippet("").
				update,
		},
	}

	for _, tc := range tests {
		func() {
			var patchedBuild *buildapi.Build
			var appliedPatch string
			openshiftClient := fakeOpenshiftClient(tc.build)
			openshiftClient.(*testclient.Fake).PrependReactor("patch", "builds",
				func(action clientgotesting.Action) (bool, runtime.Object, error) {
					if tc.errorOnBuildUpdate {
						return true, nil, fmt.Errorf("error")
					}
					var err error
					patchAction := action.(clientgotesting.PatchActionImpl)
					appliedPatch = string(patchAction.Patch)
					patchedBuild, err = validation.ApplyBuildPatch(tc.build, patchAction.Patch)
					if err != nil {
						panic(fmt.Sprintf("unexpected error: %v", err))
					}
					return true, patchedBuild, nil
				})
			var kubeClient kexternalclientset.Interface
			if tc.pod != nil {
				kubeClient = fakeKubeExternalClientSet(tc.pod)
			} else {
				kubeClient = fakeKubeExternalClientSet()
			}
			podDeleted := false
			podCreated := false
			kubeClient.(*kexternalclientfake.Clientset).PrependReactor("delete", "pods",
				func(action clientgotesting.Action) (bool, runtime.Object, error) {
					if tc.errorOnPodDelete {
						return true, nil, fmt.Errorf("error")
					}
					podDeleted = true
					return true, nil, nil
				})
			kubeClient.(*kexternalclientfake.Clientset).PrependReactor("create", "pods",
				func(action clientgotesting.Action) (bool, runtime.Object, error) {
					if tc.errorOnPodCreate {
						return true, nil, fmt.Errorf("error")
					}
					podCreated = true
					return true, nil, nil
				})

			bc := newFakeBuildController(openshiftClient, nil, nil, kubeClient, nil)
			defer bc.stop()

			runPolicy := tc.runPolicy
			if runPolicy == nil {
				runPolicy = &fakeRunPolicy{}
			}
			bc.runPolicies = []policy.RunPolicy{runPolicy}

			err := bc.handleBuild(tc.build)
			if err != nil {
				if !tc.expectError {
					t.Errorf("%s: unexpected error: %v", tc.name, err)
				}
			}
			if err == nil && tc.expectError {
				t.Errorf("%s: expected error, got none", tc.name)
			}
			if tc.expectUpdate == nil && patchedBuild != nil {
				t.Errorf("%s: did not expect a build update, got patch %s", tc.name, appliedPatch)
			}
			if tc.expectPodCreated != podCreated {
				t.Errorf("%s: pod created. expected: %v, actual: %v", tc.name, tc.expectPodCreated, podCreated)
			}
			if tc.expectOnComplete != runPolicy.onCompleteCalled {
				t.Errorf("%s: on complete called. expected: %v, actual: %v", tc.name, tc.expectOnComplete, runPolicy.onCompleteCalled)
			}
			if tc.expectUpdate != nil {
				if patchedBuild == nil {
					t.Errorf("%s: did not get an update. Expected: %v", tc.name, tc.expectUpdate)
					return
				}
				expectedBuild, err := buildutil.BuildDeepCopy(tc.build)
				if err != nil {
					t.Fatalf("unexpected: %v", err)
				}
				tc.expectUpdate.apply(expectedBuild)

				// For start/completion/duration fields, simply validate that they are set/not set
				if tc.expectUpdate.startTime != nil && patchedBuild.Status.StartTimestamp != nil {
					expectedBuild.Status.StartTimestamp = patchedBuild.Status.StartTimestamp
				}
				if tc.expectUpdate.completionTime != nil && patchedBuild.Status.CompletionTimestamp != nil {
					expectedBuild.Status.CompletionTimestamp = patchedBuild.Status.CompletionTimestamp
					expectedBuild.Status.Duration = patchedBuild.Status.Duration
				}
				expectedBuild.CreationTimestamp = patchedBuild.CreationTimestamp

				if !apiequality.Semantic.DeepEqual(*expectedBuild, *patchedBuild) {
					t.Errorf("%s: did not get expected %v on build. Patch: %s", tc.name, tc.expectUpdate, appliedPatch)
				}
			}
		}()
	}

}

// TestWork - High-level test of the work function to ensure that a build
// in the queue will be handled by updating the build status to pending
func TestWorkWithNewBuild(t *testing.T) {
	build := dockerStrategy(mockBuild(buildapi.BuildPhaseNew, buildapi.BuildOutput{}))
	var patchedBuild *buildapi.Build
	openshiftClient := fakeOpenshiftClient()
	openshiftClient.(*testclient.Fake).PrependReactor("patch", "builds", applyBuildPatchReaction(t, build, &patchedBuild))
	buildClient := fakeBuildClient(build)

	bc := newFakeBuildController(openshiftClient, buildClient, nil, nil, nil)
	defer bc.stop()
	bc.enqueueBuild(build)

	bc.work()

	if bc.queue.Len() > 0 {
		t.Errorf("Expected queue to be empty")
	}
	if patchedBuild == nil {
		t.Errorf("Expected patched build not to be nil")
	}

	if patchedBuild != nil && patchedBuild.Status.Phase != buildapi.BuildPhasePending {
		t.Errorf("Expected patched build status set to Pending. It is %s", patchedBuild.Status.Phase)
	}
}

func TestCreateBuildPod(t *testing.T) {
	kubeClient := fakeKubeExternalClientSet()
	bc := newFakeBuildController(nil, nil, nil, kubeClient, nil)
	defer bc.stop()
	build := dockerStrategy(mockBuild(buildapi.BuildPhaseNew, buildapi.BuildOutput{}))

	update, err := bc.createBuildPod(build)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}
	podName := buildapi.GetBuildPodName(build)
	// Validate update
	expected := &buildUpdate{}
	expected.setPodNameAnnotation(podName)
	expected.setPhase(buildapi.BuildPhasePending)
	expected.setReason("")
	expected.setMessage("")
	validateUpdate(t, "create build pod", expected, update)
	// Make sure that a pod was created
	_, err = kubeClient.Core().Pods("namespace").Get(podName, metav1.GetOptions{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCreateBuildPodWithImageStreamOutput(t *testing.T) {
	imageStream := &imageapi.ImageStream{}
	imageStream.Namespace = "isnamespace"
	imageStream.Name = "isname"
	imageStream.Status.DockerImageRepository = "namespace/image-name"
	imageClient := fakeImageClient(imageStream)
	imageStreamRef := &kapi.ObjectReference{Name: "isname:latest", Namespace: "isnamespace", Kind: "ImageStreamTag"}
	bc := newFakeBuildController(nil, nil, imageClient, nil, nil)
	defer bc.stop()
	build := dockerStrategy(mockBuild(buildapi.BuildPhaseNew, buildapi.BuildOutput{To: imageStreamRef}))
	podName := buildapi.GetBuildPodName(build)

	update, err := bc.createBuildPod(build)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := &buildUpdate{}
	expected.setPodNameAnnotation(podName)
	expected.setPhase(buildapi.BuildPhasePending)
	expected.setReason("")
	expected.setMessage("")
	expected.setOutputRef("namespace/image-name:latest")
	validateUpdate(t, "create build pod with imagestream output", expected, update)
	if len(bc.imageStreamQueue.Pop("isnamespace/isname")) > 0 {
		t.Errorf("should not have queued build update")
	}
}

func TestCreateBuildPodWithOutputImageStreamMissing(t *testing.T) {
	imageStreamRef := &kapi.ObjectReference{Name: "isname:latest", Namespace: "isnamespace", Kind: "ImageStreamTag"}
	bc := newFakeBuildController(nil, nil, nil, nil, nil)
	defer bc.stop()
	build := dockerStrategy(mockBuild(buildapi.BuildPhaseNew, buildapi.BuildOutput{To: imageStreamRef}))

	update, err := bc.createBuildPod(build)

	if err != nil {
		t.Fatalf("Expected no error")
	}
	expected := &buildUpdate{}
	expected.setReason(buildapi.StatusReasonInvalidOutputReference)
	expected.setMessage(buildapi.StatusMessageInvalidOutputRef)
	validateUpdate(t, "create build pod with image stream error", expected, update)
	if !reflect.DeepEqual(bc.imageStreamQueue.Pop("isnamespace/isname"), []string{"namespace/data-build"}) {
		t.Errorf("should have queued build update: %#v", bc.imageStreamQueue)
	}
}

func TestCreateBuildPodWithImageStreamMissing(t *testing.T) {
	imageStreamRef := &kapi.ObjectReference{Name: "isname:latest", Kind: "DockerImage"}
	bc := newFakeBuildController(nil, nil, nil, nil, nil)
	defer bc.stop()
	build := dockerStrategy(mockBuild(buildapi.BuildPhaseNew, buildapi.BuildOutput{To: imageStreamRef}))
	build.Spec.Strategy.DockerStrategy.From = &kapi.ObjectReference{Kind: "ImageStreamTag", Name: "isname:latest"}

	update, err := bc.createBuildPod(build)
	if err != nil {
		t.Fatalf("Expected no error: %v", err)
	}
	expected := &buildUpdate{}
	expected.setReason(buildapi.StatusReasonInvalidImageReference)
	expected.setMessage(buildapi.StatusMessageInvalidImageRef)
	validateUpdate(t, "create build pod with image stream error", expected, update)
	if !reflect.DeepEqual(bc.imageStreamQueue.Pop("namespace/isname"), []string{"namespace/data-build"}) {
		t.Errorf("should have queued build update: %#v", bc.imageStreamQueue)
	}
}

func TestCreateBuildPodWithImageStreamUnresolved(t *testing.T) {
	imageStream := &imageapi.ImageStream{}
	imageStream.Namespace = "isnamespace"
	imageStream.Name = "isname"
	imageStream.Status.DockerImageRepository = ""
	imageClient := fakeImageClient(imageStream)
	imageStreamRef := &kapi.ObjectReference{Name: "isname:latest", Namespace: "isnamespace", Kind: "ImageStreamTag"}
	bc := newFakeBuildController(nil, nil, imageClient, nil, nil)
	defer bc.stop()
	build := dockerStrategy(mockBuild(buildapi.BuildPhaseNew, buildapi.BuildOutput{To: imageStreamRef}))

	update, err := bc.createBuildPod(build)

	if err == nil {
		t.Fatalf("Expected error")
	}
	expected := &buildUpdate{}
	expected.setReason(buildapi.StatusReasonInvalidOutputReference)
	expected.setMessage(buildapi.StatusMessageInvalidOutputRef)
	validateUpdate(t, "create build pod with image stream error", expected, update)
	if !reflect.DeepEqual(bc.imageStreamQueue.Pop("isnamespace/isname"), []string{"namespace/data-build"}) {
		t.Errorf("should have queued build update")
	}
}

type errorStrategy struct{}

func (*errorStrategy) CreateBuildPod(build *buildapi.Build) (*v1.Pod, error) {
	return nil, fmt.Errorf("error")
}

func TestCreateBuildPodWithPodSpecCreationError(t *testing.T) {
	bc := newFakeBuildController(nil, nil, nil, nil, nil)
	defer bc.stop()
	bc.createStrategy = &errorStrategy{}
	build := dockerStrategy(mockBuild(buildapi.BuildPhaseNew, buildapi.BuildOutput{}))

	update, err := bc.createBuildPod(build)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	expected := &buildUpdate{}
	expected.setReason(buildapi.StatusReasonCannotCreateBuildPodSpec)
	expected.setMessage(buildapi.StatusMessageCannotCreateBuildPodSpec)
	validateUpdate(t, "create build pod with pod spec creation error", expected, update)
}

func TestCreateBuildPodWithNewerExistingPod(t *testing.T) {
	now := metav1.Now()
	build := dockerStrategy(mockBuild(buildapi.BuildPhaseNew, buildapi.BuildOutput{}))
	build.Status.StartTimestamp = &now

	existingPod := &v1.Pod{}
	existingPod.Name = buildapi.GetBuildPodName(build)
	existingPod.Namespace = build.Namespace
	existingPod.CreationTimestamp = metav1.NewTime(now.Time.Add(time.Hour))

	kubeClient := fakeKubeExternalClientSet(existingPod)
	errorReaction := func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.NewAlreadyExists(schema.GroupResource{Group: "", Resource: "pods"}, existingPod.Name)
	}
	kubeClient.(*kexternalclientfake.Clientset).PrependReactor("create", "pods", errorReaction)
	bc := newFakeBuildController(nil, nil, nil, kubeClient, nil)
	bc.start()
	defer bc.stop()

	update, err := bc.createBuildPod(build)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if update != nil {
		t.Errorf("unexpected update: %v", update)
	}
}

func TestCreateBuildPodWithOlderExistingPod(t *testing.T) {
	now := metav1.Now()
	build := dockerStrategy(mockBuild(buildapi.BuildPhaseNew, buildapi.BuildOutput{}))
	build.CreationTimestamp = now

	existingPod := &v1.Pod{}
	existingPod.Name = buildapi.GetBuildPodName(build)
	existingPod.Namespace = build.Namespace
	existingPod.CreationTimestamp = metav1.NewTime(now.Time.Add(-1 * time.Hour))

	kubeClient := fakeKubeExternalClientSet(existingPod)
	errorReaction := func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.NewAlreadyExists(schema.GroupResource{Group: "", Resource: "pods"}, existingPod.Name)
	}
	kubeClient.(*kexternalclientfake.Clientset).PrependReactor("create", "pods", errorReaction)
	bc := newFakeBuildController(nil, nil, nil, kubeClient, nil)
	defer bc.stop()

	update, err := bc.createBuildPod(build)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	expected := &buildUpdate{}
	expected.setPhase(buildapi.BuildPhaseError)
	expected.setReason(buildapi.StatusReasonBuildPodExists)
	expected.setMessage(buildapi.StatusMessageBuildPodExists)
	validateUpdate(t, "create build pod with pod with older existing pod", expected, update)
}

func TestCreateBuildPodWithPodCreationError(t *testing.T) {
	kubeClient := fakeKubeExternalClientSet()
	errorReaction := func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, nil, fmt.Errorf("error")
	}
	kubeClient.(*kexternalclientfake.Clientset).PrependReactor("create", "pods", errorReaction)
	bc := newFakeBuildController(nil, nil, nil, kubeClient, nil)
	defer bc.stop()
	build := dockerStrategy(mockBuild(buildapi.BuildPhaseNew, buildapi.BuildOutput{}))

	update, err := bc.createBuildPod(build)

	if err == nil {
		t.Errorf("expected error")
	}

	expected := &buildUpdate{}
	expected.setReason(buildapi.StatusReasonCannotCreateBuildPod)
	expected.setMessage(buildapi.StatusMessageCannotCreateBuildPod)
	validateUpdate(t, "create build pod with pod creation error", expected, update)
}

func TestCancelBuild(t *testing.T) {
	build := mockBuild(buildapi.BuildPhaseRunning, buildapi.BuildOutput{})
	build.Name = "canceltest"
	build.Namespace = "testns"
	pod := &v1.Pod{}
	pod.Name = "canceltest-build"
	pod.Namespace = "testns"
	client := kexternalclientfake.NewSimpleClientset(pod).Core()
	bc := BuildController{
		podClient: client,
	}
	update, err := bc.cancelBuild(build)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if _, err := client.Pods("testns").Get("canceltest-build", metav1.GetOptions{}); err == nil {
		t.Errorf("expect pod canceltest-build to have been deleted")
	}
	if update.phase == nil || *update.phase != buildapi.BuildPhaseCancelled {
		t.Errorf("expected phase to be set to cancelled")
	}
	if update.reason == nil || *update.reason != buildapi.StatusReasonCancelledBuild {
		t.Errorf("expected status reason to be set to %s", buildapi.StatusReasonCancelledBuild)
	}
	if update.message == nil || *update.message != buildapi.StatusMessageCancelledBuild {
		t.Errorf("expected status message to be set to %s", buildapi.StatusMessageCancelledBuild)
	}
}

func TestShouldIgnore(t *testing.T) {

	setCompletionTimestamp := func(build *buildapi.Build) *buildapi.Build {
		now := metav1.Now()
		build.Status.CompletionTimestamp = &now
		return build
	}

	tests := []struct {
		name         string
		build        *buildapi.Build
		expectIgnore bool
	}{
		{
			name:         "new docker build",
			build:        dockerStrategy(mockBuild(buildapi.BuildPhaseNew, buildapi.BuildOutput{})),
			expectIgnore: false,
		},
		{
			name:         "running docker build",
			build:        dockerStrategy(mockBuild(buildapi.BuildPhaseRunning, buildapi.BuildOutput{})),
			expectIgnore: false,
		},
		{
			name:         "cancelled docker build",
			build:        dockerStrategy(mockBuild(buildapi.BuildPhaseCancelled, buildapi.BuildOutput{})),
			expectIgnore: true,
		},
		{
			name:         "completed docker build with no completion timestamp",
			build:        dockerStrategy(mockBuild(buildapi.BuildPhaseComplete, buildapi.BuildOutput{})),
			expectIgnore: false,
		},
		{
			name:         "completed docker build with completion timestamp",
			build:        setCompletionTimestamp(dockerStrategy(mockBuild(buildapi.BuildPhaseComplete, buildapi.BuildOutput{}))),
			expectIgnore: true,
		},
		{
			name:         "running pipeline build",
			build:        pipelineStrategy(mockBuild(buildapi.BuildPhaseRunning, buildapi.BuildOutput{})),
			expectIgnore: true,
		},
	}

	for _, test := range tests {
		actual := shouldIgnore(test.build)
		if expected := test.expectIgnore; actual != expected {
			t.Errorf("%s: expected result: %v, actual: %v", test.name, expected, actual)
		}
	}

}

func TestIsValidTransition(t *testing.T) {
	phases := []buildapi.BuildPhase{
		buildapi.BuildPhaseNew,
		buildapi.BuildPhasePending,
		buildapi.BuildPhaseRunning,
		buildapi.BuildPhaseComplete,
		buildapi.BuildPhaseFailed,
		buildapi.BuildPhaseError,
		buildapi.BuildPhaseCancelled,
	}
	for _, fromPhase := range phases {
		for _, toPhase := range phases {
			if buildutil.IsTerminalPhase(fromPhase) && fromPhase != toPhase {
				if isValidTransition(fromPhase, toPhase) {
					t.Errorf("transition %v -> %v should be invalid", fromPhase, toPhase)
				}
				continue
			}
			if fromPhase == buildapi.BuildPhasePending && toPhase == buildapi.BuildPhaseNew {
				if isValidTransition(fromPhase, toPhase) {
					t.Errorf("transition %v -> %v should be invalid", fromPhase, toPhase)
				}
				continue
			}
			if fromPhase == buildapi.BuildPhaseRunning && (toPhase == buildapi.BuildPhaseNew || toPhase == buildapi.BuildPhasePending) {
				if isValidTransition(fromPhase, toPhase) {
					t.Errorf("transition %v -> %v shluld be invalid", fromPhase, toPhase)
				}
				continue
			}

			if !isValidTransition(fromPhase, toPhase) {
				t.Errorf("transition %v -> %v should be valid", fromPhase, toPhase)
			}
		}
	}
}

func TestIsTerminal(t *testing.T) {
	tests := map[buildapi.BuildPhase]bool{
		buildapi.BuildPhaseNew:       false,
		buildapi.BuildPhasePending:   false,
		buildapi.BuildPhaseRunning:   false,
		buildapi.BuildPhaseComplete:  true,
		buildapi.BuildPhaseFailed:    true,
		buildapi.BuildPhaseError:     true,
		buildapi.BuildPhaseCancelled: true,
	}
	for phase, expected := range tests {
		if actual := buildutil.IsTerminalPhase(phase); actual != expected {
			t.Errorf("unexpected response for %s: %v", phase, actual)
		}
	}
}

func TestSetBuildCompletionTimestampAndDuration(t *testing.T) {
	// set start time to 2 seconds ago to have some significant duration
	startTime := metav1.NewTime(time.Now().Add(time.Second * -2))
	earlierTime := metav1.NewTime(startTime.Add(time.Hour * -1))

	// Marker times used for validation
	afterStartTimeBeforeNow := metav1.NewTime(time.Time{})

	// Marker durations used for validation
	greaterThanZeroLessThanSinceStartTime := time.Duration(0)
	atLeastOneHour := time.Duration(0)
	zeroDuration := time.Duration(0)

	buildWithStartTime := &buildapi.Build{}
	buildWithStartTime.Status.StartTimestamp = &startTime
	buildWithNoStartTime := &buildapi.Build{}
	tests := []struct {
		name         string
		build        *buildapi.Build
		podStartTime *metav1.Time
		expected     *buildUpdate
	}{
		{
			name:         "build with start time",
			build:        buildWithStartTime,
			podStartTime: &earlierTime,
			expected: &buildUpdate{
				completionTime: &afterStartTimeBeforeNow,
				duration:       &greaterThanZeroLessThanSinceStartTime,
			},
		},
		{
			name:         "build with no start time",
			build:        buildWithNoStartTime,
			podStartTime: &earlierTime,
			expected: &buildUpdate{
				startTime:      &earlierTime,
				completionTime: &afterStartTimeBeforeNow,
				duration:       &atLeastOneHour,
			},
		},
		{
			name:         "build with no start time, no pod start time",
			build:        buildWithNoStartTime,
			podStartTime: nil,
			expected: &buildUpdate{
				startTime:      &afterStartTimeBeforeNow,
				completionTime: &afterStartTimeBeforeNow,
				duration:       &zeroDuration,
			},
		},
	}

	for _, test := range tests {
		update := &buildUpdate{}
		pod := &v1.Pod{}
		pod.Status.StartTime = test.podStartTime
		setBuildCompletionData(test.build, pod, update)
		// Ensure that only the fields in the expected update are set
		if test.expected.podNameAnnotation == nil && (test.expected.podNameAnnotation != update.podNameAnnotation) {
			t.Errorf("%s: podNameAnnotation should not be set", test.name)
			continue
		}
		if test.expected.phase == nil && (test.expected.phase != update.phase) {
			t.Errorf("%s: phase should not be set", test.name)
			continue
		}
		if test.expected.reason == nil && (test.expected.reason != update.reason) {
			t.Errorf("%s: reason should not be set", test.name)
			continue
		}
		if test.expected.message == nil && (test.expected.message != update.message) {
			t.Errorf("%s: message should not be set", test.name)
			continue
		}
		if test.expected.startTime == nil && (test.expected.startTime != update.startTime) {
			t.Errorf("%s: startTime should not be set", test.name)
			continue
		}
		if test.expected.completionTime == nil && (test.expected.completionTime != update.completionTime) {
			t.Errorf("%s: completionTime should not be set", test.name)
			continue
		}
		if test.expected.duration == nil && (test.expected.duration != update.duration) {
			t.Errorf("%s: duration should not be set", test.name)
			continue
		}
		if test.expected.outputRef == nil && (test.expected.outputRef != update.outputRef) {
			t.Errorf("%s: outputRef should not be set", test.name)
			continue
		}
		now := metav1.NewTime(time.Now().Add(2 * time.Second))
		if test.expected.startTime != nil {
			if update.startTime == nil {
				t.Errorf("%s: expected startTime to be set", test.name)
				continue
			}
			switch test.expected.startTime {
			case &afterStartTimeBeforeNow:
				if !update.startTime.Time.After(startTime.Time) && !update.startTime.Time.Before(now.Time) {
					t.Errorf("%s: startTime (%v) not within expected range (%v - %v)", test.name, update.startTime, startTime, now)
					continue
				}
			default:
				if !update.startTime.Time.Equal(test.expected.startTime.Time) {
					t.Errorf("%s: startTime (%v) not equal expected time (%v)", test.name, update.startTime, test.expected.startTime)
					continue
				}
			}
		}
		if test.expected.completionTime != nil {
			if update.completionTime == nil {
				t.Errorf("%s: expected completionTime to be set", test.name)
				continue
			}
			switch test.expected.completionTime {
			case &afterStartTimeBeforeNow:
				if !update.completionTime.Time.After(startTime.Time) && !update.completionTime.Time.Before(now.Time) {
					t.Errorf("%s: completionTime (%v) not within expected range (%v - %v)", test.name, update.completionTime, startTime, now)
					continue
				}
			default:
				if !update.completionTime.Time.Equal(test.expected.completionTime.Time) {
					t.Errorf("%s: completionTime (%v) not equal expected time (%v)", test.name, update.completionTime, test.expected.completionTime)
					continue
				}
			}
		}
		if test.expected.duration != nil {
			if update.duration == nil {
				t.Errorf("%s: expected duration to be set", test.name)
				continue
			}
			switch test.expected.duration {
			case &greaterThanZeroLessThanSinceStartTime:
				sinceStart := now.Rfc3339Copy().Time.Sub(startTime.Rfc3339Copy().Time)
				if !(*update.duration > 0) || !(*update.duration <= sinceStart) {
					t.Errorf("%s: duration (%v) not within expected range (%v - %v)", test.name, update.duration, 0, sinceStart)
					continue
				}
			case &atLeastOneHour:
				if !(*update.duration >= time.Hour) {
					t.Errorf("%s: duration (%v) is not at least one hour", test.name, update.duration)
					continue
				}
			default:
				if *update.duration != *test.expected.duration {
					t.Errorf("%s: duration (%v) not equal expected duration (%v)", test.name, update.duration, test.expected.duration)
					continue
				}
			}
		}
	}
}

func mockBuild(phase buildapi.BuildPhase, output buildapi.BuildOutput) *buildapi.Build {
	return &buildapi.Build{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "data-build",
			Namespace: "namespace",
			Annotations: map[string]string{
				buildapi.BuildConfigAnnotation: "test-bc",
			},
			Labels: map[string]string{
				"name": "dataBuild",
				buildapi.BuildRunPolicyLabel: string(buildapi.BuildRunPolicyParallel),
				buildapi.BuildConfigLabel:    "test-bc",
			},
		},
		Spec: buildapi.BuildSpec{
			CommonSpec: buildapi.CommonSpec{
				Source: buildapi.BuildSource{
					Git: &buildapi.GitBuildSource{
						URI: "http://my.build.com/the/build/Dockerfile",
					},
					ContextDir: "contextimage",
				},
				Output: output,
			},
		},
		Status: buildapi.BuildStatus{
			Phase: phase,
		},
	}
}

func dockerStrategy(build *buildapi.Build) *buildapi.Build {
	build.Spec.Strategy = buildapi.BuildStrategy{
		DockerStrategy: &buildapi.DockerBuildStrategy{},
	}
	return build
}

func pipelineStrategy(build *buildapi.Build) *buildapi.Build {
	build.Spec.Strategy = buildapi.BuildStrategy{
		JenkinsPipelineStrategy: &buildapi.JenkinsPipelineBuildStrategy{},
	}
	return build
}

func fakeOpenshiftClient(objects ...runtime.Object) client.Interface {
	return testclient.NewSimpleFake(objects...)
}

func fakeImageClient(objects ...runtime.Object) imageinternalclientset.Interface {
	return imageinternalfakeclient.NewSimpleClientset(objects...)
}

func fakeBuildClient(objects ...runtime.Object) buildinternalclientset.Interface {
	return buildinternalfakeclient.NewSimpleClientset(objects...)
}

func fakeKubeExternalClientSet(objects ...runtime.Object) kexternalclientset.Interface {
	return kexternalclientfake.NewSimpleClientset(objects...)
}

func fakeKubeInternalClientSet(objects ...runtime.Object) kinternalclientset.Interface {
	return kinternalclientfake.NewSimpleClientset(objects...)
}

func fakeKubeExternalInformers(clientSet kexternalclientset.Interface) kexternalinformers.SharedInformerFactory {
	return kexternalinformers.NewSharedInformerFactory(clientSet, 0)
}

type fakeBuildController struct {
	*BuildController
	kubeExternalInformers kexternalinformers.SharedInformerFactory
	buildInformers        buildinformersinternal.SharedInformerFactory
	imageInformers        imageinformersinternal.SharedInformerFactory
	stopChan              chan struct{}
}

func (c *fakeBuildController) start() {
	c.kubeExternalInformers.Start(c.stopChan)
	c.imageInformers.Start(c.stopChan)
	c.buildInformers.Start(c.stopChan)
	if !cache.WaitForCacheSync(wait.NeverStop, c.buildStoreSynced, c.podStoreSynced, c.secretStoreSynced, c.imageStreamStoreSynced) {
		panic("cannot sync cache")
	}
}

func (c *fakeBuildController) stop() {
	close(c.stopChan)
}

func newFakeBuildController(openshiftClient client.Interface, buildClient buildinternalclientset.Interface, imageClient imageinternalclientset.Interface, kubeExternalClient kexternalclientset.Interface, kubeInternalClient kinternalclientset.Interface) *fakeBuildController {
	if openshiftClient == nil {
		openshiftClient = fakeOpenshiftClient()
	}
	if buildClient == nil {
		buildClient = fakeBuildClient()
	}
	if imageClient == nil {
		imageClient = fakeImageClient()
	}
	if kubeExternalClient == nil {
		kubeExternalClient = fakeKubeExternalClientSet()
	}
	if kubeInternalClient == nil {
		builderSA := kapi.ServiceAccount{}
		builderSA.Name = "builder"
		builderSA.Namespace = "namespace"
		kubeInternalClient = fakeKubeInternalClientSet(&builderSA)
	}

	kubeExternalInformers := fakeKubeExternalInformers(kubeExternalClient)
	buildInformers := buildinformersinternal.NewSharedInformerFactory(buildClient, 0)
	imageInformers := imageinformersinternal.NewSharedInformerFactory(imageClient, 0)
	stopChan := make(chan struct{})

	params := &BuildControllerParams{
		BuildInformer:       buildInformers.Build().InternalVersion().Builds(),
		BuildConfigInformer: buildInformers.Build().InternalVersion().BuildConfigs(),
		ImageStreamInformer: imageInformers.Image().InternalVersion().ImageStreams(),
		PodInformer:         kubeExternalInformers.Core().V1().Pods(),
		SecretInformer:      kubeExternalInformers.Core().V1().Secrets(),
		KubeClientExternal:  kubeExternalClient,
		KubeClientInternal:  kubeInternalClient,
		OpenshiftClient:     openshiftClient,
		DockerBuildStrategy: &strategy.DockerBuildStrategy{
			Image: "test/image:latest",
			Codec: kapi.Codecs.LegacyCodec(buildapi.LegacySchemeGroupVersion),
		},
		SourceBuildStrategy: &strategy.SourceBuildStrategy{
			Image: "test/image:latest",
			Codec: kapi.Codecs.LegacyCodec(buildapi.LegacySchemeGroupVersion),
		},
		CustomBuildStrategy: &strategy.CustomBuildStrategy{
			Codec: kapi.Codecs.LegacyCodec(buildapi.LegacySchemeGroupVersion),
		},
		BuildDefaults:  builddefaults.BuildDefaults{},
		BuildOverrides: buildoverrides.BuildOverrides{},
	}
	bc := &fakeBuildController{
		BuildController:       NewBuildController(params),
		stopChan:              stopChan,
		kubeExternalInformers: kubeExternalInformers,
		buildInformers:        buildInformers,
		imageInformers:        imageInformers,
	}
	bc.BuildController.recorder = &record.FakeRecorder{}
	bc.start()
	return bc
}

func validateUpdate(t *testing.T, name string, expected, actual *buildUpdate) {
	if expected.podNameAnnotation == nil {
		if actual.podNameAnnotation != nil {
			t.Errorf("%s: podNameAnnotation should be nil. Actual: %s", name, *actual.podNameAnnotation)
		}
	} else {
		if actual.podNameAnnotation == nil {
			t.Errorf("%s: podNameAnnotation should not be nil.", name)
		} else {
			if *expected.podNameAnnotation != *actual.podNameAnnotation {
				t.Errorf("%s: unexpected value for podNameAnnotation. Expected: %s. Actual: %s", name, *expected.podNameAnnotation, *actual.podNameAnnotation)
			}
		}
	}
	if expected.phase == nil {
		if actual.phase != nil {
			t.Errorf("%s: phase should be nil. Actual: %s", name, *actual.phase)
		}
	} else {
		if actual.phase == nil {
			t.Errorf("%s: phase should not be nil.", name)
		} else {
			if *expected.phase != *actual.phase {
				t.Errorf("%s: unexpected value for phase. Expected: %s. Actual: %s", name, *expected.phase, *actual.phase)
			}
		}
	}
	if expected.reason == nil {
		if actual.reason != nil {
			t.Errorf("%s: reason should be nil. Actual: %s", name, *actual.reason)
		}
	} else {
		if actual.reason == nil {
			t.Errorf("%s: reason should not be nil.", name)
		} else {
			if *expected.reason != *actual.reason {
				t.Errorf("%s: unexpected value for reason. Expected: %s. Actual: %s", name, *expected.reason, *actual.reason)
			}
		}
	}
	if expected.message == nil {
		if actual.message != nil {
			t.Errorf("%s: message should be nil. Actual: %s", name, *actual.message)
		}
	} else {
		if actual.message == nil {
			t.Errorf("%s: message should not be nil.", name)
		} else {
			if *expected.message != *actual.message {
				t.Errorf("%s: unexpected value for message. Expected: %s. Actual: %s", name, *expected.message, *actual.message)
			}
		}
	}
	if expected.startTime == nil {
		if actual.startTime != nil {
			t.Errorf("%s: startTime should be nil. Actual: %s", name, *actual.startTime)
		}
	} else {
		if actual.startTime == nil {
			t.Errorf("%s: startTime should not be nil.", name)
		} else {
			if !(*expected.startTime).Equal(*actual.startTime) {
				t.Errorf("%s: unexpected value for startTime. Expected: %s. Actual: %s", name, *expected.startTime, *actual.startTime)
			}
		}
	}
	if expected.completionTime == nil {
		if actual.completionTime != nil {
			t.Errorf("%s: completionTime should be nil. Actual: %s", name, *actual.completionTime)
		}
	} else {
		if actual.completionTime == nil {
			t.Errorf("%s: completionTime should not be nil.", name)
		} else {
			if !(*expected.completionTime).Equal(*actual.completionTime) {
				t.Errorf("%s: unexpected value for completionTime. Expected: %v. Actual: %v", name, *expected.completionTime, *actual.completionTime)
			}
		}
	}
	if expected.duration == nil {
		if actual.duration != nil {
			t.Errorf("%s: duration should be nil. Actual: %s", name, *actual.duration)
		}
	} else {
		if actual.duration == nil {
			t.Errorf("%s: duration should not be nil.", name)
		} else {
			if *expected.duration != *actual.duration {
				t.Errorf("%s: unexpected value for duration. Expected: %v. Actual: %v", name, *expected.duration, *actual.duration)
			}
		}
	}
	if expected.outputRef == nil {
		if actual.outputRef != nil {
			t.Errorf("%s: outputRef should be nil. Actual: %s", name, *actual.outputRef)
		}
	} else {
		if actual.outputRef == nil {
			t.Errorf("%s: outputRef should not be nil.", name)
		} else {
			if *expected.outputRef != *actual.outputRef {
				t.Errorf("%s: unexpected value for outputRef. Expected: %s. Actual: %s", name, *expected.outputRef, *actual.outputRef)
			}
		}
	}
}

func applyBuildPatchReaction(t *testing.T, build *buildapi.Build, buildPtr **buildapi.Build) func(action clientgotesting.Action) (bool, runtime.Object, error) {
	return func(action clientgotesting.Action) (bool, runtime.Object, error) {
		patchAction := action.(clientgotesting.PatchActionImpl)
		var err error
		(*buildPtr), err = validation.ApplyBuildPatch(build, patchAction.Patch)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
			return true, nil, nil
		}
		return true, *buildPtr, nil
	}
}

type updateBuilder struct {
	update *buildUpdate
}

func newUpdate() *updateBuilder {
	return &updateBuilder{update: &buildUpdate{}}
}

func (b *updateBuilder) phase(phase buildapi.BuildPhase) *updateBuilder {
	b.update.setPhase(phase)
	return b
}

func (b *updateBuilder) reason(reason buildapi.StatusReason) *updateBuilder {
	b.update.setReason(reason)
	return b
}

func (b *updateBuilder) message(message string) *updateBuilder {
	b.update.setMessage(message)
	return b
}

func (b *updateBuilder) startTime(startTime metav1.Time) *updateBuilder {
	b.update.setStartTime(startTime)
	return b
}

func (b *updateBuilder) completionTime(completionTime metav1.Time) *updateBuilder {
	b.update.setCompletionTime(completionTime)
	return b
}

func (b *updateBuilder) duration(duration time.Duration) *updateBuilder {
	b.update.setDuration(duration)
	return b
}

func (b *updateBuilder) outputRef(ref string) *updateBuilder {
	b.update.setOutputRef(ref)
	return b
}

func (b *updateBuilder) podNameAnnotation(podName string) *updateBuilder {
	b.update.setPodNameAnnotation(podName)
	return b
}

func (b *updateBuilder) logSnippet(message string) *updateBuilder {
	b.update.setLogSnippet(message)
	return b
}

type fakeRunPolicy struct {
	notRunnable      bool
	onCompleteCalled bool
}

func (f *fakeRunPolicy) IsRunnable(*buildapi.Build) (bool, error) {
	return !f.notRunnable, nil
}

func (f *fakeRunPolicy) OnComplete(*buildapi.Build) error {
	f.onCompleteCalled = true
	return nil
}

func (f *fakeRunPolicy) Handles(buildapi.BuildRunPolicy) bool {
	return true
}

func mockBuildPod(build *buildapi.Build) *v1.Pod {
	pod := &v1.Pod{}
	pod.Name = buildapi.GetBuildPodName(build)
	pod.Namespace = build.Namespace
	pod.Annotations = map[string]string{}
	pod.Annotations[buildapi.BuildAnnotation] = build.Name
	return pod
}
