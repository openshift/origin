package certgraphanalysis

import (
	"strings"

	"github.com/openshift/library-go/pkg/certs/cert-inspection/certgraphapi"
)

// TODO these should all be eliminated in favor of self-describing annotations.

func guessLogicalNamesForCertKeyPairList(in *certgraphapi.CertKeyPairList) {
	for i := range in.Items {
		meaning := guessMeaningForCertKeyPair(in.Items[i])
		in.Items[i].LogicalName = meaning.name
		in.Items[i].Description = meaning.description
	}
}

func newSecretLocation(namespace, name string) certgraphapi.InClusterSecretLocation {
	return certgraphapi.InClusterSecretLocation{
		Namespace: namespace,
		Name:      name,
	}
}

var secretLocationToLogicalName = map[certgraphapi.InClusterSecretLocation]logicalMeaning{
	newSecretLocation("openshift-kube-apiserver", "aggregator-client"):                                newMeaning("aggregator-front-proxy-client", "Client certificate used by the kube-apiserver to communicate to aggregated apiservers."),
	newSecretLocation("openshift-kube-apiserver-operator", "aggregator-client-signer"):                newMeaning("aggregator-front-proxy-signer", "Signer for the kube-apiserver to create client certificates for aggregated apiservers to recognize as a front-proxy."),
	newSecretLocation("openshift-kube-apiserver-operator", "node-system-admin-client"):                newMeaning("per-master-debugging-client", "Client certificate (system:masters) placed on each master to allow communication to kube-apiserver for debugging."),
	newSecretLocation("openshift-kube-apiserver-operator", "node-system-admin-signer"):                newMeaning("per-master-debugging-signer", "Signer for the per-master-debugging-client."),
	newSecretLocation("openshift-kube-apiserver-operator", "kube-control-plane-signer"):               newMeaning("kube-control-plane-signer", "Signer for kube-controller-manager and kube-scheduler client certificates."),
	newSecretLocation("openshift-kube-controller-manager", "kube-controller-manager-client-cert-key"): newMeaning("kube-controller-manager-client", "Client certificate used by the kube-controller-manager to authenticate to the kube-apiserver."),
	newSecretLocation("openshift-kube-apiserver", "check-endpoints-client-cert-key"):                  newMeaning("kube-apiserver-check-endpoints", "Client certificate used by the network connectivity checker of the kube-apiserver."),
	newSecretLocation("openshift-kube-scheduler", "kube-scheduler-client-cert-key"):                   newMeaning("kube-scheduler-client", "Client certificate used by the kube-scheduler to authenticate to the kube-apiserver."),
	newSecretLocation("openshift-kube-apiserver-operator", "kube-apiserver-to-kubelet-signer"):        newMeaning("kube-apiserver-to-kubelet-signer", "Signer for the kube-apiserver-to-kubelet-client so kubelets can recognize the kube-apiserver."),
	newSecretLocation("openshift-kube-apiserver", "kubelet-client"):                                   newMeaning("kube-apiserver-to-kubelet-client", "Client certificate used by the kube-apiserver to authenticate to the kubelet for requests like exec and logs."),
	newSecretLocation("openshift-kube-controller-manager-operator", "csr-signer-signer"):              newMeaning("kube-controller-manager-csr-signer-signer", "Signer used by the kube-controller-manager-operator to sign signing certificates for the CSR API."),
	newSecretLocation("openshift-kube-controller-manager", "csr-signer"):                              newMeaning("kube-controller-manager-csr-signer", "Signer used by the kube-controller-manager to sign CSR API requests."),
	newSecretLocation("openshift-service-ca", "signing-key"):                                          newMeaning("service-serving-signer", "Signer used by service-ca to sign serving certificates for internal service DNS names."),
	newSecretLocation("openshift-kube-apiserver-operator", "loadbalancer-serving-signer"):             newMeaning("kube-apiserver-load-balancer-signer", "Signer used by the kube-apiserver operator to create serving certificates for the kube-apiserver via internal and external load balancers."),
	newSecretLocation("openshift-kube-apiserver", "internal-loadbalancer-serving-certkey"):            newMeaning("kube-apiserver-internal-load-balancer-serving", "Serving certificate used by the kube-apiserver to terminate requests via the internal load balancer."),
	newSecretLocation("openshift-kube-apiserver", "external-loadbalancer-serving-certkey"):            newMeaning("kube-apiserver-external-load-balancer-serving", "Serving certificate used by the kube-apiserver to terminate requests via the external load balancer."),
	newSecretLocation("openshift-kube-apiserver-operator", "localhost-recovery-serving-signer"):       newMeaning("kube-apiserver-recovery-signer", "Signer used by the kube-apiserver to create serving certificates for the kube-apiserver via the localhost recovery SNI ServerName"),
	newSecretLocation("openshift-kube-apiserver", "localhost-recovery-serving-certkey"):               newMeaning("kube-apiserver-recovery-serving", "Serving certificate used by the kube-apiserver to terminate requests via the localhost recovery SNI ServerName."),
	newSecretLocation("openshift-kube-apiserver-operator", "service-network-serving-signer"):          newMeaning("kube-apiserver-service-network-signer", "Signer used by the kube-apiserver to create serving certificates for the kube-apiserver via the service network."),
	newSecretLocation("openshift-kube-apiserver", "service-network-serving-certkey"):                  newMeaning("kube-apiserver-service-network-serving", "Serving certificate used by the kube-apiserver to terminate requests via the service network."),
	newSecretLocation("openshift-kube-apiserver-operator", "localhost-serving-signer"):                newMeaning("kube-apiserver-localhost-signer", "Signer used by the kube-apiserver to create serving certificates for the kube-apiserver via localhost."),
	newSecretLocation("openshift-kube-apiserver", "localhost-serving-cert-certkey"):                   newMeaning("kube-apiserver-localhost-serving", "Serving certificate used by the kube-apiserver to terminate requests via localhost."),
	newSecretLocation("openshift-machine-config-operator", "machine-config-server-tls"):               newMeaning("mco-mystery-cert", "TODO: team needs to make description"),
	newSecretLocation("openshift-config", "etcd-signer"):                                              newMeaning("etcd-signer", "Signer for etcd to create client and serving certificates."),
	newSecretLocation("", ""): newMeaning("", ""),
	newSecretLocation("", ""): newMeaning("", ""),
	newSecretLocation("", ""): newMeaning("", ""),
	newSecretLocation("", ""): newMeaning("", ""),
	newSecretLocation("", ""): newMeaning("", ""),
}

func guessMeaningForCertKeyPair(in certgraphapi.CertKeyPair) logicalMeaning {
	for _, loc := range in.Spec.SecretLocations {
		if meaning, ok := secretLocationToLogicalName[loc]; ok {
			return meaning
		}
	}

	for _, loc := range in.Spec.SecretLocations {
		if loc.Namespace != "openshift-etcd" {
			continue
		}
		if !strings.HasPrefix(loc.Name, "etcd-serving-metrics-") {
			continue
		}
		master := loc.Name[len("etcd-serving-metrics-"):]
		return newMeaning("etcd-metrics-for-master-"+master, "")
	}

	// service serving certs
	if in.Spec.CertMetadata.CertIdentifier.Issuer != nil &&
		strings.HasPrefix(in.Spec.CertMetadata.CertIdentifier.Issuer.CommonName, "openshift-service-serving-signer") {
		return newMeaning(in.Spec.CertMetadata.CertIdentifier.CommonName, "")
	}

	return newMeaning("", "")
}

func guessLogicalNamesForCABundleList(in *certgraphapi.CertificateAuthorityBundleList) {
	for i := range in.Items {
		meaning := guessMeaningForCABundle(in.Items[i])
		in.Items[i].LogicalName = meaning.name
		in.Items[i].Description = meaning.description
	}
}

func newConfigMapLocation(namespace, name string) certgraphapi.InClusterConfigMapLocation {
	return certgraphapi.InClusterConfigMapLocation{
		Namespace: namespace,
		Name:      name,
	}
}

type logicalMeaning struct {
	name        string
	description string
}

func newMeaning(name, description string) logicalMeaning {
	return logicalMeaning{
		name:        name,
		description: description,
	}
}

var configmapLocationToLogicalName = map[certgraphapi.InClusterConfigMapLocation]logicalMeaning{
	newConfigMapLocation("openshift-config-managed", "kube-apiserver-aggregator-client-ca"):          newMeaning("aggregator-front-proxy-ca", "CA for aggregated apiservers to recognize kube-apiserver as front-proxy."),
	newConfigMapLocation("openshift-kube-apiserver-operator", "node-system-admin-ca"):                newMeaning("kube-apiserver-per-master-debugging-client-ca", "CA for kube-apiserver to recognize local system:masters rendered to each master."),
	newConfigMapLocation("openshift-config-managed", "kube-apiserver-client-ca"):                     newMeaning("kube-apiserver-total-client-ca", "CA for kube-apiserver to recognize all known certificate based clients."),
	newConfigMapLocation("openshift-kube-apiserver-operator", "kube-control-plane-signer-ca"):        newMeaning("kube-apiserver-kcm-and-ks-client-ca", "CA for kube-apiserver to recognize the kube-controller-manager and kube-scheduler client certificates."),
	newConfigMapLocation("openshift-config", "initial-kube-apiserver-server-ca"):                     newMeaning("kube-apiserver-from-installer-client-ca", "CA for the kube-apiserver to recognize clients created by the installer."),
	newConfigMapLocation("openshift-kube-apiserver-operator", "kube-apiserver-to-kubelet-client-ca"): newMeaning("kubelet-to-recognize-kube-apiserver-client-ca", "CA for the kubelet to recognize the kube-apiserver client certificate."),
	newConfigMapLocation("openshift-kube-controller-manager-operator", "csr-controller-signer-ca"):   newMeaning("kube-controller-manager-csr-signer-signer-ca", "CA to recognize the kube-controller-manager's signer for signing new CSR signing certificates."),
	newConfigMapLocation("openshift-config-managed", "csr-controller-ca"):                            newMeaning("kube-controller-manager-csr-ca", "CA to recognize the CSRs (both serving and client) signed by the kube-controller-manager."),
	newConfigMapLocation("openshift-config", "etcd-ca-bundle"):                                       newMeaning("etcd-ca", "CA for recognizing etcd serving, peer, and client certificates."),
	newConfigMapLocation("openshift-config-managed", "service-ca"):                                   newMeaning("service-ca", "CA for recognizing serving certificates for services that were signed by our service-ca controller."),
	newConfigMapLocation("openshift-kube-controller-manager", "serviceaccount-ca"):                   newMeaning("service-account-token-ca.crt", "CA for recognizing kube-apiserver.  This is injected into each service account token secret at ca.crt."),
	newConfigMapLocation("openshift-config-managed", "default-ingress-cert"):                         newMeaning("router-wildcard-serving-ca", "REVIEW: CA for recognizing the default router wildcard serving certificate."),
	newConfigMapLocation("openshift-kube-apiserver-operator", "localhost-recovery-serving-ca"):       newMeaning("kube-apiserver-recovery-serving-ca", "CA for recognizing the kube-apiserver when connecting via the localhost recovery SNI ServerName."),
	newConfigMapLocation("openshift-kube-apiserver-operator", "service-network-serving-ca"):          newMeaning("kube-apiserver-service-network-serving-ca", "CA for recognizing the kube-apiserver when connecting via the service network (kuberentes.default.svc)."),
	newConfigMapLocation("openshift-kube-apiserver-operator", "localhost-serving-ca"):                newMeaning("kube-apiserver-localhost-serving-ca", "CA for recognizing the kube-apiserver when connecting via localhost."),
	newConfigMapLocation("openshift-kube-apiserver-operator", "loadbalancer-serving-ca"):             newMeaning("kube-apiserver-load-balancer-serving-ca", "CA for recognizing the kube-apiserver when connecting via the internal or external load balancers."),
	newConfigMapLocation("openshift-config-managed", "kube-apiserver-server-ca"):                     newMeaning("kube-apiserver-total-serving-ca", "CA for recognizing the kube-apiserver when connecting via any means."),
	newConfigMapLocation("openshift-config", "admin-kubeconfig-client-ca"):                           newMeaning("kube-apiserver-admin-kubeconfig-client-ca", "CA for kube-apiserver to recognize the system:master created by the installer."),
	newConfigMapLocation("openshift-etcd", "etcd-metrics-proxy-client-ca"):                           newMeaning("etcd-metrics-ca", "CA used to recognize etcd metrics serving and client certificates."), // 4.8 version
	newConfigMapLocation("openshift-config", "etcd-metric-serving-ca"):                               newMeaning("etcd-metrics-ca", "CA used to recognize etcd metrics serving and client certificates."), // 4.7 version
	newConfigMapLocation("openshift-config-managed", "trusted-ca-bundle"):                            newMeaning("proxy-ca", "CA used to recognize proxy servers.  By default this will contain standard root CAs on the cluster-network-operator pod."),
	newConfigMapLocation("", ""): newMeaning("", ""),
}

func guessMeaningForCABundle(in certgraphapi.CertificateAuthorityBundle) logicalMeaning {
	for _, loc := range in.Spec.ConfigMapLocations {
		if meaning, ok := configmapLocationToLogicalName[loc]; ok {
			return meaning
		}
	}
	return logicalMeaning{}
}
