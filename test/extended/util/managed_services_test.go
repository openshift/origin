package util

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestIsAroHCP(t *testing.T) {
	tests := []struct {
		name        string
		namespace   string
		deployments []appsv1.Deployment
		expected    bool
		expectError bool
	}{
		{
			name:        "no deployments found",
			namespace:   "test-namespace",
			deployments: []appsv1.Deployment{},
			expected:    false,
			expectError: false,
		},
		{
			name:      "deployment without control-plane-operator label",
			namespace: "test-namespace",
			deployments: []appsv1.Deployment{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "other-deployment",
						Namespace: "test-namespace",
						Labels: map[string]string{
							"app": "other-app",
						},
					},
					Spec: appsv1.DeploymentSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{
										Name: "other-container",
										Env: []corev1.EnvVar{
											{Name: "MANAGED_SERVICE", Value: "ARO-HCP"},
										},
									},
								},
							},
						},
					},
				},
			},
			expected:    false,
			expectError: false,
		},
		{
			name:      "deployment with correct label but no control-plane-operator container",
			namespace: "test-namespace",
			deployments: []appsv1.Deployment{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "hypershift-deployment",
						Namespace: "test-namespace",
						Labels: map[string]string{
							"hypershift.openshift.io/managed-by": "control-plane-operator",
						},
					},
					Spec: appsv1.DeploymentSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{
										Name: "other-container",
										Env: []corev1.EnvVar{
											{Name: "MANAGED_SERVICE", Value: "ARO-HCP"},
										},
									},
								},
							},
						},
					},
				},
			},
			expected:    false,
			expectError: false,
		},
		{
			name:      "deployment with control-plane-operator container but no ARO-HCP env var",
			namespace: "test-namespace",
			deployments: []appsv1.Deployment{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "hypershift-deployment",
						Namespace: "test-namespace",
						Labels: map[string]string{
							"hypershift.openshift.io/managed-by": "control-plane-operator",
						},
					},
					Spec: appsv1.DeploymentSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{
										Name: "control-plane-operator",
										Env: []corev1.EnvVar{
											{Name: "OTHER_VAR", Value: "other-value"},
										},
									},
								},
							},
						},
					},
				},
			},
			expected:    false,
			expectError: false,
		},
		{
			name:      "deployment with control-plane-operator container and wrong MANAGED_SERVICE value",
			namespace: "test-namespace",
			deployments: []appsv1.Deployment{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "hypershift-deployment",
						Namespace: "test-namespace",
						Labels: map[string]string{
							"hypershift.openshift.io/managed-by": "control-plane-operator",
						},
					},
					Spec: appsv1.DeploymentSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{
										Name: "control-plane-operator",
										Env: []corev1.EnvVar{
											{Name: "MANAGED_SERVICE", Value: "ROSA"},
										},
									},
								},
							},
						},
					},
				},
			},
			expected:    false,
			expectError: false,
		},
		{
			name:      "deployment with control-plane-operator container and correct ARO-HCP env var",
			namespace: "test-namespace",
			deployments: []appsv1.Deployment{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "hypershift-deployment",
						Namespace: "test-namespace",
						Labels: map[string]string{
							"hypershift.openshift.io/managed-by": "control-plane-operator",
						},
					},
					Spec: appsv1.DeploymentSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{
										Name: "control-plane-operator",
										Env: []corev1.EnvVar{
											{Name: "MANAGED_SERVICE", Value: "ARO-HCP"},
										},
									},
								},
							},
						},
					},
				},
			},
			expected:    true,
			expectError: false,
		},
		{
			name:      "multiple deployments, one with ARO-HCP",
			namespace: "test-namespace",
			deployments: []appsv1.Deployment{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "other-deployment",
						Namespace: "test-namespace",
						Labels: map[string]string{
							"hypershift.openshift.io/managed-by": "control-plane-operator",
						},
					},
					Spec: appsv1.DeploymentSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{
										Name: "control-plane-operator",
										Env: []corev1.EnvVar{
											{Name: "MANAGED_SERVICE", Value: "ROSA"},
										},
									},
								},
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "aro-hcp-deployment",
						Namespace: "test-namespace",
						Labels: map[string]string{
							"hypershift.openshift.io/managed-by": "control-plane-operator",
						},
					},
					Spec: appsv1.DeploymentSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{
										Name: "control-plane-operator",
										Env: []corev1.EnvVar{
											{Name: "MANAGED_SERVICE", Value: "ARO-HCP"},
										},
									},
								},
							},
						},
					},
				},
			},
			expected:    true,
			expectError: false,
		},
		{
			name:      "deployment with multiple containers, control-plane-operator has ARO-HCP",
			namespace: "test-namespace",
			deployments: []appsv1.Deployment{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "hypershift-deployment",
						Namespace: "test-namespace",
						Labels: map[string]string{
							"hypershift.openshift.io/managed-by": "control-plane-operator",
						},
					},
					Spec: appsv1.DeploymentSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{
										Name: "sidecar-container",
										Env: []corev1.EnvVar{
											{Name: "MANAGED_SERVICE", Value: "ROSA"},
										},
									},
									{
										Name: "control-plane-operator",
										Env: []corev1.EnvVar{
											{Name: "MANAGED_SERVICE", Value: "ARO-HCP"},
											{Name: "OTHER_VAR", Value: "other-value"},
										},
									},
								},
							},
						},
					},
				},
			},
			expected:    true,
			expectError: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Create fake client
			fakeClient := fake.NewClientset()

			// Add deployments to fake client
			for _, deployment := range test.deployments {
				_, err := fakeClient.AppsV1().Deployments(test.namespace).Create(context.TODO(), &deployment, metav1.CreateOptions{})
				if err != nil {
					t.Fatalf("Failed to create deployment: %v", err)
				}
			}

			// Call the function
			result, err := IsAroHCP(context.TODO(), test.namespace, fakeClient)

			// Check error expectations
			if test.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !test.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}

			// Check result
			if result != test.expected {
				t.Errorf("IsAroHCP() = %v, expected %v", result, test.expected)
			}
		})
	}
}
