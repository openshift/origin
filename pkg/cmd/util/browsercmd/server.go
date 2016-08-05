package browsercmd

import "net"

type ServerImplementation struct {
	listener net.Listener
}

func (s *ServerImplementation) Start(h Handler) (string, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	s.listener = listener
	address := listener.Addr().String()
	_, port, err := net.SplitHostPort(address)
	if err != nil {
		return "", err
	}
	return port, nil
}

func (s *ServerImplementation) Stop() error {
	return s.listener.Close()
}
