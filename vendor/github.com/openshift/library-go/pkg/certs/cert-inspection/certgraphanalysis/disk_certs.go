package certgraphanalysis

import (
	"context"
	"crypto/ecdsa"
	"crypto/rsa"
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

		fmt.Fprintf(os.Stdout, "Checking if %s is a certificate or secret key.\n", path)
		if details, err := parseFileAsTLSArtifact(path, prefix, dir); err == nil && details != nil {
			for i, detail := range details {
				fmt.Fprintf(os.Stdout, "Found certkeypair #%d in %s.\n", i+1, path)
				options.rewriteCertKeyPair(metav1.ObjectMeta{}, detail)
				ret = append(ret, detail)
			}
			return nil
		}
		return nil
	})
	return ret, err
}

func parseBlockAsTLSArtifact(path, prefix string, bytes []byte) (*certgraphapi.CertKeyPair, []byte, error) {
	block, rest := pem.Decode(bytes)
	if block == nil {
		return nil, rest, fmt.Errorf("empty block")
	}
	switch block.Type {
	case "CERTIFICATE":
		certificates, err := cert.ParseCertsPEM(bytes)
		if err != nil {
			return nil, rest, err
		}
		detail, err := parseBlockAsCertificate(certificates, path, prefix)
		if err != nil {
			return nil, rest, err
		}
		fmt.Fprintf(os.Stdout, "Found valid certificate in %s \n", path)
		return detail, rest, nil
	case "RSA PRIVATE KEY":
		key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, rest, err
		}
		fmt.Fprintf(os.Stdout, "Found RSA private key in %s \n", path)
		return parseBlockAsRSAPrivateKey(key, path, prefix), rest, nil
	case "EC PRIVATE KEY":
		key, err := x509.ParseECPrivateKey(block.Bytes)
		if err != nil {
			return nil, rest, err
		}
		fmt.Fprintf(os.Stdout, "Found ECDSA private key in %s \n", path)
		return parseBlockAsECDSAPrivateKey(key, path, prefix), rest, nil
	}
	return nil, rest, fmt.Errorf("unexpected block type: %s", block.Type)
}

func parseBlockAsCertificate(certificates []*x509.Certificate, path, prefix string) (*certgraphapi.CertKeyPair, error) {
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
		return nil, err
	}
	detail.Spec.OnDiskLocations = []certgraphapi.OnDiskCertKeyPairLocation{
		{
			Cert: buildOnDiskLocationFromPath(path, prefix),
		}}
	return detail, nil
}

func parseBlockAsRSAPrivateKey(key *rsa.PrivateKey, path, prefix string) *certgraphapi.CertKeyPair {
	return &certgraphapi.CertKeyPair{
		Spec: certgraphapi.CertKeyPairSpec{
			SecretLocations: nil,
			OnDiskLocations: []certgraphapi.OnDiskCertKeyPairLocation{
				{
					Key: buildOnDiskLocationFromPath(path, prefix),
				}},
			CertMetadata: certgraphapi.CertKeyMetadata{
				CertIdentifier: certgraphapi.CertIdentifier{
					PubkeyModulus: key.N.String(),
				},
			},
		},
	}
}

func parseBlockAsECDSAPrivateKey(key *ecdsa.PrivateKey, path, prefix string) *certgraphapi.CertKeyPair {
	return &certgraphapi.CertKeyPair{
		Spec: certgraphapi.CertKeyPairSpec{
			SecretLocations: nil,
			OnDiskLocations: []certgraphapi.OnDiskCertKeyPairLocation{
				{
					Key: buildOnDiskLocationFromPath(path, prefix),
				}},
			CertMetadata: certgraphapi.CertKeyMetadata{
				CertIdentifier: certgraphapi.CertIdentifier{
					PubkeyModulus: fmt.Sprintf("%s %s", key.X.String(), key.Y.String()),
				},
			},
		},
	}
}

func parseFileAsTLSArtifact(path, prefix, dir string) ([]*certgraphapi.CertKeyPair, error) {
	var details []*certgraphapi.CertKeyPair

	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	// Parse all blocks
	for len(bytes) > 0 {
		fmt.Fprintf(os.Stdout, "Parsing block with length %d \n", len(bytes))
		detail, remainder, err := parseBlockAsTLSArtifact(path, prefix, bytes)
		if err != nil {
			fmt.Fprintf(os.Stdout, "Failed to parse current block: %v \n", err)
		} else {
			details = append(details, detail)
		}
		if len(remainder) == len(bytes) || len(remainder) == 0 {
			fmt.Fprintf(os.Stdout, "no blocks to parse left\n")
			break
		}
		bytes = remainder
		fmt.Fprintf(os.Stdout, "Remainder: %d \n", len(remainder))
	}
	return details, nil
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

	detail.Spec.OnDiskLocations = []certgraphapi.OnDiskLocation{buildOnDiskLocationFromPath(path, prefix)}
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

func buildOnDiskLocationFromPath(path, prefix string) certgraphapi.OnDiskLocation {
	pathWithoutPrefix := strings.Replace(path, prefix, "", 1)
	return certgraphapi.OnDiskLocation{
		Path: pathWithoutPrefix,
		// TODO[vrutkovs]: fill in other settings
	}
}
