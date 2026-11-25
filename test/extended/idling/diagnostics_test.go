package idling

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
	"k8s.io/utils/ptr"

	appsv1 "github.com/openshift/api/apps/v1"
)

const testNamespace = "e2e-test-cli-idling-k7x2q"

// stubDeploymentConfigs serves one canned DeploymentConfig read.
type stubDeploymentConfigs struct {
	dc          *appsv1.DeploymentConfig
	err         error
	requested   string
	hadDeadline bool
}

func (s *stubDeploymentConfigs) Get(ctx context.Context, name string, _ metav1.GetOptions) (*appsv1.DeploymentConfig, error) {
	s.requested = name
	_, s.hadDeadline = ctx.Deadline()
	return s.dc, s.err
}

func notFound(resource, name string) error {
	return kapierrors.NewNotFound(schema.GroupResource{Resource: resource}, name)
}

// failRead makes the fake client return err for the named resource.
func failRead(kube *fake.Clientset, resource string, err error) {
	kube.PrependReactor("get", resource, func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, err
	})
}

func TestIdlingSetupDiagnosticsOnFailureIsFailureOnly(t *testing.T) {
	tests := []struct {
		name            string
		waitErr         error
		expectCollected bool
	}{
		{name: "successful wait collects nothing", waitErr: nil},
		{name: "failed wait collects a snapshot", waitErr: errors.New("timed out waiting for the condition"), expectCollected: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			kube := fake.NewSimpleClientset()
			deploymentConfigs := &stubDeploymentConfigs{}
			built := 0
			newClients := func() idlingDiagnosticsClients {
				built++
				return idlingDiagnosticsClients{kube: kube, namespace: testNamespace, deploymentConfigs: deploymentConfigs}
			}

			summary, collected := idlingSetupDiagnosticsOnFailure(context.Background(), newClients, test.waitErr, "idling-echo", "idling-echo")

			if collected != test.expectCollected {
				t.Fatalf("collected = %t, want %t", collected, test.expectCollected)
			}
			if test.expectCollected {
				if summary == "" {
					t.Error("summary is empty, want a snapshot")
				}
				if !deploymentConfigs.hadDeadline {
					t.Error("the reads ran without the shared deadline")
				}
				return
			}
			if summary != "" {
				t.Errorf("summary = %q, want empty", summary)
			}
			if built != 0 {
				t.Errorf("built clients %d times on the success path, want 0", built)
			}
			if actions := kube.Actions(); len(actions) != 0 {
				t.Errorf("issued %d reads on the success path, want none", len(actions))
			}
			if deploymentConfigs.requested != "" {
				t.Errorf("read DeploymentConfig %q on the success path, want none", deploymentConfigs.requested)
			}
		})
	}
}

func TestCollectIdlingSetupDiagnostics(t *testing.T) {
	failedDC := &appsv1.DeploymentConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "idling-echo", Namespace: testNamespace},
		Spec:       appsv1.DeploymentConfigSpec{Replicas: 2},
		Status: appsv1.DeploymentConfigStatus{
			LatestVersion:       1,
			Replicas:            2,
			UnavailableReplicas: 2,
			Conditions: []appsv1.DeploymentCondition{
				{Type: appsv1.DeploymentAvailable, Status: corev1.ConditionFalse},
				{Type: appsv1.DeploymentProgressing, Status: corev1.ConditionUnknown},
			},
		},
	}
	failedRC := &corev1.ReplicationController{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "idling-echo-1",
			Namespace:   testNamespace,
			Annotations: map[string]string{appsv1.DeploymentStatusAnnotation: string(appsv1.DeploymentStatusFailed)},
		},
		Spec: corev1.ReplicationControllerSpec{Replicas: ptr.To(int32(2))},
	}
	deployerPod := func(state corev1.ContainerState) *corev1.Pod {
		return &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "idling-echo-1-deploy", Namespace: testNamespace},
			Status: corev1.PodStatus{
				Phase:             corev1.PodFailed,
				ContainerStatuses: []corev1.ContainerStatus{{Name: deployerContainerName, State: state}},
			},
		}
	}
	failedDeployer := deployerPod(corev1.ContainerState{
		Terminated: &corev1.ContainerStateTerminated{ExitCode: 1, Reason: "Error"},
	})
	emptyEndpoints := &corev1.Endpoints{ObjectMeta: metav1.ObjectMeta{Name: "idling-echo", Namespace: testNamespace}}

	tests := []struct {
		name                 string
		objects              []runtime.Object
		deploymentConfigs    *stubDeploymentConfigs
		deploymentConfigName string
		failingReads         map[string]error
		// expectSummary pins the whole line, including field order.
		expectSummary  string
		expectContains []string
		expectAbsent   []string
	}{
		{
			name:                 "full rollout snapshot",
			objects:              []runtime.Object{emptyEndpoints, failedRC, failedDeployer},
			deploymentConfigs:    &stubDeploymentConfigs{dc: failedDC},
			deploymentConfigName: "idling-echo",
			expectSummary:        "dc.read=ok dc.latestVersion=1 dc.spec.replicas=2 dc.status.replicas=2 dc.status.ready=0 dc.status.available=0 dc.status.updated=0 dc.status.unavailable=2 dc.condition.Available=False dc.condition.Progressing=Unknown dc.condition.ReplicaFailure=absent rc.read=ok rc.phase=Failed rc.spec.replicas=2 rc.status.replicas=0 rc.status.ready=0 rc.status.available=0 deployer.read=ok deployer.phase=Failed deployer.container.state=terminated deployer.container.reason=Error deployer.container.exitCode=1 endpoints.read=ok endpoints.subsets=0 endpoints.readyTotal=0 endpoints.notReadyTotal=0",
			expectContains: []string{
				"dc.read=ok", "dc.latestVersion=1", "dc.spec.replicas=2", "dc.status.ready=0",
				"dc.status.unavailable=2", "dc.condition.Available=False",
				"dc.condition.Progressing=Unknown", "dc.condition.ReplicaFailure=absent",
				"rc.read=ok", "rc.phase=Failed", "rc.spec.replicas=2", "rc.status.ready=0",
				"deployer.read=ok", "deployer.phase=Failed", "deployer.container.state=terminated",
				"deployer.container.reason=Error", "deployer.container.exitCode=1",
				"endpoints.read=ok", "endpoints.subsets=0", "endpoints.readyTotal=0", "endpoints.notReadyTotal=0",
			},
		},
		{
			name: "fixture without a DeploymentConfig reads only the endpoints",
			objects: []runtime.Object{&corev1.Endpoints{
				ObjectMeta: metav1.ObjectMeta{Name: "idling-echo", Namespace: testNamespace},
				Subsets: []corev1.EndpointSubset{{
					NotReadyAddresses: []corev1.EndpointAddress{{IP: "10.128.2.37"}},
				}},
			}},
			expectContains: []string{
				"dc.read=skipped", "rc.read=skipped", "deployer.read=skipped",
				"endpoints.read=ok", "endpoints.subsets=1", "endpoints.readyTotal=0", "endpoints.notReadyTotal=1",
			},
			expectAbsent: []string{"dc.spec.replicas", "rc.phase", "deployer.container.state"},
		},
		{
			name:                 "unavailable DeploymentConfig read leaves derived reads skipped",
			objects:              []runtime.Object{emptyEndpoints},
			deploymentConfigs:    &stubDeploymentConfigs{err: errors.New("Get \"https://api.example.invalid:6443/apis\": net/http: TLS handshake timeout")},
			deploymentConfigName: "idling-echo",
			expectContains:       []string{"dc.read=unavailable", "rc.read=skipped", "deployer.read=skipped", "endpoints.read=ok"},
			expectAbsent:         []string{"dc.latestVersion", "TLS handshake timeout"},
		},
		{
			name:                 "a nil object without an error is not an empty object",
			objects:              []runtime.Object{emptyEndpoints},
			deploymentConfigs:    &stubDeploymentConfigs{},
			deploymentConfigName: "idling-echo",
			expectContains:       []string{"dc.read=unavailable", "rc.read=skipped", "deployer.read=skipped"},
			expectAbsent:         []string{"dc.status.replicas"},
		},
		{
			name:           "unreadable endpoints are not reported as empty endpoints",
			objects:        []runtime.Object{emptyEndpoints},
			failingReads:   map[string]error{"endpoints": errors.New("connection reset by peer")},
			expectContains: []string{"endpoints.read=unavailable"},
			expectAbsent:   []string{"endpoints.subsets", "endpoints.readyTotal", "connection reset"},
		},
		{
			name:           "missing endpoints are reported as missing",
			failingReads:   map[string]error{"endpoints": notFound("endpoints", "idling-echo")},
			expectContains: []string{"endpoints.read=notfound"},
			expectAbsent:   []string{"endpoints.readyTotal"},
		},
		{
			name:    "DeploymentConfig without a rollout skips the ReplicationController",
			objects: []runtime.Object{emptyEndpoints},
			deploymentConfigs: &stubDeploymentConfigs{dc: &appsv1.DeploymentConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "idling-echo", Namespace: testNamespace},
				Spec:       appsv1.DeploymentConfigSpec{Replicas: 2},
			}},
			deploymentConfigName: "idling-echo",
			expectContains:       []string{"dc.read=ok", "dc.latestVersion=0", "rc.read=skipped", "deployer.read=skipped"},
		},
		{
			name:                 "a missing ReplicationController still reaches the deployer pod",
			objects:              []runtime.Object{emptyEndpoints, failedDeployer},
			deploymentConfigs:    &stubDeploymentConfigs{dc: failedDC},
			deploymentConfigName: "idling-echo",
			expectContains:       []string{"rc.read=notfound", "deployer.read=ok", "deployer.container.state=terminated"},
			expectAbsent:         []string{"rc.phase"},
		},
		{
			name:                 "missing deployer pod keeps the rollout snapshot",
			objects:              []runtime.Object{emptyEndpoints, failedRC},
			deploymentConfigs:    &stubDeploymentConfigs{dc: failedDC},
			deploymentConfigName: "idling-echo",
			expectContains:       []string{"rc.read=ok", "rc.phase=Failed", "deployer.read=notfound"},
			expectAbsent:         []string{"deployer.phase"},
		},
		{
			name: "a rollout still running",
			objects: []runtime.Object{
				emptyEndpoints,
				&corev1.ReplicationController{ObjectMeta: metav1.ObjectMeta{Name: "idling-echo-1", Namespace: testNamespace}},
				deployerPod(corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}),
			},
			deploymentConfigs:    &stubDeploymentConfigs{dc: failedDC},
			deploymentConfigName: "idling-echo",
			expectContains:       []string{"rc.phase=absent", "rc.spec.replicas=0", "deployer.container.state=running"},
			expectAbsent:         []string{"deployer.container.exitCode", "deployer.container.reason"},
		},
		{
			name: "a deployer container that has not started",
			objects: []runtime.Object{
				emptyEndpoints,
				failedRC,
				deployerPod(corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "ContainerCreating"}}),
			},
			deploymentConfigs:    &stubDeploymentConfigs{dc: failedDC},
			deploymentConfigName: "idling-echo",
			expectContains:       []string{"deployer.container.state=waiting"},
			expectAbsent:         []string{"ContainerCreating", "deployer.container.exitCode"},
		},
		{
			name: "a deployer pod with no container status yet",
			objects: []runtime.Object{
				emptyEndpoints,
				failedRC,
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{Name: "idling-echo-1-deploy", Namespace: testNamespace},
					Status:     corev1.PodStatus{Phase: corev1.PodPending},
				},
			},
			deploymentConfigs:    &stubDeploymentConfigs{dc: failedDC},
			deploymentConfigName: "idling-echo",
			expectContains:       []string{"deployer.phase=Pending", "deployer.container.state=none"},
		},
		{
			name: "only the deployer container is reported",
			objects: []runtime.Object{
				emptyEndpoints,
				failedRC,
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{Name: "idling-echo-1-deploy", Namespace: testNamespace},
					Status: corev1.PodStatus{
						Phase: corev1.PodFailed,
						ContainerStatuses: []corev1.ContainerStatus{{
							Name: "sidecar",
							State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{
								ExitCode: 42,
								Reason:   "OOMKilled",
							}},
						}},
					},
				},
			},
			deploymentConfigs:    &stubDeploymentConfigs{dc: failedDC},
			deploymentConfigName: "idling-echo",
			expectContains:       []string{"deployer.read=ok", "deployer.container.state=none"},
			expectAbsent:         []string{"exitCode=42", "OOMKilled"},
		},
		{
			name: "a terminated deployer container without a reason",
			objects: []runtime.Object{
				emptyEndpoints,
				failedRC,
				deployerPod(corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 255}}),
			},
			deploymentConfigs:    &stubDeploymentConfigs{dc: failedDC},
			deploymentConfigName: "idling-echo",
			expectContains:       []string{"deployer.container.reason=none", "deployer.container.exitCode=255"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			kube := fake.NewSimpleClientset(test.objects...)
			for resource, err := range test.failingReads {
				failRead(kube, resource, err)
			}
			clients := idlingDiagnosticsClients{kube: kube, namespace: testNamespace}
			if test.deploymentConfigs != nil {
				clients.deploymentConfigs = test.deploymentConfigs
			}

			summary := collectIdlingSetupDiagnostics(context.Background(), clients, "idling-echo", test.deploymentConfigName).summary()

			if test.expectSummary != "" && summary != test.expectSummary {
				t.Errorf("summary =\n%s\nwant\n%s", summary, test.expectSummary)
			}
			for _, want := range test.expectContains {
				if !strings.Contains(summary, want) {
					t.Errorf("summary is missing %q:\n%s", want, summary)
				}
			}
			for _, unwanted := range test.expectAbsent {
				if strings.Contains(summary, unwanted) {
					t.Errorf("summary unexpectedly contains %q:\n%s", unwanted, summary)
				}
			}
			for _, action := range kube.Actions() {
				if action.GetNamespace() != testNamespace {
					t.Errorf("read outside the test namespace: %s %s in %q", action.GetVerb(), action.GetResource().Resource, action.GetNamespace())
				}
				if action.GetVerb() != "get" {
					t.Errorf("unexpected %q on %s, diagnostics only read named objects", action.GetVerb(), action.GetResource().Resource)
				}
			}
			if test.deploymentConfigs != nil && test.deploymentConfigName != "" && test.deploymentConfigs.requested != test.deploymentConfigName {
				t.Errorf("requested DeploymentConfig %q, want the fixture's %q", test.deploymentConfigs.requested, test.deploymentConfigName)
			}
		})
	}
}

func TestSummaryUsesFixedVocabularies(t *testing.T) {
	kube := fake.NewSimpleClientset(
		&corev1.Endpoints{ObjectMeta: metav1.ObjectMeta{Name: "idling-echo", Namespace: testNamespace}},
		&corev1.ReplicationController{ObjectMeta: metav1.ObjectMeta{
			Name:        "idling-echo-3",
			Namespace:   testNamespace,
			Annotations: map[string]string{appsv1.DeploymentStatusAnnotation: "SomethingUnexpected"},
		}},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "idling-echo-3-deploy", Namespace: testNamespace},
			Status: corev1.PodStatus{
				Phase: corev1.PodPhase("WeirdPhase"),
				ContainerStatuses: []corev1.ContainerStatus{{
					Name: deployerContainerName,
					State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{
						ExitCode: 137,
						Reason:   "StoppedBecauseOfSomethingOddAndVerbose",
					}},
				}},
			},
		},
	)
	clients := idlingDiagnosticsClients{
		kube:      kube,
		namespace: testNamespace,
		deploymentConfigs: &stubDeploymentConfigs{dc: &appsv1.DeploymentConfig{
			ObjectMeta: metav1.ObjectMeta{Name: "idling-echo", Namespace: testNamespace},
			Status: appsv1.DeploymentConfigStatus{
				LatestVersion: 3,
				Conditions: []appsv1.DeploymentCondition{
					{Type: appsv1.DeploymentAvailable, Status: corev1.ConditionStatus("MaybeAvailableIsItNot")},
					{Type: appsv1.DeploymentConditionType("SomeCustomCondition"), Status: corev1.ConditionTrue},
				},
			},
		}},
	}

	summary := collectIdlingSetupDiagnostics(context.Background(), clients, "idling-echo", "idling-echo").summary()

	for _, want := range []string{
		"dc.condition.Available=other",
		"rc.phase=other",
		"deployer.phase=other",
		"deployer.container.reason=other",
		"deployer.container.exitCode=137",
	} {
		if !strings.Contains(summary, want) {
			t.Errorf("summary is missing %q:\n%s", want, summary)
		}
	}
	for _, unwanted := range []string{"SomeCustomCondition", "MaybeAvailableIsItNot", "SomethingUnexpected", "WeirdPhase", "StoppedBecauseOfSomethingOddAndVerbose"} {
		if strings.Contains(summary, unwanted) {
			t.Errorf("summary leaked unallowlisted value %q:\n%s", unwanted, summary)
		}
	}
}

func TestSummaryDoesNotLeakSensitiveValues(t *testing.T) {
	const (
		podIP        = "10.128.2.37"
		token        = "sha256~notarealtokenvalue"
		apiErrorText = "Get \"https://172.30.0.1:443/apis/apps.openshift.io/v1\": net/http: TLS handshake timeout"
	)

	kube := fake.NewSimpleClientset(
		&corev1.Endpoints{
			ObjectMeta: metav1.ObjectMeta{Name: "idling-echo", Namespace: testNamespace},
			Subsets: []corev1.EndpointSubset{{
				NotReadyAddresses: []corev1.EndpointAddress{{IP: podIP, Hostname: "idling-echo-1-abcde"}},
			}},
		},
		&corev1.ReplicationController{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "idling-echo-1",
				Namespace: testNamespace,
				Annotations: map[string]string{
					appsv1.DeploymentStatusAnnotation:       string(appsv1.DeploymentStatusFailed),
					"openshift.io/deployment.status-reason": apiErrorText,
					"example.com/token":                     token,
				},
			},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "idling-echo-1-deploy", Namespace: testNamespace},
			Status: corev1.PodStatus{
				Phase: corev1.PodFailed,
				PodIP: podIP,
				ContainerStatuses: []corev1.ContainerStatus{{
					Name:  deployerContainerName,
					Image: "quay.example.invalid/openshift/deployer:latest",
					State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{
						ExitCode: 1,
						Reason:   "Error",
						Message:  apiErrorText,
					}},
				}},
			},
		},
	)
	clients := idlingDiagnosticsClients{
		kube:      kube,
		namespace: testNamespace,
		deploymentConfigs: &stubDeploymentConfigs{dc: &appsv1.DeploymentConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "idling-echo",
				Namespace:   testNamespace,
				Annotations: map[string]string{"example.com/token": token},
			},
			Status: appsv1.DeploymentConfigStatus{
				LatestVersion: 1,
				Conditions: []appsv1.DeploymentCondition{{
					Type:    appsv1.DeploymentProgressing,
					Status:  corev1.ConditionFalse,
					Reason:  "ProgressDeadlineExceeded",
					Message: fmt.Sprintf("replication controller idling-echo-1 in %s has failed progressing", testNamespace),
				}},
			},
		}},
	}

	summary := collectIdlingSetupDiagnostics(context.Background(), clients, "idling-echo", "idling-echo").summary()

	for _, secret := range []string{
		testNamespace, podIP, token, apiErrorText, "idling-echo", "172.30.0.1",
		"ProgressDeadlineExceeded", "quay.example.invalid",
	} {
		if strings.Contains(summary, secret) {
			t.Errorf("summary leaked %q:\n%s", secret, summary)
		}
	}

	safe := regexp.MustCompile(`^[A-Za-z0-9 ._,=-]+$`)
	if !safe.MatchString(summary) {
		t.Errorf("summary contains characters outside the expected key=value vocabulary:\n%s", summary)
	}
	if !strings.Contains(summary, "deployer.container.exitCode=1") {
		t.Errorf("summary lost the deployer termination evidence:\n%s", summary)
	}
}

func TestDeploymentConfigNameFor(t *testing.T) {
	tests := []struct {
		name      string
		resources map[string][]string
		expected  string
	}{
		{
			name: "DeploymentConfig fixture",
			resources: map[string][]string{
				"service":                  {"idling-echo"},
				deploymentConfigResource:   {"idling-echo"},
				"route.route.openshift.io": {"idling-echo"},
			},
			expected: "idling-echo",
		},
		{
			name:      "ReplicationController fixture",
			resources: map[string][]string{"service": {"idling-echo"}, "replicationcontroller": {"idling-echo"}},
		},
		{
			name:      "Deployment fixture",
			resources: map[string][]string{"service": {"idling-echo"}, "deployment.apps": {"idling-echo"}},
		},
		{
			name: "no resources",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := deploymentConfigNameFor(test.resources); got != test.expected {
				t.Errorf("deploymentConfigNameFor() = %q, want %q", got, test.expected)
			}
		})
	}
}
