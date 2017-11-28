package dockerproxy

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/golang/glog"
)

const (
	defaultCaFile   = "ca.pem"
	defaultKeyFile  = "key.pem"
	defaultCertFile = "cert.pem"

	initialInterval = 2 * time.Second
	maxInterval     = 1 * time.Minute
)

type Config struct {
	Client      *docker.Client
	ListenAddrs []string

	HostnameFromLabel   string
	HostnameMatch       string
	HostnameReplacement string
	Image               string
	Version             string
}

type wait struct {
	ident string
	ch    chan error
	done  bool
}

type Proxy struct {
	sync.Mutex
	config              Config
	hostnameMatchRegexp *regexp.Regexp
	normalisedAddrs     []string
	quit                chan struct{}
}

func StubProxy(c Config) (*Proxy, error) {
	if c.Client == nil {
		client, err := docker.NewClientFromEnv()
		if err != nil {
			return nil, err
		}
		c.Client = client
	}
	p := &Proxy{
		config: c,
		quit:   make(chan struct{}),
	}
	return p, nil
}

func NewProxy(c Config) (*Proxy, error) {
	p, err := StubProxy(c)
	if err != nil {
		return nil, err
	}

	p.hostnameMatchRegexp, err = regexp.Compile(c.HostnameMatch)
	if err != nil {
		return nil, fmt.Errorf("Incorrect hostname match '%s': %s", c.HostnameMatch, err.Error())
	}

	return p, nil
}

func (proxy *Proxy) Dial() (net.Conn, error) {
	proto := "tcp"
	addr := proxy.config.Client.Endpoint()
	switch {
	case strings.HasPrefix(addr, "unix://"):
		proto = "unix"
		addr = strings.TrimPrefix(addr, "unix://")
	case strings.HasPrefix(addr, "tcp://"):
		addr = strings.TrimPrefix(addr, "tcp://")
	}
	return net.Dial(proto, addr)
}

func (proxy *Proxy) Listen() []net.Listener {
	listeners := []net.Listener{}
	proxy.normalisedAddrs = []string{}
	unixAddrs := []string{}
	for _, addr := range proxy.config.ListenAddrs {
		if strings.HasPrefix(addr, "unix://") || strings.HasPrefix(addr, "/") {
			unixAddrs = append(unixAddrs, addr)
			continue
		}
		listener, normalisedAddr, err := proxy.listen(addr)
		if err != nil {
			glog.Fatalf("Unable to listen: %s", err)
		}
		listeners = append(listeners, listener)
		proxy.normalisedAddrs = append(proxy.normalisedAddrs, normalisedAddr)
	}

	if len(unixAddrs) > 0 {
		for _, unixAddr := range unixAddrs {
			listener, _, err := proxy.listen(unixAddr)
			if err != nil {
				glog.Fatalf("Unable to listen: %s", err)
			}
			listeners = append(listeners, listener)
			proxy.normalisedAddrs = append(proxy.normalisedAddrs, unixAddr)
		}
	}

	for _, addr := range proxy.normalisedAddrs {
		glog.Infof("Docker proxy listening on %s", addr)
	}

	return listeners
}

// ServeWithReady starts a server on each provided listener, invoking ready when all listeners
// have been spawned, and exits with a fatal error if any listener closes.
// TODO: parameterize the http.Server here.
func ServeWithReady(handler http.Handler, listeners []net.Listener, ready func()) {
	errs := make(chan error)
	for _, listener := range listeners {
		go func(listener net.Listener) {
			errs <- (&http.Server{Handler: handler}).Serve(listener)
		}(listener)
	}
	// It would be better if we could delay calling Done() until all
	// the listeners are ready, but it doesn't seem to be possible to
	// hook the right point in http.Server
	ready()
	for range listeners {
		err := <-errs
		if err != nil {
			glog.Fatalf("Serve failed: %s", err)
		}
	}
}

func (proxy *Proxy) StatusHTTP(w http.ResponseWriter, r *http.Request) {
	for _, addr := range proxy.normalisedAddrs {
		fmt.Fprintln(w, addr)
	}
}

func copyOwnerAndPermissions(from, to string) error {
	stat, err := os.Stat(from)
	if err != nil {
		return err
	}
	if err = os.Chmod(to, stat.Mode()); err != nil {
		return err
	}

	moreStat, ok := stat.Sys().(*syscall.Stat_t)
	if !ok {
		return nil
	}

	if err = os.Chown(to, int(moreStat.Uid), int(moreStat.Gid)); err != nil {
		return err
	}

	return nil
}

func (proxy *Proxy) listen(protoAndAddr string) (net.Listener, string, error) {
	var (
		listener    net.Listener
		err         error
		proto, addr string
	)

	if protoAddrParts := strings.SplitN(protoAndAddr, "://", 2); len(protoAddrParts) == 2 {
		proto, addr = protoAddrParts[0], protoAddrParts[1]
	} else if strings.HasPrefix(protoAndAddr, "/") {
		proto, addr = "unix", protoAndAddr
	} else {
		proto, addr = "tcp", protoAndAddr
	}

	switch proto {
	case "tcp":
		listener, err = net.Listen(proto, addr)
		if err != nil {
			return nil, "", err
		}
		if proxy.config.Client.TLSConfig != nil {
			listener = tls.NewListener(listener, proxy.config.Client.TLSConfig)
		}

	case "unix":
		// remove socket from last invocation
		if err := os.Remove(addr); err != nil && !os.IsNotExist(err) {
			return nil, "", err
		}
		listener, err = net.Listen(proto, addr)
		if err != nil {
			return nil, "", err
		}
		dockerEndpoint := proxy.config.Client.Endpoint()
		if strings.HasPrefix(dockerEndpoint, "unix://") {
			if err = copyOwnerAndPermissions(strings.TrimPrefix(dockerEndpoint, "unix://"), addr); err != nil {
				return nil, "", err
			}
		}

	default:
		return nil, "", fmt.Errorf("invalid protocol format %q for %q", proto, protoAndAddr)
	}

	return listener, fmt.Sprintf("%s://%s", proto, addr), nil
}

func (proxy *Proxy) Stop() {
	close(proxy.quit)
}
