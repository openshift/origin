package aggregated_logging

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	kapi "k8s.io/kubernetes/pkg/api"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/util/sets"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/diagnostics/types"
	oauthapi "github.com/openshift/origin/pkg/oauth/api"
)

const (
	kibanaProxyOauthClientName = "kibana-proxy"
	kibanaProxySecretName      = "logging-kibana-proxy"
	oauthSecretKeyName         = "oauth-secret"
)

//checkKibana verifies the various integration points between Kibana and logging
func checkKibana(r types.DiagnosticResult, osClient *client.Client, kClient *kclient.Client, project string) {
	oauthclient, err := osClient.OAuthClients().Get(kibanaProxyOauthClientName)
	if err != nil {
		r.Error("AGL0115", err, fmt.Sprintf("Error retrieving the OauthClient '%s': %s. Unable to check Kibana", kibanaProxyOauthClientName, err))
		return
	}
	checkKibanaSecret(r, osClient, kClient, project, oauthclient)
	checkKibanaRoutesInOauthClient(r, osClient, project, oauthclient)
}

//checkKibanaSecret confirms the secret used by kibana matches that configured in the oauth client
func checkKibanaSecret(r types.DiagnosticResult, osClient *client.Client, kClient *kclient.Client, project string, oauthclient *oauthapi.OAuthClient) {
	r.Debug("AGL0100", "Checking oauthclient secrets...")
	secret, err := kClient.Secrets(project).Get(kibanaProxySecretName)
	if err != nil {
		r.Error("AGL0105", err, fmt.Sprintf("Error retrieving the secret '%s': %s", kibanaProxySecretName, err))
		return
	}
	decoded, err := decodeSecret(secret, oauthSecretKeyName)
	if err != nil {
		r.Error("AGL0110", err, fmt.Sprintf("Unable to decode Kibana Secret: %s", err))
		return
	}
	if decoded != oauthclient.Secret {
		r.Debug("AGL0120", fmt.Sprintf("OauthClient Secret:    '%s'", oauthclient.Secret))
		r.Debug("AGL0125", fmt.Sprintf("Decoded Kibana Secret: '%s'", decoded))
		message := fmt.Sprintf("The %s OauthClient.Secret does not match the decoded oauth secret in '%s'", kibanaProxyOauthClientName, kibanaProxySecretName)
		r.Error("AGL0130", errors.New(message), message)
	}
}

//checkKibanaRoutesInOauthClient verifies the client contains the correct redirect uris
func checkKibanaRoutesInOauthClient(r types.DiagnosticResult, osClient *client.Client, project string, oauthclient *oauthapi.OAuthClient) {
	r.Debug("AGL0141", "Checking oauthclient redirectURIs for the logging routes...")
	routeList, err := osClient.Routes(project).List(kapi.ListOptions{LabelSelector: loggingSelector.AsSelector()})
	if err != nil {
		r.Error("AGL0143", err, fmt.Sprintf("Error retrieving the logging routes: %s", err))
		return
	}
	redirectUris, err := parseRedirectUris(oauthclient.RedirectURIs)
	if err != nil {
		r.Error("AGL0145", err, "Error parsing the OAuthClient.RedirectURIs")
		return
	}
	for _, route := range routeList.Items {
		if !redirectUris.Has(route.Spec.Host) {
			message := fmt.Sprintf("OauthClient '%s' does not include a redirectURI for route '%s' which is '%s'", oauthclient.ObjectMeta.Name, route.ObjectMeta.Name, route.Spec.Host)
			r.Error("AGL0147", errors.New(message), message)
		}
	}

	return
}

func parseRedirectUris(uris []string) (sets.String, error) {
	urls := sets.String{}
	for _, uri := range uris {
		url, err := url.Parse(uri)
		if err != nil {
			return urls, err
		}
		urls.Insert(url.Host)
	}
	return urls, nil
}

// decodeSecret decodes a base64 encoded entry in a secret and returns the value as decoded string
func decodeSecret(secret *kapi.Secret, key string) (string, error) {
	value, ok := secret.Data[key]
	if !ok {
		return "", errors.New(fmt.Sprintf("The %s secret did not have a data entry for %s", secret.ObjectMeta.Name, key))
	}
	return strings.TrimSpace(string(value)), nil
}
