package migrators

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	clientgotesting "k8s.io/client-go/testing"
)

func TestInprocessProcessor(t *testing.T) {
	scenarios := []struct {
		name         string
		workerFunc   func(*unstructured.Unstructured) error
		validateFunc func(ts *testing.T, actions []clientgotesting.Action, count int, err error)
		resources    []runtime.Object
		gvr          schema.GroupVersionResource
	}{
		// scenario 1:
		{
			name: "worker function is executed",
			workerFunc: func(obj *unstructured.Unstructured) error {
				if obj.GetKind() != "Secret" {
					return fmt.Errorf("incorrect kind %v", obj.GetKind())
				}
				return nil
			},
			validateFunc: func(ts *testing.T, actions []clientgotesting.Action, count int, err error) {
				if err != nil {
					t.Error(err)
				}
				if err := validateActionsVerbs(actions, []string{"list:secrets"}); err != nil {
					t.Error(err)
				}
				if count != 100 {
					t.Errorf("workerFunc haven't seen 100 only %d", count)
				}
			},
			resources: func() []runtime.Object {
				ret := []runtime.Object{}
				ret = append(ret, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "cm1", Namespace: "ns1"}})
				ret = append(ret, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "cm2", Namespace: "ns1"}})
				ret = append(ret, createSecrets(100)...)
				return ret
			}(),
			gvr: schema.GroupResource{Resource: "secrets"}.WithVersion("v1"),
		},

		// scenario 2:
		{
			name: "handles panic",
			workerFunc: func(obj *unstructured.Unstructured) error {
				panic("nasty panic")
			},
			validateFunc: func(ts *testing.T, actions []clientgotesting.Action, count int, err error) {
				if err == nil {
					t.Error("expected to receive an error")
				}
				if err := validateActionsVerbs(actions, []string{"list:secrets"}); err != nil {
					t.Error(err)
				}
			},
			resources: func() []runtime.Object {
				ret := []runtime.Object{}
				ret = append(ret, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "cm1", Namespace: "ns1"}})
				ret = append(ret, createSecrets(100)...)
				return ret
			}(),
			gvr: schema.GroupResource{Resource: "secrets"}.WithVersion("v1"),
		},

		// scenario 3:
		{
			name: "handles more than one page (default is 500 items)",
			workerFunc: func(obj *unstructured.Unstructured) error {
				if obj.GetKind() != "Secret" {
					return fmt.Errorf("incorrect kind %v", obj.GetKind())
				}
				return nil
			},
			validateFunc: func(ts *testing.T, actions []clientgotesting.Action, count int, err error) {
				if err != nil {
					t.Error(err)
				}
				if err := validateActionsVerbs(actions, []string{"list:secrets"}); err != nil {
					t.Error(err)
				}
				if count != 500*4 {
					t.Errorf("workerFunc haven't seen all 500 * 4 only %d", count)
				}
			},
			resources: func() []runtime.Object {
				ret := []runtime.Object{}
				ret = append(ret, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "cm1", Namespace: "ns1"}})
				ret = append(ret, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "cm2", Namespace: "ns1"}})
				ret = append(ret, createSecrets(500*4)...)
				return ret
			}(),
			gvr: schema.GroupResource{Resource: "secrets"}.WithVersion("v1"),
		},

		// scenario 4:
		{
			name: "handles an empty list",
			workerFunc: func(obj *unstructured.Unstructured) error {
				return fmt.Errorf("an empty list passed but received %v", obj)
			},
			validateFunc: func(ts *testing.T, actions []clientgotesting.Action, count int, err error) {
				if err != nil {
					t.Error(err)
				}
				if err := validateActionsVerbs(actions, []string{"list:secrets"}); err != nil {
					t.Error(err)
				}
				if count != 0 {
					t.Errorf("workerFunc seen %d object", count)
				}
			},
			resources: func() []runtime.Object {
				ret := []runtime.Object{}
				ret = append(ret, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "cm1", Namespace: "ns1"}})
				ret = append(ret, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "cm2", Namespace: "ns1"}})
				return ret
			}(),
			gvr: schema.GroupResource{Resource: "secrets"}.WithVersion("v1"),
		},

		// scenario 5:
		{
			name: "stops further processing on worker error",
			workerFunc: func(obj *unstructured.Unstructured) error {
				if obj.GetKind() != "Secret" {
					return fmt.Errorf("incorrect kind %v", obj.GetKind())
				}
				return fmt.Errorf("fake error for %v", obj.GetName())
			},
			validateFunc: func(ts *testing.T, actions []clientgotesting.Action, count int, err error) {
				if err == nil {
					t.Error("expected to receive an error but none was returned")
				}
				if err := validateActionsVerbs(actions, []string{"list:secrets"}); err != nil {
					t.Error(err)
				}
				// it is hard to give an exact number because we don't know how many workers are progressing
				// mainly due to propagation time (closing `onWorkerErrorCtx` which propagates the stop signal to `workCh`)
				if count >= 30 {
					t.Errorf("workerFunc shouldn't have processed >= %d items, expected < 30 ", count)
				}
			},
			resources: func() []runtime.Object {
				ret := []runtime.Object{}
				ret = append(ret, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "cm1", Namespace: "ns1"}})
				ret = append(ret, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "cm2", Namespace: "ns1"}})
				ret = append(ret, createSecrets(500*4)...)
				return ret
			}(),
			gvr: schema.GroupResource{Resource: "secrets"}.WithVersion("v1"),
		},
	}
	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			// prepare
			scheme := runtime.NewScheme()
			unstructuredObjs := []runtime.Object{}
			for _, rawObject := range scenario.resources {
				rawUnstructured, err := runtime.DefaultUnstructuredConverter.ToUnstructured(rawObject.DeepCopyObject())
				if err != nil {
					t.Fatal(err)
				}
				unstructured.SetNestedField(rawUnstructured, "v1", "apiVersion")
				unstructured.SetNestedField(rawUnstructured, reflect.TypeOf(rawObject).Elem().Name(), "kind")
				unstructuredObjs = append(unstructuredObjs, &unstructured.Unstructured{Object: rawUnstructured})
			}
			dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme, unstructuredObjs...)

			// act
			totalCountCh := make(chan int)
			listProcessor := newListProcessor(context.TODO(), dynamicClient, func(obj *unstructured.Unstructured) error {
				totalCountCh <- 1
				return scenario.workerFunc(obj)
			})

			// validate
			totalCount := 0
			wg := sync.WaitGroup{}
			wg.Add(1)
			go func() {
				defer wg.Done()
				for i := range totalCountCh {
					totalCount += i
				}
			}()

			err := listProcessor.run(scenario.gvr)
			close(totalCountCh)
			wg.Wait()
			scenario.validateFunc(t, dynamicClient.Actions(), totalCount, err)
		})
	}
}

func TestInprocessProcessorContextCancellation(t *testing.T) {
	// prepare
	ctx, cancelCtxFn := context.WithCancel(context.TODO())
	lock := sync.Mutex{}

	workerFunc := func(obj *unstructured.Unstructured) error {
		lock.Lock()
		defer lock.Unlock()
		time.Sleep(100 * time.Millisecond)
		cancelCtxFn()
		return nil
	}

	resources := func() []runtime.Object {
		ret := []runtime.Object{}
		ret = append(ret, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "cm1", Namespace: "ns1"}})
		ret = append(ret, createSecrets(500*4)...)
		return ret
	}()
	gvr := schema.GroupResource{Resource: "secrets"}.WithVersion("v1")

	scheme := runtime.NewScheme()
	unstructuredObjs := []runtime.Object{}
	for _, rawObject := range resources {
		rawUnstructured, err := runtime.DefaultUnstructuredConverter.ToUnstructured(rawObject.DeepCopyObject())
		if err != nil {
			t.Fatal(err)
		}
		unstructured.SetNestedField(rawUnstructured, "v1", "apiVersion")
		unstructured.SetNestedField(rawUnstructured, reflect.TypeOf(rawObject).Elem().Name(), "kind")
		unstructuredObjs = append(unstructuredObjs, &unstructured.Unstructured{Object: rawUnstructured})
	}
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme, unstructuredObjs...)

	// act
	listProcessor := newListProcessor(ctx, dynamicClient, func(obj *unstructured.Unstructured) error {
		return workerFunc(obj)
	})

	// validate
	testTimeoutCh := time.After(6 * time.Second)
	testCompletedCh := make(chan bool)
	defer close(testCompletedCh)
	go func() {
		err := listProcessor.run(gvr)
		if err == nil {
			t.Error("expected to receive an error")
		}
		if err := validateActionsVerbs(dynamicClient.Actions(), []string{"list:secrets"}); err != nil {
			t.Error(err)
		}
		testCompletedCh <- true
	}()

	select {
	case <-testTimeoutCh:
		t.Fatal("timeout waiting for context propagation")
	case <-testCompletedCh:
	}
}

func validateActionsVerbs(actualActions []clientgotesting.Action, expectedActions []string) error {
	actionString := func(a clientgotesting.Action) string {
		return a.GetVerb() + ":" + a.GetResource().Resource
	}
	actionStrings := func(actions []clientgotesting.Action) []string {
		res := make([]string, 0, len(actions))
		for _, a := range actions {
			res = append(res, actionString(a))
		}
		return res
	}

	if len(actualActions) != len(expectedActions) {
		return fmt.Errorf("expected to get %d actions but got %d\nexpected=%v \n got=%v", len(expectedActions), len(actualActions), expectedActions, actionStrings(actualActions))
	}
	for i, a := range actualActions {
		if got, expected := actionString(a), expectedActions[i]; got != expected {
			return fmt.Errorf("at %d got %s, expected %s", i, got, expected)
		}
	}
	return nil
}

func createSecrets(count int) []runtime.Object {
	ret := make([]runtime.Object, count)
	for i := 0; i < count; i++ {
		ret[i] = &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("secret%d", i), Namespace: "ns2"}}
	}
	return ret
}
