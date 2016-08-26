package browsercmd

import (
	"net"
	"net/http"
)

type ServerImplementation struct {
	listener net.Listener
}

func (s *ServerImplementation) Start(ch CreateHandler) (Handler, string, error) {
	var err error
	s.listener, err = net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, "", err
	}
	_, port, err := net.SplitHostPort(s.listener.Addr().String())
	if err != nil {
		return nil, "", err
	}
	h, err := ch.Create(port)
	if err != nil {
		return nil, "", err
	}
	http.HandleFunc("/token", h.HandleRequest)
	go http.Serve(s.listener, nil)
	return h, port, nil
}

func (s *ServerImplementation) Stop() error {
	return s.listener.Close()
}

func NewServer() Server {
	return &ServerImplementation{}
}
