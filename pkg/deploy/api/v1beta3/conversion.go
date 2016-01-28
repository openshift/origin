package v1beta3

import (
	"fmt"
	"math"
	"strings"

	"k8s.io/kubernetes/pkg/conversion"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/intstr"

	newer "github.com/openshift/origin/pkg/deploy/api"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

func convert_v1beta3_DeploymentTriggerImageChangeParams_To_api_DeploymentTriggerImageChangeParams(in *DeploymentTriggerImageChangeParams, out *newer.DeploymentTriggerImageChangeParams, s conversion.Scope) error {
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

func convert_api_DeploymentTriggerImageChangeParams_To_v1beta3_DeploymentTriggerImageChangeParams(in *newer.DeploymentTriggerImageChangeParams, out *DeploymentTriggerImageChangeParams, s conversion.Scope) error {
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

func convert_v1beta3_RollingDeploymentStrategyParams_To_api_RollingDeploymentStrategyParams(in *RollingDeploymentStrategyParams, out *newer.RollingDeploymentStrategyParams, s conversion.Scope) error {
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

func convert_api_RollingDeploymentStrategyParams_To_v1beta3_RollingDeploymentStrategyParams(in *newer.RollingDeploymentStrategyParams, out *RollingDeploymentStrategyParams, s conversion.Scope) error {
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

func addConversionFuncs(scheme *runtime.Scheme) {
	err := scheme.AddConversionFuncs(
		convert_v1beta3_DeploymentTriggerImageChangeParams_To_api_DeploymentTriggerImageChangeParams,
		convert_api_DeploymentTriggerImageChangeParams_To_v1beta3_DeploymentTriggerImageChangeParams,

		convert_v1beta3_RollingDeploymentStrategyParams_To_api_RollingDeploymentStrategyParams,
		convert_api_RollingDeploymentStrategyParams_To_v1beta3_RollingDeploymentStrategyParams,
	)
	if err != nil {
		panic(err)
	}
}
