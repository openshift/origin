package browsercmd

import (
	"fmt"
	"net"
	"net/http"
)

func handler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "TEST %#v", r.URL.Query())
}

type ServerImplementation struct {
	listener net.Listener
}

func (s *ServerImplementation) Start(h Handler) (string, error) {
	http.HandleFunc("/token", handler)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	s.listener = listener
	_, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		return "", err
	}
	go http.Serve(listener, nil)
	return port, nil
}

func (s *ServerImplementation) Stop() error {
	return s.listener.Close()
}
