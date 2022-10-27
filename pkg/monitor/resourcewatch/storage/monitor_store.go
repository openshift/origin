package storage

import (
	"context"
	"fmt"
	"os"
	"time"

	configv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/origin/pkg/monitor"
	"github.com/openshift/origin/pkg/monitor/monitorapi"

	corev1 "k8s.io/api/core/v1"
	_ "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

var (
	coreScheme = runtime.NewScheme()
	coreCodecs = serializer.NewCodecFactory(coreScheme)
)

type monitorStorage struct {
	monitor     *monitor.Monitor
	artifactDir string
	client      kubernetes.Interface
	startTime   time.Time
}

// NewMonitorStorage creates a monitor store for resource watch
func NewMonitorStorage(artifactDir string, client kubernetes.Interface) (ResourceWatchStore, error) {
	storage := &monitorStorage{
		monitor:     monitor.NewMonitorWithInterval(time.Hour),
		artifactDir: artifactDir,
		client:      client,
		startTime:   time.Now(),
	}

	return storage, nil
}

func decodeObject(objUnstructured *unstructured.Unstructured, obj runtime.Object) error {
	objectBytes, err := runtime.Encode(unstructured.UnstructuredJSONScheme, objUnstructured)
	if err != nil {
		return fmt.Errorf("encoding unstructured error: %v", err)
	}
	err = runtime.DecodeInto(coreCodecs.UniversalDecoder(corev1.SchemeGroupVersion), objectBytes, obj)
	if err != nil {
		return fmt.Errorf("decoding object error: %v", err)
	}
	return nil
}

func (s *monitorStorage) handleEvents(objUnstructured *unstructured.Unstructured) {
	eventObj := corev1.Event{}
	err := decodeObject(objUnstructured, &eventObj)
	if err != nil {
		klog.Warningf("Decoding %s failed with error: %v", objUnstructured.GetName(), err)
	}
	monitor.EventAddOrUpdateFunc(context.TODO(), s.monitor, s.client, s.startTime, &eventObj)
}

// OnAdd processes add event for all monitored resources
func (s *monitorStorage) OnAdd(obj interface{}) {
	objUnstructured, ok := obj.(*unstructured.Unstructured)
	if !ok {
		klog.Warningf("Object is not unstructured: %v", obj)
	}
	gvk := objUnstructured.GroupVersionKind()
	switch {
	case gvk.Group == "" && gvk.Kind == "Node":
		{
			nodeObj := corev1.Node{}
			err := decodeObject(objUnstructured, &nodeObj)
			if err != nil {
				klog.Warningf("Decoding %s failed with error: %v", objUnstructured.GetName(), err)
			}
			monitor.NodeAddFunc(nodeObj)
		}
	case gvk.Group == "" && gvk.Kind == "Pod":
		{
			podObj := corev1.Pod{}
			err := decodeObject(objUnstructured, &podObj)
			if err != nil {
				klog.Warningf("Decoding %s failed with error: %v", objUnstructured.GetName(), err)
			}
			monitor.PodAddFunc(s.monitor, &podObj)
		}
	case gvk.Group == "" && gvk.Kind == "Event":
		s.handleEvents(objUnstructured)
	case gvk.Group == "config.openshift.io" && gvk.Kind == "ClusterOperator":
		{
			coObj := configv1.ClusterOperator{}
			err := decodeObject(objUnstructured, &coObj)
			if err != nil {
				klog.Warningf("Decoding %s failed with error: %v", objUnstructured.GetName(), err)
			}
			monitor.ClusterOperatorAddFunc(s.monitor, s.startTime, &coObj)
		}
	case gvk.Group == "config.openshift.io" && gvk.Kind == "ClusterVersion":
		{
			cvObj := configv1.ClusterVersion{}
			err := decodeObject(objUnstructured, &cvObj)
			if err != nil {
				klog.Warningf("Decoding %s failed with error: %v", objUnstructured.GetName(), err)
			}
			monitor.ClusterVersionAddFunc(s.monitor, s.startTime, &cvObj)
		}
	default:
		{
			name := objUnstructured.GetName()
			s.monitor.Record(monitorapi.Condition{
				Level:   monitorapi.Info,
				Locator: fmt.Sprintf("%s/%s", gvk.Kind, name),
				Message: fmt.Sprintf("added"),
			})
		}
	}
}

// OnUpdate processes update event for all monitored resources
func (s *monitorStorage) OnUpdate(old, obj interface{}) {
	objUnstructured, ok := obj.(*unstructured.Unstructured)
	if !ok {
		klog.Warningf("Object is not unstructured: %v", obj)
	}
	oldObjUnstructured, ok := old.(*unstructured.Unstructured)
	if !ok {
		klog.Warningf("Old object is not unstructured: %v", obj)
	}
	gvk := objUnstructured.GroupVersionKind()
	switch {
	case gvk.Group == "" && gvk.Kind == "Node":
		{
			nodeObj := corev1.Node{}
			err := decodeObject(objUnstructured, &nodeObj)
			if err != nil {
				klog.Warningf("Decoding %s failed with error: %v", objUnstructured.GetName(), err)
			}

			oldObj := corev1.Node{}
			err = decodeObject(oldObjUnstructured, &oldObj)
			if err != nil {
				klog.Warningf("Decoding %s failed with error: %v", oldObjUnstructured.GetName(), err)
			}
			monitor.NodeUpdateFunc(s.monitor, &oldObj, &nodeObj)

		}
	case gvk.Group == "" && gvk.Kind == "Pod":
		{
			podObj := corev1.Pod{}
			err := decodeObject(objUnstructured, &podObj)
			if err != nil {
				klog.Warningf("Decoding %s failed with error: %v", objUnstructured.GetName(), err)
			}

			oldObj := corev1.Pod{}
			err = decodeObject(oldObjUnstructured, &oldObj)
			if err != nil {
				klog.Warningf("Decoding %s failed with error: %v", oldObjUnstructured.GetName(), err)
			}
			monitor.PodUpdateFunc(s.monitor, &oldObj, &podObj)
		}
	case gvk.Group == "" && gvk.Kind == "Event":
		s.handleEvents(objUnstructured)
	case gvk.Group == "config.openshift.io" && gvk.Kind == "ClusterOperator":
		{
			coObj := configv1.ClusterOperator{}
			err := decodeObject(objUnstructured, &coObj)
			if err != nil {
				klog.Warningf("Decoding %s failed with error: %v", objUnstructured.GetName(), err)
			}

			oldObj := configv1.ClusterOperator{}
			err = decodeObject(oldObjUnstructured, &oldObj)
			if err != nil {
				klog.Warningf("Decoding %s failed with error: %v", oldObjUnstructured.GetName(), err)
			}
			monitor.ClusterOperatorUpdateFunc(s.monitor, &oldObj, &coObj)
		}
	case gvk.Group == "config.openshift.io" && gvk.Kind == "ClusterVersion":
		{
			cvObj := configv1.ClusterVersion{}
			err := decodeObject(objUnstructured, &cvObj)
			if err != nil {
				klog.Warningf("Decoding %s failed with error: %v", objUnstructured.GetName(), err)
			}

			oldObj := configv1.ClusterVersion{}
			err = decodeObject(oldObjUnstructured, &oldObj)
			if err != nil {
				klog.Warningf("Decoding %s failed with error: %v", oldObjUnstructured.GetName(), err)
			}
			monitor.ClusterVersionUpdateFunc(s.monitor, &oldObj, &cvObj)
		}
	default:
		{
			name := objUnstructured.GetName()
			s.monitor.Record(monitorapi.Condition{
				Level:   monitorapi.Info,
				Locator: fmt.Sprintf("%s/%s", gvk.Kind, name),
				Message: fmt.Sprintf("updated"),
			})
		}
	}
}

// OnDelete processes delete event for all monitored resources
func (s *monitorStorage) OnDelete(obj interface{}) {
	objUnstructured, ok := obj.(*unstructured.Unstructured)
	if !ok {
		klog.Warningf("Object is not unstructured: %v", obj)
	}
	gvk := objUnstructured.GroupVersionKind()
	switch {
	case gvk.Group == "" && gvk.Kind == "Node":
		{
			nodeObj := corev1.Node{}
			err := decodeObject(objUnstructured, &nodeObj)
			if err != nil {
				klog.Warningf("Decoding %s failed with error: %v", objUnstructured.GetName(), err)
			}
			monitor.NodeDeleteFunc(s.monitor, &nodeObj)
		}
	case gvk.Group == "" && gvk.Kind == "Pod":
		{
			podObj := corev1.Pod{}
			err := decodeObject(objUnstructured, &podObj)
			if err != nil {
				klog.Warningf("Decoding %s failed with error: %v", objUnstructured.GetName(), err)
			}
			monitor.PodDeleteFunc(s.monitor, &podObj)
		}
	case gvk.Group == "config.openshift.io" && gvk.Kind == "ClusterOperator":
		{
			coObj := configv1.ClusterOperator{}
			err := decodeObject(objUnstructured, &coObj)
			if err != nil {
				klog.Warningf("Decoding %s failed with error: %v", objUnstructured.GetName(), err)
			}

			monitor.ClusterOperatorDeleteFunc(s.monitor, &coObj)
		}
	case gvk.Group == "config.openshift.io" && gvk.Kind == "ClusterVersion":
		{
			cvObj := configv1.ClusterVersion{}
			err := decodeObject(objUnstructured, &cvObj)
			if err != nil {
				klog.Warningf("Decoding %s failed with error: %v", objUnstructured.GetName(), err)
			}

			monitor.ClusterVersionDeleteFunc(s.monitor, &cvObj)
		}
	default:
		{
			name := objUnstructured.GetName()
			s.monitor.Record(monitorapi.Condition{
				Level:   monitorapi.Info,
				Locator: fmt.Sprintf("%s/%s", gvk.Kind, name),
				Message: fmt.Sprintf("deleted"),
			})
		}
	}
}

// End writes captured events to disc
func (s *monitorStorage) End() {
	recordedEvents := s.monitor.Intervals(time.Time{}, time.Time{})
	recordedResources := s.monitor.CurrentResourceState()
	timeSuffix := fmt.Sprintf("_%s", time.Now().UTC().Format("20060102-150405"))

	eventDir := fmt.Sprintf("%s/monitor-events", s.artifactDir)
	if err := os.MkdirAll(eventDir, os.ModePerm); err != nil {
		klog.Warningf("Failed to create monitor-events directory, err: %v", err)
		return
	}
	err := monitor.WriteEventsForJobRun(eventDir, recordedResources, recordedEvents, timeSuffix)
	if err != nil {
		klog.Warningf("Failed to write event data, err: %v", err)
		return
	}
}

func init() {
	if err := corev1.AddToScheme(coreScheme); err != nil {
		panic(err)
	}
	if err := configv1.Install(coreScheme); err != nil {
		panic(err)
	}
}
