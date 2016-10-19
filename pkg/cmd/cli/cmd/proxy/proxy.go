package proxy

import (
	"fmt"
	"io"
	"net/http"

	"github.com/elazarl/goproxy"
	"github.com/spf13/cobra"

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

	proxy := goproxy.NewProxyHttpServer()
	proxy.Verbose = true
	proxy.OnRequest().DoFunc(func(r *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
		r.Header.Set("X-Proxy-Authorization", o.UserToken)
		return r, nil
	})

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
