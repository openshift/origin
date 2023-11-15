package certgraphanalysis

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/openshift/library-go/pkg/certs/cert-inspection/certgraphapi"
	"k8s.io/client-go/util/cert"
)

func gatherSecretsFromDisk(ctx context.Context, dir string, options ...certGenerationOptions) ([]*certgraphapi.CertKeyPair, error) {
	ret := []*certgraphapi.CertKeyPair{}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		if detail, err := parseFileAsSecretKey(path, dir); err == nil && detail != nil {
			ret = append(ret, detail)
			return nil
		}

		if detail, err := parseFileAsCertificate(path, dir); err == nil && detail != nil {
			ret = append(ret, detail)
			return nil
		}

		return nil
	})
	return ret, err
}

func parseFileAsSecretKey(path, dir string) (*certgraphapi.CertKeyPair, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(bytes)
	if block == nil || block.Type != "RSA PRIVATE KEY" {
		return nil, fmt.Errorf("PEM block type must be RSA PRIVATE KEY")
	}
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	return &certgraphapi.CertKeyPair{
		Spec: certgraphapi.CertKeyPairSpec{
			SecretLocations: nil,
			OnDiskLocations: []certgraphapi.OnDiskCertKeyPairLocation{
				{
					Key: certgraphapi.OnDiskLocation{
						Path: path,
						// TODO[vrutkovs]: fill in other settings
					},
				}},
			CertMetadata: certgraphapi.CertKeyMetadata{
				CertIdentifier: certgraphapi.CertIdentifier{
					PubkeyModulus: key.N.String(),
				},
			},
		},
	}, nil
}

func parseFileAsCA(path, dir string) (*certgraphapi.CertificateAuthorityBundle, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	certificates, err := cert.ParseCertsPEM(bytes)
	if err != nil {
		return nil, nil
	}
	// Check that the first certificat is a CA
	if len(certificates) == 0 {
		return nil, fmt.Errorf("no certificates found")
	}
	if !certificates[0].IsCA {
		// Not a CA
		return nil, fmt.Errorf("not a CA")
	}
	detail, err := toCABundle(certificates)
	if err != nil {
		return detail, err
	}

	detail.Spec.OnDiskLocations = []certgraphapi.OnDiskLocation{
		{
			Path: path,
			// TODO[vrutkovs]: fill in other settings
		}}
	return detail, nil
}

func parseFileAsCertificate(path, dir string) (*certgraphapi.CertKeyPair, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	certificates, err := cert.ParseCertsPEM(bytes)
	if err != nil {
		return nil, nil
	}
	// Only parse first cert (last in the chain), as the rest are CA(s)
	if len(certificates) == 0 {
		return nil, fmt.Errorf("no certificates found")
	}
	certificate := certificates[0]
	if certificate.IsCA {
		// This is a CA
		return nil, fmt.Errorf("not certificate but a CA")
	}
	detail, err := toCertKeyPair(certificate)
	if err != nil {
		return detail, nil
	}
	detail.Spec.OnDiskLocations = []certgraphapi.OnDiskCertKeyPairLocation{
		{
			Cert: certgraphapi.OnDiskLocation{
				Path: path,
				// TODO[vrutkovs]: fill in other settings
			},
		}}
	return detail, nil
}

func gatherCABundlesFromDisk(ctx context.Context, dir string, options ...certGenerationOptions) ([]*certgraphapi.CertificateAuthorityBundle, error) {
	ret := []*certgraphapi.CertificateAuthorityBundle{}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if detail, err := parseFileAsCA(path, dir); err == nil && detail != nil {
			ret = append(ret, detail)
			return nil
		}
		return nil
	})
	return ret, err
}
