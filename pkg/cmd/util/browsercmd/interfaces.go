package browsercmd

import (
	"net/http"

	"github.com/RangelReale/osincli"
)

type Handler interface {
	client
	state
}

type client interface {
	HandleRequest(w http.ResponseWriter, req *http.Request)
	HandleData(data *osincli.AuthorizeData) error
	HandleSuccess(w http.ResponseWriter, req *http.Request)
	HandleError(err error, w http.ResponseWriter, req *http.Request)
	GetData() (*osincli.AccessData, error)
}

type state interface {
	GenerateState() string
	CheckState(data *osincli.AuthorizeData) bool
}

type Server interface {
	Start(ch CreateHandler) (Handler, string, error)
	Stop() error
}

type Browser interface {
	Open(rawURL string) error
}

type CreateHandler interface {
	Create(port string) (Handler, error)
}
