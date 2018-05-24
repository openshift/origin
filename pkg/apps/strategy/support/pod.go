package support

import (
	"fmt"
	"time"

	"github.com/openshift/origin/pkg/api/apihelpers"
	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
	"github.com/openshift/origin/pkg/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
)

func createHookPodManifest(hook *appsapi.LifecycleHook, rc *corev1.ReplicationController, strategy *appsapi.DeploymentStrategy, hookType string,
	startTime time.Time) (*corev1.Pod, error) {

	exec := hook.ExecNewPod

	baseContainer := &corev1.Container{}

	for _, container := range rc.Spec.Template.Spec.Containers {
		if container.Name == exec.ContainerName {
			baseContainer = &container
			break
		}
	}
	if baseContainer == nil {
		return nil, fmt.Errorf("no container named '%s' found in rc template", exec.ContainerName)
	}

	// Build a merged environment; hook environment takes precedence over base
	// container environment
	envMap := map[string]corev1.EnvVar{}
	mergedEnv := []corev1.EnvVar{}
	for _, env := range baseContainer.Env {
		envMap[env.Name] = env
	}

	for _, env := range exec.Env {
		newEnv := corev1.EnvVar{}
		if err := legacyscheme.Scheme.Convert(&env, &newEnv, nil); err != nil {
			return nil, err
		}
		envMap[env.Name] = newEnv
	}
	for k, v := range envMap {
		mergedEnv = append(mergedEnv, corev1.EnvVar{Name: k, Value: v.Value, ValueFrom: v.ValueFrom})
	}
	mergedEnv = append(mergedEnv, corev1.EnvVar{Name: "OPENSHIFT_DEPLOYMENT_NAME", Value: rc.Name})
	mergedEnv = append(mergedEnv, corev1.EnvVar{Name: "OPENSHIFT_DEPLOYMENT_NAMESPACE", Value: rc.Namespace})

	// Assigning to a variable since its address is required
	defaultActiveDeadline := appsapi.MaxDeploymentDurationSeconds
	if strategy.ActiveDeadlineSeconds != nil {
		defaultActiveDeadline = *(strategy.ActiveDeadlineSeconds)
	}
	maxDeploymentDurationSeconds := defaultActiveDeadline - int64(time.Since(startTime).Seconds())

	// Let the kubelet manage retries if requested
	restartPolicy := corev1.RestartPolicyNever
	if hook.FailurePolicy == appsapi.LifecycleHookFailurePolicyRetry {
		restartPolicy = corev1.RestartPolicyOnFailure
	}

	// Transfer any requested volumes to the hook pod.
	volumes := []corev1.Volume{}
	volumeNames := sets.NewString()
	for _, volume := range rc.Spec.Template.Spec.Volumes {
		for _, name := range exec.Volumes {
			if volume.Name == name {
				volumes = append(volumes, volume)
				volumeNames.Insert(volume.Name)
			}
		}
	}
	// Transfer any volume mounts associated with transferred volumes.
	volumeMounts := []corev1.VolumeMount{}
	for _, mount := range baseContainer.VolumeMounts {
		if volumeNames.Has(mount.Name) {
			volumeMounts = append(volumeMounts, corev1.VolumeMount{
				Name:      mount.Name,
				ReadOnly:  mount.ReadOnly,
				MountPath: mount.MountPath,
				SubPath:   mount.SubPath,
			})
		}
	}

	// Transfer image pull secrets from the pod spec.
	imagePullSecrets := []corev1.LocalObjectReference{}
	for _, pullSecret := range rc.Spec.Template.Spec.ImagePullSecrets {
		imagePullSecrets = append(imagePullSecrets, corev1.LocalObjectReference{Name: pullSecret.Name})
	}

	gracePeriod := int64(10)
	podSecurityContext := rc.Spec.Template.Spec.SecurityContext.DeepCopy()
	securityContextCopy := baseContainer.SecurityContext.DeepCopy()

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      apihelpers.GetPodName(rc.Name, hookType),
			Namespace: rc.Namespace,
			Annotations: map[string]string{
				appsapi.DeploymentAnnotation: rc.Name,
			},
			Labels: map[string]string{
				appsapi.DeploymentPodTypeLabel:        hookType,
				appsapi.DeployerPodForDeploymentLabel: rc.Name,
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:            "lifecycle",
					Image:           baseContainer.Image,
					ImagePullPolicy: baseContainer.ImagePullPolicy,
					Command:         exec.Command,
					WorkingDir:      baseContainer.WorkingDir,
					Env:             mergedEnv,
					Resources:       baseContainer.Resources,
					VolumeMounts:    volumeMounts,
					SecurityContext: securityContextCopy,
				},
			},
			SecurityContext:       podSecurityContext,
			Volumes:               volumes,
			ActiveDeadlineSeconds: &maxDeploymentDurationSeconds,
			// Setting the node selector on the hook pod so that it is created
			// on the same set of nodes as the rc pods.
			NodeSelector:                  rc.Spec.Template.Spec.NodeSelector,
			RestartPolicy:                 restartPolicy,
			ImagePullSecrets:              imagePullSecrets,
			TerminationGracePeriodSeconds: &gracePeriod,
		},
	}

	util.MergeInto(pod.Labels, strategy.Labels, 0)
	util.MergeInto(pod.Annotations, strategy.Annotations, 0)

	return pod, nil
}
