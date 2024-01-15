package certdocs

import (
	"bytes"
	"fmt"
	"hash/fnv"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/gonum/graph"
	"github.com/gonum/graph/encoding/dot"
	"github.com/gonum/graph/simple"
	"github.com/gonum/graph/traverse"
	"github.com/openshift/library-go/pkg/certs/cert-inspection/certgraphapi"
)

const mdTemplate_caBundle = `
{{ .Heading }} {{ .Name }}
![PKI Graph]({{ .CAPNG }})

{{ .Description }}

**Bundled Certificates**

| CommonName | Issuer CommonName | Validity | PublicKey Algorithm |
| ----------- | ----------- | ----------- | ----------- |
{{ .MD_CertificateRows }}

{{ .Heading }}# {{ .Name }} Locations
| Namespace | ConfigMap Name |
| ----------- | ----------- |
{{ .MD_ConfigMapLocationRows }}

| File | Permissions | User | Group | SE Linux |
| ----------- | ----------- | ----------- | ----------- | ----------- |
{{ .MD_FileLocationRows }}
`

var template_caBundle = template.Must(template.New("mdTemplate_caBundle").Parse(mdTemplate_caBundle))

type md_caBundle struct {
	Heading     string
	Name        string
	CAPNG       string
	Description string

	MD_ConfigMapLocationRows string
	MD_FileLocationRows      string
	MD_CertificateRows       string
}

func GetMarkdownForCABundle(heading string, pkiGraph graph.Directed, caBundle *certgraphapi.CertificateAuthorityBundle, commonNameToLink map[string]string, outputDir string) (string, []string, error) {
	data := md_caBundle{
		Heading:     heading,
		Name:        caBundle.Name,
		CAPNG:       makeSafeString(fmt.Sprintf("subca-%s.png", hashCABundleName(caBundle.Name))),
		Description: caBundle.Description,
	}
	if len(caBundle.LogicalName) > 0 {
		data.Name = caBundle.LogicalName
	}

	localGraph, err := graphForCABundle(pkiGraph, caBundle)
	if err != nil {
		return "", nil, err
	}
	localGraphDOT, err := dot.Marshal(localGraph, Quote("Local Certificate"), "", "  ", false)
	if err != nil {
		return "", nil, err
	}
	dotFile := filepath.Join(outputDir, makeSafeString(fmt.Sprintf("subca-%s.dot", hashCABundleName(caBundle.Name))))
	if err := ioutil.WriteFile(dotFile, []byte(localGraphDOT), 0644); err != nil {
		return "", nil, err
	}
	png := exec.Command("dot", "-Kdot", "-T", "png", "-o", filepath.Join(outputDir, data.CAPNG), dotFile)
	png.Stderr = os.Stderr
	if err := png.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "%v", err)
	}

	if len(caBundle.Spec.ConfigMapLocations) > 0 {
		locationStrings := []string{}
		for _, curr := range caBundle.Spec.ConfigMapLocations {
			locationStrings = append(locationStrings, fmt.Sprintf("| %v | %v |", curr.Namespace, curr.Name))
		}
		data.MD_ConfigMapLocationRows = strings.Join(locationStrings, "\n")
	}
	if len(caBundle.Spec.OnDiskLocations) > 0 {
		locationStrings := []string{}
		for _, curr := range caBundle.Spec.OnDiskLocations {
			locationStrings = append(locationStrings, fmt.Sprintf("| %v | %v | %v | %v | %v |", curr.Path, curr.Permissions, curr.User, curr.Group, curr.SELinuxOptions))
		}
		data.MD_FileLocationRows = strings.Join(locationStrings, "\n")
	}

	if len(caBundle.Spec.CertificateMetadata) > 0 {
		certDetails := []string{}
		for _, curr := range caBundle.Spec.CertificateMetadata {
			if len(curr.CertIdentifier.CommonName) == 0 {
				fmt.Fprintf(os.Stderr, "CA bundle %v has a weird case of no common name in it.\n", caBundle.LogicalName)
				continue
			}
			certDetails = append(certDetails, fmt.Sprintf(
				"| %v | %v | %v | %v %v |",
				getHumanCommonNameLink(curr.CertIdentifier, commonNameToLink),
				getHumanIssuerCommonNameLink(curr.CertIdentifier, commonNameToLink),
				curr.ValidityDuration,
				curr.PublicKeyAlgorithm,
				curr.PublicKeyBitSize,
			))
		}
		data.MD_CertificateRows = strings.Join(certDetails, "\n")
	}

	stringBuf := bytes.NewBufferString("")
	if err := template_caBundle.Execute(stringBuf, data); err != nil {
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

func graphForCABundle(pkiGraph graph.Directed, caBundle *certgraphapi.CertificateAuthorityBundle) (graph.Directed, error) {
	var certNode graph.Node
	(&traverse.BreadthFirst{}).WalkAll(graph.Undirect{G: pkiGraph}, nil, nil,
		func(currNode graph.Node) {
			if item := currNode.(graphNode).GetCABundle(); item != nil {
				if item.Name == caBundle.Name {
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

func hashCABundleName(in string) string {
	h := fnv.New32a()
	h.Write([]byte(in))
	return fmt.Sprintf("%d", h.Sum32())
}
