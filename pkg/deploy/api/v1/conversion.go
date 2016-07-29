package v1

import (
	"fmt"
	"math"
	"reflect"
	"strings"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/apis/extensions/v1beta1"
	"k8s.io/kubernetes/pkg/conversion"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/intstr"

	oapi "github.com/openshift/origin/pkg/api"
	newer "github.com/openshift/origin/pkg/deploy/api"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

func Convert_v1_DeploymentTriggerImageChangeParams_To_api_DeploymentTriggerImageChangeParams(in *DeploymentTriggerImageChangeParams, out *newer.DeploymentTriggerImageChangeParams, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*DeploymentTriggerImageChangeParams))(in)
	}

	if err := s.DefaultConvert(in, out, conversion.IgnoreMissingFields); err != nil {
		return err
	}
	switch in.From.Kind {
	case "ImageStreamTag":
	case "ImageStream", "ImageRepository":
		out.From.Kind = "ImageStreamTag"
		if !strings.Contains(out.From.Name, ":") {
			out.From.Name = imageapi.JoinImageStreamTag(out.From.Name, imageapi.DefaultImageTag)
		}
	default:
		// Will be handled by validation
	}
	return nil
}

func Convert_api_DeploymentTriggerImageChangeParams_To_v1_DeploymentTriggerImageChangeParams(in *newer.DeploymentTriggerImageChangeParams, out *DeploymentTriggerImageChangeParams, s conversion.Scope) error {
	if err := s.DefaultConvert(in, out, conversion.IgnoreMissingFields); err != nil {
		return err
	}
	switch in.From.Kind {
	case "ImageStreamTag":
	case "ImageStream", "ImageRepository":
		out.From.Kind = "ImageStreamTag"
		if !strings.Contains(out.From.Name, ":") {
			out.From.Name = imageapi.JoinImageStreamTag(out.From.Name, imageapi.DefaultImageTag)
		}
	default:
		// Will be handled by validation
	}
	return nil
}

func Convert_v1_RollingDeploymentStrategyParams_To_api_RollingDeploymentStrategyParams(in *RollingDeploymentStrategyParams, out *newer.RollingDeploymentStrategyParams, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*RollingDeploymentStrategyParams))(in)
	}
	out.UpdatePeriodSeconds = in.UpdatePeriodSeconds
	out.IntervalSeconds = in.IntervalSeconds
	out.TimeoutSeconds = in.TimeoutSeconds
	out.UpdatePercent = in.UpdatePercent

	if in.Pre != nil {
		if err := s.Convert(&in.Pre, &out.Pre, 0); err != nil {
			return err
		}
	}
	if in.Post != nil {
		if err := s.Convert(&in.Post, &out.Post, 0); err != nil {
			return err
		}
	}

	if in.UpdatePercent != nil {
		pct := intstr.FromString(fmt.Sprintf("%d%%", int(math.Abs(float64(*in.UpdatePercent)))))
		if *in.UpdatePercent > 0 {
			out.MaxSurge = pct
		} else {
			out.MaxUnavailable = pct
		}
	} else {
		if err := s.Convert(in.MaxUnavailable, &out.MaxUnavailable, 0); err != nil {
			return err
		}
		if err := s.Convert(in.MaxSurge, &out.MaxSurge, 0); err != nil {
			return err
		}
	}
	return nil
}

func Convert_api_RollingDeploymentStrategyParams_To_v1_RollingDeploymentStrategyParams(in *newer.RollingDeploymentStrategyParams, out *RollingDeploymentStrategyParams, s conversion.Scope) error {
	out.UpdatePeriodSeconds = in.UpdatePeriodSeconds
	out.IntervalSeconds = in.IntervalSeconds
	out.TimeoutSeconds = in.TimeoutSeconds
	out.UpdatePercent = in.UpdatePercent

	if in.Pre != nil {
		if err := s.Convert(&in.Pre, &out.Pre, 0); err != nil {
			return err
		}
	}
	if in.Post != nil {
		if err := s.Convert(&in.Post, &out.Post, 0); err != nil {
			return err
		}
	}

	if out.MaxUnavailable == nil {
		out.MaxUnavailable = &intstr.IntOrString{}
	}
	if out.MaxSurge == nil {
		out.MaxSurge = &intstr.IntOrString{}
	}
	if in.UpdatePercent != nil {
		pct := intstr.FromString(fmt.Sprintf("%d%%", int(math.Abs(float64(*in.UpdatePercent)))))
		if *in.UpdatePercent > 0 {
			out.MaxSurge = &pct
		} else {
			out.MaxUnavailable = &pct
		}
	} else {
		if err := s.Convert(&in.MaxUnavailable, out.MaxUnavailable, 0); err != nil {
			return err
		}
		if err := s.Convert(&in.MaxSurge, out.MaxSurge, 0); err != nil {
			return err
		}
	}
	return nil
}

func Convert_v1beta1_ReplicaSet_to_api_ReplicationController(in *v1beta1.ReplicaSet, out *api.ReplicationController, s conversion.Scope) error {
	intermediate1 := &extensions.ReplicaSet{}
	if err := v1beta1.Convert_v1beta1_ReplicaSet_To_extensions_ReplicaSet(in, intermediate1, s); err != nil {
		return err
	}

	intermediate2 := &v1.ReplicationController{}
	if err := v1.Convert_extensions_ReplicaSet_to_v1_ReplicationController(intermediate1, intermediate2, s); err != nil {
		return err
	}

	return v1.Convert_v1_ReplicationController_To_api_ReplicationController(intermediate2, out, s)
}

func originalKindAnnotation(in runtime.Object, out map[string]string) map[string]string {
	if _, exists := out[newer.OriginalKindAnnotation]; !exists {
		gvk := in.GetObjectKind().GroupVersionKind()
		out[newer.OriginalKindAnnotation] = gvk.Version + "." + gvk.Kind
	}
	return out
}

func nonConvertibleField(name string, value interface{}, out *map[string]string) {
	newMap := *out
	newMap[api.NonConvertibleAnnotationPrefix+"/"+name] = reflect.ValueOf(value).String()
	*out = newMap
}

func Convert_extensions_DeploymentSpec_To_v1_DeploymentConfigSpec(in *extensions.DeploymentSpec, out *DeploymentConfigSpec, annotations *map[string]string, s conversion.Scope) {
	out.Replicas = in.Replicas
	out.MinReadySeconds = in.MinReadySeconds
	out.RevisionHistoryLimit = in.RevisionHistoryLimit
	out.Paused = in.Paused

	if in.Selector != nil {
		api.Convert_unversioned_LabelSelector_to_map(in.Selector, &out.Selector, s)
	}

	switch in.Strategy.Type {
	case extensions.RecreateDeploymentStrategyType:
		out.Strategy.Type = DeploymentStrategyTypeRecreate
	case extensions.RollingUpdateDeploymentStrategyType:
		out.Strategy.Type = DeploymentStrategyTypeRolling
		out.Strategy.RollingParams = &RollingDeploymentStrategyParams{
			MaxSurge:       &in.Strategy.RollingUpdate.MaxSurge,
			MaxUnavailable: &in.Strategy.RollingUpdate.MaxUnavailable,
		}
	}
}

func Convert_api_DeploymentConfigSpec_To_v1beta1_DeploymentSpec(in *newer.DeploymentConfigSpec, out *v1beta1.DeploymentSpec, annotations *map[string]string, s conversion.Scope) {
	out.Replicas = &in.Replicas
	out.MinReadySeconds = in.MinReadySeconds
	out.RevisionHistoryLimit = in.RevisionHistoryLimit
	out.Paused = in.Paused

	if in.Test == true {
		nonConvertibleField("spec.test", true, annotations)
	}

	immediate := &unversioned.LabelSelector{}
	api.Convert_map_to_unversioned_LabelSelector(&in.Selector, immediate, s)
	s.Convert(&immediate, out.Selector, 0)

	switch in.Strategy.Type {
	case newer.DeploymentStrategyTypeRolling:
		out.Strategy.Type = v1beta1.RecreateDeploymentStrategyType
		if in.Strategy.RollingParams.UpdatePeriodSeconds != nil {
			nonConvertibleField("spec.strategy.updatePeriodSeconds", *in.Strategy.RollingParams.UpdatePeriodSeconds, annotations)
		}
		if in.Strategy.RollingParams.IntervalSeconds != nil {
			nonConvertibleField("spec.strategy.intervalSeconds", *in.Strategy.RollingParams.IntervalSeconds, annotations)
		}
		if in.Strategy.RollingParams.TimeoutSeconds != nil {
			nonConvertibleField("spec.strategy.timeoutSeconds", *in.Strategy.RollingParams.TimeoutSeconds, annotations)
		}
		if in.Strategy.RollingParams.MaxSurge.IntValue() > 0 {
			nonConvertibleField("spec.strategy.maxSurge", in.Strategy.RollingParams.MaxSurge, annotations)
		}
		if in.Strategy.RollingParams.UpdatePercent != nil {
			nonConvertibleField("spec.strategy.UpdatePercent", *in.Strategy.RollingParams.UpdatePercent, annotations)
		}
		if in.Strategy.RollingParams.Pre != nil {
			nonConvertibleField("spec.strategy.pre", *in.Strategy.RollingParams.Pre, annotations)
		}
		if in.Strategy.RollingParams.Post != nil {
			nonConvertibleField("spec.strategy.post", *in.Strategy.RollingParams.Post, annotations)
		}
	case newer.DeploymentStrategyTypeRecreate:
		out.Strategy.Type = v1beta1.RollingUpdateDeploymentStrategyType
		// Add non-convertible field annotations
		if in.Strategy.RecreateParams.Pre != nil {
			nonConvertibleField("spec.strategy.pre", *in.Strategy.RecreateParams.Pre, annotations)
		}
		if in.Strategy.RecreateParams.Post != nil {
			nonConvertibleField("spec.strategy.post", *in.Strategy.RecreateParams.Post, annotations)
		}
		if in.Strategy.RecreateParams.Mid != nil {
			nonConvertibleField("spec.strategy.mid", *in.Strategy.RecreateParams.Mid, annotations)
		}
		if in.Strategy.RecreateParams.TimeoutSeconds != nil {
			nonConvertibleField("spec.strategy.timeoutSeconds", *in.Strategy.RecreateParams.TimeoutSeconds, annotations)
		}
	}
}

func Convert_extensions_DeploymentStatus_To_v1_DeploymentConfigStatus(in *extensions.DeploymentStatus, out *DeploymentConfigStatus, annotations *map[string]string) {
	out.ObservedGeneration = in.ObservedGeneration
	out.Replicas = in.Replicas
	out.UpdatedReplicas = in.UpdatedReplicas
	out.AvailableReplicas = in.AvailableReplicas
	out.UnavailableReplicas = in.UnavailableReplicas
	// TODO: Handle LatestVersion
}

func Convert_api_DeploymentConfigStatus_To_v1beta1_DeploymentStatus(in *newer.DeploymentConfigStatus, out *v1beta1.DeploymentStatus, annotations *map[string]string) {
	out.ObservedGeneration = in.ObservedGeneration
	out.Replicas = in.Replicas
	out.UpdatedReplicas = in.UpdatedReplicas
	out.AvailableReplicas = in.AvailableReplicas
	out.UnavailableReplicas = in.UnavailableReplicas
	nonConvertibleField("status.latestVersion", in.LatestVersion, annotations)
}

//  d._internal -> dc.v1
func Convert_extensions_Deployment_To_v1_DeploymentConfig(in *extensions.Deployment, out *DeploymentConfig, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*extensions.Deployment))(in)
	}
	if err := api.Convert_unversioned_TypeMeta_To_unversioned_TypeMeta(&in.TypeMeta, &out.TypeMeta, s); err != nil {
		return err
	}
	if err := v1.Convert_api_ObjectMeta_To_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}

	Convert_extensions_DeploymentSpec_To_v1_DeploymentConfigSpec(&in.Spec, &out.Spec, &out.ObjectMeta.Annotations, s)
	Convert_extensions_DeploymentStatus_To_v1_DeploymentConfigStatus(&in.Status, &out.Status, &out.ObjectMeta.Annotations)

	return v1.Convert_api_PodTemplateSpec_To_v1_PodTemplateSpec(&in.Spec.Template, out.Spec.Template, s)
}

//  dc._internal -> d.v1beta1
func Convert_api_DeploymentConfig_To_v1beta1_Deployment(in *newer.DeploymentConfig, out *v1beta1.Deployment, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*newer.DeploymentConfig))(in)
	}
	if err := api.Convert_unversioned_TypeMeta_To_unversioned_TypeMeta(&in.TypeMeta, &out.TypeMeta, s); err != nil {
		return err
	}
	if err := v1.Convert_api_ObjectMeta_To_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}

	Convert_api_DeploymentConfigSpec_To_v1beta1_DeploymentSpec(&in.Spec, &out.Spec, &out.ObjectMeta.Annotations, s)
	Convert_api_DeploymentConfigStatus_To_v1beta1_DeploymentStatus(&in.Status, &out.Status, &out.ObjectMeta.Annotations)

	return v1.Convert_api_PodTemplateSpec_To_v1_PodTemplateSpec(in.Spec.Template, &out.Spec.Template, s)
}

func Convert_v1_DeploymentConfig_To_extensions_Deployment(in *DeploymentConfig, out *extensions.Deployment, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*DeploymentConfig))(in)
	}

	immediateSpec := &newer.DeploymentConfigSpec{}
	immediateExtSpec := &v1beta1.DeploymentSpec{}

	if err := s.Convert(&in.Spec, immediateSpec, 0); err != nil {
		return err
	}

	Convert_api_DeploymentConfigSpec_To_v1beta1_DeploymentSpec(immediateSpec, immediateExtSpec, &out.ObjectMeta.Annotations, s)

	if err := s.Convert(immediateExtSpec, &out.Spec, 0); err != nil {
		return err
	}

	immediateStatus := &newer.DeploymentConfigStatus{}
	immediateExtStatus := &v1beta1.DeploymentStatus{}
	if err := s.Convert(&in.Status, immediateStatus, 0); err != nil {
		return err
	}

	Convert_api_DeploymentConfigStatus_To_v1beta1_DeploymentStatus(immediateStatus, immediateExtStatus, &out.ObjectMeta.Annotations)

	if err := s.Convert(immediateExtStatus, &out.Status, 0); err != nil {
		return err
	}

	out.SetAnnotations(originalKindAnnotation(in, out.GetAnnotations()))

	return v1.Convert_v1_PodTemplateSpec_To_api_PodTemplateSpec(in.Spec.Template, &out.Spec.Template, s)
}

func Convert_v1beta1_Deployment_To_api_DeploymentConfig(in *v1beta1.Deployment, out *newer.DeploymentConfig, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1beta1.Deployment))(in)
	}

	immediateSpec := &extensions.DeploymentSpec{}
	immediateExtSpec := &DeploymentConfigSpec{}
	if err := s.Convert(&in.Spec, immediateSpec, 0); err != nil {
		return err
	}

	Convert_extensions_DeploymentSpec_To_v1_DeploymentConfigSpec(immediateSpec, immediateExtSpec, &out.ObjectMeta.Annotations, s)

	if err := s.Convert(immediateExtSpec, &out.Spec, 0); err != nil {
		return err
	}

	immediateStatus := &extensions.DeploymentStatus{}
	immediateExtStatus := &DeploymentConfigStatus{}
	if err := s.Convert(&in.Status, immediateStatus, 0); err != nil {
		return err
	}

	Convert_extensions_DeploymentStatus_To_v1_DeploymentConfigStatus(immediateStatus, immediateExtStatus, &out.ObjectMeta.Annotations)

	if err := s.Convert(immediateExtStatus, &out.Status, 0); err != nil {
		return err
	}

	out.SetAnnotations(originalKindAnnotation(in, out.GetAnnotations()))

	return v1.Convert_v1_PodTemplateSpec_To_api_PodTemplateSpec(&in.Spec.Template, out.Spec.Template, s)
}

func addConversionFuncs(scheme *runtime.Scheme) {
	err := scheme.AddConversionFuncs(
		Convert_v1_DeploymentTriggerImageChangeParams_To_api_DeploymentTriggerImageChangeParams,
		Convert_api_DeploymentTriggerImageChangeParams_To_v1_DeploymentTriggerImageChangeParams,

		Convert_v1_RollingDeploymentStrategyParams_To_api_RollingDeploymentStrategyParams,
		Convert_api_RollingDeploymentStrategyParams_To_v1_RollingDeploymentStrategyParams,

		Convert_v1beta1_ReplicaSet_to_api_ReplicationController,

		Convert_api_DeploymentConfig_To_v1beta1_Deployment,
		Convert_v1_DeploymentConfig_To_extensions_Deployment,
		Convert_v1beta1_Deployment_To_api_DeploymentConfig,
		Convert_extensions_Deployment_To_v1_DeploymentConfig,
	)
	if err != nil {
		panic(err)
	}

	if err := scheme.AddFieldLabelConversionFunc("v1", "DeploymentConfig",
		oapi.GetFieldLabelConversionFunc(newer.DeploymentConfigToSelectableFields(&newer.DeploymentConfig{}), nil),
	); err != nil {
		panic(err)
	}
}
