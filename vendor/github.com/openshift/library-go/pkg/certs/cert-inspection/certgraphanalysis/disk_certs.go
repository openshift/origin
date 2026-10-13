package certgraphanalysis

import (
	"context"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/openshift/library-go/pkg/certs/cert-inspection/certgraphapi"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/cert"
)

var errIsCA = fmt.Errorf("not certificate but a CA")

func gatherSecretsFromDisk(ctx context.Context, dir string, options certGenerationOptionList) ([]*certgraphapi.CertKeyPair, []*certgraphapi.OnDiskLocationWithMetadata, error) {
	ret := []*certgraphapi.CertKeyPair{}
	metadataList := []*certgraphapi.OnDiskLocationWithMetadata{}

	_, err := os.Stat(dir)
	if os.IsNotExist(err) {
		return ret, metadataList, nil
	}

	fmt.Fprintf(os.Stdout, "Gathering secrets from %s.\n", dir)
	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		fmt.Fprintf(os.Stdout, "Checking if %s is a certificate or secret key.\n", path)
		if details, err := parseFileAsCertKeyPair(path); err == nil && details != nil {
			for i, detail := range details {
				fmt.Fprintf(os.Stdout, "Found certkeypair #%d in %s.\n", i+1, path)
				options.rewriteCertKeyPair(metav1.ObjectMeta{}, detail)

				needsMetadataCollected := false
				for i, loc := range detail.Spec.OnDiskLocations {
					if loc.Cert.Path == path {
						needsMetadataCollected = true
					}
					detail.Spec.OnDiskLocations[i].Cert.Path = options.rewritePath(loc.Cert.Path)
					fmt.Fprintf(os.Stdout, "Rewrite cert result: %s\n", detail.Spec.OnDiskLocations[i].Cert.Path)

					if loc.Key.Path == path {
						needsMetadataCollected = true
					}

					detail.Spec.OnDiskLocations[i].Key.Path = options.rewritePath(loc.Key.Path)
					fmt.Fprintf(os.Stdout, "Rewrite key result: %s\n", detail.Spec.OnDiskLocations[i].Key.Path)
				}
				ret = append(ret, detail)

				if needsMetadataCollected {
					metadata := getOnDiskLocationMetadata(path)
					metadata.Path = options.rewritePath(metadata.Path)
					metadataList = append(metadataList, metadata)

				}
			}
			return nil
		}
		return nil
	})
	return ret, metadataList, err
}

func parseBlockAsTLSArtifact(path string, bytes []byte) (*certgraphapi.CertKeyPair, []byte, error) {
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
		detail, err := parseBlockAsCertificate(certificates, path)
		if errors.Is(err, errIsCA) {
			// Stop processing the certificate - block found to be a CA and it can't be mixed
			return nil, []byte{}, err
		} else if err != nil {
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
		return parseBlockAsRSAPrivateKey(key, path), rest, nil
	case "EC PRIVATE KEY":
		key, err := x509.ParseECPrivateKey(block.Bytes)
		if err != nil {
			return nil, rest, err
		}
		fmt.Fprintf(os.Stdout, "Found ECDSA private key in %s \n", path)
		return parseBlockAsECDSAPrivateKey(key, path), rest, nil
	}
	return nil, rest, fmt.Errorf("unexpected block type: %s", block.Type)
}

func parseBlockAsCertificate(certificates []*x509.Certificate, path string) (*certgraphapi.CertKeyPair, error) {
	// Only parse first cert (last in the chain), as the rest are CA(s)
	if len(certificates) == 0 {
		return nil, fmt.Errorf("no certificates found")
	}
	certificate := certificates[0]
	if certificate.IsCA {
		// This is a CA
		return nil, errIsCA
	}

	detail, err := toCertKeyPair(certificate)
	if err != nil {
		return nil, err
	}
	detail.Spec.OnDiskLocations = []certgraphapi.OnDiskCertKeyPairLocation{
		{
			Cert: certgraphapi.OnDiskLocation{
				Path: path,
			},
		}}
	return detail, nil
}

func parseBlockAsRSAPrivateKey(key *rsa.PrivateKey, path string) *certgraphapi.CertKeyPair {
	return &certgraphapi.CertKeyPair{
		Spec: certgraphapi.CertKeyPairSpec{
			SecretLocations: nil,
			OnDiskLocations: []certgraphapi.OnDiskCertKeyPairLocation{
				{
					Key: certgraphapi.OnDiskLocation{
						Path: path,
					},
				}},
			CertMetadata: certgraphapi.CertKeyMetadata{
				CertIdentifier: certgraphapi.CertIdentifier{
					PubkeyModulus: key.N.String(),
				},
			},
		},
	}
}

func parseBlockAsECDSAPrivateKey(key *ecdsa.PrivateKey, path string) *certgraphapi.CertKeyPair {
	return &certgraphapi.CertKeyPair{
		Spec: certgraphapi.CertKeyPairSpec{
			SecretLocations: nil,
			OnDiskLocations: []certgraphapi.OnDiskCertKeyPairLocation{
				{
					Key: certgraphapi.OnDiskLocation{
						Path: path,
					},
				}},
			CertMetadata: certgraphapi.CertKeyMetadata{
				CertIdentifier: certgraphapi.CertIdentifier{
					PubkeyModulus: fmt.Sprintf("%s %s", key.X.String(), key.Y.String()),
				},
			},
		},
	}
}

func parseFileAsCertKeyPair(path string) ([]*certgraphapi.CertKeyPair, error) {
	var details []*certgraphapi.CertKeyPair

	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	// Parse as kubeconfig
	if kubeConfig, err := parseFileAsKubeConfig(path); err == nil {
		details := []*certgraphapi.CertKeyPair{}
		if rawConfig, err := kubeConfig.RawConfig(); err == nil {
			for _, authInfo := range rawConfig.AuthInfos {
				if certKeyPairs, err := GetCertKeyPairsFromKubeConfig(authInfo, nil); err == nil {
					for i := range certKeyPairs {
						certKeyPairs[i].Spec.OnDiskLocations = []certgraphapi.OnDiskCertKeyPairLocation{
							{
								Cert: certgraphapi.OnDiskLocation{
									Path: path,
								},
								Key: certgraphapi.OnDiskLocation{
									Path: path,
								},
							}}
					}
					details = append(details, certKeyPairs...)
				}
			}
			return details, nil
		}
	}
	// Parse all blocks
	for len(bytes) > 0 {
		fmt.Fprintf(os.Stdout, "Parsing block with length %d \n", len(bytes))
		detail, remainder, err := parseBlockAsTLSArtifact(path, bytes)
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

func parseFileAsCA(path string) (*certgraphapi.CertificateAuthorityBundle, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	// Parse as kubeconfig
	if kubeConfig, err := parseFileAsKubeConfig(path); err == nil {
		if clientConfig, err := kubeConfig.ClientConfig(); err == nil {
			fmt.Fprintf(os.Stdout, "Found a valid kubeconfig\n")
			if detail, err := GetCAFromKubeConfig(clientConfig, "", ""); err == nil {
				detail.Spec.OnDiskLocations = []certgraphapi.OnDiskLocation{
					{
						Path: path,
					},
				}
				return detail, nil
			}
		}

	}
	certificates, err := cert.ParseCertsPEM(bytes)
	if err != nil {
		return nil, nil
	}
	// Check that the first certificate is a CA
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

	detail.Spec.OnDiskLocations = []certgraphapi.OnDiskLocation{{
		Path: path,
	}}
	return detail, nil
}

func parseFileAsKubeConfig(path string) (clientcmd.OverridingClientConfig, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return clientcmd.NewClientConfigFromBytes(bytes)
}

func gatherCABundlesFromDisk(ctx context.Context, dir string, options certGenerationOptionList) ([]*certgraphapi.CertificateAuthorityBundle, []*certgraphapi.OnDiskLocationWithMetadata, error) {
	ret := []*certgraphapi.CertificateAuthorityBundle{}
	metadataList := []*certgraphapi.OnDiskLocationWithMetadata{}

	_, err := os.Stat(dir)
	if os.IsNotExist(err) {
		return ret, metadataList, nil
	}

	fmt.Fprintf(os.Stdout, "Gathering CA bundles from %s.\n", dir)
	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		fmt.Fprintf(os.Stdout, "Checking if %s is a CA bundle.\n", path)
		if detail, err := parseFileAsCA(path); err == nil && detail != nil {
			fmt.Fprintf(os.Stdout, "Found CA bundle in %s.\n", path)
			options.rewriteCABundle(metav1.ObjectMeta{}, detail)

			needsMetadataCollected := false
			for i, loc := range detail.Spec.OnDiskLocations {
				if loc.Path == path {
					needsMetadataCollected = true
				}

				detail.Spec.OnDiskLocations[i].Path = options.rewritePath(loc.Path)
				fmt.Fprintf(os.Stdout, "Rewrite CA result: %s\n", detail.Spec.OnDiskLocations[i].Path)
			}
			ret = append(ret, detail)

			if needsMetadataCollected {
				metadata := getOnDiskLocationMetadata(path)
				metadata.Path = options.rewritePath(metadata.Path)
				metadataList = append(metadataList, metadata)
			}
			return nil
		}
		return nil
	})
	return ret, metadataList, err
}
