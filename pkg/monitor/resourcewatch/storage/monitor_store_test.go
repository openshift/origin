package storage

import (
	"fmt"
	configv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	monitorserialization "github.com/openshift/origin/pkg/monitor/serialization"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func Test_Add(t *testing.T) {
	tests := []struct {
		name       string
		apiVersion string
		objs       []runtime.Object
		expected   monitorapi.Intervals
	}{
		{
			name:       "add_nodes",
			apiVersion: "v1",
			objs: []runtime.Object{
				&corev1.Node{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Node",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "testnode-1",
					},
					Status: corev1.NodeStatus{},
				},
				&corev1.Node{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Node",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "testnode-2",
					},
					Status: corev1.NodeStatus{},
				},
			},
			expected: monitorapi.Intervals{},
		},
		{
			name:       "add_pods",
			apiVersion: "v1",
			objs: []runtime.Object{
				&corev1.Pod{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Pod",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "testpod-1",
					},
					Status: corev1.PodStatus{},
				},
				&corev1.Pod{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Pod",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "testpod-2",
					},
					Status: corev1.PodStatus{},
				},
			},
			expected: monitorapi.Intervals{
				monitorapi.EventInterval{
					Condition: monitorapi.Condition{
						Level:   monitorapi.Info,
						Locator: "ns/ pod/testpod-1 node/ uid/",
						Message: "reason/Created ",
					},
				},
				monitorapi.EventInterval{
					Condition: monitorapi.Condition{
						Level:   monitorapi.Info,
						Locator: "ns/ pod/testpod-2 node/ uid/",
						Message: "reason/Created ",
					},
				},
			},
		},
		{
			name:       "add_events",
			apiVersion: "v1",
			objs: []runtime.Object{
				&corev1.Event{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Event",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "testevent-1",
					},
					LastTimestamp: metav1.Time{Time: time.Now().Add(time.Minute)},
					Message:       "testevent-1",
					Count:         1,
					Reason:        "testevent-1",
				},
				&corev1.Event{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Event",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "testevent-2",
					},
					LastTimestamp: metav1.Time{Time: time.Now().Add(time.Minute)},
					Message:       "testevent-2",
					Count:         2,
					Reason:        "testevent-2",
				},
			},
			expected: monitorapi.Intervals{
				monitorapi.EventInterval{
					Condition: monitorapi.Condition{
						Level:   monitorapi.Info,
						Locator: "/",
						Message: "reason/testevent-1 testevent-1",
					},
				},
				monitorapi.EventInterval{
					Condition: monitorapi.Condition{
						Level:   monitorapi.Info,
						Locator: "/",
						Message: "reason/testevent-2 testevent-2 (2 times)",
					},
				},
			},
		},
		{
			name:       "add_cluster_operator",
			apiVersion: "config.openshift.io/v1",
			objs: []runtime.Object{
				&configv1.ClusterOperator{
					TypeMeta: metav1.TypeMeta{
						Kind:       "ClusterOperator",
						APIVersion: "config.openshift.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:              "testclusteroperator-1",
						CreationTimestamp: metav1.Time{Time: time.Now().Add(time.Minute)},
					},
				},
				&configv1.ClusterOperator{
					TypeMeta: metav1.TypeMeta{
						Kind:       "ClusterOperator",
						APIVersion: "config.openshift.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:              "testclusteroperator-2",
						CreationTimestamp: metav1.Time{Time: time.Now().Add(time.Minute)},
					},
				},
			},
			expected: monitorapi.Intervals{
				monitorapi.EventInterval{
					Condition: monitorapi.Condition{
						Level:   monitorapi.Info,
						Locator: "clusteroperator/testclusteroperator-1",
						Message: "created",
					},
				},
				monitorapi.EventInterval{
					Condition: monitorapi.Condition{
						Level:   monitorapi.Info,
						Locator: "clusteroperator/testclusteroperator-2",
						Message: "created",
					},
				},
			},
		},
		{
			name:       "add_cluster_version",
			apiVersion: "config.openshift.io/v1",
			objs: []runtime.Object{
				&configv1.ClusterVersion{
					TypeMeta: metav1.TypeMeta{
						Kind:       "ClusterVersion",
						APIVersion: "config.openshift.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:              "testclusterversion-1",
						CreationTimestamp: metav1.Time{Time: time.Now().Add(time.Minute)},
					},
				},
				&configv1.ClusterVersion{
					TypeMeta: metav1.TypeMeta{
						Kind:       "ClusterVersion",
						APIVersion: "config.openshift.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:              "testclusterversion-2",
						CreationTimestamp: metav1.Time{Time: time.Now().Add(time.Minute)},
					},
				},
			},
			expected: monitorapi.Intervals{
				monitorapi.EventInterval{
					Condition: monitorapi.Condition{
						Level:   monitorapi.Info,
						Locator: "clusterversion/testclusterversion-1",
						Message: "created",
					},
				},
				monitorapi.EventInterval{
					Condition: monitorapi.Condition{
						Level:   monitorapi.Info,
						Locator: "clusterversion/testclusterversion-2",
						Message: "created",
					},
				},
			},
		},
		{
			name:       "add_Authentication",
			apiVersion: "config.openshift.io/v1",
			objs: []runtime.Object{
				&configv1.Authentication{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Authentication",
						APIVersion: "config.openshift.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:              "testauthentication-1",
						CreationTimestamp: metav1.Time{Time: time.Now().Add(time.Minute)},
					},
				},
				&configv1.Authentication{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Authentication",
						APIVersion: "config.openshift.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:              "testauthentication-2",
						CreationTimestamp: metav1.Time{Time: time.Now().Add(time.Minute)},
					},
				},
			},
			expected: monitorapi.Intervals{
				monitorapi.EventInterval{
					Condition: monitorapi.Condition{
						Level:   monitorapi.Info,
						Locator: "Authentication/testauthentication-1",
						Message: "added",
					},
				},
				monitorapi.EventInterval{
					Condition: monitorapi.Condition{
						Level:   monitorapi.Info,
						Locator: "Authentication/testauthentication-2",
						Message: "added",
					},
				},
			},
		},
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Errorf("error getting working directory, err: %v", err)
	}
	artifactDir := fmt.Sprintf("%s/test_add", wd)
	const mediaType = runtime.ContentTypeJSON
	info, ok := runtime.SerializerInfoForMediaType(coreCodecs.SupportedMediaTypes(), mediaType)
	if !ok {
		t.Errorf("unable to locate encoder -- %q is not a supported media type", mediaType)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := os.RemoveAll(artifactDir); err != nil {
				t.Errorf("Failed to remove output directory, err: %v", err)
			}
			if err := os.MkdirAll(artifactDir, os.ModePerm); err != nil {
				t.Errorf("Failed to create output directory, err: %v", err)
			}
			objList := []unstructured.Unstructured{}
			var enc runtime.Encoder
			if tt.apiVersion == "config.openshift.io/v1" {
				enc = coreCodecs.EncoderForVersion(info.Serializer, configv1.GroupVersion)
			} else {
				enc = coreCodecs.EncoderForVersion(info.Serializer, corev1.SchemeGroupVersion)
			}
			for _, obj := range tt.objs {
				objectBytes, err := runtime.Encode(enc, obj)
				if err != nil {
					t.Errorf("error: %v while encoding object: %v", err, obj)
				}
				var objUnstructured unstructured.Unstructured
				err = runtime.DecodeInto(unstructured.UnstructuredJSONScheme, objectBytes, &objUnstructured)
				if err != nil {
					t.Errorf("error: %v while decoding into unstructured object", err)
				}
				objList = append(objList, objUnstructured)
			}

			store, err := NewMonitorStorage(artifactDir, &kubernetes.Clientset{})
			if err != nil {
				t.Errorf("error calling NewMonitorStorage, err: %v", err)
			}
			for _, obj := range objList {
				store.OnAdd(&obj)
			}
			store.End()

			// Now analyze the result
			eventDir := fmt.Sprintf("%s/monitor-events", artifactDir)
			err = filepath.Walk(eventDir, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if strings.Contains(path, "e2e-events") {
					events, err := monitorserialization.EventsFromFile(path)
					if err != nil {
						return fmt.Errorf("error unmarshalling events: %v", err)
					}
					if len(events) != len(tt.expected) {
						return fmt.Errorf("len of events %d is not equal len of expected: %v", len(events), len(tt.expected))
					}
					for i := range events {
						if reflect.DeepEqual(events[i].Condition, tt.expected[i].Condition) == false {
							return fmt.Errorf("event at index %d: %+v is not equal to expected: %v", i, events[i], tt.expected[i])
						}
					}

					return nil
				}
				return nil
			})
			if err != nil {
				t.Errorf("test result analysis error: %+v", err)
			}
		})
	}
	if err := os.RemoveAll(artifactDir); err != nil {
		t.Errorf("Failed to remove output directory, err: %v", err)
	}
}

func Test_Update(t *testing.T) {
	tests := []struct {
		name       string
		apiVersion string
		objs       []runtime.Object
		oldObjs    []runtime.Object
		expected   monitorapi.Intervals
	}{
		{
			name:       "update_nodes",
			apiVersion: "v1",
			objs: []runtime.Object{
				&corev1.Node{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Node",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "testnode-1",
					},
					Status: corev1.NodeStatus{
						Conditions: []corev1.NodeCondition{{
							Type:   corev1.NodeReady,
							Status: corev1.ConditionFalse,
						}},
					},
				},
				&corev1.Node{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Node",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "testnode-2",
					},
					Status: corev1.NodeStatus{
						Conditions: []corev1.NodeCondition{{
							Type:   corev1.NodeReady,
							Status: corev1.ConditionTrue,
						}},
					},
				},
			},
			oldObjs: []runtime.Object{
				&corev1.Node{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Node",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "testnode-1",
					},
					Status: corev1.NodeStatus{
						Conditions: []corev1.NodeCondition{{
							Type:   corev1.NodeReady,
							Status: corev1.ConditionTrue,
						}},
					},
				},
				&corev1.Node{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Node",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "testnode-2",
					},
					Status: corev1.NodeStatus{
						Conditions: []corev1.NodeCondition{{
							Type:   corev1.NodeReady,
							Status: corev1.ConditionFalse,
						}},
					},
				},
			},
			expected: monitorapi.Intervals{
				monitorapi.EventInterval{
					Condition: monitorapi.Condition{
						Level:   monitorapi.Warning,
						Locator: "node/testnode-1",
						Message: "condition/Ready status/False reason/ roles/ changed",
					},
				},
				monitorapi.EventInterval{
					Condition: monitorapi.Condition{
						Level:   monitorapi.Warning,
						Locator: "node/testnode-2",
						Message: "condition/Ready status/True reason/ roles/ changed",
					},
				},
			},
		},
		{
			name:       "update_pods",
			apiVersion: "v1",
			objs: []runtime.Object{
				&corev1.Pod{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Pod",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "testpod-1",
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodFailed,
					},
				},
				&corev1.Pod{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Pod",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "testpod-2",
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodSucceeded,
					},
				},
			},
			oldObjs: []runtime.Object{
				&corev1.Pod{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Pod",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "testpod-1",
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodSucceeded,
					},
				},
				&corev1.Pod{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Pod",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "testpod-2",
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodPending,
					},
				},
			},
			expected: monitorapi.Intervals{
				monitorapi.EventInterval{
					Condition: monitorapi.Condition{
						Level:   monitorapi.Error,
						Locator: "ns/ pod/testpod-1 node/ uid/",
						Message: "reason/Failed (): ",
					},
				},
			},
		},
		{
			name:       "update_events",
			apiVersion: "v1",
			objs: []runtime.Object{
				&corev1.Event{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Event",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "testevent-1",
					},
					LastTimestamp: metav1.Time{Time: time.Now().Add(time.Minute)},
					Message:       "testevent-1 updated",
					Count:         1,
					Reason:        "testevent-1",
				},
				&corev1.Event{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Event",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "testevent-2",
					},
					LastTimestamp: metav1.Time{Time: time.Now().Add(time.Minute)},
					Message:       "testevent-2 updated",
					Count:         2,
					Reason:        "testevent-2",
				},
			},
			oldObjs: []runtime.Object{
				&corev1.Event{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Event",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "testevent-1",
					},
					LastTimestamp: metav1.Time{Time: time.Now().Add(time.Minute)},
					Message:       "testevent-1",
					Count:         1,
					Reason:        "testevent-1",
				},
				&corev1.Event{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Event",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "testevent-2",
					},
					LastTimestamp: metav1.Time{Time: time.Now().Add(time.Minute)},
					Message:       "testevent-2",
					Count:         2,
					Reason:        "testevent-2",
				},
			},
			expected: monitorapi.Intervals{
				monitorapi.EventInterval{
					Condition: monitorapi.Condition{
						Level:   monitorapi.Info,
						Locator: "/",
						Message: "reason/testevent-1 testevent-1 updated",
					},
				},
				monitorapi.EventInterval{
					Condition: monitorapi.Condition{
						Level:   monitorapi.Info,
						Locator: "/",
						Message: "reason/testevent-2 testevent-2 updated (2 times)",
					},
				},
			},
		},
		{
			name:       "add_cluster_operator",
			apiVersion: "config.openshift.io/v1",
			objs: []runtime.Object{
				&configv1.ClusterOperator{
					TypeMeta: metav1.TypeMeta{
						Kind:       "ClusterOperator",
						APIVersion: "config.openshift.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:              "testclusteroperator-1",
						CreationTimestamp: metav1.Time{Time: time.Now().Add(time.Minute)},
					},
					Status: configv1.ClusterOperatorStatus{
						Conditions: []configv1.ClusterOperatorStatusCondition{
							{
								Type:               "Available",
								Status:             configv1.ConditionTrue,
								LastTransitionTime: metav1.Time{Time: time.Now().Add(time.Minute)},
								Message:            "availableMessage",
							},
							{
								Type:               "Degraded",
								Status:             configv1.ConditionStatus("degraded"),
								LastTransitionTime: metav1.Time{Time: time.Now().Add(time.Minute)},
								Message:            "degradedMessage",
							},
							{
								Type:               "Progressing",
								Status:             configv1.ConditionStatus("progressing"),
								LastTransitionTime: metav1.Time{Time: time.Now().Add(time.Minute)},
								Message:            "progressingMessage",
							},
						},
					},
				},
			},
			oldObjs: []runtime.Object{
				&configv1.ClusterOperator{
					TypeMeta: metav1.TypeMeta{
						Kind:       "ClusterOperator",
						APIVersion: "config.openshift.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:              "testclusteroperator-1",
						CreationTimestamp: metav1.Time{Time: time.Now().Add(time.Minute)},
					},
					Status: configv1.ClusterOperatorStatus{
						Conditions: []configv1.ClusterOperatorStatusCondition{
							{
								Type:               "Available",
								Status:             configv1.ConditionFalse,
								LastTransitionTime: metav1.Time{Time: time.Now().Add(time.Minute)},
								Message:            "availableMessage",
							},
							{
								Type:               "Degraded",
								Status:             configv1.ConditionStatus("degraded"),
								LastTransitionTime: metav1.Time{Time: time.Now().Add(time.Minute)},
								Message:            "degradedMessage",
							},
							{
								Type:               "Progressing",
								Status:             configv1.ConditionStatus("progressing"),
								LastTransitionTime: metav1.Time{Time: time.Now().Add(time.Minute)},
								Message:            "progressingMessage",
							},
						},
					},
				},
			},
			expected: monitorapi.Intervals{
				monitorapi.EventInterval{
					Condition: monitorapi.Condition{
						Level:   monitorapi.Warning,
						Locator: "clusteroperator/testclusteroperator-1",
						Message: "condition/Available status/True changed: availableMessage",
					},
				},
			},
		},
		{
			name:       "add_cluster_version",
			apiVersion: "config.openshift.io/v1",
			objs: []runtime.Object{
				&configv1.ClusterVersion{
					TypeMeta: metav1.TypeMeta{
						Kind:       "ClusterVersion",
						APIVersion: "config.openshift.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:              "testclusterversion-1",
						CreationTimestamp: metav1.Time{Time: time.Now().Add(time.Minute)},
					},
					Status: configv1.ClusterVersionStatus{
						History: []configv1.UpdateHistory{
							{
								CompletionTime: &metav1.Time{Time: time.Now().Add(time.Minute)},
								Version:        "4.13",
								State:          configv1.CompletedUpdate,
							},
							{
								Version: "4.12",
							},
						},
					},
				},
			},
			oldObjs: []runtime.Object{
				&configv1.ClusterVersion{
					TypeMeta: metav1.TypeMeta{
						Kind:       "ClusterVersion",
						APIVersion: "config.openshift.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:              "testclusterversion-1",
						CreationTimestamp: metav1.Time{Time: time.Now().Add(time.Minute)},
					},
					Status: configv1.ClusterVersionStatus{
						History: []configv1.UpdateHistory{
							{
								Version: "4.12",
							},
						},
					},
				},
			},
			expected: monitorapi.Intervals{
				monitorapi.EventInterval{
					Condition: monitorapi.Condition{
						Level:   monitorapi.Warning,
						Locator: "clusterversion/testclusterversion-1",
						Message: "cluster reached 4.13",
					},
				},
			},
		},
		{
			name:       "update_Authentication",
			apiVersion: "config.openshift.io/v1",
			objs: []runtime.Object{
				&configv1.Authentication{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Authentication",
						APIVersion: "config.openshift.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:              "testauthentication-1",
						CreationTimestamp: metav1.Time{Time: time.Now().Add(time.Minute)},
					},
				},
				&configv1.Authentication{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Authentication",
						APIVersion: "config.openshift.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:              "testauthentication-2",
						CreationTimestamp: metav1.Time{Time: time.Now().Add(time.Minute)},
					},
				},
			},
			oldObjs: []runtime.Object{
				&configv1.Authentication{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Authentication",
						APIVersion: "config.openshift.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:              "testauthentication-1",
						CreationTimestamp: metav1.Time{Time: time.Now().Add(time.Minute)},
					},
				},
				&configv1.Authentication{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Authentication",
						APIVersion: "config.openshift.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:              "testauthentication-2",
						CreationTimestamp: metav1.Time{Time: time.Now().Add(time.Minute)},
					},
				},
			},
			expected: monitorapi.Intervals{
				monitorapi.EventInterval{
					Condition: monitorapi.Condition{
						Level:   monitorapi.Info,
						Locator: "Authentication/testauthentication-1",
						Message: "updated",
					},
				},
				monitorapi.EventInterval{
					Condition: monitorapi.Condition{
						Level:   monitorapi.Info,
						Locator: "Authentication/testauthentication-2",
						Message: "updated",
					},
				},
			},
		},
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Errorf("error getting working directory, err: %v", err)
	}
	artifactDir := fmt.Sprintf("%s/test_update", wd)
	const mediaType = runtime.ContentTypeJSON
	info, ok := runtime.SerializerInfoForMediaType(coreCodecs.SupportedMediaTypes(), mediaType)
	if !ok {
		t.Errorf("unable to locate encoder -- %q is not a supported media type", mediaType)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if len(tt.objs) != len(tt.oldObjs) {
				t.Errorf("should have the same number of items in objs and oldObjs")
			}
			if err := os.RemoveAll(artifactDir); err != nil {
				t.Errorf("Failed to remove output directory, err: %v", err)
			}
			if err := os.MkdirAll(artifactDir, os.ModePerm); err != nil {
				t.Errorf("Failed to create output directory, err: %v", err)
			}
			objList := []unstructured.Unstructured{}
			oldList := []unstructured.Unstructured{}
			var enc runtime.Encoder
			if tt.apiVersion == "config.openshift.io/v1" {
				enc = coreCodecs.EncoderForVersion(info.Serializer, configv1.GroupVersion)
			} else {
				enc = coreCodecs.EncoderForVersion(info.Serializer, corev1.SchemeGroupVersion)
			}
			for i := range tt.objs {
				obj := tt.objs[i]
				objectBytes, err := runtime.Encode(enc, obj)
				if err != nil {
					t.Errorf("error: %v while encoding object: %v", err, obj)
				}
				var objUnstructured unstructured.Unstructured
				err = runtime.DecodeInto(unstructured.UnstructuredJSONScheme, objectBytes, &objUnstructured)
				if err != nil {
					t.Errorf("error: %v while decoding into unstructured object", err)
				}
				objList = append(objList, objUnstructured)
				old := tt.oldObjs[i]
				objectBytes, err = runtime.Encode(enc, old)
				if err != nil {
					t.Errorf("error: %v while encoding object: %v", err, old)
				}
				err = runtime.DecodeInto(unstructured.UnstructuredJSONScheme, objectBytes, &objUnstructured)
				if err != nil {
					t.Errorf("error: %v while decoding into unstructured object", err)
				}
				oldList = append(oldList, objUnstructured)
			}

			store, err := NewMonitorStorage(artifactDir, &kubernetes.Clientset{})
			if err != nil {
				t.Errorf("error calling NewMonitorStorage, err: %v", err)
			}
			for i := range objList {
				obj := objList[i]
				old := oldList[i]
				store.OnUpdate(&old, &obj)
			}
			store.End()

			// Now analyze the result
			eventDir := fmt.Sprintf("%s/monitor-events", artifactDir)
			err = filepath.Walk(eventDir, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if strings.Contains(path, "e2e-events") {
					events, err := monitorserialization.EventsFromFile(path)
					if err != nil {
						return fmt.Errorf("error unmarshalling events: %v", err)
					}
					if len(events) != len(tt.expected) {
						return fmt.Errorf("len of events %d is not equal len of expected: %v", len(events), len(tt.expected))
					}
					for i := range events {
						if reflect.DeepEqual(events[i].Condition, tt.expected[i].Condition) == false {
							return fmt.Errorf("event at index %d: %+v is not equal to expected: %v", i, events[i], tt.expected[i])
						}
					}

					return nil
				}
				return nil
			})
			if err != nil {
				t.Errorf("test result analysis error: %+v", err)
			}
		})
	}
	if err := os.RemoveAll(artifactDir); err != nil {
		t.Errorf("Failed to remove output directory, err: %v", err)
	}
}

func Test_Delete(t *testing.T) {
	tests := []struct {
		name       string
		apiVersion string
		objs       []runtime.Object
		expected   monitorapi.Intervals
	}{
		{
			name:       "delete_nodes",
			apiVersion: "v1",
			objs: []runtime.Object{
				&corev1.Node{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Node",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "testnode-1",
					},
					Status: corev1.NodeStatus{},
				},
				&corev1.Node{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Node",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "testnode-2",
					},
					Status: corev1.NodeStatus{},
				},
			},
			expected: monitorapi.Intervals{
				monitorapi.EventInterval{
					Condition: monitorapi.Condition{
						Level:   monitorapi.Warning,
						Locator: "node/testnode-1",
						Message: "roles/ deleted",
					},
				},
				monitorapi.EventInterval{
					Condition: monitorapi.Condition{
						Level:   monitorapi.Warning,
						Locator: "node/testnode-2",
						Message: "roles/ deleted",
					},
				},
			},
		},
		{
			name:       "delete_pods",
			apiVersion: "v1",
			objs: []runtime.Object{
				&corev1.Pod{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Pod",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "testpod-1",
					},
					Status: corev1.PodStatus{},
				},
				&corev1.Pod{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Pod",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "testpod-2",
					},
					Status: corev1.PodStatus{},
				},
			},
			expected: monitorapi.Intervals{
				monitorapi.EventInterval{
					Condition: monitorapi.Condition{
						Level:   monitorapi.Info,
						Locator: "ns/ pod/testpod-1 node/ uid/",
						Message: "reason/Deleted ",
					},
				},
				monitorapi.EventInterval{
					Condition: monitorapi.Condition{
						Level:   monitorapi.Info,
						Locator: "ns/ pod/testpod-1 node/ uid/",
						Message: "reason/DeletedBeforeScheduling ",
					},
				},
				monitorapi.EventInterval{
					Condition: monitorapi.Condition{
						Level:   monitorapi.Info,
						Locator: "ns/ pod/testpod-2 node/ uid/",
						Message: "reason/Deleted ",
					},
				},
				monitorapi.EventInterval{
					Condition: monitorapi.Condition{
						Level:   monitorapi.Info,
						Locator: "ns/ pod/testpod-2 node/ uid/",
						Message: "reason/DeletedBeforeScheduling ",
					},
				},
			},
		},
		{
			name:       "delete_cluster_operator",
			apiVersion: "config.openshift.io/v1",
			objs: []runtime.Object{
				&configv1.ClusterOperator{
					TypeMeta: metav1.TypeMeta{
						Kind:       "ClusterOperator",
						APIVersion: "config.openshift.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:              "testclusteroperator-1",
						CreationTimestamp: metav1.Time{Time: time.Now().Add(time.Minute)},
					},
				},
				&configv1.ClusterOperator{
					TypeMeta: metav1.TypeMeta{
						Kind:       "ClusterOperator",
						APIVersion: "config.openshift.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:              "testclusteroperator-2",
						CreationTimestamp: metav1.Time{Time: time.Now().Add(time.Minute)},
					},
				},
			},
			expected: monitorapi.Intervals{
				monitorapi.EventInterval{
					Condition: monitorapi.Condition{
						Level:   monitorapi.Warning,
						Locator: "clusteroperator/testclusteroperator-1",
						Message: "deleted",
					},
				},
				monitorapi.EventInterval{
					Condition: monitorapi.Condition{
						Level:   monitorapi.Warning,
						Locator: "clusteroperator/testclusteroperator-2",
						Message: "deleted",
					},
				},
			},
		},
		{
			name:       "delete_cluster_version",
			apiVersion: "config.openshift.io/v1",
			objs: []runtime.Object{
				&configv1.ClusterVersion{
					TypeMeta: metav1.TypeMeta{
						Kind:       "ClusterVersion",
						APIVersion: "config.openshift.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:              "testclusterversion-1",
						CreationTimestamp: metav1.Time{Time: time.Now().Add(time.Minute)},
					},
				},
				&configv1.ClusterVersion{
					TypeMeta: metav1.TypeMeta{
						Kind:       "ClusterVersion",
						APIVersion: "config.openshift.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:              "testclusterversion-2",
						CreationTimestamp: metav1.Time{Time: time.Now().Add(time.Minute)},
					},
				},
			},
			expected: monitorapi.Intervals{
				monitorapi.EventInterval{
					Condition: monitorapi.Condition{
						Level:   monitorapi.Warning,
						Locator: "clusterversion/testclusterversion-1",
						Message: "deleted",
					},
				},
				monitorapi.EventInterval{
					Condition: monitorapi.Condition{
						Level:   monitorapi.Warning,
						Locator: "clusterversion/testclusterversion-2",
						Message: "deleted",
					},
				},
			},
		},
		{
			name:       "delete_Authentication",
			apiVersion: "config.openshift.io/v1",
			objs: []runtime.Object{
				&configv1.Authentication{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Authentication",
						APIVersion: "config.openshift.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:              "testauthentication-1",
						CreationTimestamp: metav1.Time{Time: time.Now().Add(time.Minute)},
					},
				},
				&configv1.Authentication{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Authentication",
						APIVersion: "config.openshift.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:              "testauthentication-2",
						CreationTimestamp: metav1.Time{Time: time.Now().Add(time.Minute)},
					},
				},
			},
			expected: monitorapi.Intervals{
				monitorapi.EventInterval{
					Condition: monitorapi.Condition{
						Level:   monitorapi.Info,
						Locator: "Authentication/testauthentication-1",
						Message: "deleted",
					},
				},
				monitorapi.EventInterval{
					Condition: monitorapi.Condition{
						Level:   monitorapi.Info,
						Locator: "Authentication/testauthentication-2",
						Message: "deleted",
					},
				},
			},
		},
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Errorf("error getting working directory, err: %v", err)
	}
	artifactDir := fmt.Sprintf("%s/test_delete", wd)
	const mediaType = runtime.ContentTypeJSON
	info, ok := runtime.SerializerInfoForMediaType(coreCodecs.SupportedMediaTypes(), mediaType)
	if !ok {
		t.Errorf("unable to locate encoder -- %q is not a supported media type", mediaType)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := os.RemoveAll(artifactDir); err != nil {
				t.Errorf("Failed to remove output directory, err: %v", err)
			}
			if err := os.MkdirAll(artifactDir, os.ModePerm); err != nil {
				t.Errorf("Failed to create output directory, err: %v", err)
			}
			objList := []unstructured.Unstructured{}
			var enc runtime.Encoder
			if tt.apiVersion == "config.openshift.io/v1" {
				enc = coreCodecs.EncoderForVersion(info.Serializer, configv1.GroupVersion)
			} else {
				enc = coreCodecs.EncoderForVersion(info.Serializer, corev1.SchemeGroupVersion)
			}
			for _, obj := range tt.objs {
				objectBytes, err := runtime.Encode(enc, obj)
				if err != nil {
					t.Errorf("error: %v while encoding object: %v", err, obj)
				}
				var objUnstructured unstructured.Unstructured
				err = runtime.DecodeInto(unstructured.UnstructuredJSONScheme, objectBytes, &objUnstructured)
				if err != nil {
					t.Errorf("error: %v while decoding into unstructured object", err)
				}
				objList = append(objList, objUnstructured)
			}

			store, err := NewMonitorStorage(artifactDir, &kubernetes.Clientset{})
			if err != nil {
				t.Errorf("error calling NewMonitorStorage, err: %v", err)
			}
			for _, obj := range objList {
				store.OnDelete(&obj)
			}
			store.End()

			// Now analyze the result
			eventDir := fmt.Sprintf("%s/monitor-events", artifactDir)
			err = filepath.Walk(eventDir, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if strings.Contains(path, "e2e-events") {
					events, err := monitorserialization.EventsFromFile(path)
					if err != nil {
						return fmt.Errorf("error unmarshalling events: %v", err)
					}
					if len(events) != len(tt.expected) {
						return fmt.Errorf("len of events %d is not equal len of expected: %v", len(events), len(tt.expected))
					}
					for i := range events {
						if reflect.DeepEqual(events[i].Condition, tt.expected[i].Condition) == false {
							return fmt.Errorf("event at index %d: %+v is not equal to expected: %v", i, events[i], tt.expected[i])
						}
					}

					return nil
				}
				return nil
			})
			if err != nil {
				t.Errorf("test result analysis error: %+v", err)
			}
		})
	}
	if err := os.RemoveAll(artifactDir); err != nil {
		t.Errorf("Failed to remove output directory, err: %v", err)
	}
}
