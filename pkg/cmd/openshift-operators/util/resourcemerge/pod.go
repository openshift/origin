package resourcemerge

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
)

func ensurePodTemplateSpec(modified *bool, existing *corev1.PodTemplateSpec, required corev1.PodTemplateSpec) {
	EnsureObjectMeta(modified, &existing.ObjectMeta, required.ObjectMeta)

	ensurePodSpec(modified, &existing.Spec, required.Spec)
}

func ensurePodSpec(modified *bool, existing *corev1.PodSpec, required corev1.PodSpec) {
	// any container we specify, we require.  Any extra container is removed
	for _, required := range required.Containers {
		var existingCurr *corev1.Container
		for j, curr := range existing.Containers {
			if curr.Name == required.Name {
				existingCurr = &existing.Containers[j]
				break
			}
		}
		if existingCurr == nil {
			*modified = true
			existing.Containers = append(existing.Containers, corev1.Container{})
			existingCurr = &existing.Containers[len(existing.Containers)-1]
		}
		ensureContainer(modified, existingCurr, required)
	}
	deleted := 0
	for i, curr := range existing.Containers {
		found := false
		for _, required := range required.Containers {
			if curr.Name == required.Name {
				found = true
				break
			}
		}
		if !found {
			j := i - deleted
			existing.Containers = existing.Containers[:j+copy(existing.Containers[j:], existing.Containers[j+1:])]
		}
	}

	// any volume we specify, we require
	// TODO perhaps this should be relaxed.  If someone wants to mount them from somewhere else, we should allow it
	for _, required := range required.Volumes {
		var existingCurr *corev1.Volume
		for j, curr := range existing.Volumes {
			if curr.Name == required.Name {
				existingCurr = &existing.Volumes[j]
				break
			}
		}
		if existingCurr == nil {
			*modified = true
			existing.Volumes = append(existing.Volumes, corev1.Volume{})
			existingCurr = &existing.Volumes[len(existing.Volumes)-1]
		}
		ensureVolume(modified, existingCurr, required)
	}

	if len(required.RestartPolicy) > 0 {
		if existing.RestartPolicy != required.RestartPolicy {
			*modified = true
			existing.RestartPolicy = required.RestartPolicy
		}
	}

	SetString(modified, &existing.ServiceAccountName, required.ServiceAccountName)
	SetBool(modified, &existing.HostNetwork, required.HostNetwork)
}

func ensureContainer(modified *bool, existing *corev1.Container, required corev1.Container) {
	SetString(modified, &existing.Name, required.Name)
	SetString(modified, &existing.Image, required.Image)

	// if you want modify the launch, you need to modify it in the config, not in the launch args
	SetStringSlice(modified, &existing.Command, required.Command)
	SetStringSlice(modified, &existing.Args, required.Args)

	SetStringIfSet(modified, &existing.WorkingDir, required.WorkingDir)

	// any port we specify, we require
	for _, required := range required.Ports {
		var existingCurr *corev1.ContainerPort
		for j, curr := range existing.Ports {
			if curr.Name == required.Name {
				existingCurr = &existing.Ports[j]
				break
			}
		}
		if existingCurr == nil {
			*modified = true
			existing.Ports = append(existing.Ports, corev1.ContainerPort{})
			existingCurr = &existing.Ports[len(existing.Ports)-1]
		}
		ensureContainerPort(modified, existingCurr, required)
	}

	// any volume mount we specify, we require
	for _, required := range required.VolumeMounts {
		var existingCurr *corev1.VolumeMount
		for j, curr := range existing.VolumeMounts {
			if curr.Name == required.Name {
				existingCurr = &existing.VolumeMounts[j]
				break
			}
		}
		if existingCurr == nil {
			*modified = true
			existing.VolumeMounts = append(existing.VolumeMounts, corev1.VolumeMount{})
			existingCurr = &existing.VolumeMounts[len(existing.VolumeMounts)-1]
		}
		ensureVolumeMount(modified, existingCurr, required)
	}

	if required.LivenessProbe != nil {
		ensureProbePtr(modified, &existing.LivenessProbe, required.LivenessProbe)
	}
	if required.ReadinessProbe != nil {
		ensureProbePtr(modified, &existing.ReadinessProbe, required.ReadinessProbe)
	}

	// our security context should always win
	ensureSecurityContextPtr(modified, &existing.SecurityContext, required.SecurityContext)
}

func ensureProbePtr(modified *bool, existing **corev1.Probe, required *corev1.Probe) {
	// if we have no required, then we don't care what someone else has set
	if required == nil {
		return
	}
	if *existing == nil {
		*modified = true
		*existing = required
		return
	}
	ensureProbe(modified, *existing, *required)
}

func ensureProbe(modified *bool, existing *corev1.Probe, required corev1.Probe) {
	SetInt32IfSet(modified, &existing.InitialDelaySeconds, required.InitialDelaySeconds)

	ensureProbeHandler(modified, &existing.Handler, required.Handler)
}

func ensureProbeHandler(modified *bool, existing *corev1.Handler, required corev1.Handler) {
	if !equality.Semantic.DeepEqual(required, *existing) {
		*modified = true
		*existing = required
	}
}

func ensureContainerPort(modified *bool, existing *corev1.ContainerPort, required corev1.ContainerPort) {
	if !equality.Semantic.DeepEqual(required, *existing) {
		*modified = true
		*existing = required
	}
}

func ensureVolumeMount(modified *bool, existing *corev1.VolumeMount, required corev1.VolumeMount) {
	if !equality.Semantic.DeepEqual(required, *existing) {
		*modified = true
		*existing = required
	}
}

func ensureVolume(modified *bool, existing *corev1.Volume, required corev1.Volume) {
	if !equality.Semantic.DeepEqual(required, *existing) {
		*modified = true
		*existing = required
	}
}

func ensureSecurityContext(modified *bool, existing *corev1.SecurityContext, required corev1.SecurityContext) {
	// these are missing for now
	//Capabilities *Capabilities `json:"capabilities,omitempty" protobuf:"bytes,1,opt,name=capabilities"`
	//SELinuxOptions *SELinuxOptions `json:"seLinuxOptions,omitempty" protobuf:"bytes,3,opt,name=seLinuxOptions"`
	setBoolPtr(modified, &existing.Privileged, required.Privileged)
	setInt64Ptr(modified, &existing.RunAsUser, required.RunAsUser)
	setBoolPtr(modified, &existing.RunAsNonRoot, required.RunAsNonRoot)
	setBoolPtr(modified, &existing.ReadOnlyRootFilesystem, required.ReadOnlyRootFilesystem)
	setBoolPtr(modified, &existing.AllowPrivilegeEscalation, required.AllowPrivilegeEscalation)
}

func ensureSecurityContextPtr(modified *bool, existing **corev1.SecurityContext, required *corev1.SecurityContext) {
	// if we have no required, then we don't care what someone else has set
	if required == nil {
		return
	}

	if *existing == nil {
		*modified = true
		*existing = required
		return
	}
	ensureSecurityContext(modified, *existing, *required)
}
