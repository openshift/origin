package osincli

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
)

const (
	pkce_s256         = "S256"
	code_verifier_len = 96 // 96 8-bit values (ASCII) == 128 6-bit values (base64)
)

func GeneratePKCE() (string, string, string, error) {
	random := make([]byte, code_verifier_len)
	if _, err := rand.Read(random); err != nil {
		return "", "", "", err
	}
	verifier := base64.RawURLEncoding.EncodeToString(random)
	hash := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(hash[:])
	return challenge, pkce_s256, verifier, nil
}

func PopulatePKCE(c *ClientConfig) error {
	challenge, method, verifier, err := GeneratePKCE()
	if err != nil {
		return err
	}
	c.CodeChallenge = challenge
	c.CodeChallengeMethod = method
	c.CodeVerifier = verifier
	return nil
}
