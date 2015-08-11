/*
Copyright 2014 The Kubernetes Authors All rights reserved.

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

package v1beta3

import (
	"fmt"
	"reflect"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/conversion"
)

func addConversionFuncs() {
	// Add non-generated conversion functions
	err := api.Scheme.AddConversionFuncs(
		convert_v1beta3_Container_To_api_Container,
		convert_api_Container_To_v1beta3_Container,
		convert_v1beta3_ServiceSpec_To_api_ServiceSpec,
		convert_api_ServiceSpec_To_v1beta3_ServiceSpec,
		convert_v1beta3_PodSpec_To_api_PodSpec,
		convert_api_PodSpec_To_v1beta3_PodSpec,
		convert_v1beta3_ContainerState_To_api_ContainerState,
		convert_api_ContainerState_To_v1beta3_ContainerState,
		convert_api_ContainerStateTerminated_To_v1beta3_ContainerStateTerminated,
		convert_v1beta3_ContainerStateTerminated_To_api_ContainerStateTerminated,
		convert_v1beta3_StatusDetails_To_api_StatusDetails,
		convert_api_StatusDetails_To_v1beta3_StatusDetails,
		convert_v1beta3_StatusCause_To_api_StatusCause,
		convert_api_StatusCause_To_v1beta3_StatusCause,
		convert_api_ReplicationControllerSpec_To_v1beta3_ReplicationControllerSpec,
		convert_v1beta3_ReplicationControllerSpec_To_api_ReplicationControllerSpec,
		convert_v1beta3_VolumeSource_To_api_VolumeSource,
		convert_api_VolumeSource_To_v1beta3_VolumeSource,
	)
	if err != nil {
		// If one of the conversion functions is malformed, detect it immediately.
		panic(err)
	}

	// Add field conversion funcs.
	err = api.Scheme.AddFieldLabelConversionFunc("v1beta3", "Pod",
		func(label, value string) (string, string, error) {
			switch label {
			case "metadata.name",
				"metadata.namespace",
				"metadata.labels",
				"metadata.annotations",
				"status.podIP",
				"status.phase":
				return label, value, nil
			case "spec.host":
				return "spec.nodeName", value, nil
			default:
				return "", "", fmt.Errorf("field label not supported: %s", label)
			}
		})
	if err != nil {
		// If one of the conversion functions is malformed, detect it immediately.
		panic(err)
	}
	err = api.Scheme.AddFieldLabelConversionFunc("v1beta3", "Node",
		func(label, value string) (string, string, error) {
			switch label {
			case "metadata.name":
				return label, value, nil
			case "spec.unschedulable":
				return label, value, nil
			default:
				return "", "", fmt.Errorf("field label not supported: %s", label)
			}
		})
	if err != nil {
		// If one of the conversion functions is malformed, detect it immediately.
		panic(err)
	}
	err = api.Scheme.AddFieldLabelConversionFunc("v1beta3", "ReplicationController",
		func(label, value string) (string, string, error) {
			switch label {
			case "metadata.name",
				"status.replicas":
				return label, value, nil
			default:
				return "", "", fmt.Errorf("field label not supported: %s", label)
			}
		})
	if err != nil {
		// If one of the conversion functions is malformed, detect it immediately.
		panic(err)
	}
	err = api.Scheme.AddFieldLabelConversionFunc("v1beta3", "Event",
		func(label, value string) (string, string, error) {
			switch label {
			case "involvedObject.kind",
				"involvedObject.namespace",
				"involvedObject.name",
				"involvedObject.uid",
				"involvedObject.apiVersion",
				"involvedObject.resourceVersion",
				"involvedObject.fieldPath",
				"reason",
				"source":
				return label, value, nil
			default:
				return "", "", fmt.Errorf("field label not supported: %s", label)
			}
		})
	if err != nil {
		// If one of the conversion functions is malformed, detect it immediately.
		panic(err)
	}
	err = api.Scheme.AddFieldLabelConversionFunc("v1beta3", "Namespace",
		func(label, value string) (string, string, error) {
			switch label {
			case "status.phase":
				return label, value, nil
			default:
				return "", "", fmt.Errorf("field label not supported: %s", label)
			}
		})
	if err != nil {
		// If one of the conversion functions is malformed, detect it immediately.
		panic(err)
	}
	err = api.Scheme.AddFieldLabelConversionFunc("v1beta3", "Secret",
		func(label, value string) (string, string, error) {
			switch label {
			case "type":
				return label, value, nil
			default:
				return "", "", fmt.Errorf("field label not supported: %s", label)
			}
		})
	if err != nil {
		// If one of the conversion functions is malformed, detect it immediately.
		panic(err)
	}
	err = api.Scheme.AddFieldLabelConversionFunc("v1beta3", "ServiceAccount",
		func(label, value string) (string, string, error) {
			switch label {
			case "metadata.name":
				return label, value, nil
			default:
				return "", "", fmt.Errorf("field label not supported: %s", label)
			}
		})
	if err != nil {
		// If one of the conversion functions is malformed, detect it immediately.
		panic(err)
	}
	err = api.Scheme.AddFieldLabelConversionFunc("v1beta3", "Endpoints",
		func(label, value string) (string, string, error) {
			switch label {
			case "metadata.name":
				return label, value, nil
			default:
				return "", "", fmt.Errorf("field label not supported: %s", label)
			}
		})
	if err != nil {
		// If one of the conversion functions is malformed, detect it immediately.
		panic(err)
	}
}

func convert_v1beta3_Container_To_api_Container(in *Container, out *api.Container, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*Container))(in)
	}
	out.Name = in.Name
	out.Image = in.Image
	if in.Command != nil {
		out.Command = make([]string, len(in.Command))
		for i := range in.Command {
			out.Command[i] = in.Command[i]
		}
	}
	if in.Args != nil {
		out.Args = make([]string, len(in.Args))
		for i := range in.Args {
			out.Args[i] = in.Args[i]
		}
	}
	out.WorkingDir = in.WorkingDir
	if in.Ports != nil {
		out.Ports = make([]api.ContainerPort, len(in.Ports))
		for i := range in.Ports {
			if err := convert_v1beta3_ContainerPort_To_api_ContainerPort(&in.Ports[i], &out.Ports[i], s); err != nil {
				return err
			}
		}
	}
	if in.Env != nil {
		out.Env = make([]api.EnvVar, len(in.Env))
		for i := range in.Env {
			if err := convert_v1beta3_EnvVar_To_api_EnvVar(&in.Env[i], &out.Env[i], s); err != nil {
				return err
			}
		}
	}
	if err := s.Convert(&in.Resources, &out.Resources, 0); err != nil {
		return err
	}
	if in.VolumeMounts != nil {
		out.VolumeMounts = make([]api.VolumeMount, len(in.VolumeMounts))
		for i := range in.VolumeMounts {
			if err := convert_v1beta3_VolumeMount_To_api_VolumeMount(&in.VolumeMounts[i], &out.VolumeMounts[i], s); err != nil {
				return err
			}
		}
	}
	if in.LivenessProbe != nil {
		out.LivenessProbe = new(api.Probe)
		if err := convert_v1beta3_Probe_To_api_Probe(in.LivenessProbe, out.LivenessProbe, s); err != nil {
			return err
		}
	} else {
		out.LivenessProbe = nil
	}
	if in.ReadinessProbe != nil {
		out.ReadinessProbe = new(api.Probe)
		if err := convert_v1beta3_Probe_To_api_Probe(in.ReadinessProbe, out.ReadinessProbe, s); err != nil {
			return err
		}
	} else {
		out.ReadinessProbe = nil
	}
	if in.Lifecycle != nil {
		out.Lifecycle = new(api.Lifecycle)
		if err := convert_v1beta3_Lifecycle_To_api_Lifecycle(in.Lifecycle, out.Lifecycle, s); err != nil {
			return err
		}
	} else {
		out.Lifecycle = nil
	}
	out.TerminationMessagePath = in.TerminationMessagePath
	out.ImagePullPolicy = api.PullPolicy(in.ImagePullPolicy)
	if in.SecurityContext != nil {
		if in.SecurityContext.Capabilities != nil {
			if !reflect.DeepEqual(in.SecurityContext.Capabilities.Add, in.Capabilities.Add) ||
				!reflect.DeepEqual(in.SecurityContext.Capabilities.Drop, in.Capabilities.Drop) {
				return fmt.Errorf("container capability settings do not match security context settings, cannot convert")
			}
		}
		if in.SecurityContext.Privileged != nil {
			if in.Privileged != *in.SecurityContext.Privileged {
				return fmt.Errorf("container privileged settings do not match security context settings, cannot convert")
			}
		}
	}
	if in.SecurityContext != nil {
		out.SecurityContext = new(api.SecurityContext)
		if err := convert_v1beta3_SecurityContext_To_api_SecurityContext(in.SecurityContext, out.SecurityContext, s); err != nil {
			return err
		}
	} else {
		out.SecurityContext = nil
	}

	out.Stdin = in.Stdin
	out.TTY = in.TTY
	return nil
}

func convert_api_Container_To_v1beta3_Container(in *api.Container, out *Container, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.Container))(in)
	}
	out.Name = in.Name
	out.Image = in.Image
	if in.Command != nil {
		out.Command = make([]string, len(in.Command))
		for i := range in.Command {
			out.Command[i] = in.Command[i]
		}
	}
	if in.Args != nil {
		out.Args = make([]string, len(in.Args))
		for i := range in.Args {
			out.Args[i] = in.Args[i]
		}
	}
	out.WorkingDir = in.WorkingDir
	if in.Ports != nil {
		out.Ports = make([]ContainerPort, len(in.Ports))
		for i := range in.Ports {
			if err := convert_api_ContainerPort_To_v1beta3_ContainerPort(&in.Ports[i], &out.Ports[i], s); err != nil {
				return err
			}
		}
	}
	if in.Env != nil {
		out.Env = make([]EnvVar, len(in.Env))
		for i := range in.Env {
			if err := convert_api_EnvVar_To_v1beta3_EnvVar(&in.Env[i], &out.Env[i], s); err != nil {
				return err
			}
		}
	}
	if err := s.Convert(&in.Resources, &out.Resources, 0); err != nil {
		return err
	}
	if in.VolumeMounts != nil {
		out.VolumeMounts = make([]VolumeMount, len(in.VolumeMounts))
		for i := range in.VolumeMounts {
			if err := convert_api_VolumeMount_To_v1beta3_VolumeMount(&in.VolumeMounts[i], &out.VolumeMounts[i], s); err != nil {
				return err
			}
		}
	}
	if in.LivenessProbe != nil {
		out.LivenessProbe = new(Probe)
		if err := convert_api_Probe_To_v1beta3_Probe(in.LivenessProbe, out.LivenessProbe, s); err != nil {
			return err
		}
	} else {
		out.LivenessProbe = nil
	}
	if in.ReadinessProbe != nil {
		out.ReadinessProbe = new(Probe)
		if err := convert_api_Probe_To_v1beta3_Probe(in.ReadinessProbe, out.ReadinessProbe, s); err != nil {
			return err
		}
	} else {
		out.ReadinessProbe = nil
	}
	if in.Lifecycle != nil {
		out.Lifecycle = new(Lifecycle)
		if err := convert_api_Lifecycle_To_v1beta3_Lifecycle(in.Lifecycle, out.Lifecycle, s); err != nil {
			return err
		}
	} else {
		out.Lifecycle = nil
	}
	out.TerminationMessagePath = in.TerminationMessagePath
	out.ImagePullPolicy = PullPolicy(in.ImagePullPolicy)
	if in.SecurityContext != nil {
		out.SecurityContext = new(SecurityContext)
		if err := convert_api_SecurityContext_To_v1beta3_SecurityContext(in.SecurityContext, out.SecurityContext, s); err != nil {
			return err
		}
	} else {
		out.SecurityContext = nil
	}
	// now that we've converted set the container field from security context
	if out.SecurityContext != nil && out.SecurityContext.Privileged != nil {
		out.Privileged = *out.SecurityContext.Privileged
	}
	// now that we've converted set the container field from security context
	if out.SecurityContext != nil && out.SecurityContext.Capabilities != nil {
		out.Capabilities = *out.SecurityContext.Capabilities
	}

	out.Stdin = in.Stdin
	out.TTY = in.TTY
	return nil
}

func convert_v1beta3_ServiceSpec_To_api_ServiceSpec(in *ServiceSpec, out *api.ServiceSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*ServiceSpec))(in)
	}
	if in.Ports != nil {
		out.Ports = make([]api.ServicePort, len(in.Ports))
		for i := range in.Ports {
			if err := convert_v1beta3_ServicePort_To_api_ServicePort(&in.Ports[i], &out.Ports[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Ports = nil
	}
	if in.Selector != nil {
		out.Selector = make(map[string]string)
		for key, val := range in.Selector {
			out.Selector[key] = val
		}
	} else {
		out.Selector = nil
	}
	out.ClusterIP = in.PortalIP

	typeIn := in.Type
	if typeIn == "" {
		if in.CreateExternalLoadBalancer {
			typeIn = ServiceTypeLoadBalancer
		} else {
			typeIn = ServiceTypeClusterIP
		}
	}
	if err := s.Convert(&typeIn, &out.Type, 0); err != nil {
		return err
	}

	if in.PublicIPs != nil {
		out.ExternalIPs = make([]string, len(in.PublicIPs))
		for i := range in.PublicIPs {
			out.ExternalIPs[i] = in.PublicIPs[i]
		}
	} else {
		out.ExternalIPs = nil
	}
	out.SessionAffinity = api.ServiceAffinity(in.SessionAffinity)
	return nil
}

func convert_api_ServiceSpec_To_v1beta3_ServiceSpec(in *api.ServiceSpec, out *ServiceSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.ServiceSpec))(in)
	}
	if in.Ports != nil {
		out.Ports = make([]ServicePort, len(in.Ports))
		for i := range in.Ports {
			if err := convert_api_ServicePort_To_v1beta3_ServicePort(&in.Ports[i], &out.Ports[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Ports = nil
	}
	if in.Selector != nil {
		out.Selector = make(map[string]string)
		for key, val := range in.Selector {
			out.Selector[key] = val
		}
	} else {
		out.Selector = nil
	}
	out.PortalIP = in.ClusterIP

	if err := s.Convert(&in.Type, &out.Type, 0); err != nil {
		return err
	}
	out.CreateExternalLoadBalancer = in.Type == api.ServiceTypeLoadBalancer

	if in.ExternalIPs != nil {
		out.PublicIPs = make([]string, len(in.ExternalIPs))
		for i := range in.ExternalIPs {
			out.PublicIPs[i] = in.ExternalIPs[i]
		}
	} else {
		out.PublicIPs = nil
	}
	out.SessionAffinity = ServiceAffinity(in.SessionAffinity)
	return nil
}

func convert_v1beta3_PodSpec_To_api_PodSpec(in *PodSpec, out *api.PodSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*PodSpec))(in)
	}
	if in.Volumes != nil {
		out.Volumes = make([]api.Volume, len(in.Volumes))
		for i := range in.Volumes {
			if err := convert_v1beta3_Volume_To_api_Volume(&in.Volumes[i], &out.Volumes[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Volumes = nil
	}
	if in.Containers != nil {
		out.Containers = make([]api.Container, len(in.Containers))
		for i := range in.Containers {
			if err := convert_v1beta3_Container_To_api_Container(&in.Containers[i], &out.Containers[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Containers = nil
	}
	out.RestartPolicy = api.RestartPolicy(in.RestartPolicy)
	if in.TerminationGracePeriodSeconds != nil {
		out.TerminationGracePeriodSeconds = new(int64)
		*out.TerminationGracePeriodSeconds = *in.TerminationGracePeriodSeconds
	} else {
		out.TerminationGracePeriodSeconds = nil
	}
	if in.ActiveDeadlineSeconds != nil {
		out.ActiveDeadlineSeconds = new(int64)
		*out.ActiveDeadlineSeconds = *in.ActiveDeadlineSeconds
	} else {
		out.ActiveDeadlineSeconds = nil
	}
	out.DNSPolicy = api.DNSPolicy(in.DNSPolicy)
	if in.NodeSelector != nil {
		out.NodeSelector = make(map[string]string)
		for key, val := range in.NodeSelector {
			out.NodeSelector[key] = val
		}
	} else {
		out.NodeSelector = nil
	}
	out.ServiceAccountName = in.ServiceAccount
	out.NodeName = in.Host
	out.SecurityContext = &api.PodSecurityContext{}
	out.SecurityContext.HostNetwork = in.HostNetwork
	out.SecurityContext.HostPID = in.HostPID
	out.SecurityContext.HostIPC = in.HostIPC
	if in.ImagePullSecrets != nil {
		out.ImagePullSecrets = make([]api.LocalObjectReference, len(in.ImagePullSecrets))
		for i := range in.ImagePullSecrets {
			if err := convert_v1beta3_LocalObjectReference_To_api_LocalObjectReference(&in.ImagePullSecrets[i], &out.ImagePullSecrets[i], s); err != nil {
				return err
			}
		}
	} else {
		out.ImagePullSecrets = nil
	}
	return nil
}

func convert_api_PodSpec_To_v1beta3_PodSpec(in *api.PodSpec, out *PodSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.PodSpec))(in)
	}
	if in.Volumes != nil {
		out.Volumes = make([]Volume, len(in.Volumes))
		for i := range in.Volumes {
			if err := convert_api_Volume_To_v1beta3_Volume(&in.Volumes[i], &out.Volumes[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Volumes = nil
	}
	if in.Containers != nil {
		out.Containers = make([]Container, len(in.Containers))
		for i := range in.Containers {
			if err := convert_api_Container_To_v1beta3_Container(&in.Containers[i], &out.Containers[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Containers = nil
	}
	out.RestartPolicy = RestartPolicy(in.RestartPolicy)
	if in.TerminationGracePeriodSeconds != nil {
		out.TerminationGracePeriodSeconds = new(int64)
		*out.TerminationGracePeriodSeconds = *in.TerminationGracePeriodSeconds
	} else {
		out.TerminationGracePeriodSeconds = nil
	}
	if in.ActiveDeadlineSeconds != nil {
		out.ActiveDeadlineSeconds = new(int64)
		*out.ActiveDeadlineSeconds = *in.ActiveDeadlineSeconds
	} else {
		out.ActiveDeadlineSeconds = nil
	}
	out.DNSPolicy = DNSPolicy(in.DNSPolicy)
	if in.NodeSelector != nil {
		out.NodeSelector = make(map[string]string)
		for key, val := range in.NodeSelector {
			out.NodeSelector[key] = val
		}
	} else {
		out.NodeSelector = nil
	}
	out.ServiceAccount = in.ServiceAccountName
	out.Host = in.NodeName
	if in.SecurityContext != nil {
		out.HostNetwork = in.SecurityContext.HostNetwork
		out.HostPID = in.SecurityContext.HostPID
		out.HostIPC = in.SecurityContext.HostIPC
	}
	if in.ImagePullSecrets != nil {
		out.ImagePullSecrets = make([]LocalObjectReference, len(in.ImagePullSecrets))
		for i := range in.ImagePullSecrets {
			if err := convert_api_LocalObjectReference_To_v1beta3_LocalObjectReference(&in.ImagePullSecrets[i], &out.ImagePullSecrets[i], s); err != nil {
				return err
			}
		}
	} else {
		out.ImagePullSecrets = nil
	}
	return nil
}

func convert_api_ContainerState_To_v1beta3_ContainerState(in *api.ContainerState, out *ContainerState, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.ContainerState))(in)
	}
	if in.Waiting != nil {
		out.Waiting = new(ContainerStateWaiting)
		if err := convert_api_ContainerStateWaiting_To_v1beta3_ContainerStateWaiting(in.Waiting, out.Waiting, s); err != nil {
			return err
		}
	} else {
		out.Waiting = nil
	}
	if in.Running != nil {
		out.Running = new(ContainerStateRunning)
		if err := convert_api_ContainerStateRunning_To_v1beta3_ContainerStateRunning(in.Running, out.Running, s); err != nil {
			return err
		}
	} else {
		out.Running = nil
	}
	if in.Terminated != nil {
		out.Termination = new(ContainerStateTerminated)
		if err := convert_api_ContainerStateTerminated_To_v1beta3_ContainerStateTerminated(in.Terminated, out.Termination, s); err != nil {
			return err
		}
	} else {
		out.Termination = nil
	}
	return nil
}

func convert_v1beta3_ContainerState_To_api_ContainerState(in *ContainerState, out *api.ContainerState, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*ContainerState))(in)
	}
	if in.Waiting != nil {
		out.Waiting = new(api.ContainerStateWaiting)
		if err := convert_v1beta3_ContainerStateWaiting_To_api_ContainerStateWaiting(in.Waiting, out.Waiting, s); err != nil {
			return err
		}
	} else {
		out.Waiting = nil
	}
	if in.Running != nil {
		out.Running = new(api.ContainerStateRunning)
		if err := convert_v1beta3_ContainerStateRunning_To_api_ContainerStateRunning(in.Running, out.Running, s); err != nil {
			return err
		}
	} else {
		out.Running = nil
	}
	if in.Termination != nil {
		out.Terminated = new(api.ContainerStateTerminated)
		if err := convert_v1beta3_ContainerStateTerminated_To_api_ContainerStateTerminated(in.Termination, out.Terminated, s); err != nil {
			return err
		}
	} else {
		out.Terminated = nil
	}
	return nil
}

func convert_api_ContainerStateTerminated_To_v1beta3_ContainerStateTerminated(in *api.ContainerStateTerminated, out *ContainerStateTerminated, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.ContainerStateTerminated))(in)
	}
	out.ExitCode = in.ExitCode
	out.Signal = in.Signal
	out.Reason = in.Reason
	out.Message = in.Message
	if err := s.Convert(&in.StartedAt, &out.StartedAt, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.FinishedAt, &out.FinishedAt, 0); err != nil {
		return err
	}
	out.ContainerID = in.ContainerID
	return nil
}

func convert_v1beta3_ContainerStateTerminated_To_api_ContainerStateTerminated(in *ContainerStateTerminated, out *api.ContainerStateTerminated, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*ContainerStateTerminated))(in)
	}
	out.ExitCode = in.ExitCode
	out.Signal = in.Signal
	out.Reason = in.Reason
	out.Message = in.Message
	if err := s.Convert(&in.StartedAt, &out.StartedAt, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.FinishedAt, &out.FinishedAt, 0); err != nil {
		return err
	}
	out.ContainerID = in.ContainerID
	return nil
}

func convert_v1beta3_StatusDetails_To_api_StatusDetails(in *StatusDetails, out *unversioned.StatusDetails, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*StatusDetails))(in)
	}
	out.Name = in.ID
	out.Kind = in.Kind
	if in.Causes != nil {
		out.Causes = make([]unversioned.StatusCause, len(in.Causes))
		for i := range in.Causes {
			if err := convert_v1beta3_StatusCause_To_api_StatusCause(&in.Causes[i], &out.Causes[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Causes = nil
	}
	out.RetryAfterSeconds = in.RetryAfterSeconds
	return nil
}

func convert_api_StatusDetails_To_v1beta3_StatusDetails(in *unversioned.StatusDetails, out *StatusDetails, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*unversioned.StatusDetails))(in)
	}
	out.ID = in.Name
	out.Kind = in.Kind
	if in.Causes != nil {
		out.Causes = make([]StatusCause, len(in.Causes))
		for i := range in.Causes {
			if err := convert_api_StatusCause_To_v1beta3_StatusCause(&in.Causes[i], &out.Causes[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Causes = nil
	}
	out.RetryAfterSeconds = in.RetryAfterSeconds
	return nil
}

func convert_v1beta3_StatusCause_To_api_StatusCause(in *StatusCause, out *unversioned.StatusCause, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*StatusCause))(in)
	}
	out.Type = unversioned.CauseType(in.Type)
	out.Message = in.Message
	out.Field = in.Field
	return nil
}

func convert_api_StatusCause_To_v1beta3_StatusCause(in *unversioned.StatusCause, out *StatusCause, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*unversioned.StatusCause))(in)
	}
	out.Type = CauseType(in.Type)
	out.Message = in.Message
	out.Field = in.Field
	return nil
}

func convert_api_ReplicationControllerSpec_To_v1beta3_ReplicationControllerSpec(in *api.ReplicationControllerSpec, out *ReplicationControllerSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.ReplicationControllerSpec))(in)
	}
	out.Replicas = &in.Replicas
	if in.Selector != nil {
		out.Selector = make(map[string]string)
		for key, val := range in.Selector {
			out.Selector[key] = val
		}
	} else {
		out.Selector = nil
	}
	if in.Template != nil {
		out.Template = new(PodTemplateSpec)
		if err := convert_api_PodTemplateSpec_To_v1beta3_PodTemplateSpec(in.Template, out.Template, s); err != nil {
			return err
		}
	} else {
		out.Template = nil
	}
	return nil
}

func convert_v1beta3_ReplicationControllerSpec_To_api_ReplicationControllerSpec(in *ReplicationControllerSpec, out *api.ReplicationControllerSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*ReplicationControllerSpec))(in)
	}
	out.Replicas = *in.Replicas
	if in.Selector != nil {
		out.Selector = make(map[string]string)
		for key, val := range in.Selector {
			out.Selector[key] = val
		}
	} else {
		out.Selector = nil
	}
	if in.Template != nil {
		out.Template = new(api.PodTemplateSpec)
		if err := convert_v1beta3_PodTemplateSpec_To_api_PodTemplateSpec(in.Template, out.Template, s); err != nil {
			return err
		}
	} else {
		out.Template = nil
	}
	return nil
}

// This will convert our internal represantation of VolumeSource to its v1beta3 representation
// Used for keeping backwards compatibility for the Metadata field
func convert_api_VolumeSource_To_v1beta3_VolumeSource(in *api.VolumeSource, out *VolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.VolumeSource))(in)
	}

	if err := s.DefaultConvert(in, out, conversion.IgnoreMissingFields); err != nil {
		return err
	}

	if in.DownwardAPI != nil {
		out.Metadata = new(MetadataVolumeSource)
		if err := convert_api_DownwardAPIVolumeSource_To_v1beta3_MetadataVolumeSource(in.DownwardAPI, out.Metadata, s); err != nil {
			return err
		}
	}
	return nil
}

// downward -> metadata (api -> v1beta3)
func convert_api_DownwardAPIVolumeSource_To_v1beta3_MetadataVolumeSource(in *api.DownwardAPIVolumeSource, out *MetadataVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.DownwardAPIVolumeSource))(in)
	}
	if in.Items != nil {
		out.Items = make([]MetadataFile, len(in.Items))
		for i := range in.Items {
			if err := convert_api_DownwardAPIVolumeFile_To_v1beta3_MetadataFile(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	}
	return nil
}

func convert_api_DownwardAPIVolumeFile_To_v1beta3_MetadataFile(in *api.DownwardAPIVolumeFile, out *MetadataFile, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.DownwardAPIVolumeFile))(in)
	}
	out.Name = in.Path
	if err := convert_api_ObjectFieldSelector_To_v1beta3_ObjectFieldSelector(&in.FieldRef, &out.FieldRef, s); err != nil {
		return err
	}
	return nil
}

// This will convert the v1beta3 representation of VolumeSource to our internal representation
// Used for keeping backwards compatibility for the Metadata field
func convert_v1beta3_VolumeSource_To_api_VolumeSource(in *VolumeSource, out *api.VolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*VolumeSource))(in)
	}

	if err := s.DefaultConvert(in, out, conversion.IgnoreMissingFields); err != nil {
		return err
	}

	// if specified Metadata will stomp DownwardAPI
	if in.Metadata != nil {
		out.DownwardAPI = new(api.DownwardAPIVolumeSource)
		if err := convert_v1beta3_MetadataVolumeSource_To_api_DownwardAPIVolumeSource(in.Metadata, out.DownwardAPI, s); err != nil {
			return err
		}
	}
	return nil
}

// metadata -> downward (v1beta3 -> api)
func convert_v1beta3_MetadataVolumeSource_To_api_DownwardAPIVolumeSource(in *MetadataVolumeSource, out *api.DownwardAPIVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*MetadataVolumeSource))(in)
	}
	if in.Items != nil {
		out.Items = make([]api.DownwardAPIVolumeFile, len(in.Items))
		for i := range in.Items {
			if err := convert_v1beta3_MetadataFile_To_api_DownwardAPIVolumeFile(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	}
	return nil
}

func convert_v1beta3_MetadataFile_To_api_DownwardAPIVolumeFile(in *MetadataFile, out *api.DownwardAPIVolumeFile, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*MetadataFile))(in)
	}
	out.Path = in.Name
	if err := convert_v1beta3_ObjectFieldSelector_To_api_ObjectFieldSelector(&in.FieldRef, &out.FieldRef, s); err != nil {
		return err
	}
	return nil
}
