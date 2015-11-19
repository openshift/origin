package server

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"
)

// Server is a http.Handler which exposes netutils functionality over HTTP.
type Server struct {
	ipam IpamInterface
	mux  *http.ServeMux
}

type TLSOptions struct {
	Config   *tls.Config
	CertFile string
	KeyFile  string
}

// IpamInterface contains all the methods required by the server.
type IpamInterface interface {
	GetIP() (*net.IPNet, error)
	ReleaseIP(ip *net.IPNet) error
	//GetStats() string
}

// ListenAndServeNetutilServer initializes a server to respond to HTTP network requests on the ipam interface
func ListenAndServeNetutilServer(ipam IpamInterface, address net.IP, port uint, tlsOptions *TLSOptions) error {
	handler := NewServer(ipam)
	addr := net.JoinHostPort(address.String(), strconv.FormatUint(uint64(port), 10))
	s := &http.Server{
		Handler:        handler,
		ReadTimeout:    5 * time.Minute,
		WriteTimeout:   5 * time.Minute,
		MaxHeaderBytes: 1 << 20,
	}
	var listener net.Listener
	var err error
	if tlsOptions != nil {
		listener, err = tls.Listen("tcp", addr, tlsOptions.Config)
	} else {
		listener, err = net.Listen("tcp", addr)
	}
	if err != nil {
		return err
	}
	go s.Serve(listener)
	return nil
}

// NewServer initializes and configures the netutils_server.Server object to handle HTTP requests.
func NewServer(ipam IpamInterface) *Server {
	server := Server{
		ipam: ipam,
		mux:  http.NewServeMux(),
	}
	server.InstallDefaultHandlers()
	return &server
}

// InstallDefaultHandlers registers the default set of supported HTTP request patterns with the mux.
func (s *Server) InstallDefaultHandlers() {
	s.mux.HandleFunc("/netutils/subnet", s.handleSubnet)
	s.mux.HandleFunc("/netutils/ip/", s.handleIP)
	s.mux.HandleFunc("/netutils/gateway", s.handleGateway)
	s.mux.HandleFunc("/stats", s.handleStats)
}

// error serializes an error object into an HTTP response.
func (s *Server) error(w http.ResponseWriter, err error) {
	msg := fmt.Sprintf("Internal Error: %v", err)
	http.Error(w, msg, http.StatusInternalServerError)
}

// handleSubnet handles gateway requests
func (s *Server) handleSubnet(w http.ResponseWriter, req *http.Request) {
	w.Header().Add("Content-type", "application/json")
	w.Write([]byte("Not implemented"))
	return
}

// handleGateway handles gateway requests
func (s *Server) handleGateway(w http.ResponseWriter, req *http.Request) {
	w.Header().Add("Content-type", "application/json")
	w.Write([]byte("Not implemented"))
	return
}

// handleIP handles IP requests
func (s *Server) handleIP(w http.ResponseWriter, req *http.Request) {
	if req.Method == "GET" {
		w.Header().Add("Content-type", "application/json")
		ipnet, err := s.ipam.GetIP()
		if err != nil {
			s.error(w, err)
		} else {
			w.Write([]byte(ipnet.String()))
		}
	} else if req.Method == "DELETE" {
		ip, ipNet, err := net.ParseCIDR(req.URL.Path[len("/netutils/ip/"):])
		if err != nil {
			s.error(w, err)
		}
		delIP := &net.IPNet{IP: ip, Mask: ipNet.Mask}
		err = s.ipam.ReleaseIP(delIP)
		if err != nil {
			s.error(w, err)
		}
	} else {
		http.Error(w, "Method can only be GET/DELETE", http.StatusNotFound)
	}
	return
}

// handleStats handles stats requests
func (s *Server) handleStats(w http.ResponseWriter, req *http.Request) {
	w.Header().Add("Content-type", "application/json")
	w.Write([]byte("Not implemented"))
	return
}

// ServeHTTP responds to HTTP requests
func (s *Server) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	s.mux.ServeHTTP(w, req)
}
