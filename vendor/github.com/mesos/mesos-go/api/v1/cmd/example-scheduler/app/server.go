package app

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
)

func serveFile(filename string) (handler http.Handler, err error) {
	_, err = os.Stat(filename)
	if err != nil {
		err = fmt.Errorf("failed to locate artifact: %+v", err)
	} else {
		handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.ServeFile(w, r, filename)
		})
	}
	return
}

// returns (downloadURI, basename(path))
func serveExecutorArtifact(server server, path string, mux *http.ServeMux) (string, string, error) {
	// Create base path (http://foobar:5000/<base>)
	pathSplit := strings.Split(path, "/")
	var base string
	if len(pathSplit) > 0 {
		base = pathSplit[len(pathSplit)-1]
	} else {
		base = path
	}
	pattern := "/" + base
	h, err := serveFile(path)
	if err != nil {
		return "", "", err
	}

	mux.Handle(pattern, h)

	hostURI := fmt.Sprintf("http://%s:%d/%s", server.address, server.port, base)
	log.Println("Hosting artifact '" + path + "' at '" + hostURI + "'")

	return hostURI, base, nil
}

func newListener(server server) (*net.TCPListener, int, error) {
	addr, err := net.ResolveTCPAddr("tcp", net.JoinHostPort(server.address, strconv.Itoa(server.port)))
	if err != nil {
		return nil, 0, err
	}
	listener, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return nil, 0, err
	}
	bindAddress := listener.Addr().String()
	_, port, err := net.SplitHostPort(bindAddress)
	if err != nil {
		return nil, 0, err
	}
	iport, err := strconv.Atoi(port)
	if err != nil {
		return nil, 0, err
	}
	return listener, iport, nil
}
