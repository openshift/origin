package tlsclientconfig

import (
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"io/ioutil"
	"os"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetupCertificates(t *testing.T) {
	// Success
	tlsc := tls.Config{}
	err := SetupCertificates("testdata/full", &tlsc)
	require.NoError(t, err)
	require.NotNil(t, tlsc.RootCAs)
	// RootCAs include SystemCertPool
	loadedSubjectBytes := map[string]struct{}{}
	for _, s := range tlsc.RootCAs.Subjects() {
		loadedSubjectBytes[string(s)] = struct{}{}
	}
	systemCertPool, err := x509.SystemCertPool()
	require.NoError(t, err)
	for _, s := range systemCertPool.Subjects() {
		_, ok := loadedSubjectBytes[string(s)]
		assert.True(t, ok)
	}
	// RootCAs include our certificates
	loadedSubjectCNs := map[string]struct{}{}
	for _, s := range tlsc.RootCAs.Subjects() {
		subjectRDN := pkix.RDNSequence{}
		rest, err := asn1.Unmarshal(s, &subjectRDN)
		require.NoError(t, err)
		require.Empty(t, rest)
		subject := pkix.Name{}
		subject.FillFromRDNSequence(&subjectRDN)
		loadedSubjectCNs[subject.CommonName] = struct{}{}
	}
	_, ok := loadedSubjectCNs["containers/image test CA certificate 1"]
	assert.True(t, ok)
	_, ok = loadedSubjectCNs["containers/image test CA certificate 2"]
	assert.True(t, ok)
	// Certificates include our certificates
	require.Len(t, tlsc.Certificates, 2)
	names := []string{}
	for _, c := range tlsc.Certificates {
		require.Len(t, c.Certificate, 1)
		parsed, err := x509.ParseCertificate(c.Certificate[0])
		require.NoError(t, err)
		names = append(names, parsed.Subject.CommonName)
	}
	sort.Strings(names)
	assert.Equal(t, []string{
		"containers/image test client certificate 1",
		"containers/image test client certificate 2",
	}, names)

	// Directory does not exist
	tlsc = tls.Config{}
	err = SetupCertificates("/this/does/not/exist", &tlsc)
	require.NoError(t, err)
	assert.Equal(t, &tls.Config{}, &tlsc)

	// Directory not accessible
	unreadableDir, err := ioutil.TempDir("", "containers-image-tlsclientconfig")
	require.NoError(t, err)
	defer func() {
		_ = os.Chmod(unreadableDir, 0700)
		_ = os.Remove(unreadableDir)
	}()
	err = os.Chmod(unreadableDir, 000)
	require.NoError(t, err)
	tlsc = tls.Config{}
	err = SetupCertificates(unreadableDir, &tlsc)
	assert.NoError(t, err)
	assert.Equal(t, &tls.Config{}, &tlsc)

	// Other error reading the directory
	tlsc = tls.Config{}
	err = SetupCertificates("/dev/null/is/not/a/directory", &tlsc)
	assert.Error(t, err)

	// Unreadable system cert pool untested
	// Unreadable CA certificate
	tlsc = tls.Config{}
	err = SetupCertificates("testdata/unreadable-ca", &tlsc)
	assert.Error(t, err)

	// Misssing key file
	tlsc = tls.Config{}
	err = SetupCertificates("testdata/missing-key", &tlsc)
	assert.Error(t, err)
	// Missing certificate file
	tlsc = tls.Config{}
	err = SetupCertificates("testdata/missing-cert", &tlsc)
	assert.Error(t, err)

	// Unreadable key file
	tlsc = tls.Config{}
	err = SetupCertificates("testdata/unreadable-key", &tlsc)
	assert.Error(t, err)
	// Unreadable certificate file
	tlsc = tls.Config{}
	err = SetupCertificates("testdata/unreadable-cert", &tlsc)
	assert.Error(t, err)
}
