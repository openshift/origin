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
    server: {{ .serverURL }}
    certificate-authority-data: {{ .caCert }}
users:
- name: admin
  user:
    client-certificate-data: {{ .adminCert }}
    client-key-data: {{ .adminKey }}
contexts:
- context:
    cluster: local
    user: admin
`)

func (r *TLSAssetsRenderOptions) newAdminKubeConfig() assetslib.Asset {
	assetslib.MustCreateAssetFromTemplate(AssetAdminKubeConfig, AdminKubeConfigTemplate, r)
}
