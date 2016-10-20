package proxy

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/elazarl/goproxy"
	"github.com/spf13/cobra"

	ktransport "k8s.io/kubernetes/pkg/client/transport"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

const (
	proxyProxyLong = `
Create a local, insecure proxy which attaches local credentials to outgoing proxy connections`
)

type ProxyProxyOptions struct {
	RemoteHost string
	BindHost   string

	// TODO make this support all the restclient.Config options
	UserToken string
}

// NewCmdCreateServiceAccount is a macro command to create a new service account
func NewCmdProxyProxy(name, fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	o := &ProxyProxyOptions{}

	cmd := &cobra.Command{
		Use:  "proxy BIND_HOST OPENSHIFT_PROXY_HOST",
		Long: proxyProxyLong,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(f, args))
			cmdutil.CheckErr(o.Validate())
			cmdutil.CheckErr(o.RunProxy())
		},
	}

	cmdutil.AddOutputFlagsForMutation(cmd)
	return cmd
}

func (o *ProxyProxyOptions) Complete(f *clientcmd.Factory, args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("OPENSHIFT_PROXY_HOST is required: %v", args)
	}

	o.BindHost = args[0]
	o.RemoteHost = args[1]

	kubeconfig, err := f.ClientConfig()
	if err != nil {
		return err
	}
	o.UserToken = kubeconfig.BearerToken

	return nil
}

func (o *ProxyProxyOptions) Validate() error {
	return nil
}

func (o *ProxyProxyOptions) RunProxy() error {
	var handler http.Handler

	url, err := url.Parse(o.RemoteHost)
	if err != nil {
		return err
	}

	_ = goproxy.NewProxyHttpServer()
	// proxy.Verbose = true
	// _ = http.ProxyURL(url)
	// _ = ktransport.DebugWrappers(proxy.Tr)
	// _ = NewBearerAuthRoundTripper(o.UserToken, proxy.Tr)

	// var roundTripper http.RoundTripper
	// proxyTransport := http.DefaultTransport
	// proxyTransport.(*http.Transport).Proxy = http.ProxyURL(url)
	// roundTripper = ktransport.DebugWrappers(proxyTransport)

	myClient := &http.Client{
		Transport: NewBearerAuthRoundTripper(o.UserToken, ktransport.DebugWrappers(&Transport{Proxy: http.ProxyURL(url)})),
	}

	proxy := &proxyproxy{
		client:   myClient,
		proxyURL: url,
	}

	handler = proxy
	handler = o.withPAC(handler)

	server := &http.Server{
		Addr:           o.BindHost,
		Handler:        handler,
		MaxHeaderBytes: 1 << 20,
	}

	fmt.Printf("Serving insecurely on %s\n", o.BindHost)
	if err := server.ListenAndServe(); err != nil {
		return err
	}

	return nil
}

func (o *ProxyProxyOptions) withPAC(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/pac" {
			w.Write([]byte(`function FindProxyForURL(url, host) { 
	if (shExpMatch(host, "*.svc"))
		return "HTTPS ` + o.BindHost + `"; 
	if (shExpMatch(host, "*.svc.cluster.local"))
		return "HTTPS ` + o.BindHost + `"; 

	return "DIRECT";
}`))
			w.WriteHeader(http.StatusOK)
			return
		}

		handler.ServeHTTP(w, req)
	})
}

type proxyproxy struct {
	client *http.Client

	proxyURL *url.URL
}

func (p *proxyproxy) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	// *outreq = *req // includes shallow copies of maps, but okay

	// if closeNotifier, ok := rw.(http.CloseNotifier); ok {
	// 	if requestCanceler, ok := p.roundTripper.(requestCanceler); ok {
	// 		reqDone := make(chan struct{})
	// 		defer close(reqDone)

	// 		clientGone := closeNotifier.CloseNotify()

	// 		outreq.Body = struct {
	// 			io.Reader
	// 			io.Closer
	// 		}{
	// 			Reader: &runOnFirstRead{
	// 				Reader: outreq.Body,
	// 				fn: func() {
	// 					go func() {
	// 						select {
	// 						case <-clientGone:
	// 							requestCanceler.CancelRequest(outreq)
	// 						case <-reqDone:
	// 						}
	// 					}()
	// 				},
	// 			},
	// 			Closer: outreq.Body,
	// 		}
	// 	}
	// }

	requestURI, _ := url.ParseRequestURI(req.RequestURI)
	urlString := p.proxyURL.Scheme + "://" + p.proxyURL.Host + requestURI.Path
	outreq, err := http.NewRequest(req.Method, requestURI.String(), nil)
	if err != nil {
		fmt.Printf("##### ERROR %v\n", err)
		rw.WriteHeader(http.StatusBadGateway)
		return
	}

	copyHeader(outreq.Header, req.Header)
	outreq.Proto = "HTTP/1.1"
	outreq.ProtoMajor = 1
	outreq.ProtoMinor = 1
	outreq.Close = false
	outreq.Header.Set("Host", requestURI.Host)

	fmt.Printf("#### %q %q %q\n", req.Header.Get("Host"), req.Header.Get("Request URI"), urlString)

	res, err := p.client.Do(outreq)

	// dump, _ := httputil.DumpRequest(outreq, false)
	// fmt.Printf("SENDING %v\n", string(dump))

	// res, err := p.roundTripper.RoundTrip(outreq)
	if err != nil {
		fmt.Printf("##### ERROR %v\n", err)
		rw.WriteHeader(http.StatusBadGateway)
		return
	}

	copyHeader(rw.Header(), res.Header)

	// The "Trailer" header isn't included in the Transport's response,
	// at least for *http.Transport. Build it up from Trailer.
	if len(res.Trailer) > 0 {
		var trailerKeys []string
		for k := range res.Trailer {
			trailerKeys = append(trailerKeys, k)
		}
		rw.Header().Add("Trailer", strings.Join(trailerKeys, ", "))
	}

	rw.WriteHeader(res.StatusCode)
	if len(res.Trailer) > 0 {
		// Force chunking if we saw a response trailer.
		// This prevents net/http from calculating the length for short
		// bodies and adding a Content-Length.
		if fl, ok := rw.(http.Flusher); ok {
			fl.Flush()
		}
	}
	copyResponse(rw, res.Body)
	res.Body.Close() // close now, instead of defer, to populate res.Trailer
	copyHeader(rw.Header(), res.Trailer)
}

func copyResponse(dst io.Writer, src io.Reader) {
	var buf []byte
	io.CopyBuffer(dst, src, buf)
}

type bearerAuthRoundTripper struct {
	bearer string
	rt     http.RoundTripper
}

// NewBearerAuthRoundTripper adds the provided bearer token to a request
// unless the authorization header has already been set.
func NewBearerAuthRoundTripper(bearer string, rt http.RoundTripper) http.RoundTripper {
	return &bearerAuthRoundTripper{bearer, rt}
}

func (rt *bearerAuthRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if len(req.Header.Get("Proxy-Authorization")) != 0 {
		return rt.rt.RoundTrip(req)
	}

	req = cloneRequest(req)
	req.Header.Set("Proxy-Authorization", fmt.Sprintf("Bearer %s", rt.bearer))
	return rt.rt.RoundTrip(req)
}

func (rt *bearerAuthRoundTripper) CancelRequest(req *http.Request) {
	if canceler, ok := rt.rt.(requestCanceler); ok {
		canceler.CancelRequest(req)
	} else {
		fmt.Printf("CancelRequest not implemented")
	}
}

func (rt *bearerAuthRoundTripper) WrappedRoundTripper() http.RoundTripper { return rt.rt }

// cloneRequest returns a clone of the provided *http.Request.
// The clone is a shallow copy of the struct and its Header map.
func cloneRequest(r *http.Request) *http.Request {
	// shallow copy of the struct
	r2 := new(http.Request)
	*r2 = *r
	// deep copy of the Header
	r2.Header = make(http.Header)
	for k, s := range r.Header {
		r2.Header[k] = s
	}
	return r2
}
