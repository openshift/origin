package certdocs

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/gonum/graph"
	"github.com/gonum/graph/topo"
	"github.com/openshift/library-go/pkg/certs/cert-inspection/certgraphapi"
	"github.com/openshift/origin/pkg/cmd/update-tls-artifacts/generate-documentation/certgraph"
	"k8s.io/apimachinery/pkg/util/errors"
)

type ClusterCerts struct {
	inClusterPKI certgraphapi.PKIList
	onDiskPKI    certgraphapi.PKIList
	combinedPKI  certgraphapi.PKIList

	// disjointPKIs is a list of non-intersecting PKILists
	disjointPKIs []certgraphapi.PKIList
}

func WriteDoc(pkiList *certgraphapi.PKIList, outputDir string) error {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}
	graphDOT, err := certgraph.DOTForPKIList(pkiList)
	if err != nil {
		return err
	}
	dotFile := filepath.Join(outputDir, "cert-flow.dot")
	if err := ioutil.WriteFile(dotFile, []byte(graphDOT), 0644); err != nil {
		return err
	}

	svg := exec.Command("dot", "-Kdot", "-T", "svg", "-o", filepath.Join(outputDir, "cert-flow.svg"), dotFile)
	//svg.Stderr = os.Stderr
	if err := svg.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "%v", err)
	}
	png := exec.Command("dot", "-Kdot", "-T", "png", "-o", filepath.Join(outputDir, "cert-flow.png"), dotFile)
	//png.Stderr = os.Stderr
	if err := png.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "%v", err)
	}

	jsonBytes, err := json.MarshalIndent(pkiList, "", "  ")
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(filepath.Join(outputDir, "cert-flow.json"), jsonBytes, 0644); err != nil {
		return err
	}

	graph, err := certgraph.GraphForPKIList(pkiList)
	if err != nil {
		return err
	}
	title := pkiList.LogicalName
	if len(title) == 0 {
		title = outputDir
	}
	markdown, err := GetMarkdownForPKILIst(title, pkiList.Description, outputDir, graph)
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(filepath.Join(outputDir, "README.md"), []byte(markdown), 0644); err != nil {
		return err
	}

	return nil
}

func WriteDocs(pkiList *certgraphapi.PKIList, outputDir string) error {
	disjointPKILists, err := SeparateDisjointPKI(pkiList)
	if err != nil {
		return err
	}

	errs := []error{}
	for i, currPKI := range disjointPKILists {
		filename := fmt.Sprintf("%d", i)
		if len(currPKI.LogicalName) > 0 {
			filename = currPKI.LogicalName
		}

		err := WriteDoc(currPKI, filepath.Join(outputDir, filename))
		if err != nil {
			errs = append(errs, err)
		}
	}

	readme := GetMarkdownForSummary(disjointPKILists)
	if err := ioutil.WriteFile(filepath.Join(outputDir, "README.md"), []byte(readme), 0644); err != nil {
		return err
	}

	return errors.NewAggregate(errs)
}

func SeparateDisjointPKI(pkiList *certgraphapi.PKIList) ([]*certgraphapi.PKIList, error) {
	pkiGraph, err := certgraph.GraphForPKIList(pkiList)
	if err != nil {
		return nil, err
	}
	lists := []*certgraphapi.PKIList{}
	subgraphNodes := topo.ConnectedComponents(graph.Undirect{G: pkiGraph})

	for i := range subgraphNodes {
		curr := &certgraphapi.PKIList{}
		for j := range subgraphNodes[i] {
			currNode := subgraphNodes[i][j]
			if item := currNode.(graphNode).GetCABundle(); item != nil {
				curr.CertificateAuthorityBundles.Items = append(curr.CertificateAuthorityBundles.Items, *item)
			}
			if item := currNode.(graphNode).GetCertKeyPair(); item != nil {
				curr.CertKeyPairs.Items = append(curr.CertKeyPairs.Items, *item)
			}
		}
		curr.LogicalName = guessLogicalNamesForPKIList(*curr)
		curr.Description = guessLogicalDescriptionsForPKIList(*curr)
		lists = append(lists, curr)
	}

	return lists, nil
}

type graphNode interface {
	GetCABundle() *certgraphapi.CertificateAuthorityBundle
	GetCertKeyPair() *certgraphapi.CertKeyPair
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

// TODO these are for identifying different disjoint graphs of PKI artifacts. Selecting this based on location makes sense
// TODO but we want to be able to do things like, "make sure these don't intersect unexpectedly", "make sure we have them all", "make sure we don't have extras"
var (
	signerSecretToMeaning = map[certgraphapi.InClusterSecretLocation]logicalMeaning{
		{Namespace: "openshift-machine-config-operator", Name: "machine-config-server-tls"}:     newMeaning("MachineConfig Operator Certificates", "TODO need to work out who and what."),
		{Namespace: "openshift-operator-lifecycle-manager", Name: "pprof-cert"}:                 newMeaning("Unknown OLM pprof", "To be filled in by OLM"),
		{Namespace: "openshift-operator-lifecycle-manager", Name: "packageserver-service-cert"}: newMeaning("Unknown OLM package server", "To be filled in by OLM"),
	}

	caBundleConfigMapToMeaning = map[certgraphapi.InClusterConfigMapLocation]logicalMeaning{
		{Namespace: "openshift-config-managed", Name: "kube-apiserver-aggregator-client-ca"}: newMeaning("Aggregated API Server Certificates", "Used to secure connections between the kube-apiserver and aggregated API Servers."),
		{Namespace: "openshift-config-managed", Name: "kube-apiserver-client-ca"}:            newMeaning("kube-apiserver Client Certificates", "Used by the kube-apiserver to recognize clients using mTLS."),
		{Namespace: "openshift-etcd", Name: "etcd-metrics-proxy-serving-ca"}:                 newMeaning("etcd Metrics Certificates", "Used to access etcd metrics using mTLS."),
		{Namespace: "openshift-etcd", Name: "etcd-serving-ca"}:                               newMeaning("etcd Certificates", "Used to secure etcd internal communication and by apiservers to access etcd."), // 4.8 version
		{Namespace: "openshift-config-managed", Name: "service-ca"}:                          newMeaning("Service Serving Certificates", "Used to secure inter-service communication on the local cluster."),
		{Namespace: "openshift-config-managed", Name: "kube-apiserver-server-ca"}:            newMeaning("kube-apiserver Serving Certificates", "Used by kube-apiserver clients to recognize the kube-apiserver."),
		{Namespace: "openshift-config-managed", Name: "trusted-ca-bundle"}:                   newMeaning("Proxy Certificates", "Used by the OpenShift platform to recognize the proxy.  Other usages are side-effects which work by accident and not by principled design."),
		{Namespace: "openshift-ovn-kubernetes", Name: "ovn-ca"}:                              newMeaning("Unknown OVN 01", "To be filled in by Networking"),
		{Namespace: "openshift-ovn-kubernetes", Name: "signer-ca"}:                           newMeaning("Unknown OVN 02", "To be filled in by Networking"),
		{Namespace: "openshift-network-node-identity", Name: "network-node-identity-ca"}:     newMeaning("Unknown Networking 02", "To be filled in by Networking"),
	}
)

// we now key based on off location instead of the logicalName since remove logicalName.  We can eventually use description for the description,
// but we'll need the short name to be something.
func guessLogicalNamesForPKIList(in certgraphapi.PKIList) string {
	for _, curr := range in.CertKeyPairs.Items {
		for _, location := range curr.Spec.SecretLocations {
			if meaning, ok := signerSecretToMeaning[location]; ok {
				return meaning.name
			}
		}
	}

	for _, curr := range in.CertificateAuthorityBundles.Items {
		for _, location := range curr.Spec.ConfigMapLocations {
			if meaning, ok := caBundleConfigMapToMeaning[location]; ok {
				return meaning.name
			}
		}
	}

	return ""
}

func guessLogicalDescriptionsForPKIList(in certgraphapi.PKIList) string {
	for _, curr := range in.CertKeyPairs.Items {
		for _, location := range curr.Spec.SecretLocations {
			if meaning, ok := signerSecretToMeaning[location]; ok {
				return meaning.name
			}
		}
	}

	for _, curr := range in.CertificateAuthorityBundles.Items {
		for _, location := range curr.Spec.ConfigMapLocations {
			if meaning, ok := caBundleConfigMapToMeaning[location]; ok {
				return meaning.name
			}
		}
	}

	return ""
}
