package config

import (
	"reflect"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	restclient "k8s.io/client-go/rest"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/openshift/origin/pkg/client/config"
	userclient "github.com/openshift/origin/pkg/user/generated/internalclientset"
)

// getClusterNicknameFromConfig returns host:port of the clientConfig.Host, with .'s replaced by -'s
func getClusterNicknameFromConfig(clientCfg *restclient.Config) (string, error) {
	return config.GetClusterNicknameFromURL(clientCfg.Host)
}

// getUserNicknameFromConfig returns "username(as known by the server)/getClusterNicknameFromConfig".  This allows tab completion for switching users to
// work easily and obviously.
func getUserNicknameFromConfig(clientCfg *restclient.Config) (string, error) {
	userPartOfNick, err := getUserPartOfNickname(clientCfg)
	if err != nil {
		return "", err
	}

	clusterNick, err := getClusterNicknameFromConfig(clientCfg)
	if err != nil {
		return "", err
	}

	return userPartOfNick + "/" + clusterNick, nil
}

func getUserPartOfNickname(clientCfg *restclient.Config) (string, error) {
	userClient, err := userclient.NewForConfig(clientCfg)
	if err != nil {
		return "", err
	}
	userInfo, err := userClient.User().Users().Get("~", metav1.GetOptions{})
	if kerrors.IsNotFound(err) || kerrors.IsForbidden(err) {
		// if we're talking to kube (or likely talking to kube), take a best guess consistent with login
		switch {
		case len(clientCfg.BearerToken) > 0:
			userInfo.Name = clientCfg.BearerToken
		case len(clientCfg.Username) > 0:
			userInfo.Name = clientCfg.Username
		}
	} else if err != nil {
		return "", err
	}

	return userInfo.Name, nil
}

// getContextNicknameFromConfig returns "namespace/getClusterNicknameFromConfig/username(as known by the server)".  This allows tab completion for switching projects/context
// to work easily.  First tab is the most selective on project.  Second stanza in the next most selective on cluster name.  The chances of a user trying having
// one projects on a single server that they want to operate against with two identities is low, so username is last.
func getContextNicknameFromConfig(namespace string, clientCfg *restclient.Config) (string, error) {
	userPartOfNick, err := getUserPartOfNickname(clientCfg)
	if err != nil {
		return "", err
	}

	clusterNick, err := getClusterNicknameFromConfig(clientCfg)
	if err != nil {
		return "", err
	}

	return namespace + "/" + clusterNick + "/" + userPartOfNick, nil
}

// CreateConfig takes a clientCfg and builds a config (kubeconfig style) from it.
func CreateConfig(namespace string, clientCfg *restclient.Config) (*clientcmdapi.Config, error) {
	clusterNick, err := getClusterNicknameFromConfig(clientCfg)
	if err != nil {
		return nil, err
	}

	userNick, err := getUserNicknameFromConfig(clientCfg)
	if err != nil {
		return nil, err
	}

	contextNick, err := getContextNicknameFromConfig(namespace, clientCfg)
	if err != nil {
		return nil, err
	}

	config := clientcmdapi.NewConfig()

	credentials := clientcmdapi.NewAuthInfo()
	credentials.Token = clientCfg.BearerToken
	credentials.ClientCertificate = clientCfg.TLSClientConfig.CertFile
	if len(credentials.ClientCertificate) == 0 {
		credentials.ClientCertificateData = clientCfg.TLSClientConfig.CertData
	}
	credentials.ClientKey = clientCfg.TLSClientConfig.KeyFile
	if len(credentials.ClientKey) == 0 {
		credentials.ClientKeyData = clientCfg.TLSClientConfig.KeyData
	}
	config.AuthInfos[userNick] = credentials

	cluster := clientcmdapi.NewCluster()
	cluster.Server = clientCfg.Host
	cluster.CertificateAuthority = clientCfg.CAFile
	if len(cluster.CertificateAuthority) == 0 {
		cluster.CertificateAuthorityData = clientCfg.CAData
	}
	cluster.InsecureSkipTLSVerify = clientCfg.Insecure
	config.Clusters[clusterNick] = cluster

	context := clientcmdapi.NewContext()
	context.Cluster = clusterNick
	context.AuthInfo = userNick
	context.Namespace = namespace
	config.Contexts[contextNick] = context
	config.CurrentContext = contextNick

	return config, nil
}

// MergeConfig adds the additional Config stanzas to the startingConfig.  It blindly stomps clusters and users, but
// it searches for a matching context before writing a new one.
func MergeConfig(startingConfig, addition clientcmdapi.Config) (*clientcmdapi.Config, error) {
	ret := startingConfig

	for requestedKey, value := range addition.Clusters {
		ret.Clusters[requestedKey] = value
	}

	for requestedKey, value := range addition.AuthInfos {
		ret.AuthInfos[requestedKey] = value
	}

	requestedContextNamesToActualContextNames := map[string]string{}
	for requestedKey, newContext := range addition.Contexts {
		actualContext := clientcmdapi.NewContext()
		actualContext.AuthInfo = newContext.AuthInfo
		actualContext.Cluster = newContext.Cluster
		actualContext.Namespace = newContext.Namespace
		actualContext.Extensions = newContext.Extensions

		if existingName := findExistingContextName(startingConfig, *actualContext); len(existingName) > 0 {
			// if this already exists, just move to the next, our job is done
			requestedContextNamesToActualContextNames[requestedKey] = existingName
			continue
		}

		requestedContextNamesToActualContextNames[requestedKey] = requestedKey
		ret.Contexts[requestedKey] = actualContext
	}

	if len(addition.CurrentContext) > 0 {
		if newCurrentContext, exists := requestedContextNamesToActualContextNames[addition.CurrentContext]; exists {
			ret.CurrentContext = newCurrentContext
		} else {
			ret.CurrentContext = addition.CurrentContext
		}
	}

	return &ret, nil
}

// findExistingContextName finds the nickname for the passed context
func findExistingContextName(haystack clientcmdapi.Config, needle clientcmdapi.Context) string {
	for key, context := range haystack.Contexts {
		context.LocationOfOrigin = ""
		if reflect.DeepEqual(context, needle) {
			return key
		}
	}

	return ""
}
