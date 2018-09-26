package assets

import (
	assetslib "github.com/openshift/library-go/pkg/assets"
)

const (
	AssetAdminKubeConfig = "auth/admin.kubeconfig"
	AssetKubeConfig      = "auth/kubeconfig"
	AssetTLSKubeConfig   = "tls/kubeconfig"
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
		assetslib.MustCreateAssetFromTemplate(AssetAdminKubeConfig, KubeConfigTemplate, &r.config),
		assetslib.MustCreateAssetFromTemplate(AssetKubeConfig, KubeConfigTemplate, &r.config),
		assetslib.MustCreateAssetFromTemplate(AssetTLSKubeConfig, KubeConfigTemplate, &r.config),
	}
}
