package image

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	configv1 "github.com/openshift/api/config/v1"
)

func TestValidateImage(t *testing.T) {
	testCases := []struct {
		name        string
		imageConfig *configv1.Image
		expectError bool
	}{
		{
			name: "valid",
			imageConfig: &configv1.Image{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster",
				},
				Spec: configv1.ImageSpec{
					AdditionalTrustedCA: configv1.ConfigMapNameReference{
						Name: "test-cm",
					},
					AllowedRegistriesForImport: []configv1.RegistryLocation{
						{
							DomainName: "quay.io",
						},
						{
							DomainName: "internal-registry.corp.url:5000",
							Insecure:   true,
						},
					},
					ExternalRegistryHostnames: []string{
						"registry.openshift.com",
						"registry.redhat.io",
					},
					RegistrySources: configv1.RegistrySources{
						InsecureRegistries: []string{
							"internal-registry.corp.url:5000",
						},
					},
				},
			},
		},
		{
			name: "blocked and allowed registry sources",
			imageConfig: &configv1.Image{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster",
				},
				Spec: configv1.ImageSpec{
					RegistrySources: configv1.RegistrySources{
						AllowedRegistries: []string{
							"quay.io",
						},
						BlockedRegistries: []string{
							"docker.io",
						},
					},
				},
			},
			expectError: true,
		},
		{
			name: "blocked registry sources only",
			imageConfig: &configv1.Image{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster",
				},
				Spec: configv1.ImageSpec{
					RegistrySources: configv1.RegistrySources{
						BlockedRegistries: []string{
							"docker.io",
							"badregistry.net:5000",
						},
					},
				},
			},
		},
		{
			name: "allowed registry sources only",
			imageConfig: &configv1.Image{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster",
				},
				Spec: configv1.ImageSpec{
					RegistrySources: configv1.RegistrySources{
						AllowedRegistries: []string{
							"docker.io",
							"quay.io",
							"registry.redhat.io",
						},
					},
				},
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			validator := imageV1{}
			errs := validator.ValidateCreate(tc.imageConfig)
			if tc.expectError {
				if len(errs) == 0 {
					t.Error("expected errors on ValidateCreate, got none")
				}
			}
			if !tc.expectError && len(errs) > 0 {
				t.Errorf("received unexpected errors on ValidateCreate: %v", errs)
			}
			tc.imageConfig.ResourceVersion = "1"
			update := tc.imageConfig.DeepCopy()
			update.ResourceVersion = "2"
			errs = validator.ValidateUpdate(update, tc.imageConfig)
			if tc.expectError {
				if len(errs) == 0 {
					t.Error("expected errors on ValidateUpdate, got none")
				}
			}
			if !tc.expectError && len(errs) > 0 {
				t.Errorf("received unexpected errors on ValidateUpdate: %v", errs)
			}
			// Status updates should not error out
			errs = validator.ValidateStatusUpdate(update, tc.imageConfig)
			if len(errs) > 0 {
				t.Errorf("received unexpected errors on ValidateStatusUpdate: %v", errs)
			}
		})
	}
}
