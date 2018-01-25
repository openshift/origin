package osinserver

import (
	"fmt"
	"net/http"

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
	config.AllowClientSecretInParams = true
	config.AllowGetAccessRequest = true
	config.RedirectUriSeparator = ","
	config.ErrorStatusCode = http.StatusBadRequest

	return config
}

// defaultError implements ErrorHandler
type defaultErrorHandler struct{}

// NewDefaultErrorHandler returns a simple ErrorHandler
func NewDefaultErrorHandler() ErrorHandler {
	return defaultErrorHandler{}
}

// HandleError implements ErrorHandler
func (defaultErrorHandler) HandleError(err error, w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(http.StatusInternalServerError)
	fmt.Fprintf(w, "Error: %s", err)
}
