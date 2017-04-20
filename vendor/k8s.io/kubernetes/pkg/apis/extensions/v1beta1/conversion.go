/*
Copyright 2015 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1beta1

import (
	"encoding/json"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/kubernetes/pkg/api"
	v1 "k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/apis/autoscaling"
	"k8s.io/kubernetes/pkg/apis/extensions"
)

func addConversionFuncs(scheme *runtime.Scheme) error {
	// Add non-generated conversion functions
	err := scheme.AddConversionFuncs(
		Convert_extensions_ScaleStatus_To_v1beta1_ScaleStatus,
		Convert_v1beta1_ScaleStatus_To_extensions_ScaleStatus,
		Convert_extensions_DeploymentSpec_To_v1beta1_DeploymentSpec,
		Convert_v1beta1_DeploymentSpec_To_extensions_DeploymentSpec,
		Convert_extensions_DeploymentStrategy_To_v1beta1_DeploymentStrategy,
		Convert_v1beta1_DeploymentStrategy_To_extensions_DeploymentStrategy,
		Convert_extensions_RollingUpdateDeployment_To_v1beta1_RollingUpdateDeployment,
		Convert_v1beta1_RollingUpdateDeployment_To_extensions_RollingUpdateDeployment,
		Convert_extensions_RollingUpdateDaemonSet_To_v1beta1_RollingUpdateDaemonSet,
		Convert_v1beta1_RollingUpdateDaemonSet_To_extensions_RollingUpdateDaemonSet,
		Convert_extensions_ReplicaSetSpec_To_v1beta1_ReplicaSetSpec,
		Convert_v1beta1_ReplicaSetSpec_To_extensions_ReplicaSetSpec,
		// autoscaling
		Convert_v1beta1_HorizontalPodAutoscaler_To_autoscaling_HorizontalPodAutoscaler,
		Convert_autoscaling_HorizontalPodAutoscaler_To_v1beta1_HorizontalPodAutoscaler,
		Convert_autoscaling_CrossVersionObjectReference_To_v1beta1_SubresourceReference,
		Convert_v1beta1_SubresourceReference_To_autoscaling_CrossVersionObjectReference,
		Convert_autoscaling_HorizontalPodAutoscalerSpec_To_v1beta1_HorizontalPodAutoscalerSpec,
		Convert_v1beta1_HorizontalPodAutoscalerSpec_To_autoscaling_HorizontalPodAutoscalerSpec,
		Convert_autoscaling_HorizontalPodAutoscalerStatus_To_v1beta1_HorizontalPodAutoscalerStatus,
		Convert_v1beta1_HorizontalPodAutoscalerStatus_To_autoscaling_HorizontalPodAutoscalerStatus,
	)
	if err != nil {
		return err
	}

	// Add field label conversions for kinds having selectable nothing but ObjectMeta fields.
	for _, k := range []string{"DaemonSet", "Deployment", "Ingress"} {
		kind := k // don't close over range variables
		err = scheme.AddFieldLabelConversionFunc("extensions/v1beta1", kind,
			func(label, value string) (string, string, error) {
				switch label {
				case "metadata.name", "metadata.namespace":
					return label, value, nil
				default:
					return "", "", fmt.Errorf("field label %q not supported for %q", label, kind)
				}
			},
		)
		if err != nil {
			return err
		}
	}

	return nil
}

func Convert_extensions_ScaleStatus_To_v1beta1_ScaleStatus(in *extensions.ScaleStatus, out *ScaleStatus, s conversion.Scope) error {
	out.Replicas = int32(in.Replicas)

	out.Selector = nil
	out.TargetSelector = ""
	if in.Selector != nil {
		if in.Selector.MatchExpressions == nil || len(in.Selector.MatchExpressions) == 0 {
			out.Selector = in.Selector.MatchLabels
		}

		selector, err := metav1.LabelSelectorAsSelector(in.Selector)
		if err != nil {
			return fmt.Errorf("invalid label selector: %v", err)
		}
		out.TargetSelector = selector.String()
	}
	return nil
}

func Convert_v1beta1_ScaleStatus_To_extensions_ScaleStatus(in *ScaleStatus, out *extensions.ScaleStatus, s conversion.Scope) error {
	out.Replicas = in.Replicas

	// Normally when 2 fields map to the same internal value we favor the old field, since
	// old clients can't be expected to know about new fields but clients that know about the
	// new field can be expected to know about the old field (though that's not quite true, due
	// to kubectl apply). However, these fields are readonly, so any non-nil value should work.
	if in.TargetSelector != "" {
		labelSelector, err := metav1.ParseToLabelSelector(in.TargetSelector)
		if err != nil {
			out.Selector = nil
			return fmt.Errorf("failed to parse target selector: %v", err)
		}
		out.Selector = labelSelector
	} else if in.Selector != nil {
		out.Selector = new(metav1.LabelSelector)
		selector := make(map[string]string)
		for key, val := range in.Selector {
			selector[key] = val
		}
		out.Selector.MatchLabels = selector
	} else {
		out.Selector = nil
	}
	return nil
}

func Convert_extensions_DeploymentSpec_To_v1beta1_DeploymentSpec(in *extensions.DeploymentSpec, out *DeploymentSpec, s conversion.Scope) error {
	out.Replicas = &in.Replicas
	out.Selector = in.Selector
	if err := v1.Convert_api_PodTemplateSpec_To_v1_PodTemplateSpec(&in.Template, &out.Template, s); err != nil {
		return err
	}
	if err := Convert_extensions_DeploymentStrategy_To_v1beta1_DeploymentStrategy(&in.Strategy, &out.Strategy, s); err != nil {
		return err
	}
	if in.RevisionHistoryLimit != nil {
		out.RevisionHistoryLimit = new(int32)
		*out.RevisionHistoryLimit = int32(*in.RevisionHistoryLimit)
	}
	out.MinReadySeconds = int32(in.MinReadySeconds)
	out.Paused = in.Paused
	if in.RollbackTo != nil {
		out.RollbackTo = new(RollbackConfig)
		out.RollbackTo.Revision = int64(in.RollbackTo.Revision)
	} else {
		out.RollbackTo = nil
	}
	if in.ProgressDeadlineSeconds != nil {
		out.ProgressDeadlineSeconds = new(int32)
		*out.ProgressDeadlineSeconds = *in.ProgressDeadlineSeconds
	}
	return nil
}

func Convert_v1beta1_DeploymentSpec_To_extensions_DeploymentSpec(in *DeploymentSpec, out *extensions.DeploymentSpec, s conversion.Scope) error {
	if in.Replicas != nil {
		out.Replicas = *in.Replicas
	}
	out.Selector = in.Selector
	if err := v1.Convert_v1_PodTemplateSpec_To_api_PodTemplateSpec(&in.Template, &out.Template, s); err != nil {
		return err
	}
	if err := Convert_v1beta1_DeploymentStrategy_To_extensions_DeploymentStrategy(&in.Strategy, &out.Strategy, s); err != nil {
		return err
	}
	out.RevisionHistoryLimit = in.RevisionHistoryLimit
	out.MinReadySeconds = in.MinReadySeconds
	out.Paused = in.Paused
	if in.RollbackTo != nil {
		out.RollbackTo = new(extensions.RollbackConfig)
		out.RollbackTo.Revision = in.RollbackTo.Revision
	} else {
		out.RollbackTo = nil
	}
	if in.ProgressDeadlineSeconds != nil {
		out.ProgressDeadlineSeconds = new(int32)
		*out.ProgressDeadlineSeconds = *in.ProgressDeadlineSeconds
	}
	return nil
}

func Convert_extensions_DeploymentStrategy_To_v1beta1_DeploymentStrategy(in *extensions.DeploymentStrategy, out *DeploymentStrategy, s conversion.Scope) error {
	out.Type = DeploymentStrategyType(in.Type)
	if in.RollingUpdate != nil {
		out.RollingUpdate = new(RollingUpdateDeployment)
		if err := Convert_extensions_RollingUpdateDeployment_To_v1beta1_RollingUpdateDeployment(in.RollingUpdate, out.RollingUpdate, s); err != nil {
			return err
		}
	} else {
		out.RollingUpdate = nil
	}
	return nil
}

func Convert_v1beta1_DeploymentStrategy_To_extensions_DeploymentStrategy(in *DeploymentStrategy, out *extensions.DeploymentStrategy, s conversion.Scope) error {
	out.Type = extensions.DeploymentStrategyType(in.Type)
	if in.RollingUpdate != nil {
		out.RollingUpdate = new(extensions.RollingUpdateDeployment)
		if err := Convert_v1beta1_RollingUpdateDeployment_To_extensions_RollingUpdateDeployment(in.RollingUpdate, out.RollingUpdate, s); err != nil {
			return err
		}
	} else {
		out.RollingUpdate = nil
	}
	return nil
}

func Convert_extensions_RollingUpdateDeployment_To_v1beta1_RollingUpdateDeployment(in *extensions.RollingUpdateDeployment, out *RollingUpdateDeployment, s conversion.Scope) error {
	if out.MaxUnavailable == nil {
		out.MaxUnavailable = &intstr.IntOrString{}
	}
	if err := s.Convert(&in.MaxUnavailable, out.MaxUnavailable, 0); err != nil {
		return err
	}
	if out.MaxSurge == nil {
		out.MaxSurge = &intstr.IntOrString{}
	}
	if err := s.Convert(&in.MaxSurge, out.MaxSurge, 0); err != nil {
		return err
	}
	return nil
}

func Convert_v1beta1_RollingUpdateDeployment_To_extensions_RollingUpdateDeployment(in *RollingUpdateDeployment, out *extensions.RollingUpdateDeployment, s conversion.Scope) error {
	if err := s.Convert(in.MaxUnavailable, &out.MaxUnavailable, 0); err != nil {
		return err
	}
	if err := s.Convert(in.MaxSurge, &out.MaxSurge, 0); err != nil {
		return err
	}
	return nil
}

func Convert_extensions_RollingUpdateDaemonSet_To_v1beta1_RollingUpdateDaemonSet(in *extensions.RollingUpdateDaemonSet, out *RollingUpdateDaemonSet, s conversion.Scope) error {
	if out.MaxUnavailable == nil {
		out.MaxUnavailable = &intstr.IntOrString{}
	}
	if err := s.Convert(&in.MaxUnavailable, out.MaxUnavailable, 0); err != nil {
		return err
	}
	return nil
}

func Convert_v1beta1_RollingUpdateDaemonSet_To_extensions_RollingUpdateDaemonSet(in *RollingUpdateDaemonSet, out *extensions.RollingUpdateDaemonSet, s conversion.Scope) error {
	if err := s.Convert(in.MaxUnavailable, &out.MaxUnavailable, 0); err != nil {
		return err
	}
	return nil
}

func Convert_extensions_ReplicaSetSpec_To_v1beta1_ReplicaSetSpec(in *extensions.ReplicaSetSpec, out *ReplicaSetSpec, s conversion.Scope) error {
	out.Replicas = new(int32)
	*out.Replicas = int32(in.Replicas)
	out.MinReadySeconds = in.MinReadySeconds
	out.Selector = in.Selector
	if err := v1.Convert_api_PodTemplateSpec_To_v1_PodTemplateSpec(&in.Template, &out.Template, s); err != nil {
		return err
	}
	return nil
}

func Convert_v1beta1_ReplicaSetSpec_To_extensions_ReplicaSetSpec(in *ReplicaSetSpec, out *extensions.ReplicaSetSpec, s conversion.Scope) error {
	if in.Replicas != nil {
		out.Replicas = *in.Replicas
	}
	out.MinReadySeconds = in.MinReadySeconds
	out.Selector = in.Selector
	if err := v1.Convert_v1_PodTemplateSpec_To_api_PodTemplateSpec(&in.Template, &out.Template, s); err != nil {
		return err
	}
	return nil
}

func Convert_autoscaling_CrossVersionObjectReference_To_v1beta1_SubresourceReference(in *autoscaling.CrossVersionObjectReference, out *SubresourceReference, s conversion.Scope) error {
	out.Kind = in.Kind
	out.Name = in.Name
	out.APIVersion = in.APIVersion
	out.Subresource = "scale"
	return nil
}

func Convert_v1beta1_SubresourceReference_To_autoscaling_CrossVersionObjectReference(in *SubresourceReference, out *autoscaling.CrossVersionObjectReference, s conversion.Scope) error {
	out.Kind = in.Kind
	out.Name = in.Name
	out.APIVersion = in.APIVersion
	return nil
}

func Convert_autoscaling_HorizontalPodAutoscaler_To_v1beta1_HorizontalPodAutoscaler(in *autoscaling.HorizontalPodAutoscaler, out *HorizontalPodAutoscaler, s conversion.Scope) error {
	if err := autoConvert_autoscaling_HorizontalPodAutoscaler_To_v1beta1_HorizontalPodAutoscaler(in, out, s); err != nil {
		return err
	}

	otherMetrics := make([]MetricSpec, 0, len(in.Spec.Metrics))
	for _, metric := range in.Spec.Metrics {
		if metric.Type == autoscaling.ResourceMetricSourceType && metric.Resource != nil && metric.Resource.Name == api.ResourceCPU && metric.Resource.TargetAverageUtilization != nil {
			continue
		}

		convMetric := MetricSpec{}
		if err := Convert_autoscaling_MetricSpec_To_v1beta1_MetricSpec(&metric, &convMetric, s); err != nil {
			return err
		}
		otherMetrics = append(otherMetrics, convMetric)
	}

	// NB: we need to save the status even if it maps to a CPU utilization status in order to save the raw value as well
	currentMetrics := make([]MetricStatus, len(in.Status.CurrentMetrics))
	for i, currentMetric := range in.Status.CurrentMetrics {
		if err := Convert_autoscaling_MetricStatus_To_v1beta1_MetricStatus(&currentMetric, &currentMetrics[i], s); err != nil {
			return err
		}
	}

	if len(otherMetrics) > 0 || len(in.Status.CurrentMetrics) > 0 {
		old := out.Annotations
		out.Annotations = make(map[string]string, len(old)+2)
		if old != nil {
			for k, v := range old {
				out.Annotations[k] = v
			}
		}
	}

	if len(otherMetrics) > 0 {
		otherMetricsEnc, err := json.Marshal(otherMetrics)
		if err != nil {
			return err
		}
		out.Annotations[autoscaling.MetricSpecsAnnotation] = string(otherMetricsEnc)
	}

	if len(in.Status.CurrentMetrics) > 0 {
		currentMetricsEnc, err := json.Marshal(currentMetrics)
		if err != nil {
			return err
		}
		out.Annotations[autoscaling.MetricStatusesAnnotation] = string(currentMetricsEnc)
	}

	return nil
}

func Convert_v1beta1_HorizontalPodAutoscaler_To_autoscaling_HorizontalPodAutoscaler(in *HorizontalPodAutoscaler, out *autoscaling.HorizontalPodAutoscaler, s conversion.Scope) error {
	if err := autoConvert_v1beta1_HorizontalPodAutoscaler_To_autoscaling_HorizontalPodAutoscaler(in, out, s); err != nil {
		return err
	}

	if otherMetricsEnc, hasOtherMetrics := out.Annotations[autoscaling.MetricSpecsAnnotation]; hasOtherMetrics {
		var otherMetrics []MetricSpec
		if err := json.Unmarshal([]byte(otherMetricsEnc), &otherMetrics); err != nil {
			return err
		}

		// the normal Spec conversion could have populated out.Spec.Metrics with a single element, so deal with that
		outMetrics := make([]autoscaling.MetricSpec, len(otherMetrics)+len(out.Spec.Metrics))
		for i, metric := range otherMetrics {
			if err := Convert_v1beta1_MetricSpec_To_autoscaling_MetricSpec(&metric, &outMetrics[i], s); err != nil {
				return err
			}
		}
		if out.Spec.Metrics != nil {
			outMetrics[len(otherMetrics)] = out.Spec.Metrics[0]
		}
		out.Spec.Metrics = outMetrics
		delete(out.Annotations, autoscaling.MetricSpecsAnnotation)
	}

	if currentMetricsEnc, hasCurrentMetrics := out.Annotations[autoscaling.MetricStatusesAnnotation]; hasCurrentMetrics {
		// ignore any existing status values -- the ones here have more information
		var currentMetrics []MetricStatus
		if err := json.Unmarshal([]byte(currentMetricsEnc), &currentMetrics); err != nil {
			return err
		}

		out.Status.CurrentMetrics = make([]autoscaling.MetricStatus, len(currentMetrics))
		for i, currentMetric := range currentMetrics {
			if err := Convert_v1beta1_MetricStatus_To_autoscaling_MetricStatus(&currentMetric, &out.Status.CurrentMetrics[i], s); err != nil {
				return err
			}
		}
		delete(out.Annotations, autoscaling.MetricStatusesAnnotation)
	}

	// autoscaling/v1 formerly had an implicit default applied in the controller.  In v2alpha1, we apply it explicitly.
	// We apply it here, explicitly, since we have access to the full set of metrics from the annotation.
	if len(out.Spec.Metrics) == 0 {
		// no other metrics, no explicit CPU value set
		out.Spec.Metrics = []autoscaling.MetricSpec{
			{
				Type: autoscaling.ResourceMetricSourceType,
				Resource: &autoscaling.ResourceMetricSource{
					Name: api.ResourceCPU,
				},
			},
		}
		out.Spec.Metrics[0].Resource.TargetAverageUtilization = new(int32)
		*out.Spec.Metrics[0].Resource.TargetAverageUtilization = autoscaling.DefaultCPUUtilization
	}

	return nil
}

func Convert_autoscaling_HorizontalPodAutoscalerSpec_To_v1beta1_HorizontalPodAutoscalerSpec(in *autoscaling.HorizontalPodAutoscalerSpec, out *HorizontalPodAutoscalerSpec, s conversion.Scope) error {
	if err := Convert_autoscaling_CrossVersionObjectReference_To_v1beta1_SubresourceReference(&in.ScaleTargetRef, &out.ScaleRef, s); err != nil {
		return err
	}
	if in.MinReplicas != nil {
		out.MinReplicas = new(int32)
		*out.MinReplicas = *in.MinReplicas
	} else {
		out.MinReplicas = nil
	}
	out.MaxReplicas = in.MaxReplicas

	for _, metric := range in.Metrics {
		if metric.Type == autoscaling.ResourceMetricSourceType && metric.Resource != nil && metric.Resource.Name == api.ResourceCPU {
			if metric.Resource.TargetAverageUtilization != nil {
				out.CPUUtilization = &CPUTargetUtilization{TargetPercentage: *metric.Resource.TargetAverageUtilization}
			}
			break
		}
	}

	return nil
}

func Convert_v1beta1_HorizontalPodAutoscalerSpec_To_autoscaling_HorizontalPodAutoscalerSpec(in *HorizontalPodAutoscalerSpec, out *autoscaling.HorizontalPodAutoscalerSpec, s conversion.Scope) error {
	if err := Convert_v1beta1_SubresourceReference_To_autoscaling_CrossVersionObjectReference(&in.ScaleRef, &out.ScaleTargetRef, s); err != nil {
		return err
	}
	if in.MinReplicas != nil {
		out.MinReplicas = new(int32)
		*out.MinReplicas = int32(*in.MinReplicas)
	} else {
		out.MinReplicas = nil
	}
	out.MaxReplicas = int32(in.MaxReplicas)

	if in.CPUUtilization != nil {
		out.Metrics = []autoscaling.MetricSpec{
			{
				Type: autoscaling.ResourceMetricSourceType,
				Resource: &autoscaling.ResourceMetricSource{
					Name: api.ResourceCPU,
				},
			},
		}
		out.Metrics[0].Resource.TargetAverageUtilization = new(int32)
		*out.Metrics[0].Resource.TargetAverageUtilization = in.CPUUtilization.TargetPercentage
	}

	return nil
}

func Convert_autoscaling_HorizontalPodAutoscalerStatus_To_v1beta1_HorizontalPodAutoscalerStatus(in *autoscaling.HorizontalPodAutoscalerStatus, out *HorizontalPodAutoscalerStatus, s conversion.Scope) error {
	out.ObservedGeneration = in.ObservedGeneration
	out.LastScaleTime = in.LastScaleTime

	out.CurrentReplicas = in.CurrentReplicas
	out.DesiredReplicas = in.DesiredReplicas

	for _, metric := range in.CurrentMetrics {
		if metric.Type == autoscaling.ResourceMetricSourceType && metric.Resource != nil && metric.Resource.Name == api.ResourceCPU {
			if metric.Resource.CurrentAverageUtilization != nil {

				out.CurrentCPUUtilizationPercentage = new(int32)
				*out.CurrentCPUUtilizationPercentage = *metric.Resource.CurrentAverageUtilization
			}
		}
	}
	return nil
}

func Convert_v1beta1_HorizontalPodAutoscalerStatus_To_autoscaling_HorizontalPodAutoscalerStatus(in *HorizontalPodAutoscalerStatus, out *autoscaling.HorizontalPodAutoscalerStatus, s conversion.Scope) error {
	out.ObservedGeneration = in.ObservedGeneration
	out.LastScaleTime = in.LastScaleTime

	out.CurrentReplicas = in.CurrentReplicas
	out.DesiredReplicas = in.DesiredReplicas

	if in.CurrentCPUUtilizationPercentage != nil {
		out.CurrentMetrics = []autoscaling.MetricStatus{
			{
				Type: autoscaling.ResourceMetricSourceType,
				Resource: &autoscaling.ResourceMetricStatus{
					Name: api.ResourceCPU,
				},
			},
		}
		out.CurrentMetrics[0].Resource.CurrentAverageUtilization = new(int32)
		*out.CurrentMetrics[0].Resource.CurrentAverageUtilization = *in.CurrentCPUUtilizationPercentage
	}
	return nil
}
