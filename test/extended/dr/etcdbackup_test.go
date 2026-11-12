package dr

import (
	"testing"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

func TestSupportsEtcdBackupRecoverySchema(t *testing.T) {
	tests := []struct {
		name         string
		specFields   []string
		statusFields []string
		wantSupport  bool
		wantErr      bool
	}{
		{
			name:         "legacy PVC API",
			specFields:   []string{"pvcName"},
			statusFields: []string{"conditions", "backupJob"},
		},
		{
			name:         "local recovery API",
			specFields:   []string{"nodeSelector", "storage"},
			statusFields: []string{"conditions", "nodeName", "files"},
			wantSupport:  true,
		},
		{
			name:         "incomplete recovery API",
			specFields:   []string{"nodeSelector", "storage"},
			statusFields: []string{"conditions", "nodeName"},
			wantErr:      true,
		},
		{
			name:         "unknown API",
			specFields:   []string{"other"},
			statusFields: []string{"conditions"},
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			properties := map[string]apiextensionsv1.JSONSchemaProps{
				"spec":   {Properties: make(map[string]apiextensionsv1.JSONSchemaProps)},
				"status": {Properties: make(map[string]apiextensionsv1.JSONSchemaProps)},
			}
			spec := properties["spec"]
			for _, field := range tt.specFields {
				spec.Properties[field] = apiextensionsv1.JSONSchemaProps{}
			}
			properties["spec"] = spec
			status := properties["status"]
			for _, field := range tt.statusFields {
				status.Properties[field] = apiextensionsv1.JSONSchemaProps{}
			}
			properties["status"] = status

			crd := &apiextensionsv1.CustomResourceDefinition{
				Spec: apiextensionsv1.CustomResourceDefinitionSpec{
					Versions: []apiextensionsv1.CustomResourceDefinitionVersion{{
						Name:   "v1alpha1",
						Served: true,
						Schema: &apiextensionsv1.CustomResourceValidation{
							OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{Properties: properties},
						},
					}},
				},
			}
			supported, err := supportsEtcdBackupRecoverySchema(crd)
			if (err != nil) != tt.wantErr {
				t.Fatalf("supportsEtcdBackupRecoverySchema() error = %v, want error %t", err, tt.wantErr)
			}
			if supported != tt.wantSupport {
				t.Errorf("supportsEtcdBackupRecoverySchema() supported = %t, want %t", supported, tt.wantSupport)
			}
		})
	}
}
