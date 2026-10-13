package tokenrequest

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"

	"k8s.io/klog/v2"
)

type callbackResult struct {
	token string
	err   error
}

type callbackHandlerFunc func(*http.Request) (string, error)

type callbackServer struct {
	server      *http.Server
	tcpListener net.Listener
	listenAddr  *net.TCPAddr

	// the channel to deliver the access token or error
	resultChan chan callbackResult

	// function to call when the callback redirect is received; this should be
	// used to request an access token based on the received callback
	callbackHandler callbackHandlerFunc
}

func newCallbackServer(port int) (*callbackServer, error) {
	callbackServer := &callbackServer{
		resultChan: make(chan callbackResult, 1),
	}

	loopbackAddr, err := getLoopbackAddr()
	if err != nil {
		return nil, err
	}

	callbackServer.tcpListener, err = net.Listen("tcp", net.JoinHostPort(loopbackAddr, strconv.Itoa(port)))
	if err != nil {
		return nil, err
	}

	listenAddr, ok := callbackServer.tcpListener.Addr().(*net.TCPAddr)
	if !ok {
		return nil, fmt.Errorf("listener is not of TCP type: %v", callbackServer.tcpListener.Addr())
	}
	callbackServer.listenAddr = listenAddr

	mux := http.NewServeMux()
	mux.Handle("/callback", callbackServer)

	callbackServer.server = &http.Server{
		Handler: mux,
	}

	return callbackServer, nil
}

func (c *callbackServer) ListenAddr() *net.TCPAddr {
	return c.listenAddr
}

func (c *callbackServer) SetCallbackHandler(callbackHandler callbackHandlerFunc) {
	c.callbackHandler = callbackHandler
}

func (c *callbackServer) Start() {
	err := c.server.Serve(c.tcpListener)
	if err != nil && err != http.ErrServerClosed {
		klog.V(4).Infof("callback server failed: %v", err)
		c.resultChan <- callbackResult{err: err}
	}

	close(c.resultChan)
}

func (c *callbackServer) Shutdown(ctx context.Context) {
	err := c.server.Shutdown(ctx)
	if err != nil {
		klog.V(4).Infof("failed to shutdown callback server: %v", err)
	}
}

func (c *callbackServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if c.callbackHandler == nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("access token request failed; please return to your terminal"))
		c.resultChan <- callbackResult{err: fmt.Errorf("no callback handler set")}
		return
	}

	token, err := c.callbackHandler(r)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("access token request failed; please return to your terminal"))
		c.resultChan <- callbackResult{err: err}
		return
	}

	w.Write([]byte("access token received successfully; please return to your terminal"))
	c.resultChan <- callbackResult{token: token}
}

// getLoopbackAddr returns the first address from the host's network interfaces which is a loopback address
func getLoopbackAddr() (string, error) {
	interfaces, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}

	for _, iface := range interfaces {
		if ipaddr, ok := iface.(*net.IPNet); ok {
			if ipaddr.IP.IsLoopback() {
				return ipaddr.IP.String(), nil
			}
		}
	}

	return "", errors.New("no loopback network interfaces found")
}
