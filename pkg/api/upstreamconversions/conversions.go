package upstreamconversions

import (
	"encoding/json"
	"fmt"
	"reflect"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	kapiv1 "k8s.io/kubernetes/pkg/api/v1"
	extensionsapi "k8s.io/kubernetes/pkg/apis/extensions"
	extensionsv1beta1 "k8s.io/kubernetes/pkg/apis/extensions/v1beta1"
	"k8s.io/kubernetes/pkg/conversion"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/errors"
	"k8s.io/kubernetes/pkg/util/intstr"
	"k8s.io/kubernetes/pkg/util/validation/field"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deployapiv1 "github.com/openshift/origin/pkg/deploy/api/v1"
)

func AddToScheme(scheme *runtime.Scheme) {
	addConversionFuncs(scheme)
}

func addConversionFuncs(scheme *runtime.Scheme) {
	if err := scheme.AddConversionFuncs(
		Convert_v1beta1_ReplicaSet_to_api_ReplicationController,

		Convert_api_DeploymentConfig_To_v1beta1_Deployment,
		Convert_api_DeploymentConfigSpec_To_v1beta1_DeploymentSpec,
		Convert_api_DeploymentConfigStatus_To_v1beta1_DeploymentStatus,
		Convert_v1_DeploymentConfig_To_extensions_Deployment,

		Convert_extensions_Deployment_To_v1_DeploymentConfig,
		Convert_extensions_DeploymentSpec_To_v1_DeploymentConfigSpec,
		Convert_extensions_DeploymentStatus_To_v1_DeploymentConfigStatus,
		Convert_v1beta1_Deployment_to_api_DeploymentConfig,
	); err != nil {
		panic(err)
	}
}

func Convert_v1beta1_ReplicaSet_to_api_ReplicationController(in *extensionsv1beta1.ReplicaSet, out *kapi.ReplicationController, s conversion.Scope) error {
	intermediate1 := &extensionsapi.ReplicaSet{}
	if err := extensionsv1beta1.Convert_v1beta1_ReplicaSet_To_extensions_ReplicaSet(in, intermediate1, s); err != nil {
		return err
	}

	intermediate2 := &kapiv1.ReplicationController{}
	if err := kapiv1.Convert_extensions_ReplicaSet_to_v1_ReplicationController(intermediate1, intermediate2, s); err != nil {
		return err
	}

	return kapiv1.Convert_v1_ReplicationController_To_api_ReplicationController(intermediate2, out, s)
}

func Convert_v1beta1_Deployment_to_api_DeploymentConfig(in *extensionsv1beta1.Deployment, out *deployapi.DeploymentConfig, s conversion.Scope) error {
	intermediate1 := &extensionsapi.Deployment{}
	if err := extensionsv1beta1.Convert_v1beta1_Deployment_To_extensions_Deployment(in, intermediate1, s); err != nil {
		return err
	}

	intermediate2 := &deployapiv1.DeploymentConfig{}
	if err := Convert_extensions_Deployment_To_v1_DeploymentConfig(intermediate1, intermediate2, s); err != nil {
		return err
	}

	if err := deployapiv1.Convert_v1_DeploymentConfig_To_api_DeploymentConfig(intermediate2, out, s); err != nil {
		return err
	}

	if out.Annotations == nil {
		out.Annotations = make(map[string]string)
	}
	if _, exists := out.Annotations[kapi.OriginalKindAnnotationName]; !exists {
		out.Annotations[kapi.OriginalKindAnnotationName] = "Deployment.extensions"
	}

	return nil
}

func Convert_v1_DeploymentConfig_To_extensions_Deployment(in *deployapiv1.DeploymentConfig, out *extensionsapi.Deployment, s conversion.Scope) error {
	intermediate1 := &deployapi.DeploymentConfig{}
	if err := deployapiv1.Convert_v1_DeploymentConfig_To_api_DeploymentConfig(in, intermediate1, s); err != nil {
		return err
	}

	intermediate2 := &extensionsv1beta1.Deployment{}
	if err := Convert_api_DeploymentConfig_To_v1beta1_Deployment(intermediate1, intermediate2, s); err != nil {
		return err
	}

	if err := extensionsv1beta1.Convert_v1beta1_Deployment_To_extensions_Deployment(intermediate2, out, s); err != nil {
		return err
	}

	if out.Annotations == nil {
		out.Annotations = make(map[string]string)
	}
	if _, exists := out.Annotations[kapi.OriginalKindAnnotationName]; !exists {
		out.Annotations[kapi.OriginalKindAnnotationName] = "DeploymentConfig."
	}

	return nil
}

//  d._internal -> dc.v1
func Convert_extensions_Deployment_To_v1_DeploymentConfig(in *extensionsapi.Deployment, out *deployapiv1.DeploymentConfig, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*extensionsapi.Deployment))(in)
	}
	if err := kapi.Convert_unversioned_TypeMeta_To_unversioned_TypeMeta(&in.TypeMeta, &out.TypeMeta, s); err != nil {
		return err
	}
	if err := kapiv1.Convert_api_ObjectMeta_To_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}

	if in.Annotations == nil {
		out.Annotations = map[string]string{}
	}

	if err := Convert_extensions_DeploymentSpec_To_v1_DeploymentConfigSpec(&in.Spec, &out.Spec, s); err != nil {
		if err := errorsToNonConvertible(err, &out.Annotations); err != nil {
			return err
		}
	}

	if err := Convert_extensions_DeploymentStatus_To_v1_DeploymentConfigStatus(&in.Status, &out.Status, s); err != nil {
		if err := errorsToNonConvertible(err, &out.Annotations); err != nil {
			return err
		}
	}

	// Restore all stored non-convertible fields
	if in.ObjectMeta.Annotations != nil {
		restoreNonConvertible("status.latestVersion", in.ObjectMeta.Annotations, &out.Status.LatestVersion)
		restoreNonConvertible("status.details", in.ObjectMeta.Annotations, &out.Status.Details)
		restoreNonConvertible("spec.test", in.ObjectMeta.Annotations, &out.Spec.Test)
		prefix := "spec.strategy."
		restoreNonConvertible("spec.strategy.resources", in.ObjectMeta.Annotations, &out.Spec.Strategy.Resources)
		restoreNonConvertible("spec.strategy.annotations", in.ObjectMeta.Annotations, &out.Spec.Strategy.Annotations)
		restoreNonConvertible("spec.strategy.labels", in.ObjectMeta.Annotations, &out.Spec.Strategy.Labels)
		restoreNonConvertible("spec.strategy.triggers", in.ObjectMeta.Annotations, &out.Spec.Triggers)

		if out.Spec.Strategy.RollingParams != nil {
			prefix = "spec.strategy.rollingParams."
			out.Spec.Strategy.RollingParams.UpdatePeriodSeconds = new(int64)
			restoreNonConvertible(prefix+"updatePeriodSeconds", in.ObjectMeta.Annotations, out.Spec.Strategy.RollingParams.UpdatePeriodSeconds)
			out.Spec.Strategy.RollingParams.IntervalSeconds = new(int64)
			restoreNonConvertible(prefix+"intervalSeconds", in.ObjectMeta.Annotations, out.Spec.Strategy.RollingParams.IntervalSeconds)
			out.Spec.Strategy.RollingParams.TimeoutSeconds = new(int64)
			restoreNonConvertible(prefix+"timeoutSeconds", in.ObjectMeta.Annotations, out.Spec.Strategy.RollingParams.TimeoutSeconds)

			if _, exists := in.ObjectMeta.Annotations[kapi.NonConvertibleAnnotationPrefix+"/"+prefix+"updatePercent"]; exists {
				out.Spec.Strategy.RollingParams.UpdatePercent = new(int32)
				restoreNonConvertible(prefix+"updatePercent", in.ObjectMeta.Annotations, out.Spec.Strategy.RollingParams.UpdatePercent)
			}

			intermediate := deployapiv1.LifecycleHook{}
			restoreNonConvertible(prefix+"pre", in.ObjectMeta.Annotations, &intermediate)
			out.Spec.Strategy.RollingParams.Pre = &intermediate

			intermediate = deployapiv1.LifecycleHook{}
			restoreNonConvertible(prefix+"post", in.ObjectMeta.Annotations, &intermediate)
			out.Spec.Strategy.RollingParams.Post = &intermediate
		}

		if out.Spec.Strategy.RecreateParams != nil {
			intermediate := deployapiv1.RecreateDeploymentStrategyParams{}
			restoreNonConvertible("spec.strategy.recreateParams", in.ObjectMeta.Annotations, &intermediate)
			*out.Spec.Strategy.RecreateParams = intermediate
		}

	}

	return nil
}

//  dc._internal -> d.v1beta1
func Convert_api_DeploymentConfig_To_v1beta1_Deployment(in *deployapi.DeploymentConfig, out *extensionsv1beta1.Deployment, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapi.DeploymentConfig))(in)
	}
	if err := kapi.Convert_unversioned_TypeMeta_To_unversioned_TypeMeta(&in.TypeMeta, &out.TypeMeta, s); err != nil {
		return err
	}
	if err := kapiv1.Convert_api_ObjectMeta_To_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}

	if in.Annotations == nil {
		out.Annotations = map[string]string{}
	}

	if err := Convert_api_DeploymentConfigSpec_To_v1beta1_DeploymentSpec(&in.Spec, &out.Spec, s); err != nil {
		if err := errorsToNonConvertible(err, &out.Annotations); err != nil {
			return err
		}
	}

	if err := Convert_api_DeploymentConfigStatus_To_v1beta1_DeploymentStatus(&in.Status, &out.Status, s); err != nil {
		if err := errorsToNonConvertible(err, &out.Annotations); err != nil {
			return err
		}
	}
	return nil
}

func Convert_extensions_DeploymentSpec_To_v1_DeploymentConfigSpec(in *extensionsapi.DeploymentSpec, out *deployapiv1.DeploymentConfigSpec, s conversion.Scope) error {
	out.Replicas = in.Replicas
	out.MinReadySeconds = in.MinReadySeconds
	out.RevisionHistoryLimit = in.RevisionHistoryLimit
	out.Paused = in.Paused

	nonConvertibleFields := field.ErrorList{}

	if in.Selector != nil {
		if err := kapi.Convert_unversioned_LabelSelector_to_map(in.Selector, &out.Selector, s); err != nil {
			addNonConvertible("spec.labelSelector", in.Selector, &nonConvertibleFields)
		}
	}

	switch in.Strategy.Type {
	case extensionsapi.RecreateDeploymentStrategyType:
		out.Strategy.Type = deployapiv1.DeploymentStrategyTypeRecreate
		out.Strategy.RecreateParams = &deployapiv1.RecreateDeploymentStrategyParams{}
	case extensionsapi.RollingUpdateDeploymentStrategyType:
		out.Strategy.Type = deployapiv1.DeploymentStrategyTypeRolling
		out.Strategy.RollingParams = &deployapiv1.RollingDeploymentStrategyParams{
			MaxSurge:       &in.Strategy.RollingUpdate.MaxSurge,
			MaxUnavailable: &in.Strategy.RollingUpdate.MaxUnavailable,
		}
	}

	out.Template = &kapiv1.PodTemplateSpec{}
	if err := s.Convert(&in.Template, out.Template, 0); err != nil {
		return err
	}

	return nonConvertibleFields.ToAggregate()
}

func Convert_api_DeploymentConfigSpec_To_v1beta1_DeploymentSpec(in *deployapi.DeploymentConfigSpec, out *extensionsv1beta1.DeploymentSpec, s conversion.Scope) error {
	out.Replicas = &in.Replicas
	out.MinReadySeconds = in.MinReadySeconds
	out.RevisionHistoryLimit = in.RevisionHistoryLimit
	out.Paused = in.Paused
	// TODO: Set this based on annotation
	out.RollbackTo = &extensionsv1beta1.RollbackConfig{}

	nonConvertibleFields := field.ErrorList{}

	if in.Test == true {
		addNonConvertible("spec.test", true, &nonConvertibleFields)
	}

	if in.Selector != nil {
		out.Selector = &extensionsv1beta1.LabelSelector{}
		intermediate := &unversioned.LabelSelector{}
		if err := kapi.Convert_map_to_unversioned_LabelSelector(&in.Selector, intermediate, s); err != nil {
			return err
		}
		if err := s.Convert(intermediate, out.Selector, 0); err != nil {
			return err
		}
	}

	switch in.Strategy.Type {
	case deployapi.DeploymentStrategyTypeRolling:
		out.Strategy.Type = extensionsv1beta1.RollingUpdateDeploymentStrategyType
		if in.Strategy.RollingParams != nil {
			out.Strategy.RollingUpdate = &extensionsv1beta1.RollingUpdateDeployment{}
			// first fields we know how to convert
			out.Strategy.RollingUpdate.MaxSurge = &intstr.IntOrString{}
			s.Convert(&in.Strategy.RollingParams.MaxSurge, out.Strategy.RollingUpdate.MaxSurge, 0)

			out.Strategy.RollingUpdate.MaxUnavailable = &intstr.IntOrString{}
			s.Convert(&in.Strategy.RollingParams.MaxUnavailable, out.Strategy.RollingUpdate.MaxUnavailable, 0)

			prefix := "spec.strategy.rollingParams."
			addNonConvertible(prefix+"updatePeriodSeconds", in.Strategy.RollingParams.UpdatePeriodSeconds, &nonConvertibleFields)
			addNonConvertible(prefix+"intervalSeconds", in.Strategy.RollingParams.IntervalSeconds, &nonConvertibleFields)
			addNonConvertible(prefix+"timeoutSeconds", in.Strategy.RollingParams.TimeoutSeconds, &nonConvertibleFields)
			if in.Strategy.RollingParams.UpdatePercent != nil {
				addNonConvertible(prefix+"updatePercent", in.Strategy.RollingParams.UpdatePercent, &nonConvertibleFields)
			}
			addNonConvertible(prefix+"pre", in.Strategy.RollingParams.Pre, &nonConvertibleFields)
			addNonConvertible(prefix+"post", in.Strategy.RollingParams.Post, &nonConvertibleFields)
		} else {
			in.Strategy.RollingParams = &deployapi.RollingDeploymentStrategyParams{}
		}
	case deployapi.DeploymentStrategyTypeRecreate:
		out.Strategy.Type = extensionsv1beta1.RecreateDeploymentStrategyType
		if in.Strategy.RecreateParams != nil {
			addNonConvertible("spec.strategy.recreateParams", in.Strategy.RecreateParams, &nonConvertibleFields)
		} else {
			in.Strategy.RecreateParams = &deployapi.RecreateDeploymentStrategyParams{}
		}
	}

	addNonConvertible("spec.strategy.resources", in.Strategy.Resources, &nonConvertibleFields)
	if in.Strategy.Annotations != nil {
		addNonConvertible("spec.strategy.annotations", in.Strategy.Annotations, &nonConvertibleFields)
	}
	if in.Strategy.Labels != nil {
		addNonConvertible("spec.strategy.labels", in.Strategy.Labels, &nonConvertibleFields)
	}

	if len(in.Triggers) > 0 {
		addNonConvertible("spec.strategy.triggers", in.Triggers, &nonConvertibleFields)
	}

	if in.Template != nil {
		out.Template = kapiv1.PodTemplateSpec{}
		if err := s.Convert(in.Template, &out.Template, 0); err != nil {
			return err
		}
	}

	return nonConvertibleFields.ToAggregate()
}

func Convert_extensions_DeploymentStatus_To_v1_DeploymentConfigStatus(in *extensionsapi.DeploymentStatus, out *deployapiv1.DeploymentConfigStatus, s conversion.Scope) error {
	out.ObservedGeneration = in.ObservedGeneration
	out.Replicas = in.Replicas
	out.UpdatedReplicas = in.UpdatedReplicas
	out.AvailableReplicas = in.AvailableReplicas
	out.UnavailableReplicas = in.UnavailableReplicas
	return nil
}

func Convert_api_DeploymentConfigStatus_To_v1beta1_DeploymentStatus(in *deployapi.DeploymentConfigStatus, out *extensionsv1beta1.DeploymentStatus, s conversion.Scope) error {
	out.ObservedGeneration = in.ObservedGeneration
	out.Replicas = in.Replicas
	out.UpdatedReplicas = in.UpdatedReplicas
	out.AvailableReplicas = in.AvailableReplicas
	out.UnavailableReplicas = in.UnavailableReplicas

	nonConvertibleFields := field.ErrorList{}
	nonConvertibleFields = append(nonConvertibleFields,
		field.Invalid(field.NewPath("status.latestVersion"), in.LatestVersion, "cannot convert"),
	)

	if in.Details != nil {
		nonConvertibleFields = append(nonConvertibleFields,
			field.Invalid(field.NewPath("status.details"), *in.Details, "cannot convert"),
		)
	}

	return nonConvertibleFields.ToAggregate()
}

func setOriginalKind(in runtime.Object, out map[string]string) map[string]string {
	if out == nil {
		out = map[string]string{}
	}
	if _, exists := out[kapi.OriginalKindAnnotationName]; !exists {
		gvk := in.GetObjectKind().GroupVersionKind()
		out[kapi.OriginalKindAnnotationName] = gvk.Kind + "." + gvk.Group
	}
	return out
}

func addNonConvertible(fieldName string, in interface{}, out *field.ErrorList) {
	switch reflect.ValueOf(in).Type().Kind() {
	case reflect.Map, reflect.Ptr, reflect.Slice:
		if reflect.ValueOf(in).IsNil() {
			return
		}
	}
	*out = append(*out, field.Invalid(field.NewPath(fieldName), in, "cannot convert"))
}

// TODO this needs to return an error before merge
func restoreNonConvertible(name string, annotations map[string]string, out interface{}) {
	v, ok := annotations[kapi.NonConvertibleAnnotationPrefix+"/"+name]
	if ok && len(v) > 0 {
		if err := json.Unmarshal([]byte(v), &out); err != nil {
			fmt.Printf("ERROR: failed to decode %q non-convertible field to %#+v: %v", name, out, err)
		}
	} else {
		fmt.Printf("WARNING: requested field not found (%q)\n", name)
	}
}

// errorsToNonConvertible converts the errors.Aggregate into
// non-convertible field annotations. It returns error when the error in
// aggregate is not a field error.
func errorsToNonConvertible(err error, out *map[string]string) error {
	if out == nil {
		out = &map[string]string{}
	}
	newMap := *out
	if fieldErrs, ok := err.(errors.Aggregate); ok {
		for _, e := range fieldErrs.Errors() {
			fieldErr, ok := e.(*field.Error)
			if !ok {
				return err
			}
			// encode the original value as JSON
			b, err := json.Marshal(fieldErr.BadValue)
			if err != nil {
				return err
			}
			newMap[kapi.NonConvertibleAnnotationPrefix+"/"+fieldErr.Field] = string(b)
		}
		*out = newMap
		return nil
	}
	return err
}
