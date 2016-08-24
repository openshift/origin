package browsercmd

import (
	"net/http"

	"github.com/RangelReale/osincli"
)

type Handler interface {
	HandleRequest(*http.Request) (*osincli.AuthorizeData, error)
	HandleData(*osincli.AuthorizeData) error
	HandleError(error) error
	GetAccessData() (*osincli.AccessData, error)
	// HandleError(osincli.Error) error
}

type Server interface {
	Start(CreateHandler) (Handler, string, error)
	Stop() error
}

type Browser interface {
	Open(rawurl string) error
}

type CreateHandler interface {
	Create(port string) (Handler, error)
}
