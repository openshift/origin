package assets

import (
	assetslib "github.com/openshift/library-go/pkg/assets"
)

const AssetAdminKubeConfig = "auth/admin.kubeconfig"

var AdminKubeConfigTemplate = []byte(`apiVersion: v1
kind: Config
clusters:
- name: local
  cluster:
    server: {{ .ServerURL }}
    certificate-authority-data: {{ .CACert }}
users:
- name: admin
  user:
    client-certificate-data: {{ .AdminCert }}
    client-key-data: {{ .AdminKey }}
contexts:
- context:
    cluster: local
    user: admin
`)

func (r *TLSAssetsRenderOptions) newAdminKubeConfig() assetslib.Asset {
	return assetslib.MustCreateAssetFromTemplate(AssetAdminKubeConfig, AdminKubeConfigTemplate, &r.config)
}
