package osinserver

import (
	"github.com/RangelReale/osin"
)

func NewDefaultServerConfig() *osin.ServerConfig {
	config := osin.NewServerConfig()

	config.AllowedAuthorizeTypes = osin.AllowedAuthorizeType{
		osin.CODE,
		osin.TOKEN,
	}
	config.AllowedAccessTypes = osin.AllowedAccessType{
		osin.AUTHORIZATION_CODE,
		osin.REFRESH_TOKEN,
		osin.PASSWORD,
		osin.CLIENT_CREDENTIALS,
		osin.ASSERTION,
	}
	config.AllowGetAccessRequest = true
	config.RedirectUriSeparator = ","

	return config
}
