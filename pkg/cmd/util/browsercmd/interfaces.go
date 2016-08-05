package browsercmd

import "github.com/RangelReale/osincli"

type Handler interface {
	HandleError(osincli.Error) error
	HandleData(osincli.AuthorizeData) error
}

type Server interface {
	Start(Handler) (string, error)
	Stop() error
}

type Browser interface {
	Open(rawurl string) error
}
