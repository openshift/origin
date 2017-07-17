package v1

import (
	"strings"

	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"

	oapi "github.com/openshift/origin/pkg/api"
	newer "github.com/openshift/origin/pkg/deploy/apis/apps"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
)

func Convert_v1_DeploymentTriggerImageChangeParams_To_apps_DeploymentTriggerImageChangeParams(in *DeploymentTriggerImageChangeParams, out *newer.DeploymentTriggerImageChangeParams, s conversion.Scope) error {
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

func Convert_apps_DeploymentTriggerImageChangeParams_To_v1_DeploymentTriggerImageChangeParams(in *newer.DeploymentTriggerImageChangeParams, out *DeploymentTriggerImageChangeParams, s conversion.Scope) error {
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

func Convert_v1_RollingDeploymentStrategyParams_To_apps_RollingDeploymentStrategyParams(in *RollingDeploymentStrategyParams, out *newer.RollingDeploymentStrategyParams, s conversion.Scope) error {
	SetDefaults_RollingDeploymentStrategyParams(in)

	out.UpdatePeriodSeconds = in.UpdatePeriodSeconds
	out.IntervalSeconds = in.IntervalSeconds
	out.TimeoutSeconds = in.TimeoutSeconds

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
	if in.MaxUnavailable != nil {
		if err := s.Convert(in.MaxUnavailable, &out.MaxUnavailable, 0); err != nil {
			return err
		}
	}
	if in.MaxSurge != nil {
		if err := s.Convert(in.MaxSurge, &out.MaxSurge, 0); err != nil {
			return err
		}
	}
	return nil
}

func Convert_apps_RollingDeploymentStrategyParams_To_v1_RollingDeploymentStrategyParams(in *newer.RollingDeploymentStrategyParams, out *RollingDeploymentStrategyParams, s conversion.Scope) error {
	out.UpdatePeriodSeconds = in.UpdatePeriodSeconds
	out.IntervalSeconds = in.IntervalSeconds
	out.TimeoutSeconds = in.TimeoutSeconds

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
	if err := s.Convert(&in.MaxUnavailable, out.MaxUnavailable, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.MaxSurge, out.MaxSurge, 0); err != nil {
		return err
	}
	return nil
}

func addConversionFuncs(scheme *runtime.Scheme) error {
	err := scheme.AddConversionFuncs(
		Convert_v1_DeploymentTriggerImageChangeParams_To_apps_DeploymentTriggerImageChangeParams,
		Convert_apps_DeploymentTriggerImageChangeParams_To_v1_DeploymentTriggerImageChangeParams,

		Convert_v1_RollingDeploymentStrategyParams_To_apps_RollingDeploymentStrategyParams,
		Convert_apps_RollingDeploymentStrategyParams_To_v1_RollingDeploymentStrategyParams,
	)
	if err != nil {
		return err
	}

	if err := scheme.AddFieldLabelConversionFunc("v1", "DeploymentConfig",
		oapi.GetFieldLabelConversionFunc(newer.DeploymentConfigToSelectableFields(&newer.DeploymentConfig{}), nil),
	); err != nil {
		return err
	}
	return nil
}
