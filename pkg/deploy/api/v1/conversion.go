package v1

import (
	"fmt"
	"math"
	"reflect"
	"strings"

	"k8s.io/kubernetes/pkg/api"
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

func addConversionFuncs(scheme *runtime.Scheme) {
	err := scheme.AddConversionFuncs(
		Convert_v1_DeploymentTriggerImageChangeParams_To_api_DeploymentTriggerImageChangeParams,
		Convert_api_DeploymentTriggerImageChangeParams_To_v1_DeploymentTriggerImageChangeParams,

		Convert_v1_RollingDeploymentStrategyParams_To_api_RollingDeploymentStrategyParams,
		Convert_api_RollingDeploymentStrategyParams_To_v1_RollingDeploymentStrategyParams,

		Convert_v1beta1_ReplicaSet_to_api_ReplicationController,
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
