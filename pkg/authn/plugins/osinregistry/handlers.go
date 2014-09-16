package osinregistry

import (
	"net/http"

	"github.com/RangelReale/osin"
)

type UserContextHandlers struct {
}

func (h *UserContextHandlers) HandleAuthorize(ar *osin.AuthorizeRequest, w http.ResponseWriter, r *http.Request) bool {
}

func (h *UserContextHandlers) HandleAccess(ar *osin.AccessRequest, w http.ResponseWriter, r *http.Request) bool {
}
