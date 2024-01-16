package certdocs

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/gonum/graph/simple"

	"github.com/gonum/graph/encoding/dot"
	"github.com/gonum/graph/traverse"

	"github.com/gonum/graph"
	"github.com/openshift/library-go/pkg/certs/cert-inspection/certgraphapi"
	"github.com/openshift/origin/pkg/cmd/update-tls-artifacts/generate-documentation/certgraph"
)

const mdTemplate_certKeyPair = `
{{ .Heading }} {{ .Name }}
![PKI Graph]({{ .CertPNG }})

{{ .Description }}

| Property | Value |
| ----------- | ----------- |
| Type | {{ .Type }} |
| CommonName | {{ .CommonName }} |
| SerialNumber | {{ .SerialNumber }} |
| Issuer CommonName | {{ .IssuerCommonName }} |
| Validity | {{ .ValidityDuration }} |
| Signature Algorithm | {{ .SignatureAlgorithm }} |
| PublicKey Algorithm | {{ .PublicKeyAlgorithm }} {{ .PublicKeyBitSize }} |
| Usages | {{ .MD_Usages }} |
| ExtendedUsages | {{ .MD_ExtendedUsages }} |
{{ .AdditionalMetadataRows }}

{{ .Heading }}# {{ .Name }} Locations
| Namespace | Secret Name |
| ----------- | ----------- |
{{ .MD_SecretLocationRows }}

| File | Permissions | User | Group | SE Linux |
| ----------- | ----------- | ----------- | ----------- | ----------- |
{{ .MD_FileLocationRows }}
`

var template_certKeyPair = template.Must(template.New("mdTemplate_certKeyPair").Parse(mdTemplate_certKeyPair))

type md_certKeyPair struct {
	Heading      string
	Name         string
	CertPNG      string
	SerialNumber string
	Description  string
	Type         string

	MD_SecretLocationRows string
	MD_FileLocationRows   string

	CommonName             string
	IssuerCommonName       string
	ValidityDuration       string
	SignatureAlgorithm     string
	PublicKeyAlgorithm     string
	PublicKeyBitSize       string
	MD_Usages              string
	MD_ExtendedUsages      string
	AdditionalMetadataRows string
}

func GetMarkdownForCertKeyPair(heading string, pkiGraph graph.Directed, certKeyPair *certgraphapi.CertKeyPair, commonNameToLink map[string]string, outputDir string) (string, []string, error) {
	data := md_certKeyPair{
		Heading:            heading,
		Name:               certKeyPair.Spec.CertMetadata.CertIdentifier.CommonName,
		CertPNG:            makeSafeString(fmt.Sprintf("subcert-%s.png", certKeyPair.Name)),
		Description:        certKeyPair.Description,
		SerialNumber:       certKeyPair.Spec.CertMetadata.CertIdentifier.SerialNumber,
		Type:               markdownCertType(certKeyPair),
		CommonName:         certKeyPair.Spec.CertMetadata.CertIdentifier.CommonName,
		ValidityDuration:   certKeyPair.Spec.CertMetadata.ValidityDuration,
		SignatureAlgorithm: certKeyPair.Spec.CertMetadata.SignatureAlgorithm,
		PublicKeyAlgorithm: certKeyPair.Spec.CertMetadata.PublicKeyAlgorithm,
		PublicKeyBitSize:   certKeyPair.Spec.CertMetadata.PublicKeyBitSize,
	}
	if len(certKeyPair.LogicalName) > 0 {
		data.Name = certKeyPair.LogicalName
	}
	// TODO I'm torn between links with logical names or commonnames.
	commonNameToLink[data.CommonName] = getLink(data.Name, data.Name)

	if certKeyPair.Spec.CertMetadata.CertIdentifier.Issuer != nil {
		data.IssuerCommonName = certKeyPair.Spec.CertMetadata.CertIdentifier.Issuer.CommonName
	}
	data.IssuerCommonName = getHumanIssuerCommonNameLink(certKeyPair.Spec.CertMetadata.CertIdentifier, commonNameToLink)

	localGraph, err := graphForCert(pkiGraph, certKeyPair)
	if err != nil {
		return "", nil, err
	}
	localGraphDOT, err := dot.Marshal(localGraph, certgraph.Quote("Local Certificate"), "", "  ", false)
	if err != nil {
		return "", nil, err
	}
	dotFile := filepath.Join(outputDir, fmt.Sprintf("subcert-%s.dot", certKeyPair.Name))
	if err := ioutil.WriteFile(dotFile, []byte(localGraphDOT), 0644); err != nil {
		return "", nil, err
	}
	png := exec.Command("dot", "-Kdot", "-T", "png", "-o", filepath.Join(outputDir, data.CertPNG), dotFile)
	png.Stderr = os.Stderr
	if err := png.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "%v", err)
	}

	if len(certKeyPair.Spec.SecretLocations) > 0 {
		locationStrings := []string{}
		for _, curr := range certKeyPair.Spec.SecretLocations {
			locationStrings = append(locationStrings, fmt.Sprintf("| %v | %v |", curr.Namespace, curr.Name))
		}
		data.MD_SecretLocationRows = strings.Join(locationStrings, "\n")
	}
	if len(certKeyPair.Spec.OnDiskLocations) > 0 {
		locationStrings := []string{}
		for _, curr := range certKeyPair.Spec.OnDiskLocations {
			locationStrings = append(locationStrings, fmt.Sprintf("| %v | %v | %v | %v | %v |", curr.Cert.Path, curr.Cert.Permissions, curr.Cert.User, curr.Cert.Group, curr.Cert.SELinuxOptions))
			locationStrings = append(locationStrings, fmt.Sprintf("| %v | %v | %v | %v | %v |", curr.Key.Path, curr.Key.Permissions, curr.Key.User, curr.Key.Group, curr.Key.SELinuxOptions))
		}
		data.MD_FileLocationRows = strings.Join(locationStrings, "\n")
	}

	data.MD_Usages = markdownBulletListForTable(certKeyPair.Spec.CertMetadata.Usages...)
	data.MD_ExtendedUsages = markdownBulletListForTable(certKeyPair.Spec.CertMetadata.ExtendedUsages...)

	if certKeyPair.Spec.Details.ClientCertDetails != nil {
		organizations := markdownBulletListForTable(certKeyPair.Spec.Details.ClientCertDetails.Organizations...)
		data.AdditionalMetadataRows += fmt.Sprintf("| Organizations (User Groups) | %s |\n", organizations)
	}
	if certKeyPair.Spec.Details.ServingCertDetails != nil {
		dnsNames := markdownBulletListForTable(certKeyPair.Spec.Details.ServingCertDetails.DNSNames...)
		ipAddresses := markdownBulletListForTable(certKeyPair.Spec.Details.ServingCertDetails.IPAddresses...)
		data.AdditionalMetadataRows += fmt.Sprintf("| DNS Names | %s |\n", dnsNames)
		data.AdditionalMetadataRows += fmt.Sprintf("| IP Addresses | %s |\n", ipAddresses)
	}

	stringBuf := bytes.NewBufferString("")
	if err := template_certKeyPair.Execute(stringBuf, data); err != nil {
		return "", nil, err
	}

	tocLines := []string{
		fmt.Sprintf(
			"%v- %v",
			strings.Repeat("  ", len(heading)-1),
			getSimpleLink(data.Name),
		)}

	return stringBuf.String(), tocLines, err
}

func getHumanIssuerCommonNameLink(identifier certgraphapi.CertIdentifier, commonNameToLink map[string]string) string {
	if identifier.Issuer == nil {
		return "None"
	}

	if ret, ok := commonNameToLink[identifier.Issuer.CommonName]; ok {
		return ret
	}
	return identifier.Issuer.CommonName
}

func getHumanCommonNameLink(identifier certgraphapi.CertIdentifier, commonNameToLink map[string]string) string {
	if ret, ok := commonNameToLink[identifier.CommonName]; ok {
		return ret
	}
	return identifier.CommonName
}

func graphForCert(pkiGraph graph.Directed, certKeyPair *certgraphapi.CertKeyPair) (graph.Directed, error) {
	var certNode graph.Node
	(&traverse.BreadthFirst{}).WalkAll(graph.Undirect{G: pkiGraph}, nil, nil,
		func(currNode graph.Node) {
			if item := currNode.(graphNode).GetCertKeyPair(); item != nil {
				if item.Name == certKeyPair.Name {
					certNode = currNode
				}
			}
		},
	)
	nodeList := []graph.Node{}
	fromList := []graph.Node{}
	toList := []graph.Node{}
	if certNode != nil {
		fromList = pkiGraph.From(certNode)
		toList = pkiGraph.To(certNode)
		nodeList = append(nodeList, certNode)
		nodeList = append(nodeList, fromList...)
		nodeList = append(nodeList, toList...)
	}

	localGraph := simple.NewDirectedGraph(1.0, 0.0)
	for i := range nodeList {
		currNode := nodeList[i]
		localGraph.AddNode(currNode)
	}
	for i := range fromList {
		targetNode := fromList[i]
		localGraph.SetEdge(simple.Edge{F: certNode, T: targetNode})
	}
	for i := range toList {
		targetNode := toList[i]
		localGraph.SetEdge(simple.Edge{F: targetNode, T: certNode})
	}

	return localGraph, nil
}
