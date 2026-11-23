package networking

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	configv1 "github.com/openshift/api/config/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
)

func TestEvaluateDiagnosticsRecovery(t *testing.T) {
	t.Parallel()

	deletionTime := metav1.NewTime(time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
	oldTransition := metav1.NewTime(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))
	terminatingNamespaceMessage := "Error while updating operator configuration: could not apply (networking.k8s.io/v1, Kind=NetworkPolicy) " +
		"openshift-network-diagnostics/network-check-target: failed to apply / update (networking.k8s.io/v1, Kind=NetworkPolicy) " +
		"openshift-network-diagnostics/network-check-target: networkpolicies.networking.k8s.io \"network-check-target\" is forbidden: " +
		"unable to create new content in namespace openshift-network-diagnostics because it is being terminated"

	newNamespace := func(uid types.UID, phase corev1.NamespacePhase, deletion *metav1.Time) *corev1.Namespace {
		return &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: namespace, UID: uid, DeletionTimestamp: deletion},
			Status:     corev1.NamespaceStatus{Phase: phase},
		}
	}
	newPod := func(name string, uid types.UID, ready bool, deletion *metav1.Time) corev1.Pod {
		readyStatus := corev1.ConditionFalse
		if ready {
			readyStatus = corev1.ConditionTrue
		}
		return corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace, UID: uid, DeletionTimestamp: deletion},
			Spec:       corev1.PodSpec{NodeName: "worker-0"},
			Status: corev1.PodStatus{
				Phase:      corev1.PodRunning,
				Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: readyStatus}},
			},
		}
	}
	newOperator := func(conditions ...configv1.ClusterOperatorStatusCondition) *configv1.ClusterOperator {
		return &configv1.ClusterOperator{
			ObjectMeta: metav1.ObjectMeta{Name: networkClusterOperatorName},
			Status:     configv1.ClusterOperatorStatus{Conditions: conditions},
		}
	}
	operatorCondition := func(conditionType configv1.ClusterStatusConditionType, status configv1.ConditionStatus, message string) configv1.ClusterOperatorStatusCondition {
		return configv1.ClusterOperatorStatusCondition{Type: conditionType, Status: status, Reason: "TestReason", Message: message}
	}
	available := operatorCondition(configv1.OperatorAvailable, configv1.ConditionTrue, "")
	notDegraded := operatorCondition(configv1.OperatorDegraded, configv1.ConditionFalse, "")

	snapshot := diagnosticsSnapshot{
		namespaceUID: "old-namespace",
		podUIDs:      sets.New[types.UID]("old-source", "old-target"),
	}
	healthy := func() diagnosticsObservation {
		return diagnosticsObservation{
			namespace:  newNamespace("new-namespace", corev1.NamespaceActive, nil),
			sourcePods: []corev1.Pod{newPod("network-check-source-new", "new-source", true, nil)},
			targetPods: []corev1.Pod{newPod("network-check-target-new", "new-target", true, nil)},
			operator:   newOperator(available, notDegraded),
		}
	}

	for _, test := range []struct {
		name        string
		observe     func() diagnosticsObservation
		snapshot    diagnosticsSnapshot
		podsRemoved bool
		// wantErr lists substrings of the expected error; nil means recovery is expected.
		wantErr   []string
		wantState []string
	}{
		{
			name:        "replacement namespace with fresh ready pods and healthy operator recovers",
			observe:     healthy,
			snapshot:    snapshot,
			podsRemoved: true,
			wantState: []string{
				"namespace openshift-network-diagnostics: uid=new-namespace phase=Active deletionTimestamp=<none> replaced=true (pre-disable uid=old-namespace)",
				"pod network-check-source-new (app=network-check-source): uid=new-source phase=Running ready=true terminating=false preDisable=false node=worker-0",
				"pod network-check-target-new (app=network-check-target): uid=new-target phase=Running ready=true terminating=false preDisable=false node=worker-0",
				`clusteroperator/network: Available=True reason=TestReason message=""; Degraded=False reason=TestReason message=""; Progressing=<missing>`,
			},
		},
		{
			name: "operator conditions with old transition times recover",
			observe: func() diagnosticsObservation {
				obs := healthy()
				oldAvailable, oldNotDegraded := available, notDegraded
				oldAvailable.LastTransitionTime = oldTransition
				oldNotDegraded.LastTransitionTime = oldTransition
				obs.operator = newOperator(oldAvailable, oldNotDegraded)
				return obs
			},
			snapshot:    snapshot,
			podsRemoved: true,
		},
		{
			name: "progressing operator still recovers",
			observe: func() diagnosticsObservation {
				obs := healthy()
				obs.operator = newOperator(available, notDegraded,
					operatorCondition(configv1.OperatorProgressing, configv1.ConditionTrue, "rolling out"))
				return obs
			},
			snapshot:    snapshot,
			podsRemoved: true,
			wantState:   []string{`Progressing=True reason=TestReason message="rolling out"`},
		},
		{
			name: "missing namespace does not recover",
			observe: func() diagnosticsObservation {
				obs := healthy()
				obs.namespace = nil
				return obs
			},
			snapshot:    snapshot,
			podsRemoved: true,
			wantErr:     []string{"namespace openshift-network-diagnostics does not exist"},
			wantState:   []string{"namespace openshift-network-diagnostics: not found"},
		},
		{
			name: "old terminating namespace does not recover",
			observe: func() diagnosticsObservation {
				obs := healthy()
				obs.namespace = newNamespace("old-namespace", corev1.NamespaceTerminating, &deletionTime)
				return obs
			},
			snapshot:    snapshot,
			podsRemoved: true,
			wantErr: []string{
				"namespace openshift-network-diagnostics is Terminating, not Active",
				"namespace openshift-network-diagnostics is terminating",
			},
			wantState: []string{"phase=Terminating deletionTimestamp=2026-01-02T03:04:05Z replaced=false (pre-disable uid=old-namespace)"},
		},
		{
			name: "terminating pod does not recover",
			observe: func() diagnosticsObservation {
				obs := healthy()
				obs.targetPods = append(obs.targetPods, newPod("network-check-target-leftover", "leftover-target", true, &deletionTime))
				return obs
			},
			snapshot:    snapshot,
			podsRemoved: true,
			wantErr:     []string{"pod network-check-target-leftover is terminating"},
			wantState:   []string{"pod network-check-target-leftover (app=network-check-target): uid=leftover-target phase=Running ready=true terminating=true"},
		},
		{
			name: "pod that is not ready does not recover",
			observe: func() diagnosticsObservation {
				obs := healthy()
				obs.sourcePods = []corev1.Pod{newPod("network-check-source-new", "new-source", false, nil)}
				return obs
			},
			snapshot:    snapshot,
			podsRemoved: true,
			wantErr:     []string{"pod network-check-source-new is not Ready"},
			wantState:   []string{"pod network-check-source-new (app=network-check-source): uid=new-source phase=Running ready=false"},
		},
		{
			name: "missing target pods do not recover",
			observe: func() diagnosticsObservation {
				obs := healthy()
				obs.targetPods = nil
				return obs
			},
			snapshot:    snapshot,
			podsRemoved: true,
			wantErr:     []string{"no pods match app=network-check-target"},
			wantState:   []string{"pods app=network-check-target: none"},
		},
		{
			name: "missing source pods do not recover",
			observe: func() diagnosticsObservation {
				obs := healthy()
				obs.sourcePods = nil
				return obs
			},
			snapshot:    snapshot,
			podsRemoved: true,
			wantErr:     []string{"no pods match app=network-check-source"},
			wantState:   []string{"pods app=network-check-source: none"},
		},
		{
			name: "pre-disable pod is rejected after the disable spec saw all pods removed",
			observe: func() diagnosticsObservation {
				obs := healthy()
				obs.sourcePods = []corev1.Pod{newPod("network-check-source-old", "old-source", true, nil)}
				return obs
			},
			snapshot:    snapshot,
			podsRemoved: true,
			wantErr:     []string{"pod network-check-source-old existed before diagnostics were disabled"},
			wantState:   []string{"uid=old-source phase=Running ready=true terminating=false preDisable=true"},
		},
		{
			name: "pre-disable resources recover when the disable never removed them",
			observe: func() diagnosticsObservation {
				obs := healthy()
				obs.namespace = newNamespace("old-namespace", corev1.NamespaceActive, nil)
				obs.sourcePods = []corev1.Pod{newPod("network-check-source-old", "old-source", true, nil)}
				obs.targetPods = []corev1.Pod{newPod("network-check-target-old", "old-target", true, nil)}
				return obs
			},
			snapshot:    snapshot,
			podsRemoved: false,
			wantState: []string{
				"replaced=false (pre-disable uid=old-namespace)",
				"uid=old-target phase=Running ready=true terminating=false preDisable=true",
			},
		},
		{
			name:        "namespace that did not exist before disabling is reported as unknown replacement",
			observe:     healthy,
			snapshot:    diagnosticsSnapshot{podUIDs: sets.New[types.UID]()},
			podsRemoved: true,
			wantState:   []string{"replaced=unknown (no namespace before disabling)"},
		},
		{
			name: "degraded operator does not recover and reports the root cause",
			observe: func() diagnosticsObservation {
				obs := healthy()
				obs.operator = newOperator(available,
					operatorCondition(configv1.OperatorDegraded, configv1.ConditionTrue, terminatingNamespaceMessage))
				return obs
			},
			snapshot:    snapshot,
			podsRemoved: true,
			wantErr:     []string{"clusteroperator/network is not Degraded=False"},
			wantState:   []string{"Degraded=True reason=TestReason", "because it is being terminated"},
		},
		{
			name: "operator without Degraded condition does not recover",
			observe: func() diagnosticsObservation {
				obs := healthy()
				obs.operator = newOperator(available)
				return obs
			},
			snapshot:    snapshot,
			podsRemoved: true,
			wantErr:     []string{"clusteroperator/network is not Degraded=False"},
			wantState:   []string{"Degraded=<missing>"},
		},
		{
			name: "operator with unknown Degraded condition does not recover",
			observe: func() diagnosticsObservation {
				obs := healthy()
				obs.operator = newOperator(available, operatorCondition(configv1.OperatorDegraded, configv1.ConditionUnknown, ""))
				return obs
			},
			snapshot:    snapshot,
			podsRemoved: true,
			wantErr:     []string{"clusteroperator/network is not Degraded=False"},
			wantState:   []string{"Degraded=Unknown reason=TestReason"},
		},
		{
			name: "operator without Available condition does not recover",
			observe: func() diagnosticsObservation {
				obs := healthy()
				obs.operator = newOperator(notDegraded)
				return obs
			},
			snapshot:    snapshot,
			podsRemoved: true,
			wantErr:     []string{"clusteroperator/network is not Available=True"},
			wantState:   []string{"Available=<missing>"},
		},
		{
			name: "unavailable operator does not recover",
			observe: func() diagnosticsObservation {
				obs := healthy()
				obs.operator = newOperator(operatorCondition(configv1.OperatorAvailable, configv1.ConditionFalse, "starting"), notDegraded)
				return obs
			},
			snapshot:    snapshot,
			podsRemoved: true,
			wantErr:     []string{"clusteroperator/network is not Available=True"},
			wantState:   []string{`Available=False reason=TestReason message="starting"`},
		},
		{
			name: "missing operator does not recover",
			observe: func() diagnosticsObservation {
				obs := healthy()
				obs.operator = nil
				return obs
			},
			snapshot:    snapshot,
			podsRemoved: true,
			wantErr:     []string{"clusteroperator/network does not exist"},
			wantState:   []string{"clusteroperator/network: not found"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			summary, err := evaluateDiagnosticsRecovery(test.observe(), test.snapshot, test.podsRemoved)

			if len(test.wantErr) == 0 && err != nil {
				t.Errorf("expected recovery, got error: %v", err)
			}
			if len(test.wantErr) > 0 {
				if err == nil {
					t.Fatalf("expected an error containing %q, got recovery", test.wantErr)
				}
				for _, want := range test.wantErr {
					if !strings.Contains(err.Error(), want) {
						t.Errorf("error %q does not contain %q", err.Error(), want)
					}
				}
			}
			// The summary is what a timeout reports, so it must always describe the namespace, the pods
			// of both kinds, and the operator.
			for _, want := range append([]string{
				"namespace openshift-network-diagnostics:",
				"app=network-check-source",
				"app=network-check-target",
				"clusteroperator/network:",
			}, test.wantState...) {
				if !strings.Contains(summary, want) {
					t.Errorf("summary does not contain %q:\n%s", want, summary)
				}
			}
		})
	}
}

func TestTruncateDiagnosticsMessage(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name    string
		message string
		want    string
	}{
		{
			name:    "short message is unchanged",
			message: "short",
			want:    "short",
		},
		{
			name:    "message at the limit is unchanged",
			message: strings.Repeat("x", diagnosticsMaxMessageLength),
			want:    strings.Repeat("x", diagnosticsMaxMessageLength),
		},
		{
			name:    "message one rune over the limit is truncated",
			message: strings.Repeat("x", diagnosticsMaxMessageLength+1),
			want:    strings.Repeat("x", diagnosticsMaxMessageLength) + "...",
		},
		{
			name:    "multibyte message is cut on a rune boundary",
			message: strings.Repeat("é", diagnosticsMaxMessageLength+1),
			want:    strings.Repeat("é", diagnosticsMaxMessageLength) + "...",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := truncateDiagnosticsMessage(test.message)
			if got != test.want {
				t.Errorf("truncateDiagnosticsMessage() = %q (%d runes), want %q (%d runes)",
					got, utf8.RuneCountInString(got), test.want, utf8.RuneCountInString(test.want))
			}
			if !utf8.ValidString(got) {
				t.Errorf("truncateDiagnosticsMessage() returned invalid UTF-8: %q", got)
			}
		})
	}
}
