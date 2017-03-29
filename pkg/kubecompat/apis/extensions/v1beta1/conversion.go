package v1beta1

import (
	"encoding/json"
	"unsafe"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/kubernetes/pkg/api"
	api_v1 "k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/apis/autoscaling"
)

func addConversionFuncs(scheme *runtime.Scheme) error {
	// Add non-generated conversion functions
	err := scheme.AddConversionFuncs(
		Convert_autoscaling_HorizontalPodAutoscaler_To_v1beta1_HorizontalPodAutoscaler,
		Convert_v1beta1_HorizontalPodAutoscaler_To_autoscaling_HorizontalPodAutoscaler,
		Convert_autoscaling_CrossVersionObjectReference_To_v1beta1_SubresourceReference,
		Convert_v1beta1_SubresourceReference_To_autoscaling_CrossVersionObjectReference,
		Convert_autoscaling_HorizontalPodAutoscalerSpec_To_v1beta1_HorizontalPodAutoscalerSpec,
		Convert_v1beta1_HorizontalPodAutoscalerSpec_To_autoscaling_HorizontalPodAutoscalerSpec,
		Convert_autoscaling_HorizontalPodAutoscalerStatus_To_v1beta1_HorizontalPodAutoscalerStatus,
		Convert_v1beta1_HorizontalPodAutoscalerStatus_To_autoscaling_HorizontalPodAutoscalerStatus,
		Convert_v1beta1_MetricSpec_To_autoscaling_MetricSpec,
		Convert_autoscaling_MetricSpec_To_v1beta1_MetricSpec,
		Convert_v1beta1_MetricStatus_To_autoscaling_MetricStatus,
		Convert_autoscaling_MetricStatus_To_v1beta1_MetricStatus,
		Convert_v1beta1_ObjectMetricSource_To_autoscaling_ObjectMetricSource,
		Convert_autoscaling_ObjectMetricSource_To_v1beta1_ObjectMetricSource,
		Convert_v1beta1_ObjectMetricStatus_To_autoscaling_ObjectMetricStatus,
		Convert_autoscaling_ObjectMetricStatus_To_v1beta1_ObjectMetricStatus,
		Convert_v1beta1_PodsMetricSource_To_autoscaling_PodsMetricSource,
		Convert_autoscaling_PodsMetricSource_To_v1beta1_PodsMetricSource,
		Convert_v1beta1_PodsMetricStatus_To_autoscaling_PodsMetricStatus,
		Convert_autoscaling_PodsMetricStatus_To_v1beta1_PodsMetricStatus,
		Convert_v1beta1_ResourceMetricSource_To_autoscaling_ResourceMetricSource,
		Convert_autoscaling_ResourceMetricSource_To_v1beta1_ResourceMetricSource,
		Convert_v1beta1_ResourceMetricStatus_To_autoscaling_ResourceMetricStatus,
		Convert_autoscaling_ResourceMetricStatus_To_v1beta1_ResourceMetricStatus,
		Convert_v1beta1_CrossVersionObjectReference_To_autoscaling_CrossVersionObjectReference,
		Convert_autoscaling_CrossVersionObjectReference_To_v1beta1_CrossVersionObjectReference,
	)
	if err != nil {
		return err
	}

	return nil
}

func autoConvert_autoscaling_HorizontalPodAutoscaler_To_v1beta1_HorizontalPodAutoscaler(in *autoscaling.HorizontalPodAutoscaler, out *HorizontalPodAutoscaler, s conversion.Scope) error {
	out.ObjectMeta = in.ObjectMeta
	if err := Convert_autoscaling_HorizontalPodAutoscalerSpec_To_v1beta1_HorizontalPodAutoscalerSpec(&in.Spec, &out.Spec, s); err != nil {
		return err
	}
	if err := Convert_autoscaling_HorizontalPodAutoscalerStatus_To_v1beta1_HorizontalPodAutoscalerStatus(&in.Status, &out.Status, s); err != nil {
		return err
	}
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

func autoConvert_v1beta1_HorizontalPodAutoscaler_To_autoscaling_HorizontalPodAutoscaler(in *HorizontalPodAutoscaler, out *autoscaling.HorizontalPodAutoscaler, s conversion.Scope) error {
	out.ObjectMeta = in.ObjectMeta
	if err := Convert_v1beta1_HorizontalPodAutoscalerSpec_To_autoscaling_HorizontalPodAutoscalerSpec(&in.Spec, &out.Spec, s); err != nil {
		return err
	}
	if err := Convert_v1beta1_HorizontalPodAutoscalerStatus_To_autoscaling_HorizontalPodAutoscalerStatus(&in.Status, &out.Status, s); err != nil {
		return err
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

func Convert_autoscaling_CrossVersionObjectReference_To_v1beta1_SubresourceReference(in *autoscaling.CrossVersionObjectReference, out *SubresourceReference, s conversion.Scope) error {
	out.Kind = in.Kind
	out.Name = in.Name
	out.APIVersion = in.APIVersion
	return nil
}

func Convert_v1beta1_SubresourceReference_To_autoscaling_CrossVersionObjectReference(in *SubresourceReference, out *autoscaling.CrossVersionObjectReference, s conversion.Scope) error {
	out.Kind = in.Kind
	out.Name = in.Name
	out.APIVersion = in.APIVersion
	return nil
}

func Convert_autoscaling_HorizontalPodAutoscalerSpec_To_v1beta1_HorizontalPodAutoscalerSpec(in *autoscaling.HorizontalPodAutoscalerSpec, out *HorizontalPodAutoscalerSpec, s conversion.Scope) error {
	if err := Convert_autoscaling_CrossVersionObjectReference_To_v1beta1_SubresourceReference(&in.ScaleTargetRef, &out.ScaleRef, s); err != nil {
		return err
	}

	out.MinReplicas = in.MinReplicas
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

	out.MinReplicas = in.MinReplicas
	out.MaxReplicas = in.MaxReplicas

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

func autoConvert_v1beta1_MetricSpec_To_autoscaling_MetricSpec(in *MetricSpec, out *autoscaling.MetricSpec, s conversion.Scope) error {
	out.Type = autoscaling.MetricSourceType(in.Type)
	out.Object = (*autoscaling.ObjectMetricSource)(unsafe.Pointer(in.Object))
	out.Pods = (*autoscaling.PodsMetricSource)(unsafe.Pointer(in.Pods))
	out.Resource = (*autoscaling.ResourceMetricSource)(unsafe.Pointer(in.Resource))
	return nil
}

func Convert_v1beta1_MetricSpec_To_autoscaling_MetricSpec(in *MetricSpec, out *autoscaling.MetricSpec, s conversion.Scope) error {
	return autoConvert_v1beta1_MetricSpec_To_autoscaling_MetricSpec(in, out, s)
}

func autoConvert_autoscaling_MetricSpec_To_v1beta1_MetricSpec(in *autoscaling.MetricSpec, out *MetricSpec, s conversion.Scope) error {
	out.Type = MetricSourceType(in.Type)
	out.Object = (*ObjectMetricSource)(unsafe.Pointer(in.Object))
	out.Pods = (*PodsMetricSource)(unsafe.Pointer(in.Pods))
	out.Resource = (*ResourceMetricSource)(unsafe.Pointer(in.Resource))
	return nil
}

func Convert_autoscaling_MetricSpec_To_v1beta1_MetricSpec(in *autoscaling.MetricSpec, out *MetricSpec, s conversion.Scope) error {
	return autoConvert_autoscaling_MetricSpec_To_v1beta1_MetricSpec(in, out, s)
}

func autoConvert_v1beta1_MetricStatus_To_autoscaling_MetricStatus(in *MetricStatus, out *autoscaling.MetricStatus, s conversion.Scope) error {
	out.Type = autoscaling.MetricSourceType(in.Type)
	out.Object = (*autoscaling.ObjectMetricStatus)(unsafe.Pointer(in.Object))
	out.Pods = (*autoscaling.PodsMetricStatus)(unsafe.Pointer(in.Pods))
	out.Resource = (*autoscaling.ResourceMetricStatus)(unsafe.Pointer(in.Resource))
	return nil
}

func Convert_v1beta1_MetricStatus_To_autoscaling_MetricStatus(in *MetricStatus, out *autoscaling.MetricStatus, s conversion.Scope) error {
	return autoConvert_v1beta1_MetricStatus_To_autoscaling_MetricStatus(in, out, s)
}

func autoConvert_autoscaling_MetricStatus_To_v1beta1_MetricStatus(in *autoscaling.MetricStatus, out *MetricStatus, s conversion.Scope) error {
	out.Type = MetricSourceType(in.Type)
	out.Object = (*ObjectMetricStatus)(unsafe.Pointer(in.Object))
	out.Pods = (*PodsMetricStatus)(unsafe.Pointer(in.Pods))
	out.Resource = (*ResourceMetricStatus)(unsafe.Pointer(in.Resource))
	return nil
}

func Convert_autoscaling_MetricStatus_To_v1beta1_MetricStatus(in *autoscaling.MetricStatus, out *MetricStatus, s conversion.Scope) error {
	return autoConvert_autoscaling_MetricStatus_To_v1beta1_MetricStatus(in, out, s)
}

func autoConvert_v1beta1_ObjectMetricSource_To_autoscaling_ObjectMetricSource(in *ObjectMetricSource, out *autoscaling.ObjectMetricSource, s conversion.Scope) error {
	if err := Convert_v1beta1_CrossVersionObjectReference_To_autoscaling_CrossVersionObjectReference(&in.Target, &out.Target, s); err != nil {
		return err
	}
	out.MetricName = in.MetricName
	out.TargetValue = in.TargetValue
	return nil
}

func Convert_v1beta1_ObjectMetricSource_To_autoscaling_ObjectMetricSource(in *ObjectMetricSource, out *autoscaling.ObjectMetricSource, s conversion.Scope) error {
	return autoConvert_v1beta1_ObjectMetricSource_To_autoscaling_ObjectMetricSource(in, out, s)
}

func autoConvert_autoscaling_ObjectMetricSource_To_v1beta1_ObjectMetricSource(in *autoscaling.ObjectMetricSource, out *ObjectMetricSource, s conversion.Scope) error {
	if err := Convert_autoscaling_CrossVersionObjectReference_To_v1beta1_CrossVersionObjectReference(&in.Target, &out.Target, s); err != nil {
		return err
	}
	out.MetricName = in.MetricName
	out.TargetValue = in.TargetValue
	return nil
}

func Convert_autoscaling_ObjectMetricSource_To_v1beta1_ObjectMetricSource(in *autoscaling.ObjectMetricSource, out *ObjectMetricSource, s conversion.Scope) error {
	return autoConvert_autoscaling_ObjectMetricSource_To_v1beta1_ObjectMetricSource(in, out, s)
}

func autoConvert_v1beta1_ObjectMetricStatus_To_autoscaling_ObjectMetricStatus(in *ObjectMetricStatus, out *autoscaling.ObjectMetricStatus, s conversion.Scope) error {
	if err := Convert_v1beta1_CrossVersionObjectReference_To_autoscaling_CrossVersionObjectReference(&in.Target, &out.Target, s); err != nil {
		return err
	}
	out.MetricName = in.MetricName
	out.CurrentValue = in.CurrentValue
	return nil
}

func Convert_v1beta1_ObjectMetricStatus_To_autoscaling_ObjectMetricStatus(in *ObjectMetricStatus, out *autoscaling.ObjectMetricStatus, s conversion.Scope) error {
	return autoConvert_v1beta1_ObjectMetricStatus_To_autoscaling_ObjectMetricStatus(in, out, s)
}

func autoConvert_autoscaling_ObjectMetricStatus_To_v1beta1_ObjectMetricStatus(in *autoscaling.ObjectMetricStatus, out *ObjectMetricStatus, s conversion.Scope) error {
	if err := Convert_autoscaling_CrossVersionObjectReference_To_v1beta1_CrossVersionObjectReference(&in.Target, &out.Target, s); err != nil {
		return err
	}
	out.MetricName = in.MetricName
	out.CurrentValue = in.CurrentValue
	return nil
}

func Convert_autoscaling_ObjectMetricStatus_To_v1beta1_ObjectMetricStatus(in *autoscaling.ObjectMetricStatus, out *ObjectMetricStatus, s conversion.Scope) error {
	return autoConvert_autoscaling_ObjectMetricStatus_To_v1beta1_ObjectMetricStatus(in, out, s)
}

func autoConvert_v1beta1_PodsMetricSource_To_autoscaling_PodsMetricSource(in *PodsMetricSource, out *autoscaling.PodsMetricSource, s conversion.Scope) error {
	out.MetricName = in.MetricName
	out.TargetAverageValue = in.TargetAverageValue
	return nil
}

func Convert_v1beta1_PodsMetricSource_To_autoscaling_PodsMetricSource(in *PodsMetricSource, out *autoscaling.PodsMetricSource, s conversion.Scope) error {
	return autoConvert_v1beta1_PodsMetricSource_To_autoscaling_PodsMetricSource(in, out, s)
}

func autoConvert_autoscaling_PodsMetricSource_To_v1beta1_PodsMetricSource(in *autoscaling.PodsMetricSource, out *PodsMetricSource, s conversion.Scope) error {
	out.MetricName = in.MetricName
	out.TargetAverageValue = in.TargetAverageValue
	return nil
}

func Convert_autoscaling_PodsMetricSource_To_v1beta1_PodsMetricSource(in *autoscaling.PodsMetricSource, out *PodsMetricSource, s conversion.Scope) error {
	return autoConvert_autoscaling_PodsMetricSource_To_v1beta1_PodsMetricSource(in, out, s)
}

func autoConvert_v1beta1_PodsMetricStatus_To_autoscaling_PodsMetricStatus(in *PodsMetricStatus, out *autoscaling.PodsMetricStatus, s conversion.Scope) error {
	out.MetricName = in.MetricName
	out.CurrentAverageValue = in.CurrentAverageValue
	return nil
}

func Convert_v1beta1_PodsMetricStatus_To_autoscaling_PodsMetricStatus(in *PodsMetricStatus, out *autoscaling.PodsMetricStatus, s conversion.Scope) error {
	return autoConvert_v1beta1_PodsMetricStatus_To_autoscaling_PodsMetricStatus(in, out, s)
}

func autoConvert_autoscaling_PodsMetricStatus_To_v1beta1_PodsMetricStatus(in *autoscaling.PodsMetricStatus, out *PodsMetricStatus, s conversion.Scope) error {
	out.MetricName = in.MetricName
	out.CurrentAverageValue = in.CurrentAverageValue
	return nil
}

func Convert_autoscaling_PodsMetricStatus_To_v1beta1_PodsMetricStatus(in *autoscaling.PodsMetricStatus, out *PodsMetricStatus, s conversion.Scope) error {
	return autoConvert_autoscaling_PodsMetricStatus_To_v1beta1_PodsMetricStatus(in, out, s)
}

func autoConvert_v1beta1_ResourceMetricSource_To_autoscaling_ResourceMetricSource(in *ResourceMetricSource, out *autoscaling.ResourceMetricSource, s conversion.Scope) error {
	out.Name = api.ResourceName(in.Name)
	out.TargetAverageUtilization = (*int32)(unsafe.Pointer(in.TargetAverageUtilization))
	out.TargetAverageValue = (*resource.Quantity)(unsafe.Pointer(in.TargetAverageValue))
	return nil
}

func Convert_v1beta1_ResourceMetricSource_To_autoscaling_ResourceMetricSource(in *ResourceMetricSource, out *autoscaling.ResourceMetricSource, s conversion.Scope) error {
	return autoConvert_v1beta1_ResourceMetricSource_To_autoscaling_ResourceMetricSource(in, out, s)
}

func autoConvert_autoscaling_ResourceMetricSource_To_v1beta1_ResourceMetricSource(in *autoscaling.ResourceMetricSource, out *ResourceMetricSource, s conversion.Scope) error {
	out.Name = api_v1.ResourceName(in.Name)
	out.TargetAverageUtilization = (*int32)(unsafe.Pointer(in.TargetAverageUtilization))
	out.TargetAverageValue = (*resource.Quantity)(unsafe.Pointer(in.TargetAverageValue))
	return nil
}

func Convert_autoscaling_ResourceMetricSource_To_v1beta1_ResourceMetricSource(in *autoscaling.ResourceMetricSource, out *ResourceMetricSource, s conversion.Scope) error {
	return autoConvert_autoscaling_ResourceMetricSource_To_v1beta1_ResourceMetricSource(in, out, s)
}

func autoConvert_v1beta1_ResourceMetricStatus_To_autoscaling_ResourceMetricStatus(in *ResourceMetricStatus, out *autoscaling.ResourceMetricStatus, s conversion.Scope) error {
	out.Name = api.ResourceName(in.Name)
	out.CurrentAverageUtilization = (*int32)(unsafe.Pointer(in.CurrentAverageUtilization))
	out.CurrentAverageValue = in.CurrentAverageValue
	return nil
}

func Convert_v1beta1_ResourceMetricStatus_To_autoscaling_ResourceMetricStatus(in *ResourceMetricStatus, out *autoscaling.ResourceMetricStatus, s conversion.Scope) error {
	return autoConvert_v1beta1_ResourceMetricStatus_To_autoscaling_ResourceMetricStatus(in, out, s)
}

func autoConvert_autoscaling_ResourceMetricStatus_To_v1beta1_ResourceMetricStatus(in *autoscaling.ResourceMetricStatus, out *ResourceMetricStatus, s conversion.Scope) error {
	out.Name = api_v1.ResourceName(in.Name)
	out.CurrentAverageUtilization = (*int32)(unsafe.Pointer(in.CurrentAverageUtilization))
	out.CurrentAverageValue = in.CurrentAverageValue
	return nil
}

func Convert_autoscaling_ResourceMetricStatus_To_v1beta1_ResourceMetricStatus(in *autoscaling.ResourceMetricStatus, out *ResourceMetricStatus, s conversion.Scope) error {
	return autoConvert_autoscaling_ResourceMetricStatus_To_v1beta1_ResourceMetricStatus(in, out, s)
}

func autoConvert_v1beta1_CrossVersionObjectReference_To_autoscaling_CrossVersionObjectReference(in *CrossVersionObjectReference, out *autoscaling.CrossVersionObjectReference, s conversion.Scope) error {
	out.Kind = in.Kind
	out.Name = in.Name
	out.APIVersion = in.APIVersion
	return nil
}

func Convert_v1beta1_CrossVersionObjectReference_To_autoscaling_CrossVersionObjectReference(in *CrossVersionObjectReference, out *autoscaling.CrossVersionObjectReference, s conversion.Scope) error {
	return autoConvert_v1beta1_CrossVersionObjectReference_To_autoscaling_CrossVersionObjectReference(in, out, s)
}

func autoConvert_autoscaling_CrossVersionObjectReference_To_v1beta1_CrossVersionObjectReference(in *autoscaling.CrossVersionObjectReference, out *CrossVersionObjectReference, s conversion.Scope) error {
	out.Kind = in.Kind
	out.Name = in.Name
	out.APIVersion = in.APIVersion
	return nil
}

func Convert_autoscaling_CrossVersionObjectReference_To_v1beta1_CrossVersionObjectReference(in *autoscaling.CrossVersionObjectReference, out *CrossVersionObjectReference, s conversion.Scope) error {
	return autoConvert_autoscaling_CrossVersionObjectReference_To_v1beta1_CrossVersionObjectReference(in, out, s)
}
