package osinserver

import (
	"fmt"
	"net/http"
	"path"

	"github.com/RangelReale/osin"
	"github.com/golang/glog"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

const (
	AuthorizePath = "/authorize"
	TokenPath     = "/token"
	InfoPath      = "/info"
)

type Server struct {
	config       *osin.ServerConfig
	server       *osin.Server
	authorize    AuthorizeHandler
	access       AccessHandler
	errorHandler ErrorHandler
}

// Logger captures additional osin server errors
type Logger struct {
}

func (l Logger) Printf(format string, v ...interface{}) {
	if glog.V(2) {
		glog.ErrorDepth(3, fmt.Sprintf("osin: "+format, v...))
	}
}

func New(config *osin.ServerConfig, storage osin.Storage, authorize AuthorizeHandler, access AccessHandler, errorHandler ErrorHandler) *Server {
	server := osin.NewServer(config, storage)

	// Override tokengen to ensure we get valid length tokens
	server.AuthorizeTokenGen = TokenGen{}
	server.AccessTokenGen = TokenGen{}
	server.Logger = Logger{}

	return &Server{
		config:       config,
		server:       server,
		authorize:    authorize,
		access:       access,
		errorHandler: errorHandler,
	}
}

// Install registers the Server OAuth handlers into a mux. It is expected that the
// provided prefix will serve all operations
func (s *Server) Install(mux Mux, paths ...string) {
	for _, prefix := range paths {
		mux.HandleFunc(path.Join(prefix, AuthorizePath), s.handleAuthorize)
		mux.HandleFunc(path.Join(prefix, TokenPath), s.handleToken)
		mux.HandleFunc(path.Join(prefix, InfoPath), s.handleInfo)
	}
}

// AuthorizationHandler returns an http.Handler capable of authorizing.
// Used for implicit authorization special flows.
func (s *Server) AuthorizationHandler() http.Handler {
	return http.HandlerFunc(s.handleAuthorize)
}

// TokenHandler returns an http.Handler capable of granting tokens. Used for
// implicit token granting special flows.
func (s *Server) TokenHandler() http.Handler {
	return http.HandlerFunc(s.handleToken)
}

func (s *Server) handleAuthorize(w http.ResponseWriter, r *http.Request) {
	resp := s.server.NewResponse()
	defer resp.Close()

	if ar := s.server.HandleAuthorizeRequest(resp, r); ar != nil {

		if errorCode := r.FormValue("error"); len(errorCode) != 0 {

			// The request already has an error parameter, return directly to the user
			resp.SetErrorUri(
				r.FormValue("error"),
				r.FormValue("error_description"),
				r.FormValue("error_uri"),
				r.FormValue("state"),
			)
			// force redirect response
			resp.SetRedirect(ar.RedirectUri)

		} else {

			handled, err := s.authorize.HandleAuthorize(ar, resp, w)
			if err != nil {
				s.errorHandler.HandleError(err, w, r)
				return
			}
			if handled {
				return
			}
			s.server.FinishAuthorizeRequest(resp, r, ar)

		}
	}

	if resp.IsError && resp.InternalError != nil {
		utilruntime.HandleError(fmt.Errorf("internal error: %s", resp.InternalError))
	}
	osin.OutputJSON(resp, w, r)
}

func (s *Server) handleToken(w http.ResponseWriter, r *http.Request) {
	resp := s.server.NewResponse()
	defer resp.Close()

	if ar := s.server.HandleAccessRequest(resp, r); ar != nil {
		if err := s.access.HandleAccess(ar, w); err != nil {
			s.errorHandler.HandleError(err, w, r)
			return
		}
		s.server.FinishAccessRequest(resp, r, ar)
	}
	if resp.IsError && resp.InternalError != nil {
		utilruntime.HandleError(fmt.Errorf("internal error: %s", resp.InternalError))
	}
	osin.OutputJSON(resp, w, r)
}

func (s *Server) handleInfo(w http.ResponseWriter, r *http.Request) {
	resp := s.server.NewResponse()
	defer resp.Close()

	if ir := s.server.HandleInfoRequest(resp, r); ir != nil {
		s.server.FinishInfoRequest(resp, r, ir)
	}
	osin.OutputJSON(resp, w, r)
}
