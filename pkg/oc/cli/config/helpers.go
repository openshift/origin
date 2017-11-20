package config

import (
	"github.com/openshift/origin/pkg/cmd/util"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// TODO should be moved upstream
func RelativizeClientConfigPaths(cfg *clientcmdapi.Config, base string) (err error) {
	for k, cluster := range cfg.Clusters {
		if len(cluster.CertificateAuthority) > 0 {
			if cluster.CertificateAuthority, err = util.MakeAbs(cluster.CertificateAuthority, ""); err != nil {
				return err
			}
			if cluster.CertificateAuthority, err = util.MakeRelative(cluster.CertificateAuthority, base); err != nil {
				return err
			}
			cfg.Clusters[k] = cluster
		}
	}
	for k, authInfo := range cfg.AuthInfos {
		if len(authInfo.ClientCertificate) > 0 {
			if authInfo.ClientCertificate, err = util.MakeAbs(authInfo.ClientCertificate, ""); err != nil {
				return err
			}
			if authInfo.ClientCertificate, err = util.MakeRelative(authInfo.ClientCertificate, base); err != nil {
				return err
			}
		}
		if len(authInfo.ClientKey) > 0 {
			if authInfo.ClientKey, err = util.MakeAbs(authInfo.ClientKey, ""); err != nil {
				return err
			}
			if authInfo.ClientKey, err = util.MakeRelative(authInfo.ClientKey, base); err != nil {
				return err
			}
		}
		cfg.AuthInfos[k] = authInfo
	}
	return nil
}
