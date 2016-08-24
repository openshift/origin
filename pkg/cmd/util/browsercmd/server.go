package browsercmd

import (
	"fmt"
	"net"
	"net/http"
)

func useHandler(h Handler) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := h.HandleRequest(r)
		fmt.Println(data, err)
		if err != nil {
			e := h.HandleError(err)
			if e != nil {
				w.Write([]byte(e.Error()))
				fmt.Println(e)
			}
		} else {
			e := h.HandleData(data)
			if e != nil {
				w.Write([]byte(e.Error()))
				fmt.Println(e)
			}
		}
	}
}

type ServerImplementation struct {
	listener net.Listener
}

func (s *ServerImplementation) Start(ch CreateHandler) (Handler, string, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, "", err
	}
	s.listener = listener
	_, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		return nil, "", err
	}
	h, err := ch.Create(port)
	if err != nil {
		return nil, "", err
	}
	http.HandleFunc("/token", useHandler(h))
	go http.Serve(listener, nil)
	return h, port, nil
}

func (s *ServerImplementation) Stop() error {
	return s.listener.Close()
}
