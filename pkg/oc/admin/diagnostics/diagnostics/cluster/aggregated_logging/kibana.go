package aggregated_logging

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"

	oauthapi "github.com/openshift/origin/pkg/oauth/apis/oauth"
	oauthtypedclient "github.com/openshift/origin/pkg/oauth/generated/internalclientset/typed/oauth/internalversion"
	routetypedclient "github.com/openshift/origin/pkg/route/generated/internalclientset/typed/route/internalversion"
)

const (
	kibanaProxyOauthClientName = "kibana-proxy"
	kibanaProxySecretName      = "logging-kibana-proxy"
	oauthSecretKeyName         = "oauth-secret"
)

//checkKibana verifies the various integration points between Kibana and logging
func checkKibana(r diagnosticReporter, routeClient routetypedclient.RoutesGetter, oauthClientClient oauthtypedclient.OAuthClientsGetter, kClient kclientset.Interface, project string) {
	oauthclient, err := oauthClientClient.OAuthClients().Get(kibanaProxyOauthClientName, metav1.GetOptions{})
	if err != nil {
		r.Error("AGL0115", err, fmt.Sprintf("Error retrieving the OauthClient '%s': %s. Unable to check Kibana", kibanaProxyOauthClientName, err))
		return
	}
	checkKibanaSecret(r, kClient, project, oauthclient)
	checkKibanaRoutesInOauthClient(r, routeClient, project, oauthclient)
}

//checkKibanaSecret confirms the secret used by kibana matches that configured in the oauth client
func checkKibanaSecret(r diagnosticReporter, kClient kclientset.Interface, project string, oauthclient *oauthapi.OAuthClient) {
	r.Debug("AGL0100", "Checking oauthclient secrets...")
	secret, err := kClient.Core().Secrets(project).Get(kibanaProxySecretName, metav1.GetOptions{})
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
func checkKibanaRoutesInOauthClient(r diagnosticReporter, routeClient routetypedclient.RoutesGetter, project string, oauthclient *oauthapi.OAuthClient) {
	r.Debug("AGL0141", "Checking oauthclient redirectURIs for the logging routes...")
	routeList, err := routeClient.Routes(project).List(metav1.ListOptions{LabelSelector: loggingSelector.AsSelector().String()})
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
