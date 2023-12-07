package certgraphanalysis

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/openshift/library-go/pkg/certs/cert-inspection/certgraphapi"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/cert"
)

func gatherSecretsFromDisk(ctx context.Context, prefix, dir string, options certGenerationOptionList) ([]*certgraphapi.CertKeyPair, error) {
	ret := []*certgraphapi.CertKeyPair{}
	parentDir := filepath.Join(prefix, dir)
	_, err := os.Stat(parentDir)
	if os.IsNotExist(err) {
		return ret, nil
	}

	fmt.Fprintf(os.Stdout, "Gathering secrets from %s.\n", parentDir)
	err = filepath.WalkDir(parentDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		fmt.Fprintf(os.Stdout, "Checking if %s is a secret.\n", path)
		if detail, err := parseFileAsSecretKey(path, prefix, dir); err == nil && detail != nil {
			fmt.Fprintf(os.Stdout, "Found secret key in %s.\n", path)
			options.rewriteCertKeyPair(metav1.ObjectMeta{}, detail)
			ret = append(ret, detail)
			return nil
		}

		if detail, err := parseFileAsCertificate(path, prefix, dir); err == nil && detail != nil {
			fmt.Fprintf(os.Stdout, "Found certificate in %s.\n", path)
			options.rewriteCertKeyPair(metav1.ObjectMeta{}, detail)
			ret = append(ret, detail)
			return nil
		}

		return nil
	})
	return ret, err
}

func parseFileAsSecretKey(path, prefix, dir string) (*certgraphapi.CertKeyPair, error) {
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

	pathWithoutPrefix := strings.Replace(path, prefix, "", 1)
	return &certgraphapi.CertKeyPair{
		Spec: certgraphapi.CertKeyPairSpec{
			SecretLocations: nil,
			OnDiskLocations: []certgraphapi.OnDiskCertKeyPairLocation{
				{
					Key: certgraphapi.OnDiskLocation{
						Path: pathWithoutPrefix,
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

func parseFileAsCA(path, prefix, dir string) (*certgraphapi.CertificateAuthorityBundle, error) {
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

	pathWithoutPrefix := strings.Replace(path, prefix, "", 1)
	detail.Spec.OnDiskLocations = []certgraphapi.OnDiskLocation{
		{
			Path: pathWithoutPrefix,
			// TODO[vrutkovs]: fill in other settings
		}}
	return detail, nil
}

func parseFileAsCertificate(path, prefix, dir string) (*certgraphapi.CertKeyPair, error) {
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

	pathWithoutPrefix := strings.Replace(path, prefix, "", 1)
	detail.Spec.OnDiskLocations = []certgraphapi.OnDiskCertKeyPairLocation{
		{
			Cert: certgraphapi.OnDiskLocation{
				Path: pathWithoutPrefix,
				// TODO[vrutkovs]: fill in other settings
			},
		}}
	return detail, nil
}

func gatherCABundlesFromDisk(ctx context.Context, prefix, dir string, options certGenerationOptionList) ([]*certgraphapi.CertificateAuthorityBundle, error) {
	ret := []*certgraphapi.CertificateAuthorityBundle{}
	parentDir := filepath.Join(prefix, dir)
	_, err := os.Stat(parentDir)
	if os.IsNotExist(err) {
		return ret, nil
	}

	fmt.Fprintf(os.Stdout, "Gathering CA bundles from %s.\n", parentDir)
	err = filepath.WalkDir(parentDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		fmt.Fprintf(os.Stdout, "Checking if %s is a CA bundle.\n", path)
		if detail, err := parseFileAsCA(path, prefix, dir); err == nil && detail != nil {
			options.rewriteCABundle(metav1.ObjectMeta{}, detail)
			fmt.Fprintf(os.Stdout, "Found CA bundle in %s.\n", path)
			ret = append(ret, detail)
			return nil
		}
		return nil
	})
	return ret, err
}
