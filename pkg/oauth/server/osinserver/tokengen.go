package osinserver

import (
	"encoding/base64"
	"strings"

	"code.google.com/p/go-uuid/uuid"

	"github.com/RangelReale/osin"
)

func randomToken() string {
	// Two random uuids to get the length we want (> 32 chars when base-64 encoded)
	b := []byte{}
	b = append(b, []byte(uuid.NewRandom())...)
	b = append(b, []byte(uuid.NewRandom())...)
	// Use URLEncoding to ensure we don't get / characters
	s := base64.URLEncoding.EncodeToString(b)
	// Strip trailing ='s... they're ugly
	return strings.TrimRight(s, "=")
}

// AuthorizeTokenGen is an authorization token generator
type AuthorizeTokenGen struct {
}

// GenerateAuthorizeToken generates a random UUID code
func (a *AuthorizeTokenGen) GenerateAuthorizeToken(data *osin.AuthorizeData) (ret string, err error) {
	return randomToken(), nil
}

// AccessTokenGen is an access token generator
type AccessTokenGen struct {
}

// GenerateAccessToken generates random UUID access and refresh tokens
func (a *AccessTokenGen) GenerateAccessToken(data *osin.AccessData, generaterefresh bool) (string, string, error) {
	accesstoken := randomToken()

	refreshtoken := ""
	if generaterefresh {
		refreshtoken = randomToken()
	}
	return accesstoken, refreshtoken, nil
}
