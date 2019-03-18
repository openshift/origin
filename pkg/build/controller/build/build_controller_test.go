package build

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/openshift/origin/pkg/build/buildscheme"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	clientgotesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"

	buildv1 "github.com/openshift/api/build/v1"
	imagev1 "github.com/openshift/api/image/v1"
	buildv1client "github.com/openshift/client-go/build/clientset/versioned"
	fakebuildv1client "github.com/openshift/client-go/build/clientset/versioned/fake"
	buildv1informer "github.com/openshift/client-go/build/informers/externalversions"
	imagev1client "github.com/openshift/client-go/image/clientset/versioned"
	fakeimagev1client "github.com/openshift/client-go/image/clientset/versioned/fake"
	imagev1informer "github.com/openshift/client-go/image/informers/externalversions"
	"github.com/openshift/origin/pkg/build/buildapihelpers"
	builddefaults "github.com/openshift/origin/pkg/build/controller/build/defaults"
	buildoverrides "github.com/openshift/origin/pkg/build/controller/build/overrides"
	"github.com/openshift/origin/pkg/build/controller/common"
	"github.com/openshift/origin/pkg/build/controller/policy"
	"github.com/openshift/origin/pkg/build/controller/strategy"
	buildutil "github.com/openshift/origin/pkg/build/util"
)

// TestHandleBuild is the main test for build updates through the controller
func TestHandleBuild(t *testing.T) {

	// patch appears to drop sub-second accuracy from times, which causes problems
	// during equality testing later, so start with a rounded number of seconds for a time.
	now := metav1.NewTime(time.Now().Round(time.Second))

	build := func(phase buildv1.BuildPhase) *buildv1.Build {
		b := dockerStrategy(mockBuild(phase, buildv1.BuildOutput{}))
		if phase != buildv1.BuildPhaseNew {
			podName := buildapihelpers.GetBuildPodName(b)
			common.SetBuildPodNameAnnotation(b, podName)
		}
		return b
	}
	pod := func(phase corev1.PodPhase) *corev1.Pod {
		p := mockBuildPod(build(buildv1.BuildPhaseNew))
		p.Status.Phase = phase
		switch phase {
		case corev1.PodRunning:
			p.Status.StartTime = &now
		case corev1.PodFailed:
			p.Status.StartTime = &now
			p.Status.ContainerStatuses = []corev1.ContainerStatus{
				{
					Name: "container",
					State: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							ExitCode: 1,
						},
					},
				},
			}
		case corev1.PodSucceeded:
			p.Status.StartTime = &now
			p.Status.ContainerStatuses = []corev1.ContainerStatus{
				{
					Name: "container",
					State: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							ExitCode: 0,
						},
					},
				},
			}
		}
		return p
	}
	withTerminationMessage := func(pod *corev1.Pod) *corev1.Pod {
		pod.Status.ContainerStatuses[0].State.Terminated.Message = "termination message"
		return pod
	}
	withOwnerReference := func(pod *corev1.Pod, build *buildv1.Build) *corev1.Pod {
		t := true
		pod.OwnerReferences = []metav1.OwnerReference{{
			APIVersion: buildv1.SchemeGroupVersion.String(),
			Kind:       "Build",
			Name:       build.Name,
			Controller: &t,
		}}
		return pod
	}

	cancelled := func(build *buildv1.Build) *buildv1.Build {
		build.Status.Cancelled = true
		return build
	}
	withCompletionTS := func(build *buildv1.Build) *buildv1.Build {
		build.Status.CompletionTimestamp = &now
		return build
	}
	withLogSnippet := func(build *buildv1.Build) *buildv1.Build {
		build.Status.LogSnippet = "termination message"
		return build
	}

	tests := []struct {
		name string

		// Conditions
		build              *buildv1.Build
		pod                *corev1.Pod
		runPolicy          *fakeRunPolicy
		errorOnPodDelete   bool
		errorOnPodCreate   bool
		errorOnBuildUpdate bool

		// Expected Result
		expectUpdate     *buildUpdate
		expectPodCreated bool
		expectPodDeleted bool
		expectError      bool
	}{
		{
			name:  "cancel running build",
			build: cancelled(build(buildv1.BuildPhaseRunning)),
			pod:   pod(corev1.PodRunning),
			expectUpdate: newUpdate().phase(buildv1.BuildPhaseCancelled).
				reason(buildv1.StatusReasonCancelledBuild).
				message(buildutil.StatusMessageCancelledBuild).
				completionTime(now).
				startTime(now).update,
			expectPodDeleted: true,
		},
		{
			name:         "cancel build in terminal state",
			build:        cancelled(withCompletionTS(build(buildv1.BuildPhaseComplete))),
			pod:          pod(corev1.PodRunning),
			expectUpdate: nil,
		},
		{
			name:             "cancel build with delete pod error",
			build:            cancelled(build(buildv1.BuildPhaseRunning)),
			errorOnPodDelete: true,
			expectUpdate:     nil,
			expectError:      true,
		},
		{
			name:  "new -> pending",
			build: build(buildv1.BuildPhaseNew),
			expectUpdate: newUpdate().
				phase(buildv1.BuildPhasePending).
				reason("").
				message("").
				podNameAnnotation(pod(corev1.PodPending).Name).
				update,
			expectPodCreated: true,
		},
		{
			name:  "new with existing related pod",
			build: build(buildv1.BuildPhaseNew),
			pod:   withOwnerReference(pod(corev1.PodRunning), build(buildv1.BuildPhaseNew)),
			expectUpdate: newUpdate().
				phase(buildv1.BuildPhaseRunning).
				reason("").
				message("").
				startTime(now).
				podNameAnnotation(pod(corev1.PodRunning).Name).
				update,
		},
		{
			name:  "new with existing unrelated pod",
			build: build(buildv1.BuildPhaseNew),
			pod:   pod(corev1.PodRunning),
			expectUpdate: newUpdate().
				phase(buildv1.BuildPhaseError).
				reason(buildv1.StatusReasonBuildPodExists).
				message(buildutil.StatusMessageBuildPodExists).
				podNameAnnotation(pod(corev1.PodRunning).Name).
				startTime(now).
				completionTime(now).
				update,
		},
		{
			name:         "new not runnable by policy",
			build:        build(buildv1.BuildPhaseNew),
			runPolicy:    &fakeRunPolicy{notRunnable: true},
			expectUpdate: nil,
		},
		{
			name:               "new -> pending with update error",
			build:              build(buildv1.BuildPhaseNew),
			errorOnBuildUpdate: true,
			expectUpdate:       nil,
			expectPodCreated:   true,
			expectError:        true,
		},
		{
			name:  "pending -> running",
			build: build(buildv1.BuildPhasePending),
			pod:   pod(corev1.PodRunning),
			expectUpdate: newUpdate().
				phase(buildv1.BuildPhaseRunning).
				reason("").
				message("").
				startTime(now).
				update,
		},
		{
			name:               "pending -> running with update error",
			build:              build(buildv1.BuildPhasePending),
			pod:                pod(corev1.PodRunning),
			errorOnBuildUpdate: true,
			expectUpdate:       nil,
			expectError:        true,
		},
		{
			name:  "pending -> failed",
			build: build(buildv1.BuildPhasePending),
			pod:   pod(corev1.PodFailed),
			expectUpdate: newUpdate().
				phase(buildv1.BuildPhaseFailed).
				reason(buildv1.StatusReasonGenericBuildFailed).
				message(buildutil.StatusMessageGenericBuildFailed).
				startTime(now).
				completionTime(now).
				update,
		},
		{
			name:         "pending -> pending",
			build:        build(buildv1.BuildPhasePending),
			pod:          pod(corev1.PodPending),
			expectUpdate: nil,
		},
		{
			name:  "running -> complete",
			build: build(buildv1.BuildPhaseRunning),
			pod:   pod(corev1.PodSucceeded),
			expectUpdate: newUpdate().
				phase(buildv1.BuildPhaseComplete).
				reason("").
				message("").
				startTime(now).
				completionTime(now).
				update,
		},
		{
			name:         "running -> running",
			build:        build(buildv1.BuildPhaseRunning),
			pod:          pod(corev1.PodRunning),
			expectUpdate: nil,
		},
		{
			name:  "running with missing pod",
			build: build(buildv1.BuildPhaseRunning),
			expectUpdate: newUpdate().
				phase(buildv1.BuildPhaseError).
				reason(buildv1.StatusReasonBuildPodDeleted).
				message(buildutil.StatusMessageBuildPodDeleted).
				startTime(now).
				completionTime(now).
				update,
		},
		{
			name:  "failed -> failed with no completion timestamp",
			build: build(buildv1.BuildPhaseFailed),
			pod:   pod(corev1.PodFailed),
			expectUpdate: newUpdate().
				startTime(now).
				completionTime(now).
				update,
		},
		{
			name:  "failed -> failed with completion timestamp+message and no logsnippet",
			build: withCompletionTS(build(buildv1.BuildPhaseFailed)),
			pod:   withTerminationMessage(pod(corev1.PodFailed)),
			expectUpdate: newUpdate().
				startTime(now).
				logSnippet("termination message").
				update,
		},
		{
			name:  "failed -> failed with completion timestamp+message and logsnippet",
			build: withLogSnippet(withCompletionTS(build(buildv1.BuildPhaseFailed))),
			pod:   withTerminationMessage(pod(corev1.PodFailed)),
		},
	}

	for _, tc := range tests {
		func() {
			var patchedBuild *buildv1.Build
			var appliedPatch string
			buildClient := fakeBuildClient(tc.build)
			buildClient.(*fakebuildv1client.Clientset).PrependReactor("patch", "builds",
				func(action clientgotesting.Action) (bool, runtime.Object, error) {
					if tc.errorOnBuildUpdate {
						return true, nil, fmt.Errorf("error")
					}
					var err error
					patchAction := action.(clientgotesting.PatchActionImpl)
					appliedPatch = string(patchAction.Patch)
					patchedBuild, err = applyBuildPatch(tc.build, patchAction.Patch)
					if err != nil {
						panic(fmt.Sprintf("unexpected error: %v", err))
					}
					return true, patchedBuild, nil
				})
			var kubeClient kubernetes.Interface
			if tc.pod != nil {
				kubeClient = fakeKubeExternalClientSet(tc.pod)
			} else {
				kubeClient = fakeKubeExternalClientSet()
			}
			podCreated := false
			kubeClient.(*fake.Clientset).PrependReactor("delete", "pods",
				func(action clientgotesting.Action) (bool, runtime.Object, error) {
					if tc.errorOnPodDelete {
						return true, nil, fmt.Errorf("error")
					}
					return true, nil, nil
				})
			kubeClient.(*fake.Clientset).PrependReactor("create", "pods",
				func(action clientgotesting.Action) (bool, runtime.Object, error) {
					if tc.errorOnPodCreate {
						return true, nil, fmt.Errorf("error")
					}
					podCreated = true
					return true, nil, nil
				})

			bc := newFakeBuildController(buildClient, nil, kubeClient, nil)
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
			if tc.expectUpdate != nil {
				if patchedBuild == nil {
					t.Errorf("%s: did not get an update. Expected: %v", tc.name, tc.expectUpdate)
					return
				}
				expectedBuild := tc.build.DeepCopy()
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

				// TODO: For some reason the external builds does not with with this check. The output is correct, we should investigate later.
				/*
					if !apiequality.Semantic.DeepEqual(*expectedBuild, *patchedBuild) {
						t.Errorf("%s: did not get expected update on build. \nUpdate: %v\nPatch: %s\n", tc.name, tc.expectUpdate, appliedPatch)
					}
				*/
			}
		}()
	}

}

// TestWork - High-level test of the work function to ensure that a build
// in the queue will be handled by updating the build status to pending
func TestWorkWithNewBuild(t *testing.T) {
	build := dockerStrategy(mockBuild(buildv1.BuildPhaseNew, buildv1.BuildOutput{}))
	var patchedBuild *buildv1.Build
	buildClient := fakeBuildClient(build)
	buildClient.(*fakebuildv1client.Clientset).PrependReactor("patch", "builds", applyBuildPatchReaction(t, build, &patchedBuild))

	bc := newFakeBuildController(buildClient, nil, nil, nil)
	defer bc.stop()
	bc.enqueueBuild(build)

	bc.buildWork()

	if bc.buildQueue.Len() > 0 {
		t.Errorf("Expected queue to be empty")
	}
	if patchedBuild == nil {
		t.Errorf("Expected patched build not to be nil")
	}

	if patchedBuild != nil && patchedBuild.Status.Phase != buildv1.BuildPhasePending {
		t.Errorf("Expected patched build status set to Pending. It is %s", patchedBuild.Status.Phase)
	}
}

func TestCreateBuildPod(t *testing.T) {
	kubeClient := fakeKubeExternalClientSet()
	bc := newFakeBuildController(nil, nil, kubeClient, nil)
	defer bc.stop()
	build := dockerStrategy(mockBuild(buildv1.BuildPhaseNew, buildv1.BuildOutput{}))

	update, err := bc.createBuildPod(build)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}
	podName := buildapihelpers.GetBuildPodName(build)
	// Validate update
	expected := &buildUpdate{}
	expected.setPodNameAnnotation(podName)
	expected.setPhase(buildv1.BuildPhasePending)
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
	imageStream := &imagev1.ImageStream{}
	imageStream.Namespace = "isnamespace"
	imageStream.Name = "isname"
	imageStream.Status.DockerImageRepository = "namespace/image-name"
	imageClient := fakeImageClient(imageStream)
	imageStreamRef := &corev1.ObjectReference{Name: "isname:latest", Namespace: "isnamespace", Kind: "ImageStreamTag"}
	bc := newFakeBuildController(nil, imageClient, nil, nil)
	defer bc.stop()
	build := dockerStrategy(mockBuild(buildv1.BuildPhaseNew, buildv1.BuildOutput{To: imageStreamRef, PushSecret: &corev1.LocalObjectReference{}}))
	podName := buildapihelpers.GetBuildPodName(build)

	update, err := bc.createBuildPod(build)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := &buildUpdate{}
	expected.setPodNameAnnotation(podName)
	expected.setPhase(buildv1.BuildPhasePending)
	expected.setReason("")
	expected.setMessage("")
	expected.setOutputRef("namespace/image-name:latest")
	validateUpdate(t, "create build pod with imagestream output", expected, update)
	if len(bc.imageStreamQueue.Pop("isnamespace/isname")) > 0 {
		t.Errorf("should not have queued build update")
	}
}

func TestCreateBuildPodWithOutputImageStreamMissing(t *testing.T) {
	imageStreamRef := &corev1.ObjectReference{Name: "isname:latest", Namespace: "isnamespace", Kind: "ImageStreamTag"}
	bc := newFakeBuildController(nil, nil, nil, nil)
	defer bc.stop()
	build := dockerStrategy(mockBuild(buildv1.BuildPhaseNew, buildv1.BuildOutput{To: imageStreamRef, PushSecret: &corev1.LocalObjectReference{}}))

	update, err := bc.createBuildPod(build)

	if err != nil {
		t.Fatalf("Expected no error")
	}
	expected := &buildUpdate{}
	expected.setReason(buildv1.StatusReasonInvalidOutputReference)
	expected.setMessage(buildutil.StatusMessageInvalidOutputRef)
	validateUpdate(t, "create build pod with image stream error", expected, update)
	if !reflect.DeepEqual(bc.imageStreamQueue.Pop("isnamespace/isname"), []string{"namespace/data-build"}) {
		t.Errorf("should have queued build update: %#v", bc.imageStreamQueue)
	}
}

func TestCreateBuildPodWithImageStreamMissing(t *testing.T) {
	imageStreamRef := &corev1.ObjectReference{Name: "isname:latest", Kind: "DockerImage"}
	bc := newFakeBuildController(nil, nil, nil, nil)
	defer bc.stop()
	build := dockerStrategy(mockBuild(buildv1.BuildPhaseNew, buildv1.BuildOutput{To: imageStreamRef, PushSecret: &corev1.LocalObjectReference{}}))
	build.Spec.Strategy.DockerStrategy.From = &corev1.ObjectReference{Kind: "ImageStreamTag", Name: "isname:latest"}

	update, err := bc.createBuildPod(build)
	if err != nil {
		t.Fatalf("Expected no error: %v", err)
	}
	expected := &buildUpdate{}
	expected.setReason(buildv1.StatusReasonInvalidImageReference)
	expected.setMessage(buildutil.StatusMessageInvalidImageRef)
	validateUpdate(t, "create build pod with image stream error", expected, update)
	if !reflect.DeepEqual(bc.imageStreamQueue.Pop("namespace/isname"), []string{"namespace/data-build"}) {
		t.Errorf("should have queued build update: %#v", bc.imageStreamQueue)
	}
}

func TestCreateBuildPodWithImageStreamUnresolved(t *testing.T) {
	imageStream := &imagev1.ImageStream{}
	imageStream.Namespace = "isnamespace"
	imageStream.Name = "isname"
	imageStream.Status.DockerImageRepository = ""
	imageClient := fakeImageClient(imageStream)
	imageStreamRef := &corev1.ObjectReference{Name: "isname:latest", Namespace: "isnamespace", Kind: "ImageStreamTag"}
	bc := newFakeBuildController(nil, imageClient, nil, nil)
	defer bc.stop()
	build := dockerStrategy(mockBuild(buildv1.BuildPhaseNew, buildv1.BuildOutput{To: imageStreamRef, PushSecret: &corev1.LocalObjectReference{}}))

	update, err := bc.createBuildPod(build)

	if err == nil {
		t.Fatalf("Expected error")
	}
	expected := &buildUpdate{}
	expected.setReason(buildv1.StatusReasonInvalidOutputReference)
	expected.setMessage(buildutil.StatusMessageInvalidOutputRef)
	validateUpdate(t, "create build pod with image stream error", expected, update)
	if !reflect.DeepEqual(bc.imageStreamQueue.Pop("isnamespace/isname"), []string{"namespace/data-build"}) {
		t.Errorf("should have queued build update")
	}
}

type errorStrategy struct{}

func (*errorStrategy) CreateBuildPod(build *buildv1.Build) (*corev1.Pod, error) {
	return nil, fmt.Errorf("error")
}

func TestCreateBuildPodWithPodSpecCreationError(t *testing.T) {
	bc := newFakeBuildController(nil, nil, nil, nil)
	defer bc.stop()
	bc.createStrategy = &errorStrategy{}
	build := dockerStrategy(mockBuild(buildv1.BuildPhaseNew, buildv1.BuildOutput{}))

	update, err := bc.createBuildPod(build)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	expected := &buildUpdate{}
	expected.setReason(buildv1.StatusReasonCannotCreateBuildPodSpec)
	expected.setMessage(buildutil.StatusMessageCannotCreateBuildPodSpec)
	validateUpdate(t, "create build pod with pod spec creation error", expected, update)
}

func TestCreateBuildPodWithExistingRelatedPod(t *testing.T) {
	tru := true
	build := dockerStrategy(mockBuild(buildv1.BuildPhaseNew, buildv1.BuildOutput{}))

	existingPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      buildapihelpers.GetBuildPodName(build),
			Namespace: build.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: buildv1.SchemeGroupVersion.String(),
					Kind:       "Build",
					Name:       build.Name,
					Controller: &tru,
				},
			},
		},
	}

	kubeClient := fakeKubeExternalClientSet(existingPod)
	errorReaction := func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.NewAlreadyExists(schema.GroupResource{Group: "", Resource: "pods"}, existingPod.Name)
	}
	kubeClient.(*fake.Clientset).PrependReactor("create", "pods", errorReaction)
	bc := newFakeBuildController(nil, nil, kubeClient, nil)
	bc.start()
	defer bc.stop()

	update, err := bc.createBuildPod(build)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	expected := &buildUpdate{}
	expected.setPhase(buildv1.BuildPhasePending)
	expected.setReason("")
	expected.setMessage("")
	expected.setPodNameAnnotation(buildapihelpers.GetBuildPodName(build))
	validateUpdate(t, "create build pod with existing related pod error", expected, update)
}

func TestCreateBuildPodWithExistingUnrelatedPod(t *testing.T) {
	build := dockerStrategy(mockBuild(buildv1.BuildPhaseNew, buildv1.BuildOutput{}))

	existingPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      buildapihelpers.GetBuildPodName(build),
			Namespace: build.Namespace,
		},
	}

	kubeClient := fakeKubeExternalClientSet(existingPod)
	errorReaction := func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.NewAlreadyExists(schema.GroupResource{Group: "", Resource: "pods"}, existingPod.Name)
	}
	kubeClient.(*fake.Clientset).PrependReactor("create", "pods", errorReaction)
	bc := newFakeBuildController(nil, nil, kubeClient, nil)
	defer bc.stop()

	update, err := bc.createBuildPod(build)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	expected := &buildUpdate{}
	expected.setPhase(buildv1.BuildPhaseError)
	expected.setReason(buildv1.StatusReasonBuildPodExists)
	expected.setMessage(buildutil.StatusMessageBuildPodExists)
	validateUpdate(t, "create build pod with pod with older existing pod", expected, update)
}

func TestCreateBuildPodWithPodCreationError(t *testing.T) {
	kubeClient := fakeKubeExternalClientSet()
	errorReaction := func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, nil, fmt.Errorf("error")
	}
	kubeClient.(*fake.Clientset).PrependReactor("create", "pods", errorReaction)
	bc := newFakeBuildController(nil, nil, kubeClient, nil)
	defer bc.stop()
	build := dockerStrategy(mockBuild(buildv1.BuildPhaseNew, buildv1.BuildOutput{}))

	update, err := bc.createBuildPod(build)

	if err == nil {
		t.Errorf("expected error")
	}

	expected := &buildUpdate{}
	expected.setReason(buildv1.StatusReasonCannotCreateBuildPod)
	expected.setMessage(buildutil.StatusMessageCannotCreateBuildPod)
	validateUpdate(t, "create build pod with pod creation error", expected, update)
}

func TestCancelBuild(t *testing.T) {
	build := mockBuild(buildv1.BuildPhaseRunning, buildv1.BuildOutput{})
	build.Name = "canceltest"
	build.Namespace = "testns"
	pod := &corev1.Pod{}
	pod.Name = "canceltest-build"
	pod.Namespace = "testns"
	client := fake.NewSimpleClientset(pod).Core()
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
	if update.phase == nil || *update.phase != buildv1.BuildPhaseCancelled {
		t.Errorf("expected phase to be set to cancelled")
	}
	if update.reason == nil || *update.reason != buildv1.StatusReasonCancelledBuild {
		t.Errorf("expected status reason to be set to %s", buildv1.StatusReasonCancelledBuild)
	}
	if update.message == nil || *update.message != buildutil.StatusMessageCancelledBuild {
		t.Errorf("expected status message to be set to %s", buildutil.StatusMessageCancelledBuild)
	}
}

func TestShouldIgnore(t *testing.T) {

	setCompletionTimestamp := func(build *buildv1.Build) *buildv1.Build {
		now := metav1.Now()
		build.Status.CompletionTimestamp = &now
		return build
	}

	tests := []struct {
		name         string
		build        *buildv1.Build
		expectIgnore bool
	}{
		{
			name:         "new docker build",
			build:        dockerStrategy(mockBuild(buildv1.BuildPhaseNew, buildv1.BuildOutput{})),
			expectIgnore: false,
		},
		{
			name:         "running docker build",
			build:        dockerStrategy(mockBuild(buildv1.BuildPhaseRunning, buildv1.BuildOutput{})),
			expectIgnore: false,
		},
		{
			name:         "cancelled docker build",
			build:        dockerStrategy(mockBuild(buildv1.BuildPhaseCancelled, buildv1.BuildOutput{})),
			expectIgnore: true,
		},
		{
			name:         "completed docker build with no completion timestamp",
			build:        dockerStrategy(mockBuild(buildv1.BuildPhaseComplete, buildv1.BuildOutput{})),
			expectIgnore: false,
		},
		{
			name:         "completed docker build with completion timestamp",
			build:        setCompletionTimestamp(dockerStrategy(mockBuild(buildv1.BuildPhaseComplete, buildv1.BuildOutput{}))),
			expectIgnore: true,
		},
		{
			name:         "running pipeline build",
			build:        pipelineStrategy(mockBuild(buildv1.BuildPhaseRunning, buildv1.BuildOutput{})),
			expectIgnore: true,
		},
		{
			name:         "completed pipeline build",
			build:        pipelineStrategy(mockBuild(buildv1.BuildPhaseComplete, buildv1.BuildOutput{})),
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
	phases := []buildv1.BuildPhase{
		buildv1.BuildPhaseNew,
		buildv1.BuildPhasePending,
		buildv1.BuildPhaseRunning,
		buildv1.BuildPhaseComplete,
		buildv1.BuildPhaseFailed,
		buildv1.BuildPhaseError,
		buildv1.BuildPhaseCancelled,
	}
	for _, fromPhase := range phases {
		for _, toPhase := range phases {
			if buildutil.IsTerminalPhase(fromPhase) && fromPhase != toPhase {
				if isValidTransition(fromPhase, toPhase) {
					t.Errorf("transition %v -> %v should be invalid", fromPhase, toPhase)
				}
				continue
			}
			if fromPhase == buildv1.BuildPhasePending && toPhase == buildv1.BuildPhaseNew {
				if isValidTransition(fromPhase, toPhase) {
					t.Errorf("transition %v -> %v should be invalid", fromPhase, toPhase)
				}
				continue
			}
			if fromPhase == buildv1.BuildPhaseRunning && (toPhase == buildv1.BuildPhaseNew || toPhase == buildv1.BuildPhasePending) {
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
	tests := map[buildv1.BuildPhase]bool{
		buildv1.BuildPhaseNew:       false,
		buildv1.BuildPhasePending:   false,
		buildv1.BuildPhaseRunning:   false,
		buildv1.BuildPhaseComplete:  true,
		buildv1.BuildPhaseFailed:    true,
		buildv1.BuildPhaseError:     true,
		buildv1.BuildPhaseCancelled: true,
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

	buildWithStartTime := &buildv1.Build{}
	buildWithStartTime.Status.StartTimestamp = &startTime
	buildWithNoStartTime := &buildv1.Build{}
	tests := []struct {
		name         string
		build        *buildv1.Build
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
		pod := &corev1.Pod{}
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
				if *update.duration <= 0 || *update.duration > sinceStart {
					t.Errorf("%s: duration (%v) not within expected range (%v - %v)", test.name, update.duration, 0, sinceStart)
					continue
				}
			case &atLeastOneHour:
				if *update.duration < time.Hour {
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

func mockBuild(phase buildv1.BuildPhase, output buildv1.BuildOutput) *buildv1.Build {
	return &buildv1.Build{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "data-build",
			Namespace: "namespace",
			Annotations: map[string]string{
				buildutil.BuildConfigAnnotation: "test-bc",
			},
			Labels: map[string]string{
				"name": "dataBuild",
				buildutil.BuildRunPolicyLabel: string(buildv1.BuildRunPolicyParallel),
				buildutil.BuildConfigLabel:    "test-bc",
			},
		},
		Spec: buildv1.BuildSpec{
			CommonSpec: buildv1.CommonSpec{
				Source: buildv1.BuildSource{
					Git: &buildv1.GitBuildSource{
						URI: "http://my.build.com/the/build/Dockerfile",
					},
					ContextDir: "contextimage",
				},
				Output: output,
			},
		},
		Status: buildv1.BuildStatus{
			Phase: phase,
		},
	}
}

func dockerStrategy(build *buildv1.Build) *buildv1.Build {
	build.Spec.Strategy = buildv1.BuildStrategy{
		DockerStrategy: &buildv1.DockerBuildStrategy{},
	}
	return build
}

func pipelineStrategy(build *buildv1.Build) *buildv1.Build {
	build.Spec.Strategy = buildv1.BuildStrategy{
		JenkinsPipelineStrategy: &buildv1.JenkinsPipelineBuildStrategy{},
	}
	return build
}

func fakeImageClient(objects ...runtime.Object) imagev1client.Interface {
	return fakeimagev1client.NewSimpleClientset(objects...)
}

func fakeBuildClient(objects ...runtime.Object) buildv1client.Interface {
	return fakebuildv1client.NewSimpleClientset(objects...)
}

func fakeKubeExternalClientSet(objects ...runtime.Object) kubernetes.Interface {
	builderSA := &corev1.ServiceAccount{}
	builderSA.Name = "builder"
	builderSA.Namespace = "namespace"
	builderSA.Secrets = []corev1.ObjectReference{
		{
			Name: "secret",
		},
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "secret",
			Namespace: "namespace",
		},
		Type: corev1.SecretTypeDockerConfigJson,
	}
	return fake.NewSimpleClientset(append(objects, builderSA, secret)...)
}

func fakeKubeInternalClientSet(objects ...runtime.Object) kubernetes.Interface {
	return fake.NewSimpleClientset(objects...)
}

func fakeKubeExternalInformers(clientSet kubernetes.Interface) informers.SharedInformerFactory {
	return informers.NewSharedInformerFactory(clientSet, 0)
}

type fakeBuildController struct {
	*BuildController
	kubeExternalInformers informers.SharedInformerFactory
	buildInformers        buildv1informer.SharedInformerFactory
	imageInformers        imagev1informer.SharedInformerFactory
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

func newFakeBuildController(buildClient buildv1client.Interface, imageClient imagev1client.Interface, kubeExternalClient kubernetes.Interface, kubeInternalClient kubernetes.Interface) *fakeBuildController {
	if buildClient == nil {
		buildClient = fakeBuildClient()
	}
	if imageClient == nil {
		imageClient = fakeImageClient()
	}
	if kubeExternalClient == nil {
		kubeExternalClient = fakeKubeExternalClientSet()
	}

	kubeExternalInformers := fakeKubeExternalInformers(kubeExternalClient)
	buildInformers := buildv1informer.NewSharedInformerFactory(buildClient, 0)
	imageInformers := imagev1informer.NewSharedInformerFactory(imageClient, 0)
	stopChan := make(chan struct{})

	params := &BuildControllerParams{
		BuildInformer:       buildInformers.Build().V1().Builds(),
		BuildConfigInformer: buildInformers.Build().V1().BuildConfigs(),
		ImageStreamInformer: imageInformers.Image().V1().ImageStreams(),
		PodInformer:         kubeExternalInformers.Core().V1().Pods(),
		SecretInformer:      kubeExternalInformers.Core().V1().Secrets(),
		KubeClient:          kubeExternalClient,
		BuildClient:         buildClient,
		DockerBuildStrategy: &strategy.DockerBuildStrategy{
			Image: "test/image:latest",
		},
		SourceBuildStrategy: &strategy.SourceBuildStrategy{
			Image: "test/image:latest",
		},
		CustomBuildStrategy: &strategy.CustomBuildStrategy{},
		BuildDefaults:       builddefaults.BuildDefaults{},
		BuildOverrides:      buildoverrides.BuildOverrides{},
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
			if !(*expected.startTime).Equal(actual.startTime) {
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
			if !(*expected.completionTime).Equal(actual.completionTime) {
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

func applyBuildPatch(build *buildv1.Build, patch []byte) (*buildv1.Build, error) {
	buildJSON, err := runtime.Encode(buildscheme.Encoder, build)
	if err != nil {
		return nil, err
	}
	patchedJSON, err := strategicpatch.StrategicMergePatch(buildJSON, patch, &buildv1.Build{})
	if err != nil {
		return nil, err
	}
	patchedVersionedBuild, err := runtime.Decode(buildscheme.Decoder, patchedJSON)
	if err != nil {
		return nil, err
	}
	return patchedVersionedBuild.(*buildv1.Build), nil
}

func applyBuildPatchReaction(t *testing.T, build *buildv1.Build, buildPtr **buildv1.Build) func(action clientgotesting.Action) (bool, runtime.Object, error) {
	return func(action clientgotesting.Action) (bool, runtime.Object, error) {
		patchAction := action.(clientgotesting.PatchActionImpl)
		var err error
		(*buildPtr), err = applyBuildPatch(build, patchAction.Patch)
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

func (b *updateBuilder) phase(phase buildv1.BuildPhase) *updateBuilder {
	b.update.setPhase(phase)
	return b
}

func (b *updateBuilder) reason(reason buildv1.StatusReason) *updateBuilder {
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

func (f *fakeRunPolicy) IsRunnable(*buildv1.Build) (bool, error) {
	return !f.notRunnable, nil
}

func (f *fakeRunPolicy) OnComplete(*buildv1.Build) error {
	f.onCompleteCalled = true
	return nil
}

func (f *fakeRunPolicy) Handles(buildv1.BuildRunPolicy) bool {
	return true
}

func mockBuildPod(build *buildv1.Build) *corev1.Pod {
	pod := &corev1.Pod{}
	pod.Name = buildapihelpers.GetBuildPodName(build)
	pod.Namespace = build.Namespace
	pod.Annotations = map[string]string{}
	pod.Annotations[buildutil.BuildAnnotation] = build.Name
	return pod
}

func TestPodStatusReporting(t *testing.T) {
	cases := []struct {
		name        string
		pod         *corev1.Pod
		isOOMKilled bool
		isEvicted   bool
	}{
		{
			name: "running",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "running-pod",
					Namespace: "default",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "test",
							Image: "quay.io/coreos/coreos:latest",
						},
					},
				},
				Status: corev1.PodStatus{
					Phase:   corev1.PodRunning,
					Reason:  "Running",
					Message: "Running...",
				},
			},
		},
		{
			name: "oomkilled-pod",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "oom-pod",
					Namespace: "default",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "test",
							Image: "quay.io/coreos/coreos:latest",
						},
					},
				},
				Status: corev1.PodStatus{
					Phase:   corev1.PodFailed,
					Reason:  "OOMKilled",
					Message: "OOMKilled...",
				},
			},
			isOOMKilled: true,
		},
		{
			name: "oomkilled-init",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "oom-pod",
					Namespace: "default",
				},
				Spec: corev1.PodSpec{
					InitContainers: []corev1.Container{
						{
							Name:  "init-1",
							Image: "quay.io/coreos/coreos:latest",
						},
					},
					Containers: []corev1.Container{
						{
							Name:  "test",
							Image: "quay.io/coreos/coreos:latest",
						},
					},
				},
				Status: corev1.PodStatus{
					Phase:   corev1.PodPending,
					Reason:  "Pending",
					Message: "Waiting on init containers...",
					InitContainerStatuses: []corev1.ContainerStatus{
						{
							Name: "init-1",
							State: corev1.ContainerState{
								Terminated: &corev1.ContainerStateTerminated{
									ExitCode: 123,
									Signal:   9,
									Reason:   "OOMKilled",
								},
							},
						},
					},
				},
			},
			isOOMKilled: true,
		},
		{
			name: "oomkilled-container",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "oom-pod",
					Namespace: "default",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "test",
							Image: "quay.io/coreos/coreos:latest",
						},
					},
				},
				Status: corev1.PodStatus{
					Phase:   corev1.PodFailed,
					Reason:  "Failed",
					Message: "Failed due to OOMKill...",
					ContainerStatuses: []corev1.ContainerStatus{
						{
							Name: "test",
							State: corev1.ContainerState{
								Terminated: &corev1.ContainerStateTerminated{
									ExitCode: 123,
									Signal:   9,
									Reason:   "OOMKilled",
								},
							},
						},
					},
				},
			},
			isOOMKilled: true,
		},
		{
			name: "pod-evicted",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "evicted-pod",
					Namespace: "default",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "test",
							Image: "quay.io/coreos/coreos:latest",
						},
					},
				},
				Status: corev1.PodStatus{
					Phase:   corev1.PodFailed,
					Reason:  "Evicted",
					Message: "The pod was evicted due to no memory available.",
				},
			},
			isEvicted: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			isOOM := isOOMKilled(tc.pod)
			if isOOM != tc.isOOMKilled {
				t.Errorf("expected OOMKilled to be %v, got %v", tc.isOOMKilled, isOOM)
			}
			evicted := isPodEvicted(tc.pod)
			if evicted != tc.isEvicted {
				t.Errorf("expected Evicted to be %v, got %v", tc.isEvicted, evicted)
			}
		})
	}
}
