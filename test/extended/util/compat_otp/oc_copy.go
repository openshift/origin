package compat_otp

import (
	"context"
	"net/url"
	"strings"

	kauthnv1 "k8s.io/api/authentication/v1"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/third_party/forked/golang/netutil"

	kauthnv1typedclient "k8s.io/client-go/kubernetes/typed/authentication/v1"
	restclient "k8s.io/client-go/rest"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// getClusterNicknameFromConfig returns host:port of the clientConfig.Host, with .'s replaced by -'s
// TODO this is copied from pkg/client/config/smart_merge.go, looks like a good library-go candidate
func getClusterNicknameFromConfig(clientCfg *restclient.Config) (string, error) {
	u, err := url.Parse(clientCfg.Host)
	if err != nil {
		return "", err
	}
	hostPort := netutil.CanonicalAddr(u)

	// we need a character other than "." to avoid conflicts with.  replace with '-'
	return strings.Replace(hostPort, ".", "-", -1), nil
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
	authClient, err := kauthnv1typedclient.NewForConfig(clientCfg)
	if err != nil {
		return "", err
	}

	var username string
	selfSubjectReview, err := authClient.SelfSubjectReviews().Create(context.Background(), &kauthnv1.SelfSubjectReview{}, metav1.CreateOptions{})
	if kerrors.IsNotFound(err) || kerrors.IsForbidden(err) {
		// if we're talking to kube (or likely talking to kube), take a best guess consistent with login
		switch {
		case len(clientCfg.BearerToken) > 0:
			username = clientCfg.BearerToken
		case len(clientCfg.Username) > 0:
			username = clientCfg.Username
		}
	} else if err != nil {
		return "", err
	} else {
		username = selfSubjectReview.Status.UserInfo.Username
	}

	return username, nil
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
func createConfig(namespace string, clientCfg *restclient.Config) (*clientcmdapi.Config, error) {
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
	credentials.Exec = clientCfg.ExecProvider
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
