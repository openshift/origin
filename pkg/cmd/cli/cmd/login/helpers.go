package login

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/user/api"
	"k8s.io/kubernetes/pkg/client/restclient"
	kclientcmdapi "k8s.io/kubernetes/pkg/client/unversioned/clientcmd/api"

	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	clientcmdapi "k8s.io/kubernetes/pkg/client/unversioned/clientcmd/api"
	"k8s.io/kubernetes/pkg/util/sets"
	"k8s.io/kubernetes/pkg/util/term"
)

// getMatchingClusters examines the kubeconfig for all clusters that point to the same server
func getMatchingClusters(clientConfig restclient.Config, kubeconfig clientcmdapi.Config) sets.String {
	ret := sets.String{}

	for key, cluster := range kubeconfig.Clusters {
		if (cluster.Server == clientConfig.Host) && (cluster.InsecureSkipTLSVerify == clientConfig.Insecure) && (cluster.CertificateAuthority == clientConfig.CAFile) && (bytes.Compare(cluster.CertificateAuthorityData, clientConfig.CAData) == 0) {
			ret.Insert(key)
		}
	}

	return ret
}

// findExistingClientCA returns *either* the existing client CA file name as a string,
// *or* data in a []byte for a given host, and true if it exists in the given config
func findExistingClientCA(host string, kubeconfig clientcmdapi.Config) (string, []byte, bool) {
	for _, cluster := range kubeconfig.Clusters {
		if cluster.Server == host {
			if len(cluster.CertificateAuthority) > 0 {
				return cluster.CertificateAuthority, nil, true
			}
			if len(cluster.CertificateAuthorityData) > 0 {
				return "", cluster.CertificateAuthorityData, true
			}
		}
	}
	return "", nil, false
}

// dialToServer takes the Server URL from the given clientConfig and dials to
// make sure the server is reachable. Note the config received is not mutated.
func dialToServer(clientConfig restclient.Config) error {
	// take a RoundTripper based on the config we already have (TLS, proxies, etc)
	rt, err := restclient.TransportFor(&clientConfig)
	if err != nil {
		return err
	}

	parsedURL, err := url.Parse(clientConfig.Host)
	if err != nil {
		return err
	}

	// Do a HEAD request to serverPathToDial to make sure the server is alive.
	// We don't care about the response, any err != nil is valid for the sake of reachability.
	serverURLToDial := (&url.URL{Scheme: parsedURL.Scheme, Host: parsedURL.Host, Path: "/"}).String()
	req, err := http.NewRequest("HEAD", serverURLToDial, nil)
	if err != nil {
		return err
	}

	res, err := rt.RoundTrip(req)
	if err != nil {
		return err
	}

	defer res.Body.Close()
	return nil
}

func promptForInsecureTLS(reader io.Reader, out io.Writer) bool {
	var input bool
	if term.IsTerminal(reader) {
		fmt.Fprintln(out, "The server uses a certificate signed by an unknown authority.")
		fmt.Fprintln(out, "You can bypass the certificate check, but any data you send to the server could be intercepted by others.")
		input = cmdutil.PromptForBool(os.Stdin, out, "Use insecure connections? (y/n): ")
		fmt.Fprintln(out)
	}
	return input
}

func hasExistingInsecureCluster(clientConfigToTest restclient.Config, kubeconfig kclientcmdapi.Config) bool {
	clientConfigToTest.Insecure = true
	matchingClusters := getMatchingClusters(clientConfigToTest, kubeconfig)
	return len(matchingClusters) > 0
}

// getHostPort returns the host and port parts of the given URL string. It's
// expected that the provided URL is already normalized (always has host and port).
func getHostPort(hostURL string) (string, string, *url.URL, error) {
	parsedURL, err := url.Parse(hostURL)
	if err != nil {
		return "", "", nil, err
	}
	host, port, err := net.SplitHostPort(parsedURL.Host)
	return host, port, parsedURL, err
}

func whoAmI(client *client.Client) (*api.User, error) {
	me, err := client.Users().Get("~")
	if err != nil {
		return nil, err
	}

	return me, nil
}
