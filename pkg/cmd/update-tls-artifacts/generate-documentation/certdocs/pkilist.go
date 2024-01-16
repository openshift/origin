package certdocs

import (
	"fmt"
	"sort"
	"strings"

	"github.com/gonum/graph"
	"github.com/gonum/graph/topo"
	"github.com/openshift/library-go/pkg/certs/cert-inspection/certgraphapi"
	"github.com/openshift/origin/pkg/cmd/update-tls-artifacts/generate-documentation/certgraph"
	"k8s.io/apimachinery/pkg/util/errors"
)

func GetMarkdownForPKILIst(title, description, outputDir string, pkiGraph graph.Directed) (string, error) {
	sortedNodes, err := topo.SortStabilized(
		pkiGraph, func(nodes []graph.Node) {
			sort.Stable(certgraph.CertGraphNodeByName(nodes))
		},
	)
	if err != nil {
		return "", err
	}

	ret := fmt.Sprintf("# %v\n\n", title)
	ret += description + "\n\n"
	ret += `![PKI Graph](cert-flow.png)` + "\n\n"
	ret += `TABLE_OF_CONTENTS` + "\n\n"

	errs := []error{}
	tocLines := []string{}
	commonNameToLink := map[string]string{}
	// signing certificates
	ret += fmt.Sprintf("## %v\n\n", "Signing Certificate/Key Pairs")
	tocLines = append(tocLines, "- "+getSimpleLink("Signing Certificate/Key Pairs"))
	for _, currNode := range sortedNodes {
		certNode := currNode.(certgraph.CertGraphNode)
		if certNode.GetCertKeyPair() != nil && certNode.GetCertKeyPair().Spec.Details.SignerDetails != nil {
			md, toc, err := GetMarkdownForCertKeyPair("###", pkiGraph, certNode.GetCertKeyPair(), commonNameToLink, outputDir)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			tocLines = append(tocLines, toc...)
			ret += md + "\n\n"
		}
	}
	// serving certificates
	ret += fmt.Sprintf("## %v\n\n", "Serving Certificate/Key Pairs")
	tocLines = append(tocLines, "- "+getSimpleLink("Serving Certificate/Key Pairs"))
	for _, currNode := range sortedNodes {
		certNode := currNode.(certgraph.CertGraphNode)
		if certNode.GetCertKeyPair() != nil && certNode.GetCertKeyPair().Spec.Details.ServingCertDetails != nil {
			md, toc, err := GetMarkdownForCertKeyPair("###", pkiGraph, certNode.GetCertKeyPair(), commonNameToLink, outputDir)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			tocLines = append(tocLines, toc...)
			ret += md + "\n\n"
		}
	}
	// client certificates
	ret += fmt.Sprintf("## %v\n\n", "Client Certificate/Key Pairs")
	tocLines = append(tocLines, "- "+getSimpleLink("Client Certificate/Key Pairs"))
	for _, currNode := range sortedNodes {
		certNode := currNode.(certgraph.CertGraphNode)
		if certNode.GetCertKeyPair() != nil && certNode.GetCertKeyPair().Spec.Details.ClientCertDetails != nil {
			md, toc, err := GetMarkdownForCertKeyPair("###", pkiGraph, certNode.GetCertKeyPair(), commonNameToLink, outputDir)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			tocLines = append(tocLines, toc...)
			ret += md + "\n\n"
		}
	}
	// client certificates
	ret += fmt.Sprintf("## %v\n\n", "Certificates Without Keys")
	ret += `These certificates are present in certificate authority bundles, but do not have keys in the cluster.
This happens when the installer bootstrap clusters with a set of certificate/key pairs that are deleted during the
installation process.` + "\n\n"
	tocLines = append(tocLines, "- "+getSimpleLink("Certificates Without Keys"))
	for _, currNode := range sortedNodes {
		certNode := currNode.(certgraph.CertGraphNode)
		if certNode.GetCertKeyPair() != nil &&
			certNode.GetCertKeyPair().Spec.Details.SignerDetails == nil &&
			certNode.GetCertKeyPair().Spec.Details.ServingCertDetails == nil &&
			certNode.GetCertKeyPair().Spec.Details.ClientCertDetails == nil {
			md, toc, err := GetMarkdownForCertKeyPair("###", pkiGraph, certNode.GetCertKeyPair(), commonNameToLink, outputDir)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			tocLines = append(tocLines, toc...)
			ret += md + "\n\n"
		}
	}
	// CA bundles
	ret += fmt.Sprintf("## %v\n\n", "Certificate Authority Bundles")
	tocLines = append(tocLines, "- "+getSimpleLink("Certificate Authority Bundles"))
	for _, currNode := range sortedNodes {
		certNode := currNode.(certgraph.CertGraphNode)
		if certNode.GetCABundle() != nil {
			md, toc, err := GetMarkdownForCABundle("###", pkiGraph, certNode.GetCABundle(), commonNameToLink, outputDir)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			tocLines = append(tocLines, toc...)
			ret += md + "\n\n"
		}
	}

	toc := strings.Join(tocLines, "\n")
	ret = strings.Replace(ret, "TABLE_OF_CONTENTS", toc, 1)

	return ret, errors.NewAggregate(errs)
}

func getSimpleLink(unescapedAnchor string) string {
	return getLink(unescapedAnchor, unescapedAnchor)
}

func getLink(linkText, unescapedAnchor string) string {
	return fmt.Sprintf("[%v](#%v)", linkText,
		strings.ToLower(
			strings.ReplaceAll(
				strings.ReplaceAll(
					strings.ReplaceAll(
						strings.ReplaceAll(
							unescapedAnchor,
							" ", "-"),
						"@", ""),
					":", ""),
				"/", ""),
		),
	)
}

func makeSafeString(in string) string {
	return fmt.Sprintf("%v",
		strings.ToLower(
			strings.ReplaceAll(
				strings.ReplaceAll(
					strings.ReplaceAll(
						strings.ReplaceAll(
							strings.ReplaceAll(
								in,
								" ", "-"),
							"@", ""),
						":", ""),
					"/", ""),
				"|", ""),
		),
	)
}

func markdownCertType(certKeyPair *certgraphapi.CertKeyPair) string {
	certTypes := []string{}
	if certKeyPair.Spec.Details.SignerDetails != nil {
		certTypes = append(certTypes, "Signer")
	}
	if certKeyPair.Spec.Details.ServingCertDetails != nil {
		certTypes = append(certTypes, "Serving")
	}
	if certKeyPair.Spec.Details.ClientCertDetails != nil {
		certTypes = append(certTypes, "Client")
	}
	return strings.Join(certTypes, ",")
}

func markdownBulletListForTable(items ...string) string {
	itemStrings := []string{}
	for _, item := range items {
		itemStrings = append(itemStrings, fmt.Sprintf("- %v", item))
	}
	return strings.Join(itemStrings, "<br/>")
}

func markdownNumberedListForTable(items ...string) string {
	itemStrings := []string{}
	for i, item := range items {
		itemStrings = append(itemStrings, fmt.Sprintf("%d. %v", i+1, item))
	}
	return strings.Join(itemStrings, "<br/>")
}
