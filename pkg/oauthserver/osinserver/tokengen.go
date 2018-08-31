package osinserver

import (
	"strings"

	"github.com/RangelReale/osin"

	"github.com/openshift/origin/pkg/oauthserver/server/crypto"
)

var (
	_ osin.AuthorizeTokenGen = TokenGen{}
	_ osin.AccessTokenGen    = TokenGen{}
)

func randomToken() string {
	for {
		// guaranteed to have no / characters and no trailing ='s
		token := crypto.Random256BitsString()

		// Don't generate tokens with leading dashes... they're hard to use on the command line
		if strings.HasPrefix(token, "-") {
			continue
		}

		return token
	}
}

type TokenGen struct{}

func (TokenGen) GenerateAuthorizeToken(data *osin.AuthorizeData) (ret string, err error) {
	return randomToken(), nil
}

func (TokenGen) GenerateAccessToken(data *osin.AccessData, generaterefresh bool) (string, string, error) {
	accesstoken := randomToken()

	refreshtoken := ""
	if generaterefresh {
		refreshtoken = randomToken()
	}

	return accesstoken, refreshtoken, nil
}
