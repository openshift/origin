package api

import (
	"errors"

	jwt "github.com/dgrijalva/jwt-go"
	"github.com/openshift/origin/pkg/util/hash"
)

var (
	hasher          = hash.NewSHA256Hasher()
	saltedHasher    = hash.NewSaltedHasher(hasher)
	errInvalidToken = errors.New("invalid token")
)

type TokenClaims struct {
	UserHash   string `json:"uh,omitempty"`
	Secret     string `json:"s,omitempty"`
	SecretHash string `json:"sh,omitempty"`
}

func (t *TokenClaims) Valid() error {
	if !hasher.VerifyHashFormat(t.UserHash) {
		return errInvalidToken
	}

	secretLength := len(t.Secret)
	secretHashLength := len(t.SecretHash)
	switch {
	case secretHashLength > 0 && secretLength > 0:
		if !hasher.VerifyHash(t.Secret, t.SecretHash) {
			return errInvalidToken
		}

	case secretHashLength > 0:
		if !hasher.VerifyHashFormat(t.SecretHash) {
			return errInvalidToken
		}

	case secretLength > 0:
		if secretLength < 32 {
			return errInvalidToken
		}

	default:
		return errInvalidToken
	}

	return nil
}

type SigningMethodNone struct{}

var noSigning = &SigningMethodNone{}

func (*SigningMethodNone) Verify(signingString, signature string, key interface{}) error {
	if key != nil {
		return errors.New("no key allowed for none algorithm")
	}
	if signature != "" {
		return errors.New("no signing string allowed for none algorithm")
	}
	return nil
}
func (*SigningMethodNone) Sign(signingString string, key interface{}) (string, error) {
	if key != nil {
		return "", errors.New("no key allowed for none algorithm")
	}
	return "", nil
}

func (*SigningMethodNone) Alg() string { return "none" }

func init() {
	jwt.RegisterSigningMethod("none", func() jwt.SigningMethod {
		return noSigning
	})
}

func TokenNameFromUserAndSecret(user string, secret string) (tokenName string, bearerToken string, err error) {
	if len(user) == 0 || len(secret) < 32 {
		return "", "", errInvalidToken
	}

	userHash := hasher.Hash(user)
	secretHash := hasher.Hash(secret)

	tokenName, err = jwt.NewWithClaims(noSigning, &TokenClaims{UserHash: userHash, SecretHash: secretHash}).SignedString(nil)
	bearerToken, err = jwt.NewWithClaims(noSigning, &TokenClaims{UserHash: userHash, Secret: secret}).SignedString(nil)

	return tokenName, bearerToken, nil
}

func ClaimsFromToken(token string) (*TokenClaims, error) {
	noKey := func(t *jwt.Token) (interface{}, error) { return nil, nil }
	claims := &TokenClaims{}
	_, err := jwt.ParseWithClaims(token, claims, noKey)
	if err == nil && len(claims.SecretHash) == 0 {
		claims.SecretHash = hasher.Hash(claims.Secret)
	}
	return claims, err
}

func VerifySaltedHash(token string, salt string, saltedHash string) error {
	if len(saltedHash) == 0 && len(salt) == 0 {
		return nil
	}
	claims, err := ClaimsFromToken(token)
	if err != nil {
		return err
	}
	if !saltedHasher.VerifySaltedHash(claims.Secret, salt, saltedHash) {
		return errInvalidToken
	}
	return nil
}

func VerifyUserHash(userName string, userHash string) error {
	if !hasher.VerifyHash(userName, userHash) {
		return errInvalidToken
	}
	return nil
}
