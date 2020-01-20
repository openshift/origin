package gocertifi

import "testing"

func TestGetCerts(t *testing.T) {
	certPool, err := CACerts()
	if certPool == nil || err != nil || len(certPool.Subjects()) == 0 {
		t.Errorf("Failed to return the certificates.")
	}
}
