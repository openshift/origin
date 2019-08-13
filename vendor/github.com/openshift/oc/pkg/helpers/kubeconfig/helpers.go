package kubeconfig

import (
	"path/filepath"

	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// TODO should be moved upstream
func RelativizeClientConfigPaths(cfg *clientcmdapi.Config, base string) (err error) {
	for k, cluster := range cfg.Clusters {
		if len(cluster.CertificateAuthority) > 0 {
			if cluster.CertificateAuthority, err = clientcmdapi.MakeAbs(cluster.CertificateAuthority, ""); err != nil {
				return err
			}
			if cluster.CertificateAuthority, err = MakeRelative(cluster.CertificateAuthority, base); err != nil {
				return err
			}
			cfg.Clusters[k] = cluster
		}
	}
	for k, authInfo := range cfg.AuthInfos {
		if len(authInfo.ClientCertificate) > 0 {
			if authInfo.ClientCertificate, err = clientcmdapi.MakeAbs(authInfo.ClientCertificate, ""); err != nil {
				return err
			}
			if authInfo.ClientCertificate, err = MakeRelative(authInfo.ClientCertificate, base); err != nil {
				return err
			}
		}
		if len(authInfo.ClientKey) > 0 {
			if authInfo.ClientKey, err = clientcmdapi.MakeAbs(authInfo.ClientKey, ""); err != nil {
				return err
			}
			if authInfo.ClientKey, err = MakeRelative(authInfo.ClientKey, base); err != nil {
				return err
			}
		}
		cfg.AuthInfos[k] = authInfo
	}
	return nil
}

// TODO should use library-go's version or even better upstream above
func MakeRelative(path, base string) (string, error) {
	if len(path) > 0 {
		rel, err := filepath.Rel(base, path)
		if err != nil {
			return path, err
		}
		return rel, nil
	}
	return path, nil
}
