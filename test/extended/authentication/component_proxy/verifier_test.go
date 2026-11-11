package component_proxy

import (
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

func containerWithMount(volumeName string) corev1.Container {
	return corev1.Container{
		VolumeMounts: []corev1.VolumeMount{{Name: volumeName}},
	}
}

func deploymentWithProxyCAVolume(mount bool) *appsv1.Deployment {
	const volumeName = "proxy-ca"
	containers := []corev1.Container{{}}
	if mount {
		containers = append(containers, containerWithMount(volumeName))
	}
	return &appsv1.Deployment{
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{{
						Name: volumeName,
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{Name: ComponentProxyCAConfigMapName},
							},
						},
					}},
					Containers: containers,
				},
			},
		},
	}
}

func deploymentWithUnrelatedVolume() *appsv1.Deployment {
	return &appsv1.Deployment{
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{{
						Name: "other",
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{Name: "some-other-configmap"},
							},
						},
					}},
				},
			},
		},
	}
}

func TestTrustedCAVolumeState(t *testing.T) {
	tests := []struct {
		name        string
		deployment  *appsv1.Deployment
		wantVolume  bool
		wantMount   bool
		description string
	}{
		{
			name:        "volume and mount present",
			deployment:  deploymentWithProxyCAVolume(true),
			wantVolume:  true,
			wantMount:   true,
			description: "both volume and mount found",
		},
		{
			// A mount without the volume would be a dangling reference; the check must
			// report neither so a mount alone never satisfies expectTrustedCA.
			name:        "volume without mount reports no mount",
			deployment:  deploymentWithProxyCAVolume(false),
			wantVolume:  true,
			wantMount:   false,
			description: "volume found but no container mounts it",
		},
		{
			name:        "no proxy CA volume",
			deployment:  deploymentWithUnrelatedVolume(),
			wantVolume:  false,
			wantMount:   false,
			description: "unrelated ConfigMap volume is ignored",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotVolume, gotMount := trustedCAVolumeState(tt.deployment)
			if gotVolume != tt.wantVolume || gotMount != tt.wantMount {
				t.Fatalf("%s: trustedCAVolumeState = (volume=%v, mount=%v), want (volume=%v, mount=%v)",
					tt.description, gotVolume, gotMount, tt.wantVolume, tt.wantMount)
			}
		})
	}
}

// TestTrustedCAAssertion exercises the presence/absence decision over the fixtures,
// covering the regression where a volume without a mount wrongly satisfied an absence
// check and let AssertNoConfig pass with the proxy CA volume still configured.
func TestTrustedCAAssertion(t *testing.T) {
	tests := []struct {
		name           string
		deployment     *appsv1.Deployment
		expectPresence bool
		want           bool
	}{
		{
			name:           "presence expected, volume and mount present",
			deployment:     deploymentWithProxyCAVolume(true),
			expectPresence: true,
			want:           true,
		},
		{
			name:           "presence expected, volume without mount",
			deployment:     deploymentWithProxyCAVolume(false),
			expectPresence: true,
			want:           false,
		},
		{
			// AssertNoConfig must fail while the proxy CA volume is still configured,
			// even when no container mounts it.
			name:           "absence expected, volume without mount",
			deployment:     deploymentWithProxyCAVolume(false),
			expectPresence: false,
			want:           false,
		},
		{
			name:           "absence expected, no proxy CA volume",
			deployment:     deploymentWithUnrelatedVolume(),
			expectPresence: false,
			want:           true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			foundVolume, foundMount := trustedCAVolumeState(tt.deployment)
			if got := trustedCAStateMatches(tt.expectPresence, foundVolume, foundMount); got != tt.want {
				t.Fatalf("trustedCAStateMatches(expectPresence=%v) for %q = %v, want %v",
					tt.expectPresence, tt.name, got, tt.want)
			}
		})
	}
}
