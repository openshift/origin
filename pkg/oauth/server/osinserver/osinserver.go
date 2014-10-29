package osinserver

import (
	"net/http"
	"strings"

	"github.com/RangelReale/osin"
	"github.com/golang/glog"
)

type Server struct {
	config    *osin.ServerConfig
	server    *osin.Server
	authorize AuthorizeHandler
	access    AccessHandler
}

func New(config *osin.ServerConfig, storage osin.Storage, authorize AuthorizeHandler, access AccessHandler) *Server {
	return &Server{
		config:    config,
		server:    osin.NewServer(config, storage),
		authorize: authorize,
		access:    access,
	}
}

// Install registers the Server OAuth handlers into a mux. It is expected that the
// provided prefix will serve all operations. Path MUST NOT end in a slash.
func (s *Server) Install(mux Mux, paths ...string) {
	for _, prefix := range paths {
		prefix = strings.TrimRight(prefix, "/")

		mux.HandleFunc(prefix+"/authorize", s.handleAuthorize)
		mux.HandleFunc(prefix+"/token", s.handleToken)
		mux.HandleFunc(prefix+"/info", s.handleInfo)
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
		if s.authorize.HandleAuthorize(ar, w, r) {
			return
		}
		s.server.FinishAuthorizeRequest(resp, r, ar)
	}

	if resp.IsError && resp.InternalError != nil {
		glog.Errorf("Internal error: %s", resp.InternalError)
	}
	osin.OutputJSON(resp, w, r)
}

func (s *Server) handleToken(w http.ResponseWriter, r *http.Request) {
	resp := s.server.NewResponse()
	defer resp.Close()

	if ar := s.server.HandleAccessRequest(resp, r); ar != nil {
		s.access.HandleAccess(ar, w, r)
		s.server.FinishAccessRequest(resp, r, ar)
	}
	if resp.IsError && resp.InternalError != nil {
		glog.Errorf("Internal error: %s", resp.InternalError)
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

// func (s *Server) String() string {
// 	return fmt.Sprintf("osinserver.Server{config:%#v, authorize:%v, access:%v}", s.config, s.authorize, s.access)
// }
