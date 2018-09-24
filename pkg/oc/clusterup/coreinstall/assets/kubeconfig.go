package assets

import (
	assetslib "github.com/openshift/library-go/pkg/assets"
)

const (
	AssetKubeConfig      = "auth/kubeconfig"
	AssetAdminKubeConfig = "auth/admin.kubeconfig"
)

var KubeConfigTemplate = []byte(`apiVersion: v1
kind: Config
clusters:
- name: local
  cluster:
    server: {{ .ServerURL }}
    certificate-authority-data: {{ .CACert | base64 }}
users:
- name: admin
  user:
    client-certificate-data: {{ .AdminCert | base64 }}
    client-key-data: {{ .AdminKey | base64 }}
contexts:
- context:
    cluster: local
    user: admin
`)

func (r *TLSAssetsRenderOptions) newAdminKubeConfig() []assetslib.Asset {
	return []assetslib.Asset{
		assetslib.MustCreateAssetFromTemplate(AssetKubeConfig, KubeConfigTemplate, &r.config),
		assetslib.MustCreateAssetFromTemplate(AssetAdminKubeConfig, KubeConfigTemplate, &r.config),
	}
}
